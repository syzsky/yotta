package noderuntime

import (
	"encoding/json"
	"fmt"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"testing"
	"time"
)

func TestNavigationConfigReadsCanonicalNumbers(t *testing.T) {
	i := nodeadapter.Invocation{Config: map[string]any{"axisHeading": json.Number("90"), "axisSign": json.Number("-1")}}
	if navNumber(i, "axisHeading", 0) != 90 || navNumber(i, "axisSign", 1) != -1 {
		t.Fatal("canonical axis configuration was ignored")
	}
}

func TestNavigationPoseRequiresFreshValidMappedFields(t *testing.T) {
	now := time.Now()
	i := nodeadapter.Invocation{Config: map[string]any{"yField": "z"}}
	valid := fmt.Sprintf(`{"valid":true,"x":12,"z":34,"cameraHeading":359,"sampleTimeMs":%d}`, now.UnixMilli())
	p, err := decodeNavigationPose([]byte(valid), i, now)
	if err != nil || p.X != 12 || p.Y != 34 || p.Heading != 359 {
		t.Fatalf("%+v %v", p, err)
	}
	for _, raw := range []string{
		`{"valid":false}`, `{}`, fmt.Sprintf(`{"valid":true,"x":12,"z":34,"cameraHeading":359,"sampleTimeMs":%d}`, now.Add(-2*time.Second).UnixMilli()),
		fmt.Sprintf(`{"valid":true,"x":null,"z":34,"cameraHeading":359,"sampleTimeMs":%d}`, now.UnixMilli()),
	} {
		if _, err := decodeNavigationPose([]byte(raw), i, now); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
