package navigation

import (
	"math"
	"time"
)

type pathTurnSample struct {
	start, end time.Time
	angle      float64
}
type pathObservation struct{ turns []pathTurnSample }

func (o *pathObservation) turn(start, end time.Time, angle float64) {
	keep := 0
	for _, t := range o.turns {
		if !t.end.Before(end.Add(-time.Second)) {
			o.turns[keep] = t
			keep++
		}
	}
	o.turns = o.turns[:keep]
	if end.After(start) {
		o.turns = append(o.turns, pathTurnSample{start, end, angle})
	}
}

// Estimate only the control pose. Source observations and confirmed progress are immutable.
// Unknown acquisition times use legacy behavior; no read-duration surrogate is used.
func (o *pathObservation) predict(p Pose, now time.Time, speed float64, held bool, heldAt time.Time) Pose {
	age := now.Sub(p.SampleTime)
	if p.SampleTime.IsZero() || age <= 0 || age > 500*time.Millisecond {
		return p
	}
	steps := max(1, int(math.Ceil(age.Seconds()/0.005)))
	dt := age / time.Duration(steps)
	at := p.SampleTime
	for i := 0; i < steps; i++ {
		end := at.Add(dt)
		angle := 0.0
		for _, t := range o.turns {
			a, b := at, end
			if t.start.After(a) {
				a = t.start
			}
			if t.end.Before(b) {
				b = t.end
			}
			if b.After(a) {
				angle += t.angle * b.Sub(a).Seconds() / t.end.Sub(t.start).Seconds()
			}
		}
		if held {
			r := (p.Heading + angle/2 - p.AxisHeading) / p.AxisSign * math.Pi / 180
			motionTime := dt
			if heldAt.After(at) {
				motionTime = max(0, end.Sub(heldAt))
			}
			p.X += speed * motionTime.Seconds() * math.Cos(r)
			p.Y += speed * motionTime.Seconds() * math.Sin(r)
		}
		p.Heading = Normalize(p.Heading + angle)
		at = end
	}
	return p
}
