// Package navigationpath owns durable, source-independent coordinate paths.
package navigationpath

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
)

const Version = 1

// Reference describes a persistent coordinate frame. Observation session epochs
// deliberately do not belong here. Local frames require explicit alignment.
type Reference struct {
	Kind        string  `json:"kind"`
	Frame       string  `json:"frame"`
	Unit        string  `json:"unit"`
	AxisHeading float64 `json:"axisHeading"`
	AxisSign    int     `json:"axisSign"`
	Map         string  `json:"map"`
	Floor       string  `json:"floor"`
}

type Point struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	X    float64  `json:"x"`
	Y    float64  `json:"y"`
	Z    *float64 `json:"z"`
}

type Path struct {
	Version   int       `json:"version"`
	Reference Reference `json:"reference"`
	Points    []Point   `json:"points"`
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (r Reference) Validate() error {
	if (r.Kind != "world" && r.Kind != "local") || r.Frame == "" || r.Unit == "" || len(r.Frame) > 128 || len(r.Unit) > 32 || !finite(r.AxisHeading) || r.AxisHeading < 0 || r.AxisHeading >= 360 || (r.AxisSign != 1 && r.AxisSign != -1) {
		return errors.New("invalid path coordinate reference")
	}
	return nil
}

func (p Path) Validate() error {
	if p.Version != Version {
		return errors.New("unsupported path version")
	}
	if err := p.Reference.Validate(); err != nil {
		return err
	}
	if p.Points == nil {
		return errors.New("path points must be an array")
	}
	seen := make(map[string]bool, len(p.Points))
	for i, point := range p.Points {
		if point.ID == "" || len(point.ID) > 128 || seen[point.ID] || !finite(point.X) || !finite(point.Y) || (point.Z != nil && !finite(*point.Z)) {
			return fmt.Errorf("invalid path point at index %d", i)
		}
		seen[point.ID] = true
	}
	return nil
}

func Decode(raw []byte) (Path, error) {
	var p Path
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, errors.New("invalid path JSON")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return p, errors.New("trailing path JSON")
	}
	// Zero is a legitimate coordinate; omitted coordinates must not become zero.
	var fields struct {
		Reference map[string]json.RawMessage   `json:"reference"`
		Points    []map[string]json.RawMessage `json:"points"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return p, err
	}
	for _, key := range []string{"kind", "frame", "unit", "axisHeading", "axisSign"} {
		if v, ok := fields.Reference[key]; !ok || string(v) == "null" {
			return p, errors.New("missing path reference field")
		}
	}
	for _, point := range fields.Points {
		for _, key := range []string{"id", "x", "y"} {
			if v, ok := point[key]; !ok || string(v) == "null" {
				return p, errors.New("missing path point field")
			}
		}
	}
	return p, p.Validate()
}

func (p Path) Clone() Path {
	r := p
	r.Points = make([]Point, len(p.Points))
	copy(r.Points, p.Points)
	for i := range r.Points {
		if z := r.Points[i].Z; z != nil {
			value := *z
			r.Points[i].Z = &value
		}
	}
	return r
}

func (p Path) Reverse() Path {
	r := p.Clone()
	for a, b := 0, len(r.Points)-1; a < b; a, b = a+1, b-1 {
		r.Points[a], r.Points[b] = r.Points[b], r.Points[a]
	}
	return r
}

// Slice selects inclusive zero-based endpoints; an invalid selection is never
// silently clamped to a different route.
func (p Path) Slice(start, end int) (Path, error) {
	if err := p.Validate(); err != nil {
		return Path{}, err
	}
	if start < 0 || end < start || end >= len(p.Points) {
		return Path{}, errors.New("path point selection out of range")
	}
	r := p.Clone()
	r.Points = r.Points[start : end+1]
	return r, nil
}

func Join(paths ...Path) (Path, error) {
	if len(paths) == 0 {
		return Path{}, errors.New("no paths to join")
	}
	r := paths[0].Clone()
	for i, p := range paths {
		if err := p.Validate(); err != nil {
			return Path{}, err
		}
		if p.Reference != r.Reference {
			return Path{}, errors.New("path coordinate references do not match")
		}
		if i > 0 {
			r.Points = append(r.Points, p.Clone().Points...)
		}
	}
	return r, r.Validate()
}

// Align maps local XY through a counterclockwise coordinate-plane rotation and
// translation. Units must agree. This is explicit geometry, not localization.
func (p Path) Align(target Reference, origin Point, degrees float64) (Path, error) {
	if err := p.Validate(); err != nil {
		return Path{}, err
	}
	if err := target.Validate(); err != nil {
		return Path{}, err
	}
	if p.Reference.Kind != "local" || target.Kind != "world" || p.Reference.Unit != target.Unit || !finite(degrees) || !finite(origin.X) || !finite(origin.Y) || (origin.Z != nil && !finite(*origin.Z)) {
		return Path{}, errors.New("invalid local path alignment")
	}
	r := p.Clone()
	r.Reference = target
	a := degrees * math.Pi / 180
	c, s := math.Cos(a), math.Sin(a)
	for i := range r.Points {
		x, y := r.Points[i].X, r.Points[i].Y
		r.Points[i].X = origin.X + x*c - y*s
		r.Points[i].Y = origin.Y + x*s + y*c
		if r.Points[i].Z != nil {
			if origin.Z == nil {
				return Path{}, errors.New("height origin required for local altitude")
			}
			*r.Points[i].Z += *origin.Z
		}
	}
	return r, r.Validate()
}
