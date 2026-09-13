package navigation

import (
	"math"
	"reflect"
	"testing"
)

func checkPreparedFollow(t *testing.T, points []Waypoint, protected []bool, deviation, angle float64) ([]Waypoint, []int) {
	t.Helper()
	before := append([]Waypoint(nil), points...)
	marks := append([]bool(nil), protected...)
	reduced, indices, err := simplifyFollowPoints(points, protected, deviation, angle)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(points, before) || (len(protected) > 0 && !reflect.DeepEqual(protected, marks)) {
		t.Fatal("preparation mutated the recording or markers")
	}
	if len(reduced) != len(indices) || len(indices) == 0 || indices[0] != 0 || indices[len(indices)-1] != len(points)-1 {
		t.Fatalf("invalid endpoint mapping: %v", indices)
	}
	retained := make(map[int]bool)
	for k, i := range indices {
		retained[i] = true
		if reduced[k] != points[i] {
			t.Fatalf("retained point %d lost its original identity", k)
		}
		if k == 0 {
			continue
		}
		j := indices[k-1]
		if j >= i || i-j > followPrepareSpan {
			t.Fatalf("unordered or unbounded span: %d -> %d", j, i)
		}
		for m := j + 1; m < i; m++ {
			if d := pointChordDistance(points[m], points[j], points[i]); d > deviation+1e-10 {
				t.Fatalf("point %d exceeds deviation: %g > %g", m, d, deviation)
			}
		}
		length := waypointDistance(points[j], points[i])
		previous := 0.0
		for m := j + 1; m < i; m++ {
			if length == 0 {
				if points[m] != points[j] {
					t.Fatalf("zero chord erased excursion at %d", m)
				}
				continue
			}
			along := ((points[m].X-points[j].X)*(points[i].X-points[j].X) +
				(points[m].Y-points[j].Y)*(points[i].Y-points[j].Y)) / length
			if along < previous-1e-10 || along > length+1e-10 {
				t.Fatalf("chord %d -> %d reverses projection at %d", j, i, m)
			}
			previous = along
		}
	}
	for i, marked := range protected {
		if marked && !retained[i] {
			t.Fatalf("lost marker %d", i)
		}
	}
	return reduced, indices
}

func TestSimplifyFollowPointsStraightDense(t *testing.T) {
	points := make([]Waypoint, 200)
	for i := range points {
		points[i] = Waypoint{float64(i) / 10, 2}
	}
	reduced, ids := checkPreparedFollow(t, points, nil, 0, 75)
	if !reflect.DeepEqual(ids, []int{0, 199}) {
		t.Fatalf("straight line did not merge: %v", ids)
	}
	reduced[0].X = 42
	if points[0].X != 0 {
		t.Fatal("result aliases recording")
	}
}

func TestSimplifyFollowPointsNoiseBounded(t *testing.T) {
	points := make([]Waypoint, 201)
	for i := range points {
		points[i] = Waypoint{float64(i), 0.04 * math.Sin(float64(i))}
	}
	points[0].Y, points[200].Y = 0, 0
	_, ids := checkPreparedFollow(t, points, nil, 0.05, 75)
	if len(ids) != 2 {
		t.Fatalf("bounded noise did not merge: %v", ids)
	}
	points[73].Y = 0.3
	_, ids = checkPreparedFollow(t, points, nil, 0.05, 180)
	if len(ids) <= 2 {
		t.Fatal("intermediate excursion was ignored")
	}
}

func TestSimplifyFollowPointsCornersMarkersAndDuplicates(t *testing.T) {
	points := []Waypoint{{0, 0}, {1, 0}, {2, 0}, {2, 0}, {2, 0}, {2, 1}, {2, 2}}
	marks := []bool{false, true, true, true, false, false, false}
	_, ids := checkPreparedFollow(t, points, marks, 100, 75)
	if !reflect.DeepEqual(ids, []int{0, 1, 2, 3, 4, 6}) {
		t.Fatalf("corner after duplicates or marker identity lost: %v", ids)
	}
	_, ids = checkPreparedFollow(t, []Waypoint{{1, 1}, {1, 1}, {1, 1}, {1, 1}}, nil, 0, 75)
	if !reflect.DeepEqual(ids, []int{0, 3}) {
		t.Fatalf("duplicate run did not merge: %v", ids)
	}
	checkPreparedFollow(t, []Waypoint{{1, 1}}, []bool{true}, 0, 75)
}

func TestSimplifyFollowPointsClosedLoops(t *testing.T) {
	square := followPrepareLoopFixture("square", 41)
	subloops := append([]Waypoint{{-10, 10}}, square...)
	for _, point := range square[1:] {
		subloops = append(subloops, Waypoint{-point.X, -point.Y})
	}
	subloops = append(subloops, Waypoint{10, -10})
	for _, tc := range []struct {
		name      string
		points    []Waypoint
		spans     [][2]int
		deviation float64
	}{
		{"circle", followPrepareLoopFixture("circle", 101), [][2]int{{0, 100}}, 100},
		{"square", square, [][2]int{{0, 40}}, 0.1},
		{"two subloops", subloops, [][2]int{{1, 41}, {41, 81}}, 0.1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marks := make([]bool, len(tc.points))
			marks[7] = true
			reduced, ids := checkPreparedFollow(t, tc.points, marks, tc.deviation, 75)
			if len(ids) >= len(tc.points)/2 {
				t.Fatalf("closed interior did not simplify: %d of %d", len(ids), len(tc.points))
			}
			for _, span := range tc.spans {
				assertFollowLoopAnchors(t, tc.points, reduced, ids, span[0], span[1])
			}
		})
	}
	// Retraced segments still have ordered junctions and retain the reversal.
	points := []Waypoint{{0, 0}, {1, 0}, {2, 0}, {1, 0}, {0, 0}}
	reduced, ids := checkPreparedFollow(t, points, nil, 100, 180)
	assertFollowLoopAnchors(t, points, reduced, ids, 0, 4)
}

func assertFollowLoopAnchors(t *testing.T, original, reduced []Waypoint, indices []int, start, end int) {
	t.Helper()
	positions := make(map[int]int, len(indices))
	for k, index := range indices {
		positions[index] = k
	}
	previous := -1
	for quarter := 0; quarter <= 4; quarter++ {
		index := start + (end-start)*quarter/4
		position, ok := positions[index]
		if !ok || position < previous || reduced[position] != original[index] {
			t.Fatalf("loop %d -> %d lost ordered anchor %d", start, end, index)
		}
		previous = position
	}
	if reduced[positions[start]] != reduced[positions[end]] || positions[start] == positions[end] {
		t.Fatalf("loop %d -> %d lost distinct junction indices", start, end)
	}
}

// n includes the exact closing sample; square corners are explicit even when
// n-1 is not divisible by four, matching recordings with uneven sample counts.
func followPrepareLoopFixture(shape string, n int) []Waypoint {
	points := make([]Waypoint, n)
	if shape == "circle" {
		for i := 0; i < n-1; i++ {
			angle := 2 * math.Pi * float64(i) / float64(n-1)
			points[i] = Waypoint{100 * math.Cos(angle), 100 * math.Sin(angle)}
		}
	} else {
		corners := []Waypoint{{0, 0}, {100, 0}, {100, 100}, {0, 100}, {0, 0}}
		for side := 0; side < 4; side++ {
			start, end := (n-1)*side/4, (n-1)*(side+1)/4
			for i := start; i < end; i++ {
				points[i] = waypointLerp(corners[side], corners[side+1], float64(i-start)/float64(end-start))
			}
		}
	}
	points[n-1] = points[0]
	return points
}

func TestPreparedFollowPathDenseClosedLoops(t *testing.T) {
	for _, shape := range []string{"square", "circle"} {
		t.Run(shape, func(t *testing.T) {
			points := followPrepareLoopFixture(shape, 20000)
			marks := make([]bool, len(points))
			marks[1234], marks[12345] = true, true
			reduced, ids := checkPreparedFollow(t, points, marks, 0.1, 75)
			// A structural size bound catches wholesale retention without a
			// machine-dependent runtime assertion. These loops need <100 points.
			if len(ids) > 100 {
				t.Fatalf("dense loop retained %d of %d points", len(ids), len(points))
			}
			assertFollowLoopAnchors(t, points, reduced, ids, 0, len(points)-1)
			original, err := newFollowPath(points, 75)
			if err != nil {
				t.Fatal(err)
			}
			p, err := newPreparedFollowPath(original, marks, 0.1, 75)
			if err != nil {
				t.Fatal(err)
			}
			for _, index := range []int{1234, (len(points) - 1) / 4, (len(points) - 1) / 2, 12345, (len(points) - 1) * 3 / 4, len(points) - 1} {
				// Cursor still names the original recording, including markers
				// inside the simplified loop and its final closing sample.
				cursor := FollowCursor{NextIndex: index, Offset: original.length[index]}
				if got := p.aim(cursor, 0, 0.1); waypointDistance(got, points[index]) > 1e-9 {
					t.Fatalf("anchor %d mapped to %v, want %v", index, got, points[index])
				}
			}
			if got := p.aim(FollowCursor{NextIndex: len(points)}, 1000, 0.1); got != points[len(points)-1] {
				t.Fatalf("completion wrapped to another branch: %v", got)
			}
			t.Logf("%s: %d -> %d points", shape, len(points), len(ids))
		})
	}
}

func TestSimplifyFollowPointsOrderedBacktrack(t *testing.T) {
	// No repeated coordinates and bends below the sharp-angle threshold.
	points := []Waypoint{{0, 0}, {3, 0.1}, {1, 0.2}, {4, 0}}
	_, ids := checkPreparedFollow(t, points, nil, 100, 180)
	if len(ids) <= 2 {
		t.Fatal("geometrically close backtrack was erased")
	}
}

func TestSimplifyFollowPointsTwentyThousand(t *testing.T) {
	points := make([]Waypoint, 20000)
	for i := range points {
		points[i] = Waypoint{float64(i), 0}
	}
	_, ids := checkPreparedFollow(t, points, nil, 0, 75)
	want := (len(points)-2)/followPrepareSpan + 2
	if len(ids) != want {
		t.Fatalf("bounded chunk reduction: got %d points, want %d", len(ids), want)
	}
}

func TestSimplifyFollowPointsInvalid(t *testing.T) {
	for _, tc := range []struct {
		name             string
		points           []Waypoint
		marks            []bool
		deviation, angle float64
	}{
		{"empty", nil, nil, 1, 75},
		{"marker length", []Waypoint{{0, 0}}, []bool{}, 1, 75},
		{"negative deviation", []Waypoint{{0, 0}}, nil, -1, 75},
		{"nan deviation", []Waypoint{{0, 0}}, nil, math.NaN(), 75},
		{"infinite deviation", []Waypoint{{0, 0}}, nil, math.Inf(1), 75},
		{"zero angle", []Waypoint{{0, 0}}, nil, 1, 0},
		{"large angle", []Waypoint{{0, 0}}, nil, 1, 181},
		{"nan angle", []Waypoint{{0, 0}}, nil, 1, math.NaN()},
		{"nan coordinate", []Waypoint{{math.NaN(), 0}}, nil, 1, 75},
		{"overflow", []Waypoint{{-math.MaxFloat64, 0}, {math.MaxFloat64, 0}}, nil, 1, 75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, ids, err := simplifyFollowPoints(tc.points, tc.marks, tc.deviation, tc.angle)
			if err == nil || p != nil || ids != nil {
				t.Fatalf("expected atomic validation failure, got %v %v %v", p, ids, err)
			}
		})
	}
}

func TestPreparedFollowPathArcMapping(t *testing.T) {
	points := []Waypoint{{0, 0}, {1, 1}, {4, 0}}
	original, err := newFollowPath(points, 180)
	if err != nil {
		t.Fatal(err)
	}
	p, err := newPreparedFollowPath(original, nil, 2, 180)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.indices, []int{0, 2}) {
		t.Fatalf("expected a reduced chord, got %v", p.indices)
	}
	a, total := math.Sqrt(2), math.Sqrt(2)+math.Sqrt(10)
	for _, tc := range []struct {
		name        string
		cursor      FollowCursor
		look, wantX float64
	}{
		{"start sentinel", FollowCursor{}, 1, 0},
		{"first segment start", FollowCursor{NextIndex: 1}, 0.5, 0.5},
		{"original arc midpoint", FollowCursor{NextIndex: 2, Offset: total/2 - a}, 0, 2},
		{"midpoint lookahead", FollowCursor{NextIndex: 2, Offset: total/2 - a}, 0.5, 2.5},
		{"index edge before callback", FollowCursor{NextIndex: 1, Offset: a}, 0, 4 * a / total},
		{"index edge after callback", FollowCursor{NextIndex: 2}, 0, 4 * a / total},
		{"negative offset clamped", FollowCursor{NextIndex: 1, Offset: -1}, 0, 0},
		{"large offset clamped", FollowCursor{NextIndex: 2, Offset: 100}, 0, 4},
		{"endpoint before callback", FollowCursor{NextIndex: 2, Offset: math.Sqrt(10)}, 1, 4},
		{"completion", FollowCursor{NextIndex: 3}, 1, 4},
		{"past completion clamped", FollowCursor{NextIndex: 30}, 1, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := p.aim(tc.cursor, tc.look, 2)
			if waypointDistance(got, Waypoint{tc.wantX, 0}) > 1e-12 {
				t.Fatalf("got %v, want (%g, 0)", got, tc.wantX)
			}
		})
	}
	// Preparation owns its geometry and arc table independently of the caller.
	original.points[1] = Waypoint{100, 100}
	original.length[1] = 100
	if got := p.aim(FollowCursor{NextIndex: 2}, 0, 2); math.Abs(got.X-4*a/total) > 1e-12 {
		t.Fatalf("prepared geometry aliases original: %v", got)
	}
}

func TestPreparedFollowPathPreservesOriginalSharpGate(t *testing.T) {
	points := []Waypoint{{0, -0.5}, {1, 0}, {2, 0}, {2.2, 1}}
	original, err := newFollowPath(points, 75)
	if err != nil {
		t.Fatal(err)
	}
	p, err := newPreparedFollowPath(original, nil, 0.25, 75)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.indices, []int{0, 2, 3}) {
		t.Fatalf("expected redundant neighbor to be removed: %v", p.indices)
	}
	// Removing point 1 changes the corner from 78.69 to 64.65 degrees.
	// Its original sharp gate must survive despite the new chord direction.
	cursor := FollowCursor{NextIndex: 1}
	if got := original.aim(cursor, 100, 0.75); got != points[2] {
		t.Fatalf("fixture does not stop at the original corner: %v", got)
	}
	if got := p.aim(cursor, 100, 0.75); got != points[2] {
		t.Fatalf("prepared aim passed the original sharp gate: got %v, want %v", got, points[2])
	}
	if got := p.aim(FollowCursor{NextIndex: 3}, 100, 0.75); got != points[3] {
		t.Fatalf("aim did not continue after crossing the corner: %v", got)
	}
}

func TestPreparedFollowPathDuplicateAndSharpEdges(t *testing.T) {
	points := []Waypoint{{0, 0}, {0, 0}, {2, 0}, {2, 0}, {2, 2}, {2, 2}}
	original, err := newFollowPath(points, 75)
	if err != nil {
		t.Fatal(err)
	}
	p, err := newPreparedFollowPath(original, []bool{false, true, true, true, false, false}, 0.1, 75)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		cursor FollowCursor
		look   float64
		want   Waypoint
	}{
		{FollowCursor{NextIndex: 1}, 0, Waypoint{0, 0}},
		{FollowCursor{NextIndex: 1}, 1, Waypoint{1, 0}},
		{FollowCursor{NextIndex: 2, Offset: 1}, 0, Waypoint{1, 0}},
		{FollowCursor{NextIndex: 2, Offset: 2}, 1, Waypoint{2, 0}},
		{FollowCursor{NextIndex: 3}, 1, Waypoint{2, 0}},
		{FollowCursor{NextIndex: 4}, 1, Waypoint{2, 1}},
		{FollowCursor{NextIndex: 5}, 1, Waypoint{2, 2}},
		{FollowCursor{NextIndex: 6}, 1, Waypoint{2, 2}},
	} {
		if got := p.aim(tc.cursor, tc.look, 0.1); waypointDistance(got, tc.want) > 1e-12 {
			t.Fatalf("cursor %+v look %g: got %v want %v", tc.cursor, tc.look, got, tc.want)
		}
	}
	for _, points := range [][]Waypoint{{{3, 4}}, {{3, 4}, {3, 4}, {3, 4}}} {
		original, err := newFollowPath(points, 75)
		if err != nil {
			t.Fatal(err)
		}
		p, err := newPreparedFollowPath(original, nil, 0, 75)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i <= len(points); i++ {
			if got := p.aim(FollowCursor{NextIndex: i}, 1, 0); got != points[0] {
				t.Fatalf("zero-length path cursor %d: %v", i, got)
			}
		}
	}
}

func TestPreparedFollowPathClosedFinal(t *testing.T) {
	points := []Waypoint{{0, 0}, {2, 0}, {2, 2}, {0, 2}, {0, 0}, {0, 0}}
	original, err := newFollowPath(points, 75)
	if err != nil {
		t.Fatal(err)
	}
	p, err := newPreparedFollowPath(original, nil, 100, 75)
	if err != nil {
		t.Fatal(err)
	}
	for _, cursor := range []FollowCursor{
		{}, {NextIndex: 1}, {NextIndex: 3, Offset: 1},
		{NextIndex: 4, Offset: 1}, {NextIndex: 4, Offset: 2},
		{NextIndex: 5}, {NextIndex: 6},
	} {
		want := original.aim(cursor, 0.5, 0.1)
		if got := p.aim(cursor, 0.5, 0.1); waypointDistance(got, want) > 1e-12 {
			t.Fatalf("closed path cursor %+v: got %v want %v", cursor, got, want)
		}
	}
}

func TestPreparedFollowPathInvalid(t *testing.T) {
	if _, err := newPreparedFollowPath(followPath{}, nil, 0, 75); err == nil {
		t.Fatal("empty original accepted")
	}
	original, err := newFollowPath([]Waypoint{{0, 0}, {1, 0}}, 75)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newPreparedFollowPath(original, []bool{true}, 0, 75); err == nil {
		t.Fatal("invalid marker count accepted")
	}
}

func BenchmarkSimplifyFollowPointsLinear(b *testing.B) {
	for _, n := range []int{2000, 20000} {
		points := make([]Waypoint, n)
		for i := range points {
			points[i].X = float64(i)
		}
		name := "2000"
		if n == 20000 {
			name = "20000"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, _, err := simplifyFollowPoints(points, nil, 0, 75); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkPreparedFollowPathClosedLoops(b *testing.B) {
	for _, shape := range []string{"square", "circle"} {
		for _, size := range []struct {
			name string
			n    int
		}{{"2000", 2000}, {"20000", 20000}} {
			b.Run(shape+"/"+size.name, func(b *testing.B) {
				original, err := newFollowPath(followPrepareLoopFixture(shape, size.n), 75)
				if err != nil {
					b.Fatal(err)
				}
				p, err := newPreparedFollowPath(original, nil, 0.1, 75)
				if err != nil {
					b.Fatal(err)
				}
				b.Run("prepare", func(b *testing.B) {
					b.ReportAllocs()
					b.ReportMetric(float64(len(p.indices)), "retained")
					for i := 0; i < b.N; i++ {
						if _, err := newPreparedFollowPath(original, nil, 0.1, 75); err != nil {
							b.Fatal(err)
						}
					}
				})
				b.Run("aim", func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						index := 1 + i%(size.n-1)
						got := p.aim(FollowCursor{NextIndex: index, Offset: original.length[index] / 2}, 1000, 0.1)
						if !Finite(got.X) || !Finite(got.Y) {
							b.Fatal("nonfinite aim")
						}
					}
				})
			})
		}
	}
}
