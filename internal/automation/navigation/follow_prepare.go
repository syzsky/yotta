package navigation

import (
	"errors"
	"math"
	"sort"
)

// preparedFollowPath owns aim geometry only. Cursor advancement, arrival,
// remaining distance and marker callbacks still belong to the original path.
type preparedFollowPath struct {
	reduced followPath
	indices []int
	prefix  []float64 // original cumulative arc length at each original index
}

// newPreparedFollowPath snapshots the geometry of an original newFollowPath.
func newPreparedFollowPath(original followPath, protected []bool, deviation, sharpAngle float64) (preparedFollowPath, error) {
	points, indices, err := simplifyFollowPoints(original.points, protected, deviation, sharpAngle)
	if err != nil {
		return preparedFollowPath{}, err
	}
	reduced, err := newFollowPath(points, sharpAngle)
	if err != nil {
		return preparedFollowPath{}, err
	}
	// Changed neighbors may soften a retained corner below sharpAngle. Keep
	// its original gate as well as any sharp turns in the reduced geometry.
	for i, index := range indices {
		reduced.sharp[i] = reduced.sharp[i] || original.sharp[index]
	}
	prefix := make([]float64, len(original.points))
	for i := 1; i < len(prefix); i++ {
		prefix[i] = prefix[i-1] + waypointDistance(original.points[i-1], original.points[i])
		if !Finite(prefix[i]) {
			return preparedFollowPath{}, errors.New("navigation path length overflows")
		}
	}
	return preparedFollowPath{reduced: reduced, indices: indices, prefix: prefix}, nil
}

// aim maps original progress to the corresponding reduced chord by original arc
// fraction, then measures look in reduced geometry's world units. Binary search
// uses original indices rather than arc values: equal arcs at duplicate points
// must not jump past an unreported sharp vertex or onto another loop branch.
// The caller supplies its validated original cursor; sentinels and fractions
// are clamped, and no original progress state is changed.
func (p preparedFollowPath) aim(original FollowCursor, look, deviation float64) Waypoint {
	if original.NextIndex <= 0 {
		return p.reduced.points[0]
	}
	if original.NextIndex >= len(p.prefix) {
		return p.reduced.points[len(p.reduced.points)-1]
	}
	next := sort.SearchInts(p.indices, original.NextIndex)
	start, end := p.indices[next-1], p.indices[next]
	span := p.prefix[end] - p.prefix[start]
	fraction := 0.0
	if span > 0 {
		offset := max(0, min(original.Offset, p.prefix[original.NextIndex]-p.prefix[original.NextIndex-1]))
		fraction = max(0, min(1, ((p.prefix[original.NextIndex-1]-p.prefix[start])+offset)/span))
	}
	cursor := FollowCursor{NextIndex: next, Offset: fraction * p.reduced.length[next]}
	return p.reduced.aim(cursor, max(0, look), deviation)
}

// followPrepareSpan bounds the original segments considered by any one chord.
const followPrepareSpan = 256

// simplifyFollowPoints prepares an aim-only polyline and its strictly increasing
// original indices. The caller must keep the full recording for cursor progress
// and callbacks; indices allow interpolation using original arc progress between
// retained vertices, rather than nearest-point matching on the reduced path.
// Neither input is modified and the returned points do not alias the recording.
//
// protected is nil or has one entry per point. Deviation is a finite nonnegative
// distance; sharpAngle is a turn angle in degrees in (0, 180]. Endpoints, marked
// indices and sharp bends survive. Each exact return to a previously visited
// coordinate retains both junction indices and ordered quarter-span anchors
// (by sample index; consecutive duplicates alone are not loops). Every chord
// bounds intermediate-point deviation and preserves projection order. These
// local constraints do not guarantee global topology preservation.
//
// Fixed-size chunks and iterative midpoint splitting bound work to
// O(n log(followPrepareSpan)), with O(n) storage and no recursion. Chunk boundaries
// may retain redundant points; this deliberately does not minimize vertex count.
func simplifyFollowPoints(points []Waypoint, protected []bool, deviation, sharpAngle float64) ([]Waypoint, []int, error) {
	if (protected != nil && len(protected) != len(points)) || !Finite(deviation) || deviation < 0 ||
		!Finite(sharpAngle) || sharpAngle <= 0 || sharpAngle > 180 {
		return nil, nil, errors.New("invalid navigation path preparation settings")
	}
	path, err := newFollowPath(points, sharpAngle)
	if err != nil {
		return nil, nil, err
	}
	n := len(points)
	keep := path.sharp
	keep[0], keep[n-1] = true, true
	// Anchor each consecutive revisit span without freezing its dense interior.
	// Constant work per revisit also handles overlapping spans in linear time.
	last := make(map[Waypoint]int, n)
	for i, p := range points {
		if j, ok := last[p]; ok && j < i-1 {
			keep[j], keep[i] = true, true
			for quarter := 1; quarter <= 3; quarter++ {
				keep[j+(i-j)*quarter/4] = true
			}
		}
		last[p] = i
		keep[i] = keep[i] || (protected != nil && protected[i])
	}
	type span struct{ start, end int }
	stack := make([]span, 0, 16)
	for start := 0; start < n-1; {
		end := start + 1
		for end < n-1 && end-start < followPrepareSpan && !keep[end] {
			end++
		}
		keep[end] = true
		stack = append(stack, span{start, end})
		for len(stack) > 0 {
			s := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if s.end-s.start <= 1 || followPrepareChord(points[s.start:s.end+1], deviation) {
				continue
			}
			mid := s.start + (s.end-s.start)/2
			keep[mid] = true
			stack = append(stack, span{s.start, mid}, span{mid, s.end})
		}
		start = end
	}
	reduced := make([]Waypoint, 0, n)
	indices := make([]int, 0, n)
	for i, retain := range keep {
		if retain {
			reduced = append(reduced, points[i])
			indices = append(indices, i)
		}
	}
	return reduced, indices, nil
}

func followPrepareChord(points []Waypoint, deviation float64) bool {
	a, b := points[0], points[len(points)-1]
	length := waypointDistance(a, b)
	if !Finite(length) {
		return false
	}
	if length == 0 {
		for _, p := range points[1 : len(points)-1] {
			if p != a {
				return false
			}
		}
		return true
	}
	ux, uy := (b.X-a.X)/length, (b.Y-a.Y)/length
	previous := 0.0
	for _, p := range points[1 : len(points)-1] {
		dx, dy := p.X-a.X, p.Y-a.Y
		along := dx*ux + dy*uy
		// Unclamped projection enforces order and prevents erasing backtracks,
		// even when the whole excursion fits inside the deviation corridor.
		if !Finite(along) || along < previous || along > length {
			return false
		}
		distance := math.Abs(dx*uy - dy*ux)
		if !Finite(distance) || distance > deviation {
			return false
		}
		previous = along
	}
	return true
}
