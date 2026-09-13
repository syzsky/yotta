package navigation

import (
	"context"
	"math"
	"time"
)

// Timed steering consumes an angle over a bounded interval, not an angular
// velocity. Legacy Driver implementations can continue to supply Turn alone.
// Streaming steering stays active between observations until replaced or stopped.
type streamingPathSteering interface {
	SetSteering(context.Context, float64) error
}

type timedPathSteering interface {
	Steer(context.Context, float64, time.Duration) error
}

type pathSteering struct {
	rate       float64
	correcting bool
}

func (s *pathSteering) reset() { s.rate = 0; s.correcting = false }
func (s *pathSteering) angularRate(errorDegrees, distance, speed float64, dt time.Duration, o FollowOptions) float64 {
	// Hysteresis prevents observation noise from repeatedly reversing commands.
	// The lookahead chord already includes lateral error relative to the path.
	if !s.correcting && math.Abs(errorDegrees) >= 7 {
		s.correcting = true
	}
	if s.correcting && math.Abs(errorDegrees) <= 2 {
		s.correcting = false
	}
	requested := 0.0
	if s.correcting {
		// Pure-pursuit curvature expressed in the observed heading frame. Convert
		// radians/second to degrees, then integrate exactly once for mouse input.
		requested = 2 * speed * math.Sin(errorDegrees*math.Pi/180) / math.Max(distance, o.TrackingTolerance) * 180 / math.Pi
	}
	requested = math.Max(-o.MaxAngularVelocity, math.Min(o.MaxAngularVelocity, requested))
	maxChange := o.MaxAngularAcceleration * dt.Seconds()
	s.rate += math.Max(-maxChange, math.Min(maxChange, requested-s.rate))
	return s.rate
}

func (s *pathSteering) angle(e, d, v float64, dt time.Duration, o FollowOptions) float64 {
	angle := s.angularRate(e, d, v, dt, o) * dt.Seconds()
	return math.Max(-o.MaxTurn, math.Min(o.MaxTurn, angle))
}
