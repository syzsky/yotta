package positionsource

import (
	"encoding/json"
	"testing"
	"time"
)

func sample() Snapshot {
	return Snapshot{Descriptor: Descriptor{Protocol: Protocol, Source: "test.ocr",
		Capabilities: []Capability{Position, CameraHeading}, Frame: Frame{ID: "test/world", Unit: "pixel", AxisHeading: 90, AxisSign: 1}},
		Epoch: "session-1", Observations: map[Capability]Observation{
			Position:      {Status: "tracking", Value: json.RawMessage(`{"x":1,"y":2}`), SampleTimeMs: 100000, SampleAgeMs: 100, Sequence: 1},
			CameraHeading: {Status: "waiting", Value: json.RawMessage(`null`), SampleAgeMs: -1},
		}}
}

func TestPositionOnlyReadinessAndDelayedDelivery(t *testing.T) {
	s := sample()
	raw, _ := json.Marshal(s)
	s, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Ready(Position, time.UnixMilli(100100), time.Second) || s.Ready(CameraHeading, time.UnixMilli(100100), time.Second) {
		t.Fatal("support confused with availability")
	}
	if s.Ready(Position, time.UnixMilli(102000), time.Second) {
		t.Fatal("cached response made stale coordinates fresh")
	}
	if s.Ready(Position, time.UnixMilli(99000), time.Second) {
		t.Fatal("accepted future sample")
	}
	o := s.Observations[Position]
	o.SampleAgeMs = 2000
	s.Observations[Position] = o
	if s.Ready(Position, time.UnixMilli(100100), time.Second) {
		t.Fatal("ignored monotonic age")
	}
}

func TestRejectsBrokenCapabilityContracts(t *testing.T) {
	for _, change := range []func(*Snapshot){
		func(s *Snapshot) { delete(s.Observations, CameraHeading) },
		func(s *Snapshot) { s.Capabilities = append(s.Capabilities, Position) },
		func(s *Snapshot) { s.Frame.AxisSign = 0 },
		func(s *Snapshot) { s.Epoch = "" },
		func(s *Snapshot) {
			o := s.Observations[Position]
			o.Value = json.RawMessage(`{"x":1}`)
			s.Observations[Position] = o
		},
		func(s *Snapshot) {
			o := s.Observations[Position]
			o.Value = json.RawMessage(`null`)
			s.Observations[Position] = o
		},
		func(s *Snapshot) { o := s.Observations[Position]; o.Status = "waiting"; s.Observations[Position] = o },
		func(s *Snapshot) {
			s.Observations[CameraHeading] = Observation{Status: "tracking", Value: json.RawMessage(`360`), SampleTimeMs: 100000, Sequence: 1}
		},
	} {
		s := sample()
		change(&s)
		raw, _ := json.Marshal(s)
		if _, err := Decode(raw); err == nil {
			t.Fatalf("accepted invalid source: %s", raw)
		}
	}
	var document map[string]any
	raw, _ := json.Marshal(sample())
	_ = json.Unmarshal(raw, &document)
	delete(document["observations"].(map[string]any)["position"].(map[string]any), "sampleAgeMs")
	raw, _ = json.Marshal(document)
	if _, err := Decode(raw); err == nil {
		t.Fatal("missing age interpreted as fresh")
	}
}
