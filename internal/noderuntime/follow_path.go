package noderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/automation/navigation"
	"github.com/yottaapp/yotta/internal/navigationpath"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
)

var errPathHeight = errors.New("path waypoint altitude does not match")

type pathDriver struct {
	stream     pathSteeringStream
	inputReset bool
	*navigationDriver
	reference                  navigationpath.Reference
	point                      navigationpath.Point
	tolerance, heightTolerance float64
	observation                navigationpath.Observation
}

func (d *pathDriver) Read(ctx context.Context, after time.Time) (navigation.Pose, error) {
	state := d.i.State["position"]
	if state == nil {
		return navigation.Pose{}, navigationpath.ErrUnavailable
	}
	now := d.i.MonotonicNow
	if now == nil {
		now = time.Now
	}
	deadline := now().Add(time.Second)
	var heightSince time.Time
	for {
		if err := d.Wait(ctx, 0); err != nil {
			return navigation.Pose{}, err
		}
		snapshot, err := state.Read()
		if err != nil {
			return navigation.Pose{}, err
		}
		var raw string
		if err := json.Unmarshal(snapshot.Value.InlineJSON(), &raw); err != nil {
			return navigation.Pose{}, navigationpath.ErrUnavailable
		}
		o, err := navigationpath.Observe([]byte(raw), time.Now(), 500*time.Millisecond)
		if errors.Is(err, navigationpath.ErrReference) {
			return navigation.Pose{}, err
		}
		if err == nil && (o.Reference != d.reference || (d.havePosition && o.Epoch != d.observation.Epoch)) {
			return navigation.Pose{}, navigationpath.ErrReference
		}
		if err == nil && o.Heading != nil && (d.point.Z == nil || o.Point.Z != nil) {
			revision := time.Unix(0, snapshot.Revision)
			if revision.After(after) && o.SampleTimeMs >= d.freshAfter && (!d.havePosition || (o.Sequence > d.observation.Sequence && o.SampleTimeMs >= d.observation.SampleTimeMs)) {
				d.observation, d.havePosition = o, true
				if d.point.Z != nil && math.Hypot(o.Point.X-d.point.X, o.Point.Y-d.point.Y) <= d.tolerance && math.Abs(*o.Point.Z-*d.point.Z) > d.heightTolerance {
					if err := d.StopForward(ctx); err != nil {
						return navigation.Pose{}, err
					}
					d.inputReset = true
					if heightSince.IsZero() {
						heightSince = now()
						deadline = heightSince.Add(time.Second)
					}
					if !now().Before(deadline) {
						return navigation.Pose{}, errPathHeight
					}
					// Give a jump at the endpoint a bounded chance to land. A
					// consistently different floor still takes height-mismatch.
					if err := d.Wait(ctx, 20*time.Millisecond); err != nil {
						return navigation.Pose{}, err
					}
					continue
				}
				reset := d.inputReset
				d.inputReset = false
				return navigation.Pose{InputReset: reset, X: o.Point.X, Y: o.Point.Y, Heading: *o.Heading, Time: revision, SampleTime: time.UnixMilli(o.SampleTimeMs), AxisHeading: o.Reference.AxisHeading, AxisSign: float64(o.Reference.AxisSign)}, nil
			}
		} else {
			if err := d.StopForward(ctx); err != nil {
				return navigation.Pose{}, err
			}
			d.inputReset = true
		}
		if !now().Before(deadline) {
			if !heightSince.IsZero() && err == nil && o.Point.Z != nil && d.point.Z != nil && math.Hypot(o.Point.X-d.point.X, o.Point.Y-d.point.Y) <= d.tolerance && math.Abs(*o.Point.Z-*d.point.Z) > d.heightTolerance {
				return navigation.Pose{}, errPathHeight
			}
			return navigation.Pose{}, navigationpath.ErrUnavailable
		}
		if err := d.Wait(ctx, 20*time.Millisecond); err != nil {
			return navigation.Pose{}, err
		}
	}
}

func followPath(b nodes.Builtins) nodeadapter.Adapter      { return pathFollower(b, false) }
func followSavedPath(b nodes.Builtins) nodeadapter.Adapter { return pathFollower(b, true) }
func pathFollower(b nodes.Builtins, saved bool) nodeadapter.Adapter {
	effectID, action := nodes.FollowPathEffectID, "navigation.follow-path"
	if saved {
		effectID, action = nodes.FollowSavedPathEffectID, "navigation.follow-saved-path"
	}
	return func(ctx context.Context, i nodeadapter.Invocation) (_ nodeadapter.AdapterResult, runErr error) {
		counters := map[string]int64{"last_point": 0, "current_point": 0}
		defer func() {
			if runErr != nil && !errors.Is(runErr, context.Canceled) {
				runErr = automationFailure(nodes.NavigationFailedCode, runErr)
			}
			runErr = errors.Join(runErr, recordAdapterOutcome(ctx, i, nodeadapter.AdapterAction{EffectID: effectID, Action: action, SummaryCode: action, Counters: counters}, nodes.NavigationFailedCode, runErr))
		}()
		raw := i.Inputs["path"].InlineJSON()
		if saved {
			var err error
			raw, _, err = readBlobInput(ctx, i, "asset", navigationpath.MediaType, navigationpath.MaxEncodedBytes)
			if err != nil {
				return nodeadapter.AdapterResult{}, err
			}
		}
		path, err := navigationpath.Decode(raw)
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		start, e1 := integerInput(i, "start")
		end, e2 := integerInput(i, "end")
		tolerance, e3 := numberInput(i, "tolerance")
		height, e4 := numberInput(i, "height-tolerance")
		timeout, e5 := integerInput(i, "timeout")
		interval, e6 := integerInput(i, "interval")
		slow, e7 := numberInput(i, "slow-distance")
		if err := errors.Join(e1, e2, e3, e4, e5, e6, e7); err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		if end == -1 {
			end = int64(len(path.Points)) - 1
		}
		if path.Reference.Kind != "world" || start < 0 || end < start || end >= int64(len(path.Points)) || !navigation.Finite(height) || height < 0 || timeout < 1 || timeout > nodes.MaxDelayMilliseconds {
			return nodeadapter.AdapterResult{}, fmt.Errorf("invalid path range, reference, height tolerance or timeout")
		}
		opts := navigation.Options{Tolerance: tolerance, TurnSign: navNumber(i, "turnSign", 1), Pulse: time.Duration(interval) * time.Millisecond, StuckTimeout: 5 * time.Second, SlowDistance: slow, Now: i.MonotonicNow, Timeout: time.Duration(timeout) * time.Millisecond}
		if err := opts.Validate(); err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		d := &pathDriver{navigationDriver: &navigationDriver{i: i}, reference: path.Reference, tolerance: tolerance, heightTolerance: height}
		d.turn, err = openConfiguredTarget(ctx, i, installed.KindInput, []string{installed.OperationTurnView})
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		defer func() {
			runErr = errors.Join(runErr, d.StopForward(context.WithoutCancel(ctx)), i.Targets.Drop(context.WithoutCancel(ctx), d.turn))
		}()
		d.forward, err = openConfiguredTarget(ctx, i, installed.KindInput, []string{installed.OperationPressKeys})
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		defer func() { runErr = errors.Join(runErr, i.Targets.Drop(context.WithoutCancel(ctx), d.forward)) }()
		progress, exit, reason, recoveryAttempt, err := executePathFollow(ctx, b, i, d, path, int(start), int(end), opts)
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		last, current := int64(progress.Last)+start, int64(progress.Current)+start
		counters["last_point"], counters["current_point"] = last+1, current+1
		counters["path_"+strings.ReplaceAll(exit, "-", "_")] = 1
		counters["remaining_distance"] = int64(min(1e15, math.Ceil(progress.Distance)))
		counters["recovery_attempt"] = int64(recoveryAttempt)
		status := pathOutcomeStatus(exit, reason)
		if err := i.EmitStatus(ctx, status, counters); err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		if err := i.EmitStatus(ctx, nodes.PathProgressStatus, counters); err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		result, err := pathFollowOutputs(b, i, path, int(start), int(end), progress, recoveryAttempt, reason, nil)

		result.ExecOutputs = []string{exit}
		return result, err
	}
}
