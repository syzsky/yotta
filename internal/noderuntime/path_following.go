package noderuntime

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/automation/navigation"
	"github.com/yottaapp/yotta/internal/navigationpath"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
)

var errPathMarkerPause = errors.New("path marker requests a stopped action")

// The scheduler owns branches. This adapter reports geometric progress and
// waits for explicit recovery completion; it never executes graph nodes itself.
func executePathFollow(ctx context.Context, b nodes.Builtins, i nodeadapter.Invocation, d *pathDriver, path navigationpath.Path, start, end int, options navigation.Options) (progress navigation.Progress, exit, reason string, attempt int, resultErr error) {
	defer func() {
		if errors.Is(resultErr, installed.ErrCooperativeCameraInput) {
			resultErr = errors.Join(resultErr, i.EmitStatus(context.WithoutCancel(ctx), nodes.PathInputConflictStatus, nil))
		}
	}()
	points := make([]navigation.Waypoint, end-start+1)
	arc := make([]float64, len(points))
	protected := make([]bool, len(points))
	for n := range points {
		protected[n] = path.Points[start+n].Name != ""
		if n > 0 {
			a, b := path.Points[start+n-1], path.Points[start+n]
			protected[n] = protected[n] || (a.Z == nil) != (b.Z == nil) || (a.Z != nil && b.Z != nil && *a.Z != *b.Z)
		}
		points[n] = navigation.Waypoint{X: path.Points[start+n].X, Y: path.Points[start+n].Y}
		if n > 0 {
			arc[n] = arc[n-1] + math.Hypot(points[n].X-points[n-1].X, points[n].Y-points[n-1].Y)
		}
	}
	progressArc := func(at navigation.Progress) float64 {
		if at.Cursor.NextIndex <= 0 {
			return 0
		}
		if at.Cursor.NextIndex >= len(points) {
			return arc[len(arc)-1]
		}
		return arc[at.Cursor.NextIndex-1] + at.Cursor.Offset
	}
	now := i.MonotonicNow
	if now == nil {
		now = time.Now
	}
	limit := int(navNumber(i, "recovery-attempts", 2))
	interval := time.Duration(navNumber(i, "action-interval", 500)) * time.Millisecond
	pauseMarker := navString(i, "marker-mode", "continue") == "pause"
	connected := func(port string) bool { return i.Branch != nil && i.HasBranch != nil && i.HasBranch(port) }
	var cursor *navigation.FollowCursor
	var markers []int
	type pendingBranch struct {
		handle nodeadapter.BranchHandle
		port   string
	}
	var pending []pendingBranch
	actionsCtx, movingCtx, cancelActions, cancelMoving := newPathActionContexts(ctx)
	defer func() { cancelMoving(); cancelActions() }()
	join := func(handle nodeadapter.BranchHandle) error {
		if i.Await != nil {
			if err := i.Await(ctx, handle.Done(), 0); err != nil {
				return err
			}
		} else {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-handle.Done():
			}
		}
		return handle.Err()
	}
	stopActions := func() error {
		cancelMoving()
		cancelActions()
		var err error
		for _, handle := range pending {
			err = errors.Join(err, pathCancellationRemainder(join(handle.handle)))
		}
		pending = nil
		return err
	}
	prune := func() error {
		live := pending[:0]
		for _, handle := range pending {
			select {
			case <-handle.handle.Done():
				if err := handle.handle.Err(); err != nil {
					return err
				}
			default:
				live = append(live, handle)
			}
		}
		pending = live
		return nil
	}
	branch := func(port string, at navigation.Progress, why string, wait bool) error {
		if !connected(port) {
			return nil
		}
		outputs, err := pathFollowOutputs(b, i, path, start, end, at, attempt, why, arc)
		if err != nil {
			return err
		}
		branchCtx := actionsCtx
		if port == "moving" {
			branchCtx = movingCtx
		}
		if wait {
			branchCtx = ctx
		}
		handle, err := i.Branch(branchCtx, nodeadapter.BranchRequest{Output: port, Outputs: outputs.Outputs, CooperativeInput: !wait})
		if err != nil {
			return err
		}
		if wait {
			return join(handle)
		}
		// Coalesced ticks return the same handle; keep at most one reference.
		for _, old := range pending {
			if old.handle == handle {
				return nil
			}
		}
		pending = append(pending, pendingBranch{handle, port})
		return nil
	}
	var tickAt, statusAt time.Time
	lastRecoveryArc := 0.0
	for {
		opts := navigation.FollowOptions{Options: options, ObservationNow: d.wallTime, Resume: cursor, ProtectedPoints: protected}
		opts.OnCrossed = func(index int) error {
			if connected("marker") && path.Points[start+index].Name != "" {
				markers = append(markers, index)
			}
			return nil
		}
		opts.OnProgress = func(at navigation.Progress) error {
			progress = at
			d.point = navigationpath.Point{}
			// Transient jump height at intermediate samples is not a wrong floor.
			// The final endpoint still requires the recorded altitude when supplied.
			if at.Current == len(points)-1 {
				d.point = path.Points[end]
			}
			if err := prune(); err != nil {
				return err
			}
			if attempt > 0 && progressArc(at) > lastRecoveryArc+options.Tolerance*4 {
				attempt = 0
				lastRecoveryArc = progressArc(at)
			}
			if now().Sub(statusAt) >= 500*time.Millisecond || at.Last == len(points)-1 {
				statusAt = now()
				if err := i.EmitStatus(ctx, nodes.PathProgressStatus, pathProgressCounters(at, start, attempt, d.wallTime().UnixMilli()-d.observation.SampleTimeMs)); err != nil {
					return err
				}
			}
			if len(markers) > 0 && pauseMarker {
				return errPathMarkerPause
			}
			for _, index := range markers {
				marked := at
				marked.Current = index
				if err := branch("marker", marked, "", false); err != nil {
					return err
				}
			}
			markers = nil
			if now().Sub(tickAt) >= interval && at.Last < len(points)-1 {
				tickAt = now()
				return branch("moving", at, "", false)
			}
			return nil
		}
		var err error
		progress, err = navigation.Follow(ctx, d, points, opts)
		resume := progress.Cursor
		cursor = &resume
		if ctx.Err() != nil {
			return progress, "", "", attempt, errors.Join(ctx.Err(), err)
		}
		// Follow joins its final physical release with the navigation outcome.
		// A matching sentinel is insufficient when another leaf says release
		// failed: never resume or report a handled outcome in that case.
		for _, cause := range []error{errPathMarkerPause, navigation.ErrStuck, context.DeadlineExceeded, navigationpath.ErrUnavailable, navigation.ErrStale, navigationpath.ErrReference, errPathHeight} {
			if errors.Is(err, cause) && !pathOnlyCause(err, cause) {
				return progress, "", "", attempt, err
			}
		}
		if errors.Is(err, errPathMarkerPause) {
			if err := stopActions(); err != nil {
				return progress, "", "", attempt, err
			}
			for _, index := range markers {
				marked := progress
				marked.Current = index
				if err := branch("marker", marked, "", true); err != nil {
					return progress, "", "", attempt, err
				}
			}
			markers = nil
			actionsCtx, movingCtx, cancelActions, cancelMoving = newPathActionContexts(ctx)
			d.freshAfter = d.wallTime().UnixMilli() + 1
			continue
		}
		if errors.Is(err, navigation.ErrStuck) && connected("recover") && attempt < limit {
			if err := stopActions(); err != nil {
				return progress, "", "", attempt, err
			}
			attempt++
			lastRecoveryArc = progressArc(progress)
			reason = "no-progress"
			if errors.Is(err, navigation.ErrTurnUnresponsive) {
				reason = "turn-unresponsive"
			}
			if err := i.EmitStatus(ctx, nodes.PathRecoveringStatus, map[string]int64{"current_point": int64(start + progress.Current + 1), "recovery_attempt": int64(attempt)}); err != nil {
				return progress, "", "", attempt, err
			}
			if err := branch("recover", progress, reason, true); err != nil {
				return progress, "", "", attempt, err
			}
			actionsCtx, movingCtx, cancelActions, cancelMoving = newPathActionContexts(ctx)
			d.freshAfter = d.wallTime().UnixMilli() + 1
			continue
		}
		exit, reason = "arrived", ""
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			exit, reason = "timeout", "timeout"
		case errors.Is(err, navigation.ErrTurnUnresponsive):
			exit, reason = "stuck", "turn-unresponsive"
		case errors.Is(err, navigation.ErrStuck):
			exit, reason = "stuck", "no-progress"
		case errors.Is(err, navigationpath.ErrUnavailable), errors.Is(err, navigation.ErrStale):
			exit, reason = "unavailable", "position-unavailable"
		case errors.Is(err, navigationpath.ErrReference):
			exit, reason = "reference-mismatch", "reference-mismatch"
		case errors.Is(err, errPathHeight):
			exit, reason = "height-mismatch", "height-mismatch"
		case err != nil:
			return progress, "", "", attempt, err
		}
		// Periodic work is scoped to movement. Finish queued marker actions on
		// arrival; failure/cancellation instead cancels them with the invocation.
		if exit == "arrived" {
			cancelMoving()
			for _, handle := range pending {
				err := join(handle.handle)
				if handle.port == "moving" {
					err = pathCancellationRemainder(err)
				}
				if err != nil {
					return progress, "", "", attempt, err
				}
			}
		}
		return progress, exit, reason, attempt, nil
	}
}

// Return both cancellation handles to the invocation owner. A recovery replaces
// the scope only after stopActions has cancelled and joined the previous one.
func newPathActionContexts(ctx context.Context) (context.Context, context.Context, context.CancelFunc, context.CancelFunc) {
	actions, cancelActions := context.WithCancel(ctx)
	moving, cancelMoving := context.WithCancel(actions)
	return actions, moving, cancelActions, cancelMoving
}

func pathOnlyCause(err, cause error) bool {
	if err == cause {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if child != nil && !pathOnlyCause(child, cause) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return pathOnlyCause(wrapped, cause)
	}
	return false
}

// Expected cancellation must not hide a failed key release or child cleanup.
func pathCancellationRemainder(err error) error {
	if err == nil || err == context.Canceled {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var result error
		for _, child := range joined.Unwrap() {
			result = errors.Join(result, pathCancellationRemainder(child))
		}
		return result
	}
	if wrapped := errors.Unwrap(err); wrapped != nil && errors.Is(err, context.Canceled) {
		return pathCancellationRemainder(wrapped)
	}
	return err
}

func pathFollowOutputs(b nodes.Builtins, i nodeadapter.Invocation, path navigationpath.Path, start, end int, progress navigation.Progress, attempt int, reason string, arc []float64) (nodeadapter.AdapterResult, error) {
	current := min(len(path.Points)-1, max(start, start+progress.Current))
	point := path.Points[current]
	if arc == nil {
		arc = make([]float64, end-start+1)
		for n := 1; n < len(arc); n++ {
			a, b := path.Points[start+n-1], path.Points[start+n]
			arc[n] = arc[n-1] + math.Hypot(b.X-a.X, b.Y-a.Y)
		}
	}
	fraction := 0.0
	if progress.Cursor.NextIndex >= len(arc) {
		fraction = 1
	} else if progress.Cursor.NextIndex > 0 && arc[len(arc)-1] > 0 {
		fraction = (arc[progress.Cursor.NextIndex-1] + progress.Cursor.Offset) / arc[len(arc)-1]
	}
	return sealVisionOutputs(b, i, map[string]any{"last-index": start + progress.Last, "current-index": current, "point-id": point.ID, "point-name": point.Name, "x": progress.Pose.X, "y": progress.Pose.Y, "distance": progress.Distance, "progress": min(1, fraction), "recovery-attempt": attempt, "reason": reason})
}

func pathOutcomeStatus(exit, reason string) string {
	switch exit {
	case "stuck":
		if reason == "turn-unresponsive" {
			return nodes.PathTurningStuckStatus
		}
		return nodes.PathStuckStatus
	case "timeout":
		return nodes.NavigationTimeoutStatus
	case "unavailable":
		return nodes.PathUnavailableStatus
	case "reference-mismatch":
		return nodes.PathReferenceStatus
	case "height-mismatch":
		return nodes.PathHeightStatus
	default:
		return nodes.NavigationFinishedStatus
	}
}

func pathProgressCounters(at navigation.Progress, start, attempt int, sampleAge int64) map[string]int64 {
	counters := map[string]int64{
		"last_point": int64(at.Last + start + 1), "current_point": int64(at.Current + start + 1),
		"remaining_distance": int64(min(1e15, math.Ceil(at.Distance))), "recovery_attempt": int64(attempt),
		"heading_mdeg": pathMetric(navigation.Normalize(at.Pose.Heading)) % 360000,
		"speed_milli":  pathMetric(at.Speed), "sample_age_ms": max(0, sampleAge),
	}
	// Journal counters are nonnegative. Keep direction independently from the
	// rounded magnitude so negative coordinates and corrections remain readable.
	signed := func(name, unit string, value float64) {
		counters[name+"_"+unit+"_abs"] = pathMetric(math.Abs(value))
		negative := int64(0)
		if value < 0 {
			negative = 1
		}
		counters[name+"_negative"] = negative
	}
	signed("x", "milli", at.Pose.X)
	signed("y", "milli", at.Pose.Y)
	signed("target_x", "milli", at.Target.X)
	signed("target_y", "milli", at.Target.Y)
	signed("heading_error", "mdeg", at.HeadingError)
	return counters
}

// Bound nonnegative diagnostics before conversion; these are evidence, not inputs.
func pathMetric(value float64) int64 {
	if math.IsNaN(value) {
		return 0
	}
	return int64(math.Round(max(0, min(1e15, value*1000))))
}
