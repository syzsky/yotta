package noderuntime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yottaapp/yotta/sdk/plugin/positionsource"
)

func TestPositionSourceNavigationRequirements(t *testing.T) {
	s := positionsource.Snapshot{Descriptor: positionsource.Descriptor{Protocol: positionsource.Protocol, Source: "test",
		Capabilities: []positionsource.Capability{positionsource.Position, positionsource.CameraHeading},
		Frame:        positionsource.Frame{ID: "test/world", Unit: "raw", AxisHeading: 90, AxisSign: 1}}, Epoch: "session-a",
		Observations: map[positionsource.Capability]positionsource.Observation{
			positionsource.Position:      {Status: "tracking", Value: json.RawMessage(`{"x":12,"y":34}`), SampleTimeMs: 100000, SampleAgeMs: 200, Sequence: 7},
			positionsource.CameraHeading: {Status: "tracking", Value: json.RawMessage(`90`), SampleTimeMs: 100100, SampleAgeMs: 100, Sequence: 9},
		}}
	raw, _ := json.Marshal(s)
	p, err := decodePositionSource(raw, time.UnixMilli(100200))
	if err != nil || !p.Valid || p.X != 12 || p.Y != 34 || p.Heading != 90 || p.Frame != "test/world" || p.Unit != "raw" || p.AxisHeading != 90 || p.Epoch != "session-a" || p.Sequence != 7 || p.ReceivedAt != 100000 {
		t.Fatalf("wrong projection: %+v %v", p, err)
	}
	p, err = decodePositionSource(raw, time.UnixMilli(102000))
	if err != nil || p.Valid {
		t.Fatal("delayed response accepted")
	}
	s.Observations[positionsource.CameraHeading] = positionsource.Observation{Status: "waiting", Value: json.RawMessage(`null`), SampleAgeMs: -1}
	raw, _ = json.Marshal(s)
	p, err = decodePositionSource(raw, time.UnixMilli(100200))
	if err != nil || p.Valid {
		t.Fatal("missing heading accepted for navigation")
	}
	delete(s.Observations, positionsource.CameraHeading)
	s.Capabilities = []positionsource.Capability{positionsource.Position}
	raw, _ = json.Marshal(s)
	p, err = decodePositionSource(raw, time.UnixMilli(100200))
	if err != nil || p.Valid {
		t.Fatal("position-only source accepted for camera navigation")
	}
}
