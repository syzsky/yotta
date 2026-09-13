package navigation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	ErrTurnUnresponsive = fmt.Errorf("navigation heading did not respond: %w", ErrStuck)
	ErrNoPathProgress   = fmt.Errorf("navigation path progress stalled: %w", ErrStuck)
)

// FollowOptions reuses Options; X/Y are ignored. SlowDistance controls the
// observation interval near the final endpoint and sharp bends, not key pulses.
// Timeout is a per-point deadline, reset whenever a point is crossed in order;
// it is not a total route timeout. StuckTimeout remains the movement watchdog.
// Zero tuning values select defaults in world units derived from Tolerance.
type FollowOptions struct {
	Options
	// ObservationNow shares the source acquisition clock; Now may exclude pauses.
	// Simulated drivers default both clocks to Now.
	ObservationNow func() time.Time
	Resume         *FollowCursor
	// OnCrossed runs synchronously, in increasing zero-based index order. It
	// must not mutate the driver. Resuming does not replay reported indices,
	// including the index whose callback returned an error.
	OnCrossed func(index int) error
	// OnProgress runs once per fresh control observation, after crossing
	// callbacks and before further input. Callback errors stop and release.
	// The final invocation includes the confirmed endpoint cursor.
	OnProgress                 func(Progress) error
	MinLookahead, MaxLookahead float64
	CornerDeviation            float64
	// MaxTurn bounds each correction in degrees; SharpAngle is the bend or
	// heading error at which forward is released to align in place.
	MaxTurn, SharpAngle float64
	// Tracking settings have separate meanings from final arrival precision.
	TrackingTolerance, MaxAngularVelocity, MaxAngularAcceleration float64
	LookaheadTime                                                 time.Duration
	ProtectedPoints                                               []bool
	SimplifyDeviation                                             float64
}

type Progress struct {
	Pose Pose
	// Current is the next unreported point, clamped to the final index
	// on completion. Last is -1 before the first point is crossed.
	Current, Last int
	// Distance is cross-track distance plus untraversed arc length; it is
	// endpoint distance after completion. Cursor can resume this same path.
	Distance float64
	Cursor   FollowCursor
	// Decision evidence, in the same coordinate units as the original route.
	Target              Waypoint
	HeadingError, Speed float64
}

func (o FollowOptions) normalized() (FollowOptions, error) {
	o.X, o.Y = 0, 0
	if err := o.Options.Validate(); err != nil {
		return o, err
	}
	if o.Timeout < 0 {
		return o, errors.New("invalid navigation timeout")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.ObservationNow == nil {
		o.ObservationNow = o.Now
	}
	if o.SlowDistance == 0 {
		o.SlowDistance = o.Tolerance * 4
	}
	if o.TrackingTolerance == 0 {
		o.TrackingTolerance = o.Tolerance * 2
	}
	if o.LookaheadTime == 0 {
		o.LookaheadTime = 800 * time.Millisecond
	}
	if o.MaxAngularVelocity == 0 {
		o.MaxAngularVelocity = 90
	}
	if o.MaxAngularAcceleration == 0 {
		o.MaxAngularAcceleration = 180
	}
	if o.SimplifyDeviation == 0 {
		o.SimplifyDeviation = o.Tolerance * 0.25
	}
	if !Finite(o.TrackingTolerance) || o.TrackingTolerance <= 0 || o.LookaheadTime < 100*time.Millisecond || o.LookaheadTime > 5*time.Second || !Finite(o.MaxAngularVelocity) || o.MaxAngularVelocity <= 0 || !Finite(o.MaxAngularAcceleration) || o.MaxAngularAcceleration <= 0 || !Finite(o.SimplifyDeviation) || o.SimplifyDeviation < 0 {
		return o, errors.New("invalid path tracking settings")
	}
	if o.MinLookahead == 0 {
		o.MinLookahead = o.TrackingTolerance * 2
	}
	if o.MaxLookahead == 0 {
		o.MaxLookahead = o.TrackingTolerance * 8
	}
	if o.CornerDeviation == 0 {
		o.CornerDeviation = o.Tolerance
	}
	if o.MaxTurn == 0 {
		o.MaxTurn = 12
	}
	if o.SharpAngle == 0 {
		o.SharpAngle = 75
	}
	if !Finite(o.MinLookahead) || !Finite(o.MaxLookahead) || !Finite(o.CornerDeviation) || !Finite(o.MaxTurn) || !Finite(o.SharpAngle) ||
		o.MinLookahead <= 0 || o.MaxLookahead < o.MinLookahead || o.CornerDeviation <= 0 || o.MaxTurn <= 0 || o.MaxTurn > 45 || o.SharpAngle <= o.MaxTurn || o.SharpAngle > 180 {
		return o, errors.New("invalid navigation following settings")
	}
	return o, nil
}

// Follow tracks an ordered polyline continuously. The driver owns pose age
// validation (as for MoveTo); Follow additionally rejects invalid/replayed poses.
// Every return attempts release with an uncancelled context, even when input
// partially succeeds before returning an error. No real input is owned here.
// HoldForward must be idempotent: Follow reasserts the desired held state after
// observations, since Read/Wait may release actual input while paused or stale.
func Follow(ctx context.Context, d Driver, points []Waypoint, options FollowOptions) (result Progress, resultErr error) {
	result.Last = -1
	defer func() { resultErr = errors.Join(resultErr, d.StopForward(context.WithoutCancel(ctx))) }()
	o, err := options.normalized()
	if err != nil {
		return result, err
	}
	path, err := newFollowPath(points, o.SharpAngle)
	if err != nil {
		return result, err
	}
	prepared, err := newPreparedFollowPath(path, o.ProtectedPoints, o.SimplifyDeviation, o.SharpAngle)
	if err != nil {
		return result, err
	}
	cursor := FollowCursor{}
	if o.Resume != nil {
		cursor = *o.Resume
	}
	if cursor.NextIndex < 0 || cursor.NextIndex > len(points) || !Finite(cursor.Offset) || cursor.Offset < 0 ||
		((cursor.NextIndex == 0 || cursor.NextIndex == len(points)) && cursor.Offset != 0) ||
		(cursor.NextIndex > 0 && cursor.NextIndex < len(points) && cursor.Offset > path.length[cursor.NextIndex]) {
		return result, errors.New("invalid navigation progress cursor")
	}
	update := func() {
		result.Cursor = cursor
		result.Current = min(cursor.NextIndex, len(points)-1)
		result.Last = cursor.NextIndex - 1
		result.Distance = path.remaining(cursor, Waypoint{result.Pose.X, result.Pose.Y})
		result.Target = path.location(cursor)
		result.HeadingError = 0
	}
	update()
	pointStarted := o.Now()
	reportProgress := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if o.OnProgress != nil {
			if err := o.OnProgress(result); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if o.Timeout > 0 && o.Now().Sub(pointStarted) >= o.Timeout {
			return context.DeadlineExceeded
		}
		return nil
	}
	progressAt, turnAt := pointStarted, pointStarted
	bestDistance := math.Inf(1)
	var after time.Time
	var previous Pose
	previousAt := pointStarted
	havePrevious, held, aligned, confirming := false, false, false, false
	speed := 0.0
	heldAt := o.Now()
	var steering pathSteering
	var observer pathObservation
	var endpoint pathEndpoint
	recovering, recoveryTargetSet := false, false
	var recoveryTarget Waypoint
	stream, streaming := d.(streamingPathSteering)
	streamRate := 0.0
	lastSteer := o.Now()
	streamAt := o.Now()
	rememberStream := func() {
		now := o.Now()
		if streaming {
			observer.turn(streamAt, now, streamRate*now.Sub(streamAt).Seconds())
		}
		streamAt = now
	}
	setStream := func(rate float64) error {
		rememberStream()
		if err := stream.SetSteering(ctx, rate*o.TurnSign); err != nil {
			return err
		}
		streamRate = rate
		return nil
	}
	timedSteering, smoothSteering := d.(timedPathSteering)
	var pendingTurn float64
	var turnHeading float64
	turning := false
	turnDuration := time.Duration(0)
	stop := func() error {
		if streaming {
			if err := setStream(0); err != nil {
				return err
			}
		}
		if !held {
			return nil
		}
		if err := d.StopForward(ctx); err != nil {
			return err
		}
		held = false
		if endpoint.armed {
			endpoint.active = true
			endpoint.stop(o.Now())
		}
		steering.reset()
		return nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if o.Timeout > 0 && o.Now().Sub(pointStarted) >= o.Timeout {
			return result, context.DeadlineExceeded
		}
		pose, err := d.Read(ctx, after)
		if err != nil {
			return result, err
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if o.Timeout > 0 && o.Now().Sub(pointStarted) >= o.Timeout {
			return result, context.DeadlineExceeded
		}
		if !Finite(pose.X) || !Finite(pose.Y) || !Finite(pose.Heading) || !Finite(pose.AxisHeading) || math.Abs(pose.AxisSign) != 1 || !pose.Time.After(after) {
			return result, ErrStale
		}
		if pose.InputReset {
			observer = pathObservation{}
			streamRate = 0
			streamAt = o.Now()
			held = false
			aligned = false
			speed = 0
			havePrevious = false
			steering.reset()
			if endpoint.active {
				// Movement during a pause cannot calibrate the preceding key pulse.
				endpoint.pulsePending = false
				endpoint.stop(o.Now())
			}
			lastSteer = o.Now()
			turning = false
			pendingTurn = 0
		}
		rememberStream()
		result.Pose, after = pose, pose.Time
		now := o.Now()
		sampleAt := now
		if !pose.SampleTime.IsZero() {
			sampleAt = now.Add(-o.ObservationNow().Sub(pose.SampleTime))
		}
		pos := Waypoint{pose.X, pose.Y}
		travel := 0.0
		if havePrevious {
			travel = math.Hypot(pose.X-previous.X, pose.Y-previous.Y)
			if held && sampleAt.After(previousAt) {
				motionAt := previousAt
				if heldAt.After(motionAt) {
					motionAt = heldAt
				}
				observed := travel / max(time.Millisecond, sampleAt.Sub(motionAt)).Seconds()
				// Recent speed follows acceleration and deceleration instead of
				// retaining a transient peak for the rest of the route.
				if speed == 0 {
					speed = observed
				} else {
					weight := 1 - math.Exp(-sampleAt.Sub(previousAt).Seconds()/0.4)
					speed += weight * (observed - speed)
				}
			}
		}
		result.Speed = speed
		if pendingTurn != 0 {
			response := Delta(pose.Heading, turnHeading) * math.Copysign(1, pendingTurn)
			if response >= min(0.5, math.Abs(pendingTurn)*0.25) {
				turnAt = now
				if !held {
					progressAt = now
				}
			}
			pendingTurn = 0
		}
		// Pair the pose with its observation time before invoking callbacks.
		// A blocking callback belongs in the NEXT sample's elapsed time; using
		// its return time as this pose's timestamp would inflate peak velocity.
		previous, previousAt, havePrevious = pose, sampleAt, true
		old := cursor.NextIndex
		// Travel bounds arc advancement; capping the window also prevents a
		// discontinuous position jump from consuming a whole loop.
		projected, bend := path.match(cursor, pos, min(o.MaxLookahead, max(o.MinLookahead, travel*1.5)), o.Tolerance, o.TrackingTolerance)
		for i := old; i < projected.NextIndex; i++ {
			cursor = FollowCursor{NextIndex: i + 1}
			pointStarted = now
			update()
			if o.OnCrossed != nil {
				if err := o.OnCrossed(i); err != nil {
					return result, err
				}
			}
			if err := ctx.Err(); err != nil {
				return result, err
			}
		}
		cursor = projected
		update()
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if bend {
			if err := stop(); err != nil {
				return result, err
			}
			aligned = false
		}
		endpointDistance := waypointDistance(pos, path.points[len(points)-1])
		if recovering {
			endpointDistance = waypointDistance(pos, recoveryTarget)
		}
		// Enter only the ordered final approach, never a nearby start of a loop or
		// across an unvisited sharp gate. Earlier tracking remains unchanged.
		if streaming && !endpoint.active && cursor.NextIndex >= len(points)-1 && result.Distance <= max(o.Tolerance*6, speed*0.7) {
			endpoint.armed = true
			endpoint.speed = max(endpoint.speed, speed)
			// Dense points can report the final segment only AFTER the predictive
			// release. Do not reassert forward before that movement has settled.
			if !held {
				if err := stop(); err != nil {
					return result, err
				}
				endpoint.active = true
				endpoint.stop(o.Now())
			}
		}
		if endpoint.active && endpoint.settling {
			if !endpoint.settled(pos, sampleAt, o.Tolerance) {
				if err := reportProgress(); err != nil {
					return result, err
				}
				if now.Sub(endpoint.stoppedAt) >= o.StuckTimeout {
					return result, ErrNoPathProgress
				}
				if err := d.Wait(ctx, 20*time.Millisecond); err != nil {
					return result, err
				}
				continue
			}
			if !endpoint.correcting {
				endpoint.correcting = true
				bestDistance = result.Distance
				progressAt = now
			}
			aligned = false
		}
		if recovering && !endpoint.settling {
			if !recoveryTargetSet {
				nearest, _ := path.match(cursor, pos, o.MaxLookahead, o.Tolerance, math.Inf(1))
				recoveryTarget = prepared.aim(nearest, o.MinLookahead, max(0, o.CornerDeviation-o.SimplifyDeviation))
				recoveryTargetSet = true
			}
			endpointDistance = waypointDistance(pos, recoveryTarget)
			if endpointDistance <= o.Tolerance {
				endpoint = pathEndpoint{}
				recovering = false
				aligned = false
				bestDistance, progressAt = result.Distance, now
				continue
			}
		}
		// Reaching the end's coordinates is insufficient until every preceding
		// point has been crossed in order (including when end == start).
		atEnd := !recovering && cursor.NextIndex >= len(points)-1 && endpointDistance <= o.Tolerance &&
			(cursor.NextIndex == 0 || cursor.NextIndex == len(points) || path.length[cursor.NextIndex]-cursor.Offset <= o.Tolerance)
		if atEnd {
			if err := stop(); err != nil {
				return result, err
			}
			if confirming {
				old := cursor.NextIndex
				cursor = FollowCursor{NextIndex: len(points)}
				update()
				if old < len(points) && o.OnCrossed != nil {
					if err := o.OnCrossed(len(points) - 1); err != nil {
						return result, err
					}
				}
				if err := reportProgress(); err != nil {
					return result, err
				}
				return result, ctx.Err()
			}
			confirming = true
			if err := reportProgress(); err != nil {
				return result, err
			}
			// Yield for drivers whose Read is immediately available; the next
			// sample must still be newer than the pre-stop sample.
			if err := d.Wait(ctx, min(o.Pulse, 20*time.Millisecond)); err != nil {
				return result, err
			}
			continue
		}
		confirming = false
		if result.Distance < bestDistance-o.Tolerance*0.1 {
			bestDistance, progressAt = result.Distance, now
		}
		look := max(o.MinLookahead, min(o.MaxLookahead, speed*max(o.LookaheadTime.Seconds(), o.Pulse.Seconds()*2+turnDuration.Seconds())))
		measurement := pose
		if !pose.SampleTime.IsZero() {
			measurement.SampleTime = sampleAt
		}
		controlPose := observer.predict(measurement, now, speed, held, heldAt)
		controlPos := Waypoint{controlPose.X, controlPose.Y}
		aimCursor, _ := path.match(cursor, controlPos, o.MaxLookahead, o.Tolerance, o.TrackingTolerance)
		target := prepared.aim(aimCursor, look, max(0, o.CornerDeviation-o.SimplifyDeviation))
		if endpoint.active {
			target = path.points[len(points)-1]
			if recovering {
				target = recoveryTarget
			}
		}
		angle := Delta(Normalize(pose.AxisHeading+pose.AxisSign*math.Atan2(target.Y-controlPose.Y, target.X-controlPose.X)*180/math.Pi), controlPose.Heading)
		// Departure recovery is separate from cruising. Only the target uses a
		// relaxed corridor; confirmed progress still uses the original corridor.
		if streaming && held && !endpoint.active && cursor.NextIndex > 0 && math.Abs(angle) >= min(45, o.SharpAngle) {
			nearest, _ := path.match(cursor, pos, o.MaxLookahead, o.Tolerance, math.Inf(1))
			if waypointDistance(pos, path.location(nearest)) > o.TrackingTolerance {
				recovering, recoveryTargetSet = true, false
				endpoint.armed = true
				endpoint.speed = speed
				if err := stop(); err != nil {
					return result, err
				}
				continue
			}
		}
		result.Target, result.HeadingError = target, angle
		if err := reportProgress(); err != nil {
			return result, err
		}
		// A fixed-duration turn cannot be shortened by asking for fewer degrees.
		// If the current ray already intersects the endpoint/gate tolerance,
		// finish with short observations instead of turning past it. Otherwise
		// release at that stopping boundary before the corrective turn.
		boundary := target == path.points[len(points)-1] ||
			target == path.points[path.nextSharp[min(cursor.NextIndex, len(points)-1)]]
		targetDistance := waypointDistance(controlPos, target)
		if held && boundary && targetDistance <= speed*max(turnDuration, o.Pulse).Seconds()+o.Tolerance {
			if math.Abs(angle) < 90 && targetDistance*math.Abs(math.Sin(angle*math.Pi/180)) <= o.Tolerance*0.9 {
				angle = 0
			} else if math.Abs(angle) > 3 {
				aligned = false
			}
		}
		if math.Abs(angle) >= o.SharpAngle {
			aligned = false
		}
		alignmentTolerance := 5.0
		if smoothSteering {
			alignmentTolerance = 10
		}
		if endpoint.active {
			alignmentTolerance = 2
		}
		if !aligned && math.Abs(angle) <= alignmentTolerance {
			aligned = true
		}
		if !aligned {
			if err := stop(); err != nil {
				return result, err
			}
			if endpoint.active && endpoint.settling {
				continue
			}
		}
		if now.Sub(progressAt) >= o.StuckTimeout && (aligned || math.Abs(angle) <= 5) {
			return result, ErrNoPathProgress
		}
		if endpoint.active && aligned {
			if err := setStream(0); err != nil {
				return result, err
			}
			duration := endpoint.pulse(pos, endpointDistance, o.Tolerance)
			if err := d.HoldForward(ctx); err != nil {
				return result, err
			}
			held = true
			heldAt = o.Now()
			if err := d.Wait(ctx, duration); err != nil {
				return result, err
			}
			if err := stop(); err != nil {
				return result, err
			}
			endpoint.stop(o.Now())
			continue
		}
		if smoothSteering && aligned && held && boundary && speed > 0 {
			// Stop before the next observation latency carries us through a narrow
			// corner/endpoint window. The next fresh stopped pose still decides arrival.
			radians := result.HeadingError * math.Pi / 180
			forward := targetDistance * math.Cos(radians)
			lateral := math.Abs(targetDistance * math.Sin(radians))
			until := time.Duration(forward / speed * float64(time.Second))
			reaction := min(o.Pulse, 100*time.Millisecond)
			if !pose.SampleTime.IsZero() {
				reaction += max(0, min(500*time.Millisecond, now.Sub(sampleAt)))
			}
			if forward > 0 && lateral <= o.Tolerance*0.9 && until <= reaction {
				if streaming {
					if err := setStream(0); err != nil {
						return result, err
					}
				}
				if err := d.Wait(ctx, until); err != nil {
					return result, err
				}
				if err := stop(); err != nil {
					return result, err
				}
				continue
			}
		}
		if smoothSteering && aligned && held {
			duration := min(o.Pulse, 100*time.Millisecond)
			// Shorten the final step using measured speed without pulsing the key.
			if boundary && speed > 0 {
				duration = min(duration, max(20*time.Millisecond, time.Duration(max(o.Tolerance*0.1, targetDistance-o.Tolerance*0.5)/speed*0.5*float64(time.Second))))
			}
			duration = max(20*time.Millisecond, duration/time.Millisecond*time.Millisecond)
			turn := 0.0
			if streaming {
				period := max(duration, min(500*time.Millisecond, now.Sub(lastSteer)))
				turn = steering.angularRate(angle, targetDistance, speed, period, o) * duration.Seconds()
				lastSteer = now
			} else {
				turn = steering.angle(angle, targetDistance, speed, duration, o)
			}
			if streaming {
				if err := setStream(turn / duration.Seconds()); err != nil {
					return result, err
				}
				if math.Abs(turn) > 0.01 {
					if !turning {
						turning, turnAt = true, now
					}
					if now.Sub(turnAt) >= o.StuckTimeout {
						return result, ErrTurnUnresponsive
					}
					pendingTurn, turnHeading = turn, pose.Heading
				} else {
					turning = false
				}
				if err := d.HoldForward(ctx); err != nil {
					return result, err
				}
				if err := d.Wait(ctx, duration); err != nil {
					return result, err
				}
				continue
			}
			if math.Abs(turn) > 0.01 {
				if !turning {
					turning, turnAt = true, now
				}
				if now.Sub(turnAt) >= o.StuckTimeout {
					return result, ErrTurnUnresponsive
				}
				if err := d.HoldForward(ctx); err != nil {
					return result, err
				}
				pendingTurn, turnHeading = turn, pose.Heading
				before := o.Now()
				if err := timedSteering.Steer(ctx, turn*o.TurnSign, duration); err != nil {
					return result, err
				}
				observer.turn(before, o.Now(), turn)
				turnDuration = max(turnDuration, o.Now().Sub(before))
				if elapsed := o.Now().Sub(before); elapsed < duration {
					if err := d.Wait(ctx, duration-elapsed); err != nil {
						return result, err
					}
				}
			} else {
				turning = false
				if err := d.HoldForward(ctx); err != nil {
					return result, err
				}
				if err := d.Wait(ctx, duration); err != nil {
					return result, err
				}
			}
			continue
		}
		if streaming && !aligned {
			if !turning {
				turning, turnAt = true, now
			}
			if now.Sub(turnAt) >= o.StuckTimeout {
				return result, ErrTurnUnresponsive
			}
			dt := min(o.Pulse, 100*time.Millisecond)
			horizon := dt
			if !pose.SampleTime.IsZero() {
				horizon += max(0, min(500*time.Millisecond, now.Sub(sampleAt)))
			}
			rate := math.Copysign(min(180, math.Sqrt(2*360*math.Abs(angle)), math.Abs(angle)/horizon.Seconds()), angle)
			if err := setStream(rate); err != nil {
				return result, err
			}
			pendingTurn, turnHeading = rate*dt.Seconds(), pose.Heading
			if err := d.Wait(ctx, dt); err != nil {
				return result, err
			}
			continue
		}
		if math.Abs(angle) > 3 && (!smoothSteering || !aligned) {
			if !turning {
				turning, turnAt = true, now
			}
			if now.Sub(turnAt) >= o.StuckTimeout {
				return result, ErrTurnUnresponsive
			}
			if held {
				// This is the desired state, not proof the adapter still owns
				// physical input after a pause inside Read or Wait.
				if err := d.HoldForward(ctx); err != nil {
					return result, err
				}
			}
			// Bound lateral travel during the observed (possibly fixed) turn
			// duration as well as the angular correction itself.
			turn := max(-o.MaxTurn, min(o.MaxTurn, angle))
			if held && speed > 0 && turnDuration > 0 {
				limit := math.Asin(min(1, o.CornerDeviation/(speed*turnDuration.Seconds()))) * 180 / math.Pi
				turn = max(-limit, min(limit, turn))
			}
			pendingTurn, turnHeading = turn, pose.Heading
			before := o.Now()
			if err := d.Turn(ctx, turn*o.TurnSign); err != nil {
				return result, err
			}
			observer.turn(before, o.Now(), turn)
			turnDuration = max(turnDuration, o.Now().Sub(before))
			// Some drivers turn instantly; yielding also makes nonresponse
			// watchdogs deterministic with an injected clock.
			if elapsed := o.Now().Sub(before); elapsed < o.Pulse {
				if err := d.Wait(ctx, o.Pulse-elapsed); err != nil {
					return result, err
				}
			}
			continue
		}
		turning = false
		if streaming {
			if err := setStream(0); err != nil {
				return result, err
			}
		}
		if err := d.HoldForward(ctx); err != nil {
			return result, err
		}
		heldAt = o.Now()
		held = true
		wait := o.Pulse
		// The target chord never extends beyond a sharp bend or the endpoint.
		// Shorten observation intervals there without releasing forward.
		distance := waypointDistance(pos, target)
		if distance < o.SlowDistance {
			wait = max(time.Millisecond, o.Pulse/4)
		}
		if speed > 0 {
			wait = min(wait, max(time.Millisecond, time.Duration(max(o.Tolerance*0.1, distance-o.Tolerance*0.5)/speed*0.5*float64(time.Second))))
		}
		if err := d.Wait(ctx, wait); err != nil {
			return result, err
		}
	}
}
