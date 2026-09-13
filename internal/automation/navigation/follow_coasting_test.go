package navigation

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"
)

type coastingPathSimulation struct {
	*delayedPathSimulation
	vx, vy float64
	coast  float64
}

func (s *coastingPathSimulation) move(dt time.Duration) {
	steps := max(1, int(math.Ceil(dt.Seconds()/0.001)))
	tick := dt.Seconds() / float64(steps)
	for n := 0; n < steps; n++ {
		s.pose.Heading = Normalize(s.pose.Heading + s.rate*tick)
		x, y := 0.0, 0.0
		if s.held {
			r := (s.pose.Heading - s.pose.AxisHeading) / s.pose.AxisSign * math.Pi / 180
			x = s.speed * math.Cos(r)
			y = s.speed * math.Sin(r)
		}
		weight := 1 - math.Exp(-tick/s.coast)
		s.vx += (x - s.vx) * weight
		s.vy += (y - s.vy) * weight
		s.pose.X += s.vx * tick
		s.pose.Y += s.vy * tick
	}
	s.turnAngles = append(s.turnAngles, s.rate*dt.Seconds())
	s.now = s.now.Add(dt)
}
func (s *coastingPathSimulation) Read(ctx context.Context, after time.Time) (Pose, error) {
	s.move(time.Millisecond)
	s.pose.Time = s.now
	p := s.pose
	p.SampleTime = s.now
	s.move(s.observationDelay)
	return p, ctx.Err()
}
func (s *coastingPathSimulation) Wait(ctx context.Context, dt time.Duration) error {
	s.move(dt)
	return ctx.Err()
}
func TestFollowEndpointWithCoastingSettlesWithoutRepeatedReturnTurns(t *testing.T) {
	for _, points := range [][]Waypoint{
		{{0, 0}, {200, 0}},
		{{0, 0}, {20, 0}, {40, 0}, {60, 0}, {80, 0}, {100, 0}, {120, 0}, {140, 0}, {160, 0}, {180, 0}, {200, 0}},
	} {
		for _, coast := range []float64{0.12, 0.25, 0.4} {
			for _, delay := range []time.Duration{0, 100 * time.Millisecond, 200 * time.Millisecond} {
				t.Run(fmt.Sprintf("points=%d/coast=%.2f/delay=%v", len(points), coast, delay), func(t *testing.T) {
					s := &coastingPathSimulation{delayedPathSimulation: &delayedPathSimulation{&steeringSimulation{followSimulation: newFollowSimulation(), observationDelay: delay}, 0}}
					s.coast = coast
					s.pose.Y = 15
					s.speed = 500
					o := s.options()
					o.Tolerance = 20
					o.Timeout = 20 * time.Second
					o.StuckTimeout = 5 * time.Second
					r, e := Follow(context.Background(), s, points, o)
					travel, _ := steeringTravel(s.turnAngles)
					t.Logf("error=%v last=%d x=%.2f y=%.2f turn=%.1f holds=%d elapsed=%v", e, r.Last, s.pose.X, s.pose.Y, travel, s.holds, s.now.Sub(time.Unix(100, 0)))
					if e != nil || travel > 360 || s.holds > 6 {
						t.Fatal("endpoint keeps reversing with residual movement")
					}
					// A completed run must remain within tolerance after residual motion settles.
					s.move(time.Second)
					if math.Hypot(s.pose.X-200, s.pose.Y) > o.Tolerance {
						t.Fatalf("reported arrival while still coasting: x=%.2f y=%.2f", s.pose.X, s.pose.Y)
					}
				})
			}
		}
	}
}
