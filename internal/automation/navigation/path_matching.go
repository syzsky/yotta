package navigation

// match can reacquire adjacent segments inside a bounded forward arc window.
// A miss on the old segment must not hide a valid nearby successor. Sharp gates
// remain hard boundaries; no global nearest-point search or implicit loop jump.
func (p followPath) match(c FollowCursor, pos Waypoint, budget, tolerance, corridor float64) (FollowCursor, bool) {
	base, bend := p.project(c, pos, budget, tolerance, corridor)
	if bend || c.NextIndex == 0 || c.NextIndex >= len(p.points) {
		return base, bend
	}
	best := base
	bestDistance := waypointDistance(pos, p.location(base))
	remaining := budget
	for i, offset := c.NextIndex, c.Offset; i < len(p.points) && remaining >= 0; i, offset = i+1, 0 {
		length := p.length[i]
		if length == 0 {
			if p.sharp[i] {
				break
			}
			continue
		}
		a, b := p.points[i-1], p.points[i]
		along := ((pos.X-a.X)*(b.X-a.X) + (pos.Y-a.Y)*(b.Y-a.Y)) / length
		candidate := min(length, min(offset+remaining, max(offset, along)))
		distance := waypointDistance(pos, waypointLerp(a, b, candidate/length))
		newer := i > best.NextIndex || (i == best.NextIndex && candidate > best.Offset)
		if newer && distance <= corridor && distance < bestDistance {
			best = FollowCursor{i, candidate}
			bestDistance = distance
		}
		remaining -= length - offset
		if p.sharp[i] {
			break
		}
	}
	return p.project(best, pos, 0, tolerance, corridor)
}
