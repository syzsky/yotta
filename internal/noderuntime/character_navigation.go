package noderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/automation/navigation"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/resource"
)

type navigationDriver struct {
	i                  nodeadapter.Invocation
	turn, forward      resource.Handle
	held               resource.Handle
	frame, unit, epoch string
	axisHeading        float64
	axisSign           int
	havePosition       bool
	lastPosition       navigation.WorldPosition
	freshAfter         int64
	initialWait        time.Duration
}

func (d *navigationDriver) HoldForward(ctx context.Context) error {
	if d.held.Validate() == nil {
		return nil
	}
	handle, err := openConfiguredTarget(ctx, d.i, installed.KindHeldInput, installed.HeldInputOperations())
	if err != nil {
		return err
	}
	d.held = handle
	return navInvoke(ctx, d.i, handle, installed.OperationHoldKeys, installed.HoldKeysRequest{Keys: []string{navString(d.i, "forwardKey", "W")}})
}

func (d *navigationDriver) StopForward(ctx context.Context) error {
	if d.held.Validate() != nil {
		return nil
	}
	err := d.i.Targets.Drop(ctx, d.held)
	if err == nil {
		d.held = resource.Handle{}
	}
	return err
}

func (d *navigationDriver) Wait(ctx context.Context, duration time.Duration) error {
	if d.i.WaitWithPause == nil {
		return d.i.Wait(ctx, duration)
	}
	paused := false
	err := d.i.WaitWithPause(ctx, duration, func(cleanupCtx context.Context) error { paused = true; return d.StopForward(cleanupCtx) })
	if paused {
		d.freshAfter = time.Now().UnixMilli()
	}
	return err
}

func navString(i nodeadapter.Invocation, key, fallback string) string {
	if s, ok := i.Config[key].(string); ok {
		return s
	}
	return fallback
}
func navNumber(i nodeadapter.Invocation, key string, fallback float64) float64 {
	if n, ok := i.Config[key].(json.Number); ok {
		value, err := n.Float64()
		if err == nil {
			return value
		}
		return math.NaN()
	}
	if n, ok := i.Config[key].(float64); ok {
		return n
	}
	return fallback
}
func navInvoke(ctx context.Context, i nodeadapter.Invocation, h resource.Handle, op string, request any) error {
	payload, err := artifact.Marshal(request)
	if err != nil {
		return err
	}
	raw, err := i.Targets.Invoke(ctx, h, op, payload)
	if err != nil {
		return err
	}
	return installed.OpenEffectResponse(raw)
}
func (d *navigationDriver) Turn(ctx context.Context, angle float64) error {
	if err := navInvoke(ctx, d.i, d.turn, installed.OperationTurnView, installed.TurnViewRequest{Degrees: angle, DurationMilliseconds: 150}); err != nil {
		return err
	}
	d.freshAfter = time.Now().UnixMilli() + 1
	return d.Wait(ctx, 50*time.Millisecond)
}
func (d *navigationDriver) Forward(ctx context.Context, duration time.Duration) error {
	if err := navInvoke(ctx, d.i, d.forward, installed.OperationPressKeys, installed.PressKeysRequest{Keys: []string{navString(d.i, "forwardKey", "W")}, DurationMilliseconds: duration.Milliseconds()}); err != nil {
		return err
	}
	d.freshAfter = time.Now().UnixMilli() + 1
	return d.Wait(ctx, 50*time.Millisecond)
}
func (d *navigationDriver) Read(ctx context.Context, after time.Time) (navigation.Pose, error) {
	if err := d.Wait(ctx, 0); err != nil {
		return navigation.Pose{}, err
	}
	state := d.i.State["position"]
	if state == nil {
		return navigation.Pose{}, errors.New("position state is not bound")
	}
	now := d.i.MonotonicNow
	if now == nil {
		now = time.Now
	}
	waitBudget := time.Second
	if !d.havePosition && d.initialWait > 0 {
		waitBudget = d.initialWait
	}
	deadline := now().Add(waitBudget)
	for {
		snapshot, err := state.Read()
		if err != nil {
			return navigation.Pose{}, err
		}
		var p navigation.WorldPosition
		if err := json.Unmarshal(snapshot.Value.InlineJSON(), &p); err != nil {
			return navigation.Pose{}, navigation.ErrStale
		}
		valid := p.Fresh(time.Now(), 500*time.Millisecond)
		if d.havePosition && !valid {
			// A delayed observation stops physical input immediately. Allow a
			// bounded fresh sample to arrive before declaring the source lost.
			if err := d.StopForward(ctx); err != nil {
				return navigation.Pose{}, err
			}
		}
		revision := time.Unix(0, snapshot.Revision)
		if valid && revision.After(after) && p.ReceivedAt >= d.freshAfter {
			if d.havePosition && (p.Frame != d.frame || p.Unit != d.unit || p.Epoch != d.epoch || p.AxisHeading != d.axisHeading || p.AxisSign != d.axisSign) {
				return navigation.Pose{}, navigation.ErrStale
			}
			if !d.havePosition || p.NewerThan(d.lastPosition) {
				d.frame, d.unit, d.epoch, d.axisHeading, d.axisSign, d.havePosition = p.Frame, p.Unit, p.Epoch, p.AxisHeading, p.AxisSign, true
				d.lastPosition = p
				return navigation.Pose{X: p.X, Y: p.Y, Heading: p.Heading, Time: revision, AxisHeading: p.AxisHeading, AxisSign: float64(p.AxisSign)}, nil
			}
		}
		if !now().Before(deadline) {
			if !d.havePosition {
				return navigation.Pose{}, context.DeadlineExceeded
			}
			return navigation.Pose{}, navigation.ErrStale
		}
		if err := d.Wait(ctx, 20*time.Millisecond); err != nil {
			return navigation.Pose{}, err
		}
	}
}

func decodeNavigationPose(raw []byte, i nodeadapter.Invocation, now time.Time) (navigation.Pose, error) {
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return navigation.Pose{}, navigation.ErrStale
	}
	var valid bool
	if json.Unmarshal(data[navString(i, "validField", "valid")], &valid) != nil || !valid {
		return navigation.Pose{}, navigation.ErrStale
	}
	var p navigation.Pose
	var stamp int64
	for key, out := range map[string]*float64{navString(i, "xField", "x"): &p.X, navString(i, "yField", "y"): &p.Y, navString(i, "headingField", "cameraHeading"): &p.Heading} {
		if string(data[key]) == "null" || json.Unmarshal(data[key], out) != nil || !navigation.Finite(*out) {
			return p, navigation.ErrStale
		}
	}
	if json.Unmarshal(data[navString(i, "timeField", "sampleTimeMs")], &stamp) != nil || stamp <= 0 {
		return p, navigation.ErrStale
	}
	p.Time = time.UnixMilli(stamp)
	if age := now.Sub(p.Time); age < -100*time.Millisecond || age > time.Second {
		return p, navigation.ErrStale
	}
	return p, nil
}

func characterNavigation(b nodes.Builtins, id string) nodeadapter.Adapter {
	return func(ctx context.Context, i nodeadapter.Invocation) (_ nodeadapter.AdapterResult, runErr error) {
		defer func() {
			if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
				runErr = automationFailure(nodes.NavigationFailedCode, runErr)
			}
			runErr = errors.Join(runErr, recordAdapterOutcome(ctx, i, nodeadapter.AdapterAction{EffectID: nodes.NavigationEffect(id), Action: "automation.navigation", SummaryCode: "automation.navigation", Counters: map[string]int64{}}, nodes.NavigationFailedCode, runErr))
		}()
		turn, err := openConfiguredTarget(ctx, i, installed.KindInput, []string{installed.OperationTurnView})
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		defer func() { runErr = errors.Join(runErr, i.Targets.Drop(context.WithoutCancel(ctx), turn)) }()
		if id == nodes.TurnViewNodeID {
			angle, e1 := numberInput(i, "angle")
			duration, e2 := integerInput(i, "duration")
			if err := errors.Join(e1, e2); err != nil {
				return nodeadapter.AdapterResult{}, err
			}
			err = navInvoke(ctx, i, turn, installed.OperationTurnView, installed.TurnViewRequest{Degrees: angle, DurationMilliseconds: duration})
			return nodeadapter.AdapterResult{ExecOutputs: []string{"completed"}}, err
		}
		timeout, err := integerInput(i, "timeout")
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		// Match the duration input contract used by the editor and compiler.
		if timeout < 1 || timeout > nodes.MaxDelayMilliseconds {
			return nodeadapter.AdapterResult{}, fmt.Errorf("timeout must be between 1 and %d ms", nodes.MaxDelayMilliseconds)
		}

		if err := i.EmitStatus(ctx, nodes.NavigationWaitingStatus, map[string]int64{"timeout_ms": timeout}); err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		if id == nodes.TurnFindTemplateNodeID {
			return turnFindTemplate(ctx, ctx, b, i, turn)
		}
		d := &navigationDriver{i: i, turn: turn, initialWait: time.Duration(timeout) * time.Millisecond}
		d.forward, err = openConfiguredTarget(ctx, i, installed.KindInput, []string{installed.OperationPressKeys})
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		defer func() { runErr = errors.Join(runErr, i.Targets.Drop(context.WithoutCancel(ctx), d.forward)) }()
		x, e1 := numberInput(i, "target-x")
		y, e2 := numberInput(i, "target-y")
		tolerance, e3 := numberInput(i, "tolerance")
		pulse, e4 := integerInput(i, "interval")
		if err := errors.Join(e1, e2, e3, e4); err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		if pulse < 20 || pulse > 500 {
			return nodeadapter.AdapterResult{}, errors.New("position interval must be between 20 and 500 ms")
		}
		slow, err := numberInput(i, "slow-distance")
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		p, dist, err := navigation.MoveTo(ctx, d, navigation.Options{X: x, Y: y, Tolerance: tolerance, TurnSign: navNumber(i, "turnSign", 1), Pulse: time.Duration(pulse) * time.Millisecond, SlowDistance: slow, StuckTimeout: 5 * time.Second, Now: i.MonotonicNow, Timeout: time.Duration(timeout) * time.Millisecond})
		exit := "arrived"
		switch {
		case ctx.Err() != nil:
			return nodeadapter.AdapterResult{}, ctx.Err()
		case errors.Is(err, context.DeadlineExceeded):
			exit = "timeout"
		case errors.Is(err, navigation.ErrStale):
			exit = "unavailable"
		case errors.Is(err, navigation.ErrStuck):
			exit = "stuck"
		case err != nil:
			return nodeadapter.AdapterResult{}, err
		}
		if err := i.EmitStatus(ctx, choose(exit == "timeout", nodes.NavigationTimeoutStatus, nodes.NavigationFinishedStatus), map[string]int64{"navigation_" + exit: 1, "remaining_distance": int64(math.Ceil(dist))}); err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		result, err := sealVisionOutputs(b, i, map[string]any{"x": p.X, "y": p.Y, "distance": dist})
		result.ExecOutputs = []string{exit}
		return result, err
	}
}

func turnFindTemplate(ctx, parent context.Context, b nodes.Builtins, i nodeadapter.Invocation, turn resource.Handle) (_ nodeadapter.AdapterResult, runErr error) {
	step, e1 := numberInput(i, "step")
	limit, e2 := numberInput(i, "max-angle")
	threshold, e3 := numberInput(i, "threshold")
	settle, e4 := integerInput(i, "settle")
	if err := errors.Join(e1, e2, e3, e4); err != nil {
		return nodeadapter.AdapterResult{}, err
	}
	if !navigation.Finite(step) || math.Abs(step) < 0.1 || math.Abs(step) > 90 || !navigation.Finite(limit) || limit < 0 || limit > 360 || !navigation.Finite(threshold) || threshold < 0 || threshold > 1 || settle < 0 || settle > 60000 {
		return nodeadapter.AdapterResult{}, fmt.Errorf("invalid rotation search settings")
	}
	region, err := visionRegionInput(i)
	if err != nil {
		return nodeadapter.AdapterResult{}, err
	}
	ref, err := visionBlobInput(i, "template")
	if err != nil {
		return nodeadapter.AdapterResult{}, err
	}
	data, err := readVisionBlob(ctx, i, ref)
	if err != nil {
		return nodeadapter.AdapterResult{}, err
	}
	template, err := prepareVisionTemplate(data)
	if err != nil {
		return nodeadapter.AdapterResult{}, err
	}
	capture, err := openConfiguredTarget(ctx, i, installed.KindCapture, installed.CaptureOperations())
	if err != nil {
		return nodeadapter.AdapterResult{}, err
	}
	defer func() { runErr = errors.Join(runErr, i.Targets.Drop(context.WithoutCancel(parent), capture)) }()
	now := i.MonotonicNow
	if now == nil {
		now = time.Now
	}
	timeout, err := integerInput(i, "timeout")
	if err != nil {
		return nodeadapter.AdapterResult{}, err
	}
	deadline := now().Add(time.Duration(timeout) * time.Millisecond)
	angle := 0.0
	var match visionMatchResult
	exit := "not-found"
	counters := map[string]int64{}
	for {
		if err = i.Wait(ctx, 0); err != nil {
			break
		}
		if !now().Before(deadline) {
			err = context.DeadlineExceeded
			break
		}
		match, _, err = captureAndMatch(ctx, i, capture, template, region, threshold, counters)
		if err != nil {
			break
		}
		if match.Matched {
			exit = "found"
			break
		}
		if math.Abs(angle) >= limit {
			break
		}
		delta := math.Copysign(math.Min(math.Abs(step), limit-math.Abs(angle)), step)
		err = navInvoke(ctx, i, turn, installed.OperationTurnView, installed.TurnViewRequest{Degrees: delta, DurationMilliseconds: 150})
		if err != nil {
			break
		}
		angle += delta
		if err = i.Wait(ctx, time.Duration(settle)*time.Millisecond); err != nil {
			break
		}
	}
	if parent.Err() != nil {
		return nodeadapter.AdapterResult{}, parent.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		exit = "timeout"
	} else if err != nil {
		return nodeadapter.AdapterResult{}, err
	}
	if err := i.EmitStatus(parent, choose(exit == "timeout", nodes.NavigationTimeoutStatus, nodes.NavigationFinishedStatus), counters); err != nil {
		return nodeadapter.AdapterResult{}, err
	}
	if match.Center.Unit == "" {
		match.Center.Unit = "ratio"
		match.Bounds.Unit = "ratio"
	}
	result, err := sealVisionOutputs(b, i, map[string]any{"matched": match.Matched, "score": match.Score, "center": match.Center, "bounds": match.Bounds, "angle": angle})
	result.ExecOutputs = []string{exit}
	return result, err
}
