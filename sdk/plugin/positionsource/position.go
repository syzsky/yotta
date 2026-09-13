// Package positionsource defines observations shared by game positioning plugins.
// Capabilities describe data, not permissions or an input-control API.
package positionsource

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const Protocol = "yotta.position-source/v1"

type Capability string

const (
	Position         Capability = "position"
	Altitude         Capability = "altitude"
	CharacterHeading Capability = "character-heading"
	CameraHeading    Capability = "camera-heading"
	CameraPitch      Capability = "camera-pitch"
	Map              Capability = "map"
	Floor            Capability = "floor"
)

// Frame identifies a coordinate system, never a screen rectangle. Heading zero
// and direction are source-defined; AxisHeading/AxisSign relate XY to heading.
type Frame struct {
	// Kind is world or local; empty means an older source has not declared it.
	Kind string `json:"kind,omitempty"`
	// Recovery declares whether the same frame survives reconnect (stable),
	// requires explicit alignment (align), or is valid only in this session.
	Recovery    string  `json:"recovery,omitempty"`
	ID          string  `json:"id"`
	Unit        string  `json:"unit"`
	AxisHeading float64 `json:"axisHeading"`
	AxisSign    int     `json:"axisSign"`
}

type Descriptor struct {
	Protocol     string       `json:"protocol"`
	Source       string       `json:"source"`
	Capabilities []Capability `json:"capabilities"`
	Frame        Frame        `json:"frame"`
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Observation has its own clock metadata: combining sources must not refresh
// old coordinates using a newer camera observation. Accuracy is an optional
// error radius in frame units (position/altitude) or degrees (angles).
type Observation struct {
	Status       string          `json:"status"`
	Value        json.RawMessage `json:"value"`
	SampleTimeMs int64           `json:"sampleTimeMs"`
	SampleAgeMs  int64           `json:"sampleAgeMs"`
	Sequence     int64           `json:"sequence"`
	Accuracy     *float64        `json:"accuracy"`
}

type Snapshot struct {
	Descriptor
	Epoch        string                     `json:"epoch"`
	Observations map[Capability]Observation `json:"observations"`
}

func Decode(raw []byte) (Snapshot, error) {
	var s Snapshot
	fields, err := required(raw, "protocol", "source", "capabilities", "frame", "epoch", "observations")
	if err != nil {
		return s, err
	}
	if _, err := required(fields["frame"], "id", "unit", "axisHeading", "axisSign"); err != nil {
		return s, err
	}
	var observations map[string]json.RawMessage
	if err := json.Unmarshal(fields["observations"], &observations); err != nil {
		return s, fmt.Errorf("position source: invalid observations")
	}
	for _, raw := range observations {
		fields, err := required(raw, "status", "sampleTimeMs", "sampleAgeMs", "sequence")
		if err != nil {
			return s, err
		}
		if _, ok := fields["value"]; !ok {
			return s, fmt.Errorf("position source: missing value")
		}
		if _, ok := fields["accuracy"]; !ok {
			return s, fmt.Errorf("position source: missing accuracy")
		}
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return s, fmt.Errorf("position source: invalid JSON")
	}
	return s, s.Validate()
}

func required(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, fmt.Errorf("position source: expected object")
	}
	for _, name := range names {
		if value, ok := fields[name]; !ok || string(value) == "null" {
			return nil, fmt.Errorf("position source: missing %s", name)
		}
	}
	return fields, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (s Snapshot) Validate() error {
	if s.Frame.Kind != "" && s.Frame.Kind != "world" && s.Frame.Kind != "local" {
		return fmt.Errorf("position source: invalid frame kind")
	}
	if s.Frame.Recovery != "" && s.Frame.Recovery != "stable" && s.Frame.Recovery != "align" && s.Frame.Recovery != "session" {
		return fmt.Errorf("position source: invalid frame recovery")
	}
	if (s.Frame.Kind == "") != (s.Frame.Recovery == "") {
		return fmt.Errorf("position source: frame kind and recovery must be declared together")
	}
	if s.Protocol != Protocol || s.Source == "" || s.Epoch == "" || s.Frame.ID == "" || s.Frame.Unit == "" || !finite(s.Frame.AxisHeading) || s.Frame.AxisHeading < 0 || s.Frame.AxisHeading >= 360 || (s.Frame.AxisSign != 1 && s.Frame.AxisSign != -1) || len(s.Capabilities) == 0 {
		return fmt.Errorf("position source: invalid descriptor")
	}
	seen := map[Capability]bool{}
	for _, c := range s.Capabilities {
		o, exists := s.Observations[c]
		if seen[c] || !exists {
			return fmt.Errorf("position source: duplicate or missing capability observation")
		}
		seen[c] = true
		if o.Status != "tracking" && o.Status != "stale" && o.Status != "waiting" && o.Status != "unavailable" {
			return fmt.Errorf("position source: invalid observation status")
		}
		if o.Accuracy != nil && (!finite(*o.Accuracy) || *o.Accuracy < 0) {
			return fmt.Errorf("position source: invalid accuracy")
		}
		hasValue := len(o.Value) > 0 && string(o.Value) != "null"
		if !hasValue {
			if o.Status == "tracking" || o.Status == "stale" || o.SampleTimeMs != 0 || o.SampleAgeMs != -1 || o.Sequence != 0 || o.Accuracy != nil {
				return fmt.Errorf("position source: missing observation")
			}
		} else {
			if o.Status == "waiting" || o.SampleTimeMs <= 0 || o.SampleAgeMs < 0 || o.Sequence <= 0 {
				return fmt.Errorf("position source: invalid sample metadata")
			}
		}
		switch c {
		case Position:
			if hasValue {
				var p struct {
					X *float64 `json:"x"`
					Y *float64 `json:"y"`
				}
				if json.Unmarshal(o.Value, &p) != nil || p.X == nil || p.Y == nil || !finite(*p.X) || !finite(*p.Y) {
					return fmt.Errorf("position source: invalid planar position")
				}
			}
		case Altitude, CameraHeading, CharacterHeading, CameraPitch:
			if hasValue {
				var v float64
				if json.Unmarshal(o.Value, &v) != nil || !finite(v) || ((c == CameraHeading || c == CharacterHeading) && (v < 0 || v >= 360)) || (c == CameraPitch && (v < -90.001 || v > 90.001)) {
					return fmt.Errorf("position source: invalid angle or altitude")
				}
			}
		case Map, Floor:
			if hasValue {
				var v string
				if json.Unmarshal(o.Value, &v) != nil || v == "" {
					return fmt.Errorf("position source: invalid map or floor")
				}
			}
		default:
			return fmt.Errorf("position source: unknown capability")
		}
	}
	if len(seen) != len(s.Observations) {
		return fmt.Errorf("position source: undeclared observation")
	}
	return nil
}

// Ready applies the consumer's maximum age, including transport/cache delay.
// Producers must use the host clock or translate their clock before delivery.
func (s Snapshot) Ready(c Capability, now time.Time, maxAge time.Duration) bool {
	o, ok := s.Observations[c]
	if !ok || o.Status != "tracking" || maxAge < 0 {
		return false
	}
	age := now.UnixMilli() - o.SampleTimeMs
	return age >= -100 && age <= maxAge.Milliseconds() && o.SampleAgeMs >= 0 && o.SampleAgeMs <= maxAge.Milliseconds()
}
