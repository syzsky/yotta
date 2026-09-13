package navigation

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"
)

// followSimulation integrates both heading and translation during a real,
// nonzero Turn duration. All time is virtual; no adapter or input is opened.
type followSimulation struct {
	pose                                    Pose
	now                                     time.Time
	held                                    bool
	holds, stops, turns, movingTurns, reads int
	speed                                   float64
	turnTime                                time.Duration
	turnResponse                            float64
	staticHeading, staticPosition           bool
	trace                                   []Waypoint
	stopLocations                           []Waypoint
	turnAngles                              []float64
	readHook                                func(*followSimulation) error
	actionHook                              func(string) error
	stopError                               error
	cleanupCanceled                         bool
}

func newFollowSimulation() *followSimulation {
	return &followSimulation{pose: Pose{AxisSign: 1}, now: time.Unix(100, 0), speed: 10, turnTime: 150 * time.Millisecond, turnResponse: 1}
}
func (s *followSimulation) advance(duration time.Duration, angle float64) {
	steps := max(1, int(math.Ceil(duration.Seconds()/0.001)))
	dt := duration.Seconds() / float64(steps)
	for i := 0; i < steps; i++ {
		if !s.staticHeading {
			s.pose.Heading = Normalize(s.pose.Heading + angle/float64(steps)/2)
		}
		if s.held && !s.staticPosition {
			radians := (s.pose.Heading - s.pose.AxisHeading) / s.pose.AxisSign * math.Pi / 180
			s.pose.X += math.Cos(radians) * s.speed * dt
			s.pose.Y += math.Sin(radians) * s.speed * dt
		}
		if !s.staticHeading {
			s.pose.Heading = Normalize(s.pose.Heading + angle/float64(steps)/2)
		}
		s.trace = append(s.trace, Waypoint{s.pose.X, s.pose.Y})
	}
	s.now = s.now.Add(duration)
}
func (s *followSimulation) action(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.actionHook != nil {
		return s.actionHook(name)
	}
	return nil
}
func (s *followSimulation) Read(ctx context.Context, after time.Time) (Pose, error) {
	s.reads++
	s.advance(time.Millisecond, 0)
	s.pose.Time = s.now
	if s.readHook != nil {
		if err := s.readHook(s); err != nil {
			return Pose{}, err
		}
	}
	return s.pose, ctx.Err()
}
func (s *followSimulation) HoldForward(ctx context.Context) error {
	if !s.held {
		s.holds++
		s.held = true
	}
	return s.action(ctx, "hold")
}
func (s *followSimulation) StopForward(ctx context.Context) error {
	if ctx.Err() != nil {
		s.cleanupCanceled = true
		return ctx.Err()
	}
	if s.held {
		s.stops++
		s.stopLocations = append(s.stopLocations, Waypoint{s.pose.X, s.pose.Y})
		s.held = false
	}
	return s.stopError
}
func (s *followSimulation) Wait(ctx context.Context, d time.Duration) error {
	s.advance(d, 0)
	return s.action(ctx, "wait")
}
func (s *followSimulation) Forward(context.Context, time.Duration) error {
	panic("Follow must hold forward, never pulse it")
}
func (s *followSimulation) Turn(ctx context.Context, angle float64) error {
	s.turns++
	if s.held {
		s.movingTurns++
	}
	s.turnAngles = append(s.turnAngles, angle)
	s.advance(s.turnTime, angle*s.turnResponse)
	s.advance(50*time.Millisecond, 0)
	return s.action(ctx, "turn")
}
func (s *followSimulation) options() FollowOptions {
	return FollowOptions{Options: Options{Tolerance: 0.5, TurnSign: 1, Pulse: 100 * time.Millisecond, StuckTimeout: 2 * time.Second, Timeout: 60 * time.Second, Now: func() time.Time { return s.now }}}
}
func runFollow(t *testing.T, s *followSimulation, points []Waypoint) Progress {
	t.Helper()
	o := s.options()
	var crossed []int
	o.OnCrossed = func(i int) error { crossed = append(crossed, i); return nil }
	r, err := Follow(context.Background(), s, points, o)
	if err != nil {
		t.Fatalf("Follow: %v result=%+v sim pose=%+v holds/stops=%d/%d turns=%d time=%v", err, r, s.pose, s.holds, s.stops, s.turns, s.now)
	}
	if s.held || r.Last != len(points)-1 || r.Cursor.NextIndex != len(points) || r.Distance > o.Tolerance {
		t.Fatalf("incomplete/release: %+v held=%v", r, s.held)
	}
	want := make([]int, len(points))
	for i := range want {
		want[i] = i
	}
	if !reflect.DeepEqual(crossed, want) {
		t.Fatalf("crossed=%v want=%v", crossed, want)
	}
	return r
}

func TestFollowDenseStraightHasSameSingleHoldAsSparse(t *testing.T) {
	dense := make([]Waypoint, 401)
	for i := range dense {
		dense[i] = Waypoint{X: float64(i) / 10}
	}
	a, b := newFollowSimulation(), newFollowSimulation()
	runFollow(t, a, []Waypoint{{0, 0}, {40, 0}})
	runFollow(t, b, dense)
	if a.holds != 1 || b.holds != 1 || a.stops != 1 || b.stops != 1 || a.turns != 0 || b.turns != 0 {
		t.Fatalf("sparse=%d/%d dense=%d/%d turns=%d/%d", a.holds, a.stops, b.holds, b.stops, a.turns, b.turns)
	}
}

func TestFollowClosedLoopAndLTurn(t *testing.T) {
	for _, points := range [][]Waypoint{
		{{0, 0}, {20, 0}, {20, 20}},
		{{0, 0}, {20, 0}, {20, 20}, {0, 20}, {0, 0}},
		{{0, 0}, {20, 0}, {20, 20}, {0, 20}, {0.1, 0.1}},
	} {
		s := newFollowSimulation()
		runFollow(t, s, points)
		for _, corner := range points[1 : len(points)-1] {
			nearest := math.Inf(1)
			for _, p := range s.trace {
				nearest = min(nearest, waypointDistance(p, corner))
			}
			if nearest > 0.5 {
				t.Fatalf("skipped corner %+v: %v", corner, nearest)
			}
		}
		if s.holds > len(points)-1 {
			t.Fatalf("excessive stopping: %d path=%v stops=%v", s.holds, points, s.stopLocations)
		}
	}
}

func TestFollowSmoothTurnsRemainHeldWithBoundedDeviation(t *testing.T) {
	s := newFollowSimulation()
	points := []Waypoint{{0, 0}, {15, 0}, {30, 6}, {45, 18}, {60, 30}}
	o := s.options()
	r, err := Follow(context.Background(), s, points, o)
	if err != nil || s.movingTurns == 0 || s.holds != 1 || s.stops != 1 {
		t.Fatalf("err=%v result=%+v holds=%d stops=%d moving turns=%d", err, r, s.holds, s.stops, s.movingTurns)
	}
	for _, a := range s.turnAngles {
		if math.Abs(a) > 12 {
			t.Fatalf("unbounded turn %v", a)
		}
	}
	for _, p := range s.trace {
		nearest := math.Inf(1)
		for i := 1; i < len(points); i++ {
			nearest = min(nearest, pointChordDistance(p, points[i-1], points[i]))
		}
		if nearest > 1 {
			t.Fatalf("corner deviation too large at %+v: %v", p, nearest)
		}
	}
}

func TestFollowSelfIntersectionKeepsOrder(t *testing.T) {
	s := newFollowSimulation()
	points := []Waypoint{{0, 0}, {15, 15}, {0, 15}, {15, 0}, {0, 0}}
	visited := make([]Waypoint, 0, len(points))
	o := s.options()
	o.OnCrossed = func(i int) error { visited = append(visited, Waypoint{s.pose.X, s.pose.Y}); return nil }
	r, err := Follow(context.Background(), s, points, o)
	if err != nil || r.Last != len(points)-1 {
		t.Fatalf("%+v %v", r, err)
	}
	for i, p := range visited {
		if waypointDistance(p, points[i]) > 0.6 {
			t.Fatalf("index %d crossed at %+v", i, p)
		}
	}
}

func TestFollowFailureAlwaysReleases(t *testing.T) {
	adapterErr := errors.New("adapter failure")
	for _, mode := range []string{"cancel-wait", "cancel-turn", "cancel-read", "stale", "replay", "invalid", "read", "hold", "turn", "wait", "timeout", "static-heading", "static-position", "moving-static-heading"} {
		t.Run(mode, func(t *testing.T) {
			s := newFollowSimulation()
			o := s.options()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := adapterErr
			switch mode {
			case "static-heading":
				s.staticHeading = true
				s.pose.Heading = 90
				want = ErrTurnUnresponsive
			case "moving-static-heading":
				s.readHook = func(s *followSimulation) error {
					if s.pose.X > 8 {
						s.staticHeading = true
					}
					return nil
				}
				want = ErrTurnUnresponsive
			case "static-position":
				s.staticPosition = true
				want = ErrNoPathProgress
			case "timeout":
				o.Timeout = 100 * time.Millisecond
				want = context.DeadlineExceeded
			case "stale", "replay", "invalid", "read", "cancel-read":
				s.readHook = func(s *followSimulation) error {
					if s.reads < 3 {
						return nil
					}
					switch mode {
					case "stale":
						return ErrStale
					case "replay":
						s.pose.Time = time.Unix(100, 0)
					case "invalid":
						s.pose.X = math.NaN()
					case "read":
						return adapterErr
					case "cancel-read":
						cancel()
					}
					return nil
				}
				if mode == "cancel-read" {
					want = context.Canceled
				} else if mode != "read" {
					want = ErrStale
				}
			default:
				if mode == "turn" || mode == "cancel-turn" {
					s.pose.Heading = 90
				}
				s.actionHook = func(action string) error {
					if mode == "cancel-"+action {
						cancel()
						return ctx.Err()
					}
					if mode == action {
						return adapterErr
					}
					return nil
				}
				if mode == "cancel-wait" || mode == "cancel-turn" {
					want = context.Canceled
				}
			}
			_, err := Follow(ctx, s, []Waypoint{{0, 0}, {10, 0}, {25, 6}}, o)
			if !errors.Is(err, want) || s.held || s.cleanupCanceled {
				t.Fatalf("err=%v want=%v held=%v canceled cleanup=%v", err, want, s.held, s.cleanupCanceled)
			}
			if (want == ErrTurnUnresponsive || want == ErrNoPathProgress) && !errors.Is(err, ErrStuck) {
				t.Fatal("lost ErrStuck compatibility")
			}
			if s.now.Sub(time.Unix(100, 0)) > 10*time.Second {
				t.Fatal("watchdog did not fail promptly")
			}
		})
	}
}

func TestFollowFinalArrivalRequiresFreshStoppedPose(t *testing.T) {
	s := newFollowSimulation()
	s.readHook = func(s *followSimulation) error {
		if s.stops > 0 {
			return ErrStale
		}
		return nil
	}
	var crossed []int
	o := s.options()
	o.OnCrossed = func(i int) error { crossed = append(crossed, i); return nil }
	r, err := Follow(context.Background(), s, []Waypoint{{0, 0}, {10, 0}}, o)
	if !errors.Is(err, ErrStale) || r.Last != 0 || !reflect.DeepEqual(crossed, []int{0}) || s.held {
		t.Fatalf("premature arrival: %+v %v crossed=%v", r, err, crossed)
	}
}

func TestFollowResumeDoesNotReplayPointsOrChooseNearEnd(t *testing.T) {
	s := newFollowSimulation()
	points := []Waypoint{{0, 0}, {10, 0}, {20, 0}, {20, 20}, {0, 20}, {0, 0}}
	ctx, cancel := context.WithCancel(context.Background())
	o := s.options()
	o.OnCrossed = func(i int) error {
		if i == 1 {
			cancel()
		}
		return nil
	}
	r, err := Follow(ctx, s, points, o)
	if !errors.Is(err, context.Canceled) || r.Last != 1 || s.held {
		t.Fatalf("%+v %v", r, err)
	}
	o = s.options()
	o.Resume = &r.Cursor
	var crossed []int
	o.OnCrossed = func(i int) error { crossed = append(crossed, i); return nil }
	r, err = Follow(context.Background(), s, points, o)
	if err != nil || r.Last != 5 || !reflect.DeepEqual(crossed, []int{2, 3, 4, 5}) {
		t.Fatalf("%+v %v %v", r, err, crossed)
	}
}

func TestFollowProjectionCannotAdvanceStationaryOrJumpBranches(t *testing.T) {
	p, err := newFollowPath([]Waypoint{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}, 75)
	if err != nil {
		t.Fatal(err)
	}
	c := FollowCursor{NextIndex: 1}
	for i := 0; i < 100; i++ {
		c, _ = p.project(c, Waypoint{0, 0}, 0, 0.5, 1)
	}
	if c.NextIndex != 1 || c.Offset != 0 {
		t.Fatalf("stationary progress %+v", c)
	}
	c, _ = p.project(c, Waypoint{0, 0}, 100, 0.5, 1)
	if c.NextIndex != 1 {
		t.Fatalf("jumped to end %+v", c)
	}
	c, _ = p.project(c, Waypoint{10, 10}, 100, 0.5, 1)
	if c.NextIndex != 1 {
		t.Fatalf("skipped L %+v", c)
	}
}

func TestFollowDuplicateAndSinglePoints(t *testing.T) {
	for _, points := range [][]Waypoint{{{0, 0}}, {{0, 0}, {0, 0}, {0, 0}}, {{0, 0}, {10, 0}, {10, 0}, {10, 10}}, {{0, 0}, {10, 0}, {10, 0}}} {
		runFollow(t, newFollowSimulation(), points)
	}
}

func TestFollowInvalidInputsAndCleanupError(t *testing.T) {
	for _, points := range [][]Waypoint{nil, {{math.NaN(), 0}}, {{0, 0}, {math.Inf(1), 0}}, {{-math.MaxFloat64, 0}, {math.MaxFloat64, 0}}} {
		s := newFollowSimulation()
		_, err := Follow(context.Background(), s, points, s.options())
		if err == nil || s.holds != 0 || s.turns != 0 {
			t.Fatal("invalid path sent input")
		}
	}
	for _, cursor := range []FollowCursor{{-1, 0}, {3, 0}, {0, 1}, {1, -1}, {1, 11}, {1, math.NaN()}, {2, 1}} {
		s := newFollowSimulation()
		o := s.options()
		o.Resume = &cursor
		if _, err := Follow(context.Background(), s, []Waypoint{{0, 0}, {10, 0}}, o); err == nil {
			t.Fatalf("accepted %+v", cursor)
		}
	}
	s := newFollowSimulation()
	s.stopError = errors.New("release failure")
	_, err := Follow(context.Background(), s, []Waypoint{{0, 0}}, s.options())
	if !errors.Is(err, s.stopError) {
		t.Fatal("lost release failure")
	}
}
