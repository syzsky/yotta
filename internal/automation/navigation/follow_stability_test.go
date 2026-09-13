package navigation

import (
	"context"
	"math"
	"testing"
	"time"
)

// Model noisy observations separately from the actual body state. Input and
// observation latency advance the body; no time or overshoot is clipped.
type steeringSimulation struct {
	*followSimulation
	observationDelay time.Duration
	noise            bool
	commands         []float64
	durations        []time.Duration
}

func (s *steeringSimulation) Read(ctx context.Context, after time.Time) (Pose, error) {
	s.advance(s.observationDelay, 0)
	pose, err := s.followSimulation.Read(ctx, after)
	if s.noise {
		pose.Y += 0.18 * math.Sin(float64(s.reads)*1.7)
		pose.Heading = Normalize(pose.Heading + 1.8*math.Sin(float64(s.reads)*1.1))
	}
	return pose, err
}
func (s *steeringSimulation) Steer(ctx context.Context, angle float64, duration time.Duration) error {
	s.commands = append(s.commands, angle)
	s.durations = append(s.durations, duration)
	s.turns++
	if s.held {
		s.movingTurns++
	}
	s.turnAngles = append(s.turnAngles, angle)
	s.advance(duration, angle*s.turnResponse)
	return s.action(ctx, "turn")
}
func steeringTravel(angles []float64) (total float64, reversals int) {
	last := 0.0
	for _, a := range angles {
		total += math.Abs(a)
		if math.Abs(a) < 0.1 {
			continue
		}
		if last*a < 0 {
			reversals++
		}
		last = a
	}
	return
}
func TestFollowNoisyStraightAvoidsCameraHunting(t *testing.T) {
	for _, dense := range []bool{false, true} {
		s := &steeringSimulation{followSimulation: newFollowSimulation(), observationDelay: 40 * time.Millisecond, noise: true}
		points := []Waypoint{{0, 0}, {100, 0}}
		if dense {
			points = make([]Waypoint, 501)
			for i := range points {
				points[i] = Waypoint{X: float64(i) / 5}
			}
		}
		r, err := Follow(context.Background(), s, points, s.options())
		total, reversals := steeringTravel(s.turnAngles)
		if err != nil || r.Last != len(points)-1 || s.held {
			t.Fatalf("dense=%v result=%+v err=%v turns=%.1f reverse=%d", dense, r, err, total, reversals)
		}
		maxDeviation := 0.0
		for _, p := range s.trace {
			maxDeviation = math.Max(maxDeviation, math.Abs(p.Y))
		}
		t.Logf("dense=%v turn=%.1f reversals=%d deviation=%.3f", dense, total, reversals, maxDeviation)
		if total > 25 || reversals > 6 || maxDeviation > 0.75 || s.holds != 1 {
			t.Fatalf("dense=%v unnecessary turn=%.1f reversals=%d deviation=%.3f holds=%d", dense, total, reversals, maxDeviation, s.holds)
		}
	}
}

func TestFollowTimedSteeringShapesAndDelay(t *testing.T) {
	circle := make([]Waypoint, 101)
	for i := range circle {
		angle := float64(i) * 2 * math.Pi / 100
		circle[i] = Waypoint{20 * math.Sin(angle), 20 * (1 - math.Cos(angle))}
	}
	for name, points := range map[string][]Waypoint{"corner": {{0, 0}, {20, 0}, {20, 20}}, "loop": {{0, 0}, {20, 0}, {20, 20}, {0, 20}, {0, 0}}, "circle": circle, "crossing": {{0, 0}, {20, 20}, {0, 20}, {20, 0}, {30, 0}}} {
		t.Run(name, func(t *testing.T) {
			s := &steeringSimulation{followSimulation: newFollowSimulation(), observationDelay: 80 * time.Millisecond}
			o := s.options()
			o.StuckTimeout = 5 * time.Second
			var crossed []int
			o.OnCrossed = func(i int) error { crossed = append(crossed, i); return nil }
			r, err := Follow(context.Background(), s, points, o)
			total, reversals := steeringTravel(s.turnAngles)
			if err != nil || r.Last != len(points)-1 || s.held || len(crossed) != len(points) {
				t.Fatalf("%s err=%v result=%+v turns=%.1f reversals=%d holds=%d", name, err, r, total, reversals, s.holds)
			}
			for i, n := range crossed {
				if i != n {
					t.Fatalf("out of order: %v", crossed)
				}
			}
			if name == "circle" && (total > 430 || reversals > 4 || s.holds > 2) {
				t.Fatalf("circle excessive steering %.1f reversals=%d holds=%d", total, reversals, s.holds)
			}
			for _, dt := range s.durations {
				if dt < 20*time.Millisecond || dt > 100*time.Millisecond || dt%time.Millisecond != 0 {
					t.Fatalf("invalid duration %v", dt)
				}
			}
		})
	}
}

func TestFollowProjectionCanCatchUpInsideRouteCorridor(t *testing.T) {
	p, err := newFollowPath([]Waypoint{{0, 0}, {20, 0}, {40, 0}}, 75)
	if err != nil {
		t.Fatal(err)
	}
	c := FollowCursor{NextIndex: 2, Offset: 2}
	next, _ := p.project(c, Waypoint{35, 0.2}, 4, 0.5, 1)
	if next.Offset != 6 {
		t.Fatalf("valid pose stranded behind budget-limited cursor: %+v", next)
	}
	next2, _ := p.project(next, Waypoint{35, 0.2}, 4, 0.5, 1)
	if next2.Offset != 10 {
		t.Fatalf("progress cannot catch up: %+v", next2)
	}
	outside, _ := p.project(c, Waypoint{35, 3}, 4, 0.5, 1)
	if outside != c {
		t.Fatalf("off-route pose silently advanced: %+v", outside)
	}
}

func TestFollowProjectionCatchUpPastIntermediateEndpoint(t *testing.T) {
	p, _ := newFollowPath([]Waypoint{{0, 0}, {30, 0}, {60, 0}}, 75)
	c, _ := p.project(FollowCursor{NextIndex: 1}, Waypoint{40, 0}, 16, 0.5, 2)
	if c.Offset != 16 || c.NextIndex != 1 {
		t.Fatalf("stranded on straight continuation: %+v", c)
	}
}

func TestPathSteeringBoundsVelocityAndAcceleration(t *testing.T) {
	o, err := newFollowSimulation().options().normalized()
	if err != nil {
		t.Fatal(err)
	}
	var controller pathSteering
	previous := 0.0
	for i := 0; i < 40; i++ {
		headingError := 35.0
		if i >= 20 {
			headingError = -35
		}
		dt := time.Duration(20+i%5*20) * time.Millisecond
		angle := controller.angle(headingError, 4, 12, dt, o)
		rate := angle / dt.Seconds()
		if math.Abs(rate) > o.MaxAngularVelocity+1e-9 || math.Abs(rate-previous) > o.MaxAngularAcceleration*dt.Seconds()+1e-9 {
			t.Fatalf("step%d previous %.3f next %.3f dt=%v", i, previous, rate, dt)
		}
		previous = rate
	}
	controller.reset()
	for _, a := range []float64{2, -3, 5, -6, 4, -5} {
		if controller.angle(a, 4, 12, 100*time.Millisecond, o) != 0 {
			t.Fatal("noise enabled steering")
		}
	}
}
func TestFollowTimedSteeringStillCorrectsRealDrift(t *testing.T) {
	s := &steeringSimulation{followSimulation: newFollowSimulation(), observationDelay: 80 * time.Millisecond}
	s.pose.Y = 0.2
	s.pose.Heading = 8
	r, err := Follow(context.Background(), s, []Waypoint{{0, 0}, {100, 0}}, s.options())
	if err != nil || r.Last != 1 || s.held || len(s.commands) == 0 {
		t.Fatalf("failed to correct actual drift result=%+v err=%v commands=%v", r, err, s.commands)
	}
	for _, p := range s.trace {
		if math.Abs(p.Y) > 1.5 {
			t.Fatalf("drift %.3f", p.Y)
		}
	}
}

func TestFollowTimedSteeringIndependentOfLoopSamplingDensity(t *testing.T) {
	base := []Waypoint{{0, 0}, {20, 0}, {20, 20}, {0, 20}, {0, 0}}
	dense := []Waypoint{base[0]}
	for i := 1; i < len(base); i++ {
		for j := 1; j <= 20; j++ {
			dense = append(dense, waypointLerp(base[i-1], base[i], float64(j)/20))
		}
	}
	var totals [2]float64
	var holds [2]int
	for i, points := range [][]Waypoint{base, dense} {
		s := &steeringSimulation{followSimulation: newFollowSimulation(), observationDelay: 80 * time.Millisecond}
		o := s.options()
		o.StuckTimeout = 5 * time.Second
		r, err := Follow(context.Background(), s, points, o)
		if err != nil || r.Last != len(points)-1 {
			t.Fatalf("density%d failed %+v %v", i, r, err)
		}
		t.Logf("density%d stops=%v final=%+v", i, s.stopLocations, r.Pose)
		totals[i], _ = steeringTravel(s.turnAngles)
		holds[i] = s.holds
	}
	if math.Abs(totals[0]-totals[1]) > 5 || holds[0] != holds[1] {
		t.Fatalf("density changed movement turns=%v holds=%v", totals, holds)
	}
	t.Logf("sparse/dense total turns=%v holds=%v", totals, holds)
}
