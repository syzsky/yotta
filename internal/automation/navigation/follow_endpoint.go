package navigation

import "time"

// Final positioning separates a short movement from its residual motion. Turning
// while the previous movement is still settling creates repeated return arcs.
type pathEndpoint struct {
	armed, active, settling, observed, pulsePending, correcting bool
	stoppedAt, quietSince                                       time.Time
	previous, pulseStart                                        Waypoint
	pulseTime                                                   time.Duration
	speed                                                       float64
}

func (e *pathEndpoint) stop(now time.Time) {
	e.settling = true
	e.stoppedAt = now
	e.observed = false
}
func (e *pathEndpoint) settled(pos Waypoint, sampled time.Time, tolerance float64) bool {
	if !sampled.After(e.stoppedAt) {
		return false
	}
	if !e.observed || waypointDistance(pos, e.previous) > tolerance*0.05 {
		e.quietSince = sampled
	}
	e.observed = true
	e.previous = pos
	if sampled.Sub(e.quietSince) < 200*time.Millisecond {
		return false
	}
	e.settling = false
	if e.pulsePending {
		e.speed = max(e.speed, waypointDistance(pos, e.pulseStart)/e.pulseTime.Seconds())
		e.pulsePending = false
	}
	return true
}
func (e *pathEndpoint) pulse(pos Waypoint, distance, tolerance float64) time.Duration {
	duration := 20 * time.Millisecond
	if e.speed > 0 {
		duration = time.Duration(max(0, distance-tolerance*0.35) / e.speed * 0.6 * float64(time.Second))
	}
	duration = max(5*time.Millisecond, min(200*time.Millisecond, duration))
	e.pulseStart, e.pulseTime, e.pulsePending = pos, duration, true
	return duration
}
