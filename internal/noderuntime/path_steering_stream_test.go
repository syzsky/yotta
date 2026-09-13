package noderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/navigationpath"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/resource"
	"github.com/yottaapp/yotta/internal/targetruntime"
)

type streamedInput struct {
	requests chan installed.TurnViewRequest
	fail     error
}

func (p *streamedInput) Open(context.Context, resource.ProviderOpenRequest) (any, error) {
	return 1, nil
}
func (p *streamedInput) Close(context.Context, any) error { return nil }
func (p *streamedInput) Invoke(ctx context.Context, _ any, _ string, raw []byte) ([]byte, error) {
	var r installed.TurnViewRequest
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	select {
	case p.requests <- r:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return []byte(`{}`), p.fail
}
func newStreamedInput(t *testing.T) (*pathDriver, *streamedInput) {
	t.Helper()
	p := &streamedInput{requests: make(chan installed.TurnViewRequest, 64)}
	snapshot, e := targetruntime.NewSnapshot([]targetruntime.Installation{{Slot: "game", TargetID: "test/stream", Provider: p}})
	if e != nil {
		t.Fatal(e)
	}
	targets, e := snapshot.NewRun()
	if e != nil {
		t.Fatal(e)
	}
	i := nodeadapter.Invocation{Targets: targets, Config: map[string]any{"slot": "game"}, Wait: func(ctx context.Context, dt time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(dt):
			return nil
		}
	}}
	h, e := openConfiguredTarget(context.Background(), i, installed.KindInput, []string{installed.OperationTurnView})
	if e != nil {
		t.Fatal(e)
	}
	d := &pathDriver{navigationDriver: &navigationDriver{i: i, turn: h}, observation: navigationpath.Observation{SampleTimeMs: time.Now().UnixMilli()}}
	t.Cleanup(func() { _ = d.StopForward(context.Background()); _ = targets.Close(context.Background()) })
	return d, p
}
func TestPathStreamContinuesBetweenObservationsAndStops(t *testing.T) {
	d, p := newStreamedInput(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if e := d.SetSteering(ctx, 120); e != nil {
		t.Fatal(e)
	}
	for j := 0; j < 3; j++ {
		select {
		case r := <-p.requests:
			if r.DurationMilliseconds != 0 || r.Degrees <= 0 || r.Degrees > 12 {
				t.Fatalf("packet %+v", r)
			}
		case <-time.After(time.Second):
			t.Fatal("no continuous input without another observation")
		}
	}
	if e := d.SetSteering(ctx, -120); e != nil {
		t.Fatal(e)
	}
	reversed := false
	deadline := time.After(time.Second)
	for !reversed {
		select {
		case r := <-p.requests:
			reversed = r.Degrees < 0
		case <-deadline:
			t.Fatal("latest demand did not replace old turn")
		}
	}
	if e := d.StopForward(ctx); e != nil {
		t.Fatal(e)
	}
	for len(p.requests) > 0 {
		<-p.requests
	}
	select {
	case r := <-p.requests:
		t.Fatalf("input after joined stop %+v", r)
	case <-time.After(60 * time.Millisecond):
	}
}
func TestPathStreamExpiryCancellationAndProviderFailure(t *testing.T) {
	for _, mode := range []string{"expired", "cancelled", "failed"} {
		t.Run(mode, func(t *testing.T) {
			d, p := newStreamedInput(t)
			want := errors.New("device failed")
			if mode == "failed" {
				p.fail = want
			}
			if mode == "expired" {
				d.observation.SampleTimeMs = time.Now().Add(-time.Second).UnixMilli()
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if e := d.SetSteering(ctx, 90); e != nil {
				t.Fatal(e)
			}
			if mode == "cancelled" {
				cancel()
			}
			select {
			case <-d.stream.done:
			case <-time.After(time.Second):
				t.Fatal("writer did not finish")
			}
			e := d.StopForward(context.Background())
			switch mode {
			case "failed":
				if !errors.Is(e, want) {
					t.Fatal(e)
				}
			case "expired":
				if !errors.Is(e, navigationpath.ErrUnavailable) || len(p.requests) != 0 {
					t.Fatal("stale observation produced input", e)
				}
			case "cancelled":
				if e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}

func TestPathStreamPauseJoinsWriterAndResetsObservation(t *testing.T) {
	d, p := newStreamedInput(t)
	d.i.WaitWithPause = func(ctx context.Context, dt time.Duration, release func(context.Context) error) error {
		return release(ctx)
	}
	if err := d.SetSteering(context.Background(), 90); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.requests:
	case <-time.After(time.Second):
		t.Fatal("no input")
	}
	if err := d.Wait(context.Background(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !d.inputReset || d.stream.done != nil || d.freshAfter == 0 {
		t.Fatal("pause did not reset input and observation")
	}
	for len(p.requests) > 0 {
		<-p.requests
	}
	select {
	case <-p.requests:
		t.Fatal("input continued while paused")
	case <-time.After(60 * time.Millisecond):
	}
}
