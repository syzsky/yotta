package navigationpath

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/yottaapp/yotta/sdk/plugin/positionsource"
)

var ErrUnavailable = errors.New("path position unavailable")
var ErrReference = errors.New("path coordinate reference differs or is undeclared")

type Observation struct {
	Reference    Reference
	Point        Point
	Heading      *float64
	Epoch        string
	Sequence     int64
	SampleTimeMs int64
	Recovery     string
}

// Observe preserves independently timed optional information. A declared but
// unavailable map/floor cannot be mistaken for a reference with unknown scope.
func Observe(raw []byte, now time.Time, maxAge time.Duration) (Observation, error) {
	s, err := positionsource.Decode(raw)
	if err != nil {
		return Observation{}, err
	}
	if s.Frame.Kind == "" || s.Frame.Recovery == "" {
		return Observation{}, ErrReference
	}
	if !s.Ready(positionsource.Position, now, maxAge) {
		return Observation{}, ErrUnavailable
	}
	xy := s.Observations[positionsource.Position]
	o := Observation{Reference: Reference{Kind: s.Frame.Kind, Frame: s.Frame.ID, Unit: s.Frame.Unit, AxisHeading: s.Frame.AxisHeading, AxisSign: s.Frame.AxisSign}, Epoch: s.Epoch, Sequence: xy.Sequence, SampleTimeMs: xy.SampleTimeMs, Recovery: s.Frame.Recovery}
	if err := json.Unmarshal(xy.Value, &o.Point); err != nil {
		return Observation{}, err
	}
	for cap, dest := range map[positionsource.Capability]*string{positionsource.Map: &o.Reference.Map, positionsource.Floor: &o.Reference.Floor} {
		if obs, exists := s.Observations[cap]; exists {
			if !s.Ready(cap, now, maxAge) {
				return Observation{}, ErrUnavailable
			}
			if err := json.Unmarshal(obs.Value, dest); err != nil {
				return Observation{}, err
			}
		}
	}
	for cap, dest := range map[positionsource.Capability]**float64{positionsource.Altitude: &o.Point.Z, positionsource.CameraHeading: &o.Heading} {
		if s.Ready(cap, now, maxAge) {
			var v float64
			if err := json.Unmarshal(s.Observations[cap].Value, &v); err != nil {
				return Observation{}, err
			}
			*dest = &v
		}
	}
	return o, nil
}
