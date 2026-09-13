// Package navigation controls straight-line character movement using fresh world poses.
// It owns no device or transport; adapters supply observation and bounded input.
package navigation

import (
	"context"
	"errors"
	"math"
	"time"
)

var (
	ErrStale = errors.New("navigation position is unavailable or stale")
	ErrStuck = errors.New("navigation is not making progress")
)

type Pose struct {
	InputReset    bool
	X, Y, Heading float64
	Time          time.Time
	// SampleTime is the source acquisition time; Time remains the delivery revision.
	SampleTime            time.Time
	AxisHeading, AxisSign float64
}

type Driver interface {
	// Read must return a valid pose newer than after, or fail without sending input.
	Read(context.Context, time.Time) (Pose, error)
	Turn(context.Context, float64) error
	// Forward must release its key before returning, including cancellation.
	Forward(context.Context, time.Duration) error
	HoldForward(context.Context) error
	StopForward(context.Context) error
	Wait(context.Context, time.Duration) error
}

type Options struct {
	X, Y, Tolerance float64
	TurnSign        float64
	Pulse           time.Duration
	StuckTimeout    time.Duration
	SlowDistance    float64
	Now             func() time.Time
	Timeout         time.Duration
}

func Finite(v float64) bool                 { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func Normalize(v float64) float64           { return math.Mod(math.Mod(v, 360)+360, 360) }
func Delta(target, current float64) float64 { return Normalize(target-current+180) - 180 }
func (o Options) Validate() error {
	if !Finite(o.X) || !Finite(o.Y) || !Finite(o.Tolerance) || o.Tolerance <= 0 ||
		math.Abs(o.TurnSign) != 1 || !Finite(o.SlowDistance) || o.SlowDistance < 0 ||
		o.Pulse < 20*time.Millisecond || o.Pulse > 500*time.Millisecond || o.StuckTimeout < time.Second {
		return errors.New("invalid navigation coordinates or movement settings")
	}
	return nil
}

// MoveTo takes short forward steps, observing after each completed input. A fresh
// pose is also required at arrival so one noisy sample cannot claim completion.
func MoveTo(ctx context.Context, d Driver, o Options) (final Pose, remaining float64, resultErr error) {
	if err := o.Validate(); err != nil {
		return Pose{}, 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, d.StopForward(context.WithoutCancel(ctx))) }()
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.SlowDistance == 0 {
		o.SlowDistance = o.Tolerance * 4
	}
	var pose Pose
	var after time.Time
	best := math.Inf(1)
	progress := o.Now()
	started := progress
	turnProgress := progress
	bestTurn := math.Inf(1)
	turning := false
	arrivals := 0
	var previous Pose
	var lastPulse time.Duration
	speed := 0.0
	var lastStepAt time.Time
	heldStep := false
	for {
		if o.Timeout > 0 && o.Now().Sub(started) >= o.Timeout {
			return pose, distance(pose, o), context.DeadlineExceeded
		}
		if err := ctx.Err(); err != nil {
			return pose, distance(pose, o), err
		}
		p, err := d.Read(ctx, after)
		if err != nil {
			return pose, distance(pose, o), err
		}
		if !Finite(p.X) || !Finite(p.Y) || !Finite(p.Heading) || !Finite(p.AxisHeading) || math.Abs(p.AxisSign) != 1 || !p.Time.After(after) {
			return pose, distance(pose, o), ErrStale
		}
		pose, after = p, p.Time
		if lastPulse > 0 {
			if heldStep {
				lastPulse = o.Now().Sub(lastStepAt)
			}
			observed := math.Hypot(p.X-previous.X, p.Y-previous.Y) / lastPulse.Seconds()
			if observed > 0 {
				speed = math.Max(speed, observed)
			}
			lastPulse = 0
		}
		dist := distance(pose, o)
		if dist <= o.Tolerance {
			if err := d.StopForward(ctx); err != nil {
				return pose, dist, err
			}
			arrivals++
			if arrivals >= 2 {
				return pose, dist, nil
			}
			continue
		}
		arrivals = 0
		if dist < best-o.Tolerance*0.1 {
			best, progress = dist, o.Now()
		}
		heading := Normalize(p.AxisHeading + p.AxisSign*math.Atan2(o.Y-p.Y, o.X-p.X)*180/math.Pi)
		turn := Delta(heading, p.Heading)
		// Never walk while facing away. Limit each correction and observe again.
		if math.Abs(turn) > 5 {
			if math.Abs(turn) > 35 || dist <= o.SlowDistance {
				if err := d.StopForward(ctx); err != nil {
					return pose, dist, err
				}
			}
			if !turning {
				turning, bestTurn, turnProgress = true, math.Inf(1), o.Now()
			}
			if math.Abs(turn) < bestTurn-1 {
				bestTurn, turnProgress = math.Abs(turn), o.Now()
			}
			if o.Now().Sub(turnProgress) >= o.StuckTimeout {
				return pose, dist, ErrStuck
			}
			err = d.Turn(ctx, max(-45, min(45, turn))*o.TurnSign)
		} else {
			// Alignment is useful progress even though the position has not moved.
			// Start the translation watchdog only once we can walk toward the goal.
			if turning {
				turning, progress = false, o.Now()
			}
			if o.Now().Sub(progress) >= o.StuckTimeout {
				return pose, dist, ErrStuck
			}
			pulse := o.Pulse
			if dist <= o.SlowDistance {
				pulse = max(20*time.Millisecond, o.Pulse/4)
			}
			if speed > 0 {
				pulse = min(pulse, max(time.Millisecond, time.Duration((dist-o.Tolerance*0.5)/speed*0.6*float64(time.Second))))
			}
			previous, lastPulse = p, pulse
			lastStepAt, heldStep = o.Now(), dist > o.SlowDistance
			if dist > o.SlowDistance {
				err = d.HoldForward(ctx)
				if err == nil {
					err = d.Wait(ctx, pulse)
				}
			} else {
				err = d.StopForward(ctx)
				if err == nil {
					err = d.Forward(ctx, pulse)
				}
			}
		}
		if err != nil {
			return pose, dist, err
		}
		// Keep observing newer samples while forward input remains held.
	}
}

func distance(p Pose, o Options) float64 { return math.Hypot(o.X-p.X, o.Y-p.Y) }
