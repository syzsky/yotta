package navigation

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestFollowTimeoutIsPerSequentialPoint(t *testing.T) {
	s := newFollowSimulation()
	points := []Waypoint{{0, 0}, {10, 0}, {20, 0}, {30, 0}, {40, 0}, {50, 0}}
	o := s.options()
	o.Timeout = 1500 * time.Millisecond
	r, err := Follow(context.Background(), s, points, o)
	if err != nil || r.Last != len(points)-1 || s.now.Sub(time.Unix(100, 0)) <= o.Timeout {
		t.Fatalf("route incorrectly has a total timeout: %+v %v", r, err)
	}
	// Motion alone must not renew the deadline of a single long segment.
	s = newFollowSimulation()
	o = s.options()
	o.Timeout = 1500 * time.Millisecond
	r, err = Follow(context.Background(), s, []Waypoint{{0, 0}, {50, 0}}, o)
	if !errors.Is(err, context.DeadlineExceeded) || r.Last != 0 || s.held {
		t.Fatalf("per-point deadline lost: %+v %v", r, err)
	}
}

func TestFollowProgressEveryFreshCycleAndConfirmedFinal(t *testing.T) {
	s := newFollowSimulation()
	o := s.options()
	var reports []Progress
	o.OnProgress = func(p Progress) error {
		if len(reports) > 0 && !p.Pose.Time.After(reports[len(reports)-1].Pose.Time) {
			t.Fatal("replayed progress")
		}
		if p.Last == 1 && (s.held || s.stops != 1) {
			t.Fatal("endpoint reported before release")
		}
		reports = append(reports, p)
		return nil
	}
	r, err := Follow(context.Background(), s, []Waypoint{{0, 0}, {20, 0}}, o)
	if err != nil || len(reports) != s.reads || len(reports) < 10 || reports[len(reports)-1] != r {
		t.Fatalf("reports=%d reads=%d result=%+v err=%v", len(reports), s.reads, r, err)
	}
}

func TestFollowCallbackErrorsPreserveResumeAndCrossingOrder(t *testing.T) {
	callbackErr := errors.New("periodic action failed")
	for _, mode := range []string{"progress", "crossed", "final-progress", "final-crossed"} {
		t.Run(mode, func(t *testing.T) {
			s := newFollowSimulation()
			o := s.options()
			points := []Waypoint{{0, 0}, {10, 0}, {20, 0}, {20, 20}, {0, 20}, {0, 0}}
			var crossed []int
			o.OnCrossed = func(i int) error {
				crossed = append(crossed, i)
				if (mode == "crossed" && i == 1) || (mode == "final-crossed" && i == len(points)-1) {
					return callbackErr
				}
				return nil
			}
			o.OnProgress = func(p Progress) error {
				if (mode == "progress" && p.Cursor.NextIndex == 2 && p.Cursor.Offset > 2) || (mode == "final-progress" && p.Last == len(points)-1) {
					return callbackErr
				}
				return nil
			}
			r, err := Follow(context.Background(), s, points, o)
			if !errors.Is(err, callbackErr) || s.held || r.Last < 1 {
				t.Fatalf("callback failure: %+v %v", r, err)
			}
			if mode == "progress" && r.Cursor.Offset <= 2 {
				t.Fatal("lost arc cursor")
			}
			o = s.options()
			o.Resume = &r.Cursor
			o.OnCrossed = func(i int) error { crossed = append(crossed, i); return nil }
			r, err = Follow(context.Background(), s, points, o)
			if err != nil || r.Last != len(points)-1 || !reflect.DeepEqual(crossed, []int{0, 1, 2, 3, 4, 5}) {
				t.Fatalf("resumed with duplicate/missing points: %+v %v crossed=%v", r, err, crossed)
			}
		})
	}
}

func TestFollowCallbackFailureInDenseCrossingBatch(t *testing.T) {
	s := newFollowSimulation()
	o := s.options()
	points := make([]Waypoint, 101)
	for i := range points {
		points[i].X = float64(i) / 10
	}
	callbackErr := errors.New("marker failed")
	var crossed []int
	o.OnCrossed = func(i int) error {
		crossed = append(crossed, i)
		if i == 2 {
			return callbackErr
		}
		return nil
	}
	r, err := Follow(context.Background(), s, points, o)
	if !errors.Is(err, callbackErr) || r.Last != 2 || !reflect.DeepEqual(crossed, []int{0, 1, 2}) {
		t.Fatalf("unreported points committed: %+v %v %v", r, err, crossed)
	}
	o = s.options()
	o.Resume = &r.Cursor
	o.OnCrossed = func(i int) error { crossed = append(crossed, i); return nil }
	r, err = Follow(context.Background(), s, points, o)
	if err != nil || r.Last != 100 || len(crossed) != 101 {
		t.Fatalf("resume dense: %+v %v crossed=%v", r, err, crossed)
	}
	for i, v := range crossed {
		if i != v {
			t.Fatalf("duplicate marker %v", crossed)
		}
	}
}

func TestFollowAxisMappingAndInitialAlignment(t *testing.T) {
	for _, sign := range []float64{-1, 1} {
		s := newFollowSimulation()
		s.pose.AxisSign = sign
		s.pose.AxisHeading = 90.787
		s.pose.Heading = 270.787
		runFollow(t, s, []Waypoint{{0, 0}, {20, 0}, {20, 20}})
		if s.turns == 0 {
			t.Fatal("did not align")
		}
	}
}

func TestFollowInverseTurnCalibrationRecognizesHeadingResponse(t *testing.T) {
	s := newFollowSimulation()
	s.turnResponse = -1
	s.pose.Heading = 180
	o := s.options()
	o.TurnSign = -1
	r, err := Follow(context.Background(), s, []Waypoint{{0, 0}, {20, 0}}, o)
	if err != nil || r.Last != 1 {
		t.Fatalf("inverted input calibration: %+v %v", r, err)
	}
}

func TestFollowDenseRoundLoopDoesNotStopAtSamples(t *testing.T) {
	s := newFollowSimulation()
	points := make([]Waypoint, 201)
	for i := range points {
		a := float64(i) / 200 * 2 * math.Pi
		points[i] = Waypoint{20 * math.Sin(a), 20 - 20*math.Cos(a)}
	}
	points[len(points)-1] = points[0]
	runFollow(t, s, points)
	if s.holds != 1 || s.stops != 1 || s.movingTurns == 0 {
		t.Fatalf("round loop stopping: holds=%d stops=%v movingTurns=%d", s.holds, s.stopLocations, s.movingTurns)
	}
}

func TestFollowDynamicLookaheadAndCornerChordLimit(t *testing.T) {
	p, err := newFollowPath([]Waypoint{{0, 0}, {10, 0}, {20, 10}, {30, 10}}, 100)
	if err != nil {
		t.Fatal(err)
	}
	c := FollowCursor{NextIndex: 1, Offset: 8}
	short := p.aim(c, 1, 0.5)
	long := p.aim(c, 20, 0.5)
	if waypointDistance(p.location(c), long) <= waypointDistance(p.location(c), short) {
		t.Fatal("lookahead did not grow")
	}
	if pointChordDistance(Waypoint{10, 0}, p.location(c), long) > 0.50001 {
		t.Fatalf("cut corner: %+v", long)
	}
	sharp, _ := newFollowPath([]Waypoint{{0, 0}, {10, 0}, {10, 10}}, 75)
	if got := sharp.aim(c, 100, 0.5); got != (Waypoint{10, 0}) {
		t.Fatalf("looked past sharp gate: %+v", got)
	}
}

func TestFollowInvalidTuning(t *testing.T) {
	for _, change := range []func(*FollowOptions){
		func(o *FollowOptions) { o.MaxLookahead = math.NaN() },
		func(o *FollowOptions) { o.MinLookahead = -1 },
		func(o *FollowOptions) { o.MinLookahead = 100; o.MaxLookahead = 1 },
		func(o *FollowOptions) { o.CornerDeviation = -1 },
		func(o *FollowOptions) { o.MaxTurn = 46 },
		func(o *FollowOptions) { o.SharpAngle = 5 },
		func(o *FollowOptions) { o.Timeout = -1 },
	} {
		s := newFollowSimulation()
		o := s.options()
		change(&o)
		if _, err := Follow(context.Background(), s, []Waypoint{{0, 0}}, o); err == nil || s.reads != 0 {
			t.Fatal("invalid tuning accepted")
		}
	}
}

// endpointOvershootFixture delays one observation while the simulated input
// remains held. It advances virtual time and integrates real motion; it neither
// teleports the pose nor depends on scheduling/CPU load. Turn retains the base
// simulation's 150ms rotation plus 50ms settling interval.
type endpointOvershootFixture struct {
	*followSimulation
	injected, injectedWhileHeld     bool
	beforeOvershoot, afterOvershoot float64
	turnWithoutStop                 bool
	totalTurn                       float64
	returnHeading                   float64
	returnTraceStart                int
	stopTimes                       []time.Time
	stoppedRead                     Pose
}

func (d *endpointOvershootFixture) Read(ctx context.Context, after time.Time) (Pose, error) {
	if !d.injected && d.held && d.pose.X >= 18 {
		d.injected, d.injectedWhileHeld = true, d.held
		d.beforeOvershoot = d.pose.X
		// Deliberately pass the endpoint's 0.5-unit tolerance, but keep this
		// observation inside the existing local projection corridor.
		d.advance(time.Duration((20.75-d.pose.X)/d.speed*float64(time.Second)), 0)
		d.afterOvershoot = d.pose.X
	}
	p, err := d.followSimulation.Read(ctx, after)
	if err == nil && !d.held && d.stops == 2 {
		d.stoppedRead = p
	}
	return p, err
}

func (d *endpointOvershootFixture) StopForward(ctx context.Context) error {
	if d.held {
		d.stopTimes = append(d.stopTimes, d.now)
	}
	return d.followSimulation.StopForward(ctx)
}

func (d *endpointOvershootFixture) Turn(ctx context.Context, angle float64) error {
	if d.held || d.stops != 1 {
		d.turnWithoutStop = true
	}
	d.totalTurn += math.Abs(angle)
	return d.followSimulation.Turn(ctx, angle)
}

func (d *endpointOvershootFixture) HoldForward(ctx context.Context) error {
	if !d.held && d.holds == 1 {
		d.returnHeading = d.pose.Heading
		d.returnTraceStart = len(d.trace)
	}
	return d.followSimulation.HoldForward(ctx)
}

func TestFollowEndpointOvershootStopsBeforeReturnTurnAndConverges(t *testing.T) {
	d := &endpointOvershootFixture{followSimulation: newFollowSimulation()}
	d.pose.AxisHeading, d.pose.Heading = 90, 90
	o := d.options()
	var crossed []int
	o.OnCrossed = func(index int) error {
		crossed = append(crossed, index)
		if index == 1 {
			if d.held || d.stops != 2 || !d.stoppedRead.Time.After(d.stopTimes[1]) {
				t.Fatal("endpoint was reported before a fresh stopped observation")
			}
		}
		return nil
	}
	r, err := Follow(context.Background(), d, []Waypoint{{0, 0}, {20, 0}}, o)
	if err != nil {
		t.Fatalf("overshoot return failed: result=%+v err=%v", r, err)
	}
	if !d.injectedWhileHeld || d.beforeOvershoot >= 20-o.Tolerance || d.afterOvershoot <= 20+o.Tolerance {
		t.Fatalf("fixture did not overshoot while held: before=%v after=%v held=%v", d.beforeOvershoot, d.afterOvershoot, d.injectedWhileHeld)
	}
	if d.turnWithoutStop || d.turns == 0 || d.movingTurns != 0 || math.Abs(d.totalTurn-180) > 1e-6 {
		t.Fatalf("return must be one stopped half-turn: unstopped=%v turns=%d moving=%d total=%v", d.turnWithoutStop, d.turns, d.movingTurns, d.totalTurn)
	}
	if d.holds != 2 || d.stops != 2 || math.Abs(Delta(270, d.returnHeading)) > 3 || d.returnTraceStart == 0 {
		t.Fatalf("expected one outward hold and one negative-X return: holds=%d stops=%d return heading=%v", d.holds, d.stops, d.returnHeading)
	}
	for j := d.returnTraceStart; j < len(d.trace); j++ {
		if d.trace[j].X > d.trace[j-1].X+1e-9 || d.trace[j].X < 20-o.Tolerance {
			t.Fatalf("return oscillated or crossed the opposite arrival boundary: previous=%+v next=%+v", d.trace[j-1], d.trace[j])
		}
	}
	if d.stopLocations[0].X <= 20+o.Tolerance || d.stopLocations[1].X >= d.stopLocations[0].X ||
		r.Distance > o.Tolerance || r.Pose != d.stoppedRead || r.Last != 1 || r.Cursor.NextIndex != 2 || d.held || !reflect.DeepEqual(crossed, []int{0, 1}) {
		t.Fatalf("return did not converge to fresh stopped arrival: result=%+v stops=%v crossed=%v", r, d.stopLocations, crossed)
	}
}
