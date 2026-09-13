package navigationpath

import (
	"encoding/json"
	"math"
	"testing"
)

func fixture() Path {
	return Path{Version: 1, Reference: Reference{Kind: "world", Frame: "test/map", Unit: "raw", AxisHeading: 90, AxisSign: 1}, Points: []Point{{ID: "a", X: 1, Y: 2}, {ID: "b", X: 3, Y: 4}, {ID: "c", X: 5, Y: 6}}}
}

func TestPathRoundTripAndSelections(t *testing.T) {
	p := fixture()
	raw, _ := json.Marshal(p)
	decoded, err := Decode(raw)
	if err != nil || len(decoded.Points) != 3 {
		t.Fatal(decoded, err)
	}
	r := p.Reverse()
	r.Points[0].Name = "end"
	if p.Points[0].ID != "a" || p.Points[2].Name != "" || r.Points[0].ID != "c" {
		t.Fatal("operation mutated original")
	}
	r, err = p.Slice(1, 2)
	if err != nil || len(r.Points) != 2 || r.Points[0].ID != "b" {
		t.Fatal(r, err)
	}
	if _, err = p.Slice(-1, 2); err == nil {
		t.Fatal("clamped invalid selection")
	}
	if _, err = Join(p, p); err == nil {
		t.Fatal("duplicate stable point IDs accepted")
	}
	q := fixture()
	q.Reference.Frame = "other"
	if _, err = Join(p, q); err == nil {
		t.Fatal("mixed references accepted")
	}
}

func TestAlignmentIsExplicitAndPreservesGeometry(t *testing.T) {
	p := fixture()
	if _, err := p.Align(p.Reference, Point{}, 0); err == nil {
		t.Fatal("world path silently reanchored")
	}
	target := p.Reference
	p.Reference.Kind = "local"
	r, err := p.Align(target, Point{X: 100, Y: 200}, 90)
	if err != nil || math.Abs(r.Points[0].X-98) > 1e-8 || math.Abs(r.Points[0].Y-201) > 1e-8 || r.Points[0].ID != "a" {
		t.Fatal(r, err)
	}
	target.Unit = "meter"
	if _, err = p.Align(target, Point{}, 0); err == nil {
		t.Fatal("units silently converted")
	}
}

func TestRejectsMissingCoordinatesAndNonfiniteValues(t *testing.T) {
	p := fixture()
	p.Points[0].X = math.NaN()
	if p.Validate() == nil {
		t.Fatal("NaN accepted")
	}
	raw := []byte(`{"version":1,"reference":{"kind":"world","frame":"world","unit":"raw","axisHeading":0,"axisSign":1},"points":[{"id":"a","y":2}]}`)
	if _, err := Decode(raw); err == nil {
		t.Fatal("missing X became zero")
	}
	p = fixture()
	for i := 0; i < 20000; i++ {
		p.Points = append(p.Points, Point{ID: string(rune(i+1000)) + "-point"})
	}
	if err := p.Validate(); err != nil {
		t.Fatal("arbitrary waypoint count rejected", err)
	}
}
