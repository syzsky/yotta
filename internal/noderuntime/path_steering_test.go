package noderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/navigationpath"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/resource"
	"github.com/yottaapp/yotta/internal/targetruntime"
)

type pathSteeringProvider struct {
	t         *testing.T
	requests  []installed.TurnViewRequest
	invoke    func(context.Context) error
	dropped   int
	closeHeld func(context.Context) error
}

func (p *pathSteeringProvider) Open(_ context.Context, r resource.ProviderOpenRequest) (any, error) {
	return r.Kind, nil
}

func (p *pathSteeringProvider) Close(ctx context.Context, object any) error {
	if object == installed.KindHeldInput {
		p.dropped++
		if p.closeHeld != nil {
			return p.closeHeld(ctx)
		}
	}
	return nil
}

func holdPathSteeringFixture(t *testing.T, d *pathDriver) {
	t.Helper()
	held, err := openConfiguredTarget(context.Background(), d.i, installed.KindHeldInput, installed.HeldInputOperations())
	if err != nil {
		t.Fatal(err)
	}
	d.held = held
}

func TestPathStopForwardFreshnessAndCleanupContext(t *testing.T) {
	d, p := newPathSteeringDriver(t)
	holdPathSteeringFixture(t, d)
	type cleanupKey struct{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), cleanupKey{}, "cleanup"))
	cancel()
	var completedAt int64
	p.closeHeld = func(ctx context.Context) error {
		if ctx.Err() != nil || ctx.Value(cleanupKey{}) != "cleanup" {
			t.Fatal("release lost uncancelled cleanup context or its values")
		}
		if d.freshAfter != 1 {
			t.Fatal("freshness advanced before release completed")
		}
		completedAt = time.Now().UnixMilli()
		return nil
	}
	if err := d.StopForward(context.WithoutCancel(ctx)); err != nil {
		t.Fatal(err)
	}
	if d.freshAfter < completedAt+1 || d.freshAfter > time.Now().UnixMilli()+1 || d.held.Validate() == nil || p.dropped != 1 {
		t.Fatalf("freshness=%d completion=%d held=%+v releases=%d", d.freshAfter, completedAt, d.held, p.dropped)
	}
	// A sentinel distinguishes a true no-op from another same-millisecond bump.
	d.freshAfter = 123
	if err := d.StopForward(ctx); err != nil || d.freshAfter != 123 || p.dropped != 1 {
		t.Fatalf("repeated stop changed state: err=%v freshness=%d releases=%d", err, d.freshAfter, p.dropped)
	}
	empty, provider := newPathSteeringDriver(t)
	if err := empty.StopForward(ctx); err != nil || empty.freshAfter != 1 || provider.dropped != 0 {
		t.Fatal("stop without held input changed freshness")
	}
}

func TestPathStopForwardPreservesFailureAndLegacyStop(t *testing.T) {
	d, p := newPathSteeringDriver(t)
	holdPathSteeringFixture(t, d)
	want := errors.New("release failed")
	p.closeHeld = func(context.Context) error { return want }
	if err := d.StopForward(context.Background()); !errors.Is(err, want) || d.freshAfter != 1 || d.held.Validate() != nil {
		t.Fatalf("release failure lost: err=%v freshness=%d held=%+v", err, d.freshAfter, d.held)
	}
	legacy, provider := newPathSteeringDriver(t)
	holdPathSteeringFixture(t, legacy)
	if err := legacy.navigationDriver.StopForward(context.Background()); err != nil || legacy.freshAfter != 1 || provider.dropped != 1 {
		t.Fatal("MoveTo stop behavior changed")
	}
}

type pathStopPositionState struct {
	nodeadapter.StateBinding
	read func() (nodeadapter.StateSnapshot, error)
}

func (s pathStopPositionState) Read() (nodeadapter.StateSnapshot, error) { return s.read() }

func TestPathStopForwardReadRejectsPreStopSampleWithNewRevision(t *testing.T) {
	b, err := nodes.Build()
	if err != nil {
		t.Fatal(err)
	}
	d, _ := newPathSteeringDriver(t)
	holdPathSteeringFixture(t, d)
	d.reference = navigationpath.Reference{Kind: "world", Frame: "test/world", Unit: "raw", AxisSign: 1}
	if err := d.StopForward(context.Background()); err != nil {
		t.Fatal(err)
	}
	gate := d.freshAfter
	after := time.UnixMilli(gate).Add(-time.Second)
	reads, waits := 0, 0
	d.i.State = map[string]nodeadapter.StateBinding{"position": pathStopPositionState{read: func() (nodeadapter.StateSnapshot, error) {
		reads++
		stamp := gate - 1
		if reads > 1 {
			stamp = time.Now().UnixMilli()
		}
		raw := fmt.Sprintf(`{"protocol":"yotta.position-source/v1","source":"test.position","capabilities":["position","camera-heading"],"frame":{"id":"test/world","unit":"raw","axisHeading":0,"axisSign":1,"kind":"world","recovery":"stable"},"epoch":"test","observations":{"position":{"status":"tracking","value":{"x":%d,"y":0},"sampleTimeMs":%d,"sampleAgeMs":0,"accuracy":null,"sequence":%d},"camera-heading":{"status":"tracking","value":0,"sampleTimeMs":%d,"sampleAgeMs":0,"accuracy":null,"sequence":%d}}}`, reads, stamp, reads, stamp, reads)
		if _, err := navigationpath.Observe([]byte(raw), time.Now(), 500*time.Millisecond); err != nil {
			t.Fatalf("fixture must be a valid fresh source observation: %v", err)
		}
		encoded, err := json.Marshal(raw)
		if err != nil {
			return nodeadapter.StateSnapshot{}, err
		}
		value, err := datatype.SealInlineJSON(b.Catalog, datatype.RefResolvedType(b.StringType.TypeRef()), encoded)
		// Even the old physical sample has a state revision AFTER release.
		return nodeadapter.StateSnapshot{Value: value, Revision: time.UnixMilli(gate).UnixNano() + int64(reads)}, err
	}}}
	d.i.Wait = func(ctx context.Context, duration time.Duration) error {
		if duration > 0 {
			waits++
			if waits > 1 {
				t.Fatal("post-stop source sample was not accepted")
			}
			time.Sleep(time.Until(time.UnixMilli(gate)))
		}
		return ctx.Err()
	}
	pose, err := d.Read(context.Background(), after)
	if err != nil || reads != 2 || waits != 1 || pose.X != 2 || d.observation.SampleTimeMs < gate {
		t.Fatalf("pre-stop sample accepted or post-stop sample rejected: pose=%+v err=%v reads=%d waits=%d", pose, err, reads, waits)
	}
	if pose.SampleTime.UnixMilli() != d.observation.SampleTimeMs {
		t.Fatal("source acquisition time was replaced with delivery revision")
	}
}

func (p *pathSteeringProvider) Invoke(ctx context.Context, _ any, op string, payload []byte) ([]byte, error) {
	if op != installed.OperationTurnView {
		p.t.Fatalf("unexpected operation %q", op)
	}
	var request installed.TurnViewRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		p.t.Fatal(err)
	}
	p.requests = append(p.requests, request)
	if p.invoke != nil {
		if err := p.invoke(ctx); err != nil {
			return nil, err
		}
	}
	return []byte(`{}`), nil
}

func newPathSteeringDriver(t *testing.T) (*pathDriver, *pathSteeringProvider) {
	t.Helper()
	p := &pathSteeringProvider{t: t}
	snapshot, err := targetruntime.NewSnapshot([]targetruntime.Installation{{Slot: "game", TargetID: "test/game", Provider: p}})
	if err != nil {
		t.Fatal(err)
	}
	targets, err := snapshot.NewRun()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := targets.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	i := nodeadapter.Invocation{
		Targets: targets, Config: map[string]any{"slot": "game"},
		Wait: func(ctx context.Context, duration time.Duration) error {
			if duration != 0 {
				t.Fatalf("unexpected settling wait %v", duration)
			}
			return ctx.Err()
		},
	}
	turn, err := openConfiguredTarget(context.Background(), i, installed.KindInput, []string{installed.OperationTurnView})
	if err != nil {
		t.Fatal(err)
	}
	return &pathDriver{navigationDriver: &navigationDriver{i: i, turn: turn, freshAfter: 1}}, p
}

func TestPathSteerForwardsDurationAndGatesAfterInvocation(t *testing.T) {
	d, p := newPathSteeringDriver(t)
	var completedAt int64
	p.invoke = func(context.Context) error {
		// A future sentinel makes setting the gate before invoking observable.
		if d.freshAfter != 1<<62 {
			t.Fatalf("gate changed before completion: %d", d.freshAfter)
		}
		completedAt = time.Now().UnixMilli()
		return nil
	}
	for _, duration := range []time.Duration{20 * time.Millisecond, 37 * time.Millisecond, 100 * time.Millisecond} {
		d.freshAfter = 1 << 62
		if err := d.Steer(context.Background(), -0.125, duration); err != nil {
			t.Fatal(err)
		}
		request := p.requests[len(p.requests)-1]
		if request.Degrees != -0.125 || request.DurationMilliseconds != duration.Milliseconds() {
			t.Fatalf("request=%+v duration=%v", request, duration)
		}
		if d.freshAfter < completedAt+1 || d.freshAfter > time.Now().UnixMilli()+1 {
			t.Fatalf("gate=%d completion=%d", d.freshAfter, completedAt)
		}
	}
	if len(p.requests) != 3 {
		t.Fatalf("invocations=%d", len(p.requests))
	}
}

func TestPathSteerRejectsInvalidDuration(t *testing.T) {
	d, p := newPathSteeringDriver(t)
	for _, duration := range []time.Duration{-time.Millisecond, 0, time.Nanosecond, 20500 * time.Microsecond} {
		if err := d.Steer(context.Background(), 1, duration); err == nil {
			t.Fatalf("accepted duration %v", duration)
		}
	}
	if len(p.requests) != 0 || d.freshAfter != 1 {
		t.Fatalf("invalid duration caused input or changed freshness: %+v %d", p.requests, d.freshAfter)
	}
}

func TestPathSteerCancellationAndFailure(t *testing.T) {
	for _, phase := range []string{"before", "wait", "invoke", "provider"} {
		t.Run(phase, func(t *testing.T) {
			d, p := newPathSteeringDriver(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := error(context.Canceled)
			switch phase {
			case "before":
				cancel()
			case "wait":
				d.i.WaitWithPause = func(ctx context.Context, _ time.Duration, _ func(context.Context) error) error {
					cancel()
					return ctx.Err()
				}
			case "invoke":
				p.invoke = func(invocationCtx context.Context) error {
					cancel()
					return invocationCtx.Err()
				}
			case "provider":
				want = errors.New("turn failed")
				p.invoke = func(context.Context) error { return want }
			}
			if err := d.Steer(ctx, 1, 20*time.Millisecond); !errors.Is(err, want) {
				t.Fatalf("got %v, want %v", err, want)
			}
			wantCalls := 1
			if phase == "before" || phase == "wait" {
				wantCalls = 0
			}
			if len(p.requests) != wantCalls || d.freshAfter != 1 {
				t.Fatalf("calls=%d freshness=%d", len(p.requests), d.freshAfter)
			}
		})
	}
}

func TestPathSteerUsesPauseRelease(t *testing.T) {
	d, p := newPathSteeringDriver(t)
	held, err := openConfiguredTarget(context.Background(), d.i, installed.KindHeldInput, installed.HeldInputOperations())
	if err != nil {
		t.Fatal(err)
	}
	d.held = held
	checks := 0
	d.i.WaitWithPause = func(ctx context.Context, duration time.Duration, release func(context.Context) error) error {
		checks++
		if duration != 0 || len(p.requests) != 0 {
			t.Fatal("pause boundary must precede steering without a settling delay")
		}
		return release(ctx)
	}
	if err := d.Steer(context.Background(), 1, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if checks != 1 || p.dropped != 1 || d.held.Validate() == nil || len(p.requests) != 1 {
		t.Fatalf("checks=%d releases=%d held=%+v turns=%d", checks, p.dropped, d.held, len(p.requests))
	}
}

func TestPathSteerLeavesLegacyTurnUnchanged(t *testing.T) {
	d, p := newPathSteeringDriver(t)
	type timedSteerer interface {
		Steer(context.Context, float64, time.Duration) error
	}
	var _ timedSteerer = d
	if _, ok := any(d.navigationDriver).(timedSteerer); ok {
		t.Fatal("timed steering leaked into MoveTo driver")
	}
	var waits []time.Duration
	d.i.Wait = func(_ context.Context, duration time.Duration) error {
		waits = append(waits, duration)
		return nil
	}
	if err := d.Turn(context.Background(), 12.5); err != nil {
		t.Fatal(err)
	}
	if len(p.requests) != 1 || p.requests[0].Degrees != 12.5 || p.requests[0].DurationMilliseconds != 150 {
		t.Fatalf("legacy requests=%+v", p.requests)
	}
	if len(waits) != 1 || waits[0] != 50*time.Millisecond || d.freshAfter <= 1 {
		t.Fatalf("legacy waits=%v freshness=%d", waits, d.freshAfter)
	}
}
