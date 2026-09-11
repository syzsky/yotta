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
	X, Y, Heading float64
	Time          time.Time
}

type Driver interface {
	// Read must return a valid pose newer than after, or fail without sending input.
	Read(context.Context, time.Time) (Pose, error)
	Turn(context.Context, float64) error
	// Forward must release its key before returning, including cancellation.
	Forward(context.Context, time.Duration) error
}

type Options struct {
	X, Y, Tolerance float64
	// Heading of the positive X axis; Sign maps atan2(X,Y) to source headings.
	AxisHeading, AxisSign, TurnSign float64
	Pulse                           time.Duration
	StuckTimeout                    time.Duration
}

func Finite(v float64) bool                 { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func Normalize(v float64) float64           { return math.Mod(math.Mod(v, 360)+360, 360) }
func Delta(target, current float64) float64 { return Normalize(target-current+180) - 180 }
func (o Options) Validate() error {
	if !Finite(o.X) || !Finite(o.Y) || !Finite(o.Tolerance) || o.Tolerance <= 0 ||
		!Finite(o.AxisHeading) || math.Abs(o.AxisSign) != 1 || math.Abs(o.TurnSign) != 1 ||
		o.Pulse < 20*time.Millisecond || o.Pulse > 500*time.Millisecond || o.StuckTimeout < time.Second {
		return errors.New("invalid navigation coordinates or movement settings")
	}
	return nil
}

// MoveTo takes short forward steps, observing after each completed input. A fresh
// pose is also required at arrival so one noisy sample cannot claim completion.
func MoveTo(ctx context.Context, d Driver, o Options) (Pose, float64, error) {
	if err := o.Validate(); err != nil {
		return Pose{}, 0, err
	}
	var pose Pose
	var after time.Time
	best := math.Inf(1)
	progress := time.Now()
	turnProgress := progress
	bestTurn := math.Inf(1)
	turning := false
	arrivals := 0
	var previous Pose
	var lastPulse time.Duration
	speed := 0.0
	for {
		if err := ctx.Err(); err != nil {
			return pose, distance(pose, o), err
		}
		p, err := d.Read(ctx, after)
		if err != nil {
			return pose, distance(pose, o), err
		}
		if !Finite(p.X) || !Finite(p.Y) || !Finite(p.Heading) || !p.Time.After(after) {
			return pose, distance(pose, o), ErrStale
		}
		pose, after = p, p.Time
		if lastPulse > 0 {
			observed := math.Hypot(p.X-previous.X, p.Y-previous.Y) / lastPulse.Seconds()
			if observed > 0 {
				speed = math.Max(speed, observed)
			}
			lastPulse = 0
		}
		dist := distance(pose, o)
		if dist <= o.Tolerance {
			arrivals++
			if arrivals >= 2 {
				return pose, dist, nil
			}
			continue
		}
		arrivals = 0
		if dist < best-o.Tolerance*0.1 {
			best, progress = dist, time.Now()
		}
		heading := Normalize(o.AxisHeading + o.AxisSign*math.Atan2(o.Y-p.Y, o.X-p.X)*180/math.Pi)
		turn := Delta(heading, p.Heading)
		// Never walk while facing away. Limit each correction and observe again.
		if math.Abs(turn) > 5 {
			if !turning {
				turning, bestTurn, turnProgress = true, math.Inf(1), time.Now()
			}
			if math.Abs(turn) < bestTurn-1 {
				bestTurn, turnProgress = math.Abs(turn), time.Now()
			}
			if time.Since(turnProgress) >= o.StuckTimeout {
				return pose, dist, ErrStuck
			}
			err = d.Turn(ctx, max(-45, min(45, turn))*o.TurnSign)
		} else {
			// Alignment is useful progress even though the position has not moved.
			// Start the translation watchdog only once we can walk toward the goal.
			if turning {
				turning, progress = false, time.Now()
			}
			if time.Since(progress) >= o.StuckTimeout {
				return pose, dist, ErrStuck
			}
			pulse := o.Pulse
			if dist < o.Tolerance*4 {
				pulse = max(20*time.Millisecond, o.Pulse/4)
			}
			if speed > 0 {
				pulse = min(pulse, max(time.Millisecond, time.Duration((dist-o.Tolerance*0.5)/speed*0.6*float64(time.Second))))
			}
			previous, lastPulse = p, pulse
			err = d.Forward(ctx, pulse)
		}
		if err != nil {
			return pose, dist, err
		}
		// Do not act again on a sample produced during the previous action.
		after = time.Now()
	}
}

func distance(p Pose, o Options) float64 { return math.Hypot(o.X-p.X, o.Y-p.Y) }
