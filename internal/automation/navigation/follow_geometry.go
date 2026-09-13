package navigation

import (
	"errors"
	"math"
)

// Waypoint is a point in the Driver's world coordinate frame.
type Waypoint struct{ X, Y float64 }

// FollowCursor belongs to the exact same ordered path on resume. NextIndex is
// the next unreported point (len(points) means complete). Offset is arc length
// along points[NextIndex-1] -> points[NextIndex], or zero at either sentinel.
// A zero cursor starts at point zero; it never chooses a nearest path branch.
type FollowCursor struct {
	NextIndex int
	Offset    float64
}

type followPath struct {
	points         []Waypoint
	length, suffix []float64
	sharp          []bool
	nextSharp      []int
}

func newFollowPath(points []Waypoint, sharpAngle float64) (followPath, error) {
	p := followPath{points: append([]Waypoint(nil), points...), length: make([]float64, len(points)), suffix: make([]float64, len(points)), sharp: make([]bool, len(points))}
	if len(points) == 0 {
		return p, errors.New("navigation path is empty")
	}
	for i, point := range points {
		if !Finite(point.X) || !Finite(point.Y) {
			return p, errors.New("invalid navigation waypoint")
		}
		if i > 0 {
			p.length[i] = waypointDistance(points[i-1], point)
			if !Finite(p.length[i]) {
				return p, errors.New("navigation segment overflows")
			}
		}
	}
	for i := len(points) - 2; i >= 0; i-- {
		p.suffix[i] = p.suffix[i+1] + p.length[i+1]
		if !Finite(p.suffix[i]) {
			return p, errors.New("navigation path length overflows")
		}
	}
	// Ignore duplicate samples when classifying bends, retaining their indices.
	previous := -1
	for i := 0; i < len(points)-1; i++ {
		if i > 0 && p.length[i] > 0 {
			previous = i - 1
		}
		if previous >= 0 && p.length[i+1] > 0 {
			a, b, c := points[previous], points[i], points[i+1]
			p.sharp[i] = math.Abs(Delta(math.Atan2(c.Y-b.Y, c.X-b.X)*180/math.Pi, math.Atan2(b.Y-a.Y, b.X-a.X)*180/math.Pi)) >= sharpAngle
		}
	}
	p.nextSharp = make([]int, len(points))
	next := len(points) - 1
	for i := len(points) - 1; i >= 0; i-- {
		if p.sharp[i] {
			next = i
		}
		p.nextSharp[i] = next
	}
	return p, nil
}

func waypointDistance(a, b Waypoint) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }
func waypointLerp(a, b Waypoint, f float64) Waypoint {
	return Waypoint{a.X + (b.X-a.X)*f, a.Y + (b.Y-a.Y)*f}
}
func (p followPath) location(c FollowCursor) Waypoint {
	if c.NextIndex == 0 {
		return p.points[0]
	}
	if c.NextIndex == len(p.points) {
		return p.points[len(p.points)-1]
	}
	if p.length[c.NextIndex] == 0 {
		return p.points[c.NextIndex]
	}
	return waypointLerp(p.points[c.NextIndex-1], p.points[c.NextIndex], c.Offset/p.length[c.NextIndex])
}
func (p followPath) remaining(c FollowCursor, pos Waypoint) float64 {
	if c.NextIndex == len(p.points) {
		return waypointDistance(pos, p.points[len(p.points)-1])
	}
	return waypointDistance(pos, p.location(c)) + p.suffix[c.NextIndex] + p.length[c.NextIndex] - c.Offset
}

// project examines only the current segment and sequential successors within a
// bounded local arc budget. It cannot search globally across loops or crossings.
// Sharp vertices are gates: they require proximity and end this observation's
// projection, even if there is travel budget left.
func (p followPath) project(c FollowCursor, pos Waypoint, budget, tolerance, corridor float64) (FollowCursor, bool) {
	for c.NextIndex < len(p.points)-1 {
		i := c.NextIndex
		if i == 0 {
			if waypointDistance(pos, p.points[0]) > tolerance {
				break
			}
			c.NextIndex++
			continue
		}
		length := p.length[i]
		if length == 0 {
			if p.sharp[i] && waypointDistance(pos, p.points[i]) > tolerance {
				break
			}
			c.NextIndex++
			c.Offset = 0
			if p.sharp[i] {
				return c, true
			}
			continue
		}
		a, b := p.points[i-1], p.points[i]
		ux, uy := (b.X-a.X)/length, (b.Y-a.Y)/length
		along := (pos.X-a.X)*ux + (pos.Y-a.Y)*uy
		target := min(length, max(c.Offset, along))
		near := waypointDistance(pos, b) <= tolerance
		if p.sharp[i] && near {
			target = length
		}
		candidate := min(target, c.Offset+budget)
		// Corridor membership uses the physical projection. Testing the
		// budget-limited cursor instead strands progress behind a valid pose;
		// every later steering target can then remain behind the character.
		projected := waypointLerp(a, b, target/length)
		separation := waypointDistance(pos, projected)
		// A single held turn can cross several dense samples. Beyond a
		// segment's end use its lateral corridor, not distance back to the
		// vertex; adjacency and the shared arc budget still bound advancement.
		if along >= length {
			separation = math.Abs((pos.X-a.X)*uy - (pos.Y-a.Y)*ux)
		}
		if separation > corridor {
			break
		}
		budget -= candidate - c.Offset
		c.Offset = candidate
		if c.Offset < length || (p.sharp[i] && !near) {
			break
		}
		c.NextIndex++
		c.Offset = 0
		if p.sharp[i] {
			return c, true
		}
	}
	// The endpoint is projected but never reported here: arrival requires a
	// second fresh observation after forward has been released.
	if i := c.NextIndex; i > 0 && i == len(p.points)-1 && p.length[i] > 0 {
		a, b := p.points[i-1], p.points[i]
		along := ((pos.X-a.X)*(b.X-a.X) + (pos.Y-a.Y)*(b.Y-a.Y)) / p.length[i]
		offset := min(p.length[i], min(c.Offset+budget, max(c.Offset, along)))
		if waypointDistance(pos, waypointLerp(a, b, min(p.length[i], max(0, along))/p.length[i])) <= corridor {
			c.Offset = offset
		}
	}
	return c, false
}

func pointChordDistance(p, a, b Waypoint) float64 {
	l := waypointDistance(a, b)
	if l == 0 {
		return waypointDistance(p, a)
	}
	f := ((p.X-a.X)*(b.X-a.X) + (p.Y-a.Y)*(b.Y-a.Y)) / l / l
	return waypointDistance(p, waypointLerp(a, b, max(0, min(1, f))))
}

// aim extends a local chord only while all intervening vertices remain within
// deviation of it. Sharp bends cap the chord at their vertex.
func (p followPath) aim(c FollowCursor, look, deviation float64) Waypoint {
	if c.NextIndex == 0 || c.NextIndex == len(p.points) {
		return p.location(c)
	}
	start := p.location(c)
	result := start
	vertices := []Waypoint{start}
	for i, offset := c.NextIndex, c.Offset; i < len(p.points); i, offset = i+1, 0 {
		step := min(look, p.length[i]-offset)
		candidate := p.points[i]
		if p.length[i] > 0 && offset+step < p.length[i] {
			candidate = waypointLerp(p.points[i-1], p.points[i], (offset+step)/p.length[i])
		}
		valid := func(q Waypoint) bool {
			for _, v := range vertices {
				if pointChordDistance(v, start, q) > deviation {
					return false
				}
			}
			return true
		}
		if !valid(candidate) {
			lo, hi := 0.0, 1.0
			for n := 0; n < 24; n++ {
				mid := (lo + hi) / 2
				if valid(waypointLerp(result, candidate, mid)) {
					lo = mid
				} else {
					hi = mid
				}
			}
			return waypointLerp(result, candidate, lo)
		}
		result = candidate
		look -= step
		if look <= 0 || p.sharp[i] {
			break
		}
		vertices = append(vertices, result)
	}
	return result
}
