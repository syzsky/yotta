package navigation

import (
	"context"
	"math"
	"testing"
	"time"
)

type delayedPathSimulation struct {
	*steeringSimulation
	rate float64
}

func (s *delayedPathSimulation) Read(ctx context.Context, after time.Time) (Pose, error) {
	p, e := s.followSimulation.Read(ctx, after)
	p.SampleTime = s.now
	s.turnAngles = append(s.turnAngles, s.rate*s.observationDelay.Seconds())
	s.advance(s.observationDelay, s.rate*s.observationDelay.Seconds())
	return p, e
}
func (s *delayedPathSimulation) SetSteering(ctx context.Context, rate float64) error {
	s.rate = rate
	return ctx.Err()
}
func (s *delayedPathSimulation) Wait(ctx context.Context, dt time.Duration) error {
	s.turnAngles = append(s.turnAngles, s.rate*dt.Seconds())
	s.advance(dt, s.rate*dt.Seconds())
	return ctx.Err()
}
func (s *delayedPathSimulation) StopForward(ctx context.Context) error {
	s.rate = 0
	return s.followSimulation.StopForward(ctx)
}
func TestFollowDelayedWideS(t *testing.T) {
	for _, delay := range []time.Duration{0, 100 * time.Millisecond, 200 * time.Millisecond} {
		t.Run(delay.String(), func(t *testing.T) {
			points := make([]Waypoint, 121)
			for i := range points {
				x := float64(i) * 50
				points[i] = Waypoint{x, 250 * (1 - math.Cos(x*math.Pi/1000))}
			}
			s := &delayedPathSimulation{&steeringSimulation{followSimulation: newFollowSimulation(), observationDelay: delay}, 0}
			s.speed = 500
			o := s.options()
			o.Tolerance = 20
			o.StuckTimeout = 5 * time.Second
			o.Timeout = 30 * time.Second
			r, e := Follow(context.Background(), s, points, o)
			total, rev := steeringTravel(s.turnAngles)
			t.Logf("time=%v holds=%d turn=%.1f reverse=%d last=%d err=%v", s.now.Sub(time.Unix(100, 0)), s.holds, total, rev, r.Last, e)
			if e != nil || s.holds > 2 || total > 600 {
				t.Fail()
			}
		})
	}
}

func TestPathMatchReacquiresAdjacentSegmentWithoutSkippingGate(t *testing.T) {
	p, _ := newFollowPath([]Waypoint{{0, 0}, {20, 0}, {40, 10}, {60, 20}}, 75)
	c := FollowCursor{NextIndex: 1, Offset: 18}
	next, _ := p.match(c, Waypoint{32, 6}, 20, .5, 2)
	if next.NextIndex != 2 || next.Offset <= 0 {
		t.Fatalf("adjacent segment not acquired: %+v", next)
	}
	gate, _ := newFollowPath([]Waypoint{{0, 0}, {20, 0}, {20, 20}, {0, 20}}, 75)
	next, _ = gate.match(c, Waypoint{20, 10}, 30, .5, 2)
	if next.NextIndex != 1 {
		t.Fatalf("skipped unvisited sharp gate: %+v", next)
	}
	next, _ = p.match(c, Waypoint{200, 200}, 20, .5, 2)
	if next != c {
		t.Fatalf("off-route jump advanced progress: %+v", next)
	}
}

func TestFollowStreamingLargeInitialTurn(t *testing.T) {
	for _, delay := range []time.Duration{0, 100 * time.Millisecond, 200 * time.Millisecond} {
		t.Run(delay.String(), func(t *testing.T) {
			s := &delayedPathSimulation{&steeringSimulation{followSimulation: newFollowSimulation(), observationDelay: delay}, 0}
			s.speed = 500
			s.pose.Heading = 170
			o := s.options()
			o.Tolerance = 20
			o.StuckTimeout = 5 * time.Second
			r, err := Follow(context.Background(), s, []Waypoint{{0, 0}, {2000, 0}}, o)
			total, _ := steeringTravel(s.turnAngles)
			elapsed := s.now.Sub(time.Unix(100, 0))
			t.Logf("duration=%v holds=%d angle=%.1f", elapsed, s.holds, total)
			if err != nil || r.Last != 1 || s.holds > 2 || elapsed > 8*time.Second || total > 210 {
				t.Fatalf("slow or unstable large turn: %+v %v", r, err)
			}
		})
	}
}

func TestPathObservationUsesOnlyAcquiredTimeAndHeldMotion(t *testing.T) {
	now := time.Unix(100, 0)
	p := Pose{SampleTime: now.Add(-200 * time.Millisecond), AxisSign: 1}
	observer := pathObservation{}
	observer.turn(p.SampleTime, now, 90)
	predicted := observer.predict(p, now, 100, false, time.Time{})
	if math.Abs(predicted.Heading-90) > 0.001 || predicted.X != 0 || predicted.Y != 0 || p.Heading != 0 {
		t.Fatalf("invalid prediction %+v", predicted)
	}
	observer = pathObservation{}
	predicted = observer.predict(p, now, 100, true, now.Add(-50*time.Millisecond))
	if math.Abs(predicted.X-5) > 0.001 {
		t.Fatalf("motion before forward held: %+v", predicted)
	}
	for _, stamp := range []time.Time{{}, now.Add(time.Second), now.Add(-time.Second)} {
		p.SampleTime = stamp
		if got := observer.predict(p, now, 100, true, time.Time{}); got != p {
			t.Fatalf("extrapolated unknown/stale time: %+v", got)
		}
	}
}

type shiftedPathSource struct{ *delayedPathSimulation }

func (s *shiftedPathSource) Read(ctx context.Context, after time.Time) (Pose, error) {
	p, err := s.delayedPathSimulation.Read(ctx, after)
	p.SampleTime = p.SampleTime.Add(time.Minute)
	return p, err
}
func TestFollowSourceClockCanDifferFromPausedRunClock(t *testing.T) {
	s := &shiftedPathSource{&delayedPathSimulation{&steeringSimulation{followSimulation: newFollowSimulation(), observationDelay: 200 * time.Millisecond}, 0}}
	s.speed = 500
	s.pose.Heading = 170
	o := s.options()
	o.Tolerance = 20
	o.StuckTimeout = 5 * time.Second
	o.ObservationNow = func() time.Time { return s.now.Add(time.Minute) }
	r, err := Follow(context.Background(), s, []Waypoint{{0, 0}, {2000, 0}}, o)
	if err != nil || r.Last != 1 || s.holds != 1 || s.now.Sub(time.Unix(100, 0)) > 8*time.Second {
		t.Fatalf("source/run clock mismatch: %+v %v", r, err)
	}
}
