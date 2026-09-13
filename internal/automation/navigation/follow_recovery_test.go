package navigation

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestFollowRoundedBendDoesNotOrbit(t *testing.T) {
	for _, radius := range []float64{200, 400, 800} {
		points := []Waypoint{{0, 0}, {400, 0}, {800, 0}}
		for i := 1; i <= 9; i++ {
			a := float64(i) * 5 * math.Pi / 180
			points = append(points, Waypoint{800 + radius*math.Sin(a), radius * (1 - math.Cos(a))})
		}
		end := points[len(points)-1]
		points = append(points, Waypoint{end.X + 800/math.Sqrt2, end.Y + 800/math.Sqrt2})
		s := &coastingPathSimulation{delayedPathSimulation: &delayedPathSimulation{&steeringSimulation{followSimulation: newFollowSimulation(), observationDelay: 100 * time.Millisecond}, 0}, coast: 0.25}
		s.speed = 500
		o := s.options()
		o.Tolerance = 20
		o.StuckTimeout = 5 * time.Second
		r, e := Follow(context.Background(), s, points, o)
		turn, _ := steeringTravel(s.turnAngles)
		t.Logf("radius=%.0f last=%d turn=%.1f holds=%d error=%v", radius, r.Last, turn, s.holds, e)
		if e != nil || turn > 450 {
			t.Errorf("orbit on rounded bend")
		}
	}
}
