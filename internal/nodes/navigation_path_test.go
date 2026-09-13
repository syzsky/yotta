package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/yottaapp/yotta/internal/navigationpath"
)

func TestPathOperationsRejectInvalidRangesAndMixedFrames(t *testing.T) {
	b, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	p := navigationpath.Path{Version: 1, Reference: navigationpath.Reference{Kind: "world", Frame: "test", Unit: "raw", AxisSign: 1}, Points: []navigationpath.Point{{ID: "a"}, {ID: "b", X: 2}}}
	raw, _ := json.Marshal(p)
	for _, op := range []string{"slice-path", "path-point"} {
		d, _ := b.Definition(PathNodePrefix + op)
		_, err := d.EvaluateInline(context.Background(), map[string]json.RawMessage{"path": raw, "start": json.RawMessage(`-1`), "end": json.RawMessage(`9`), "index": json.RawMessage(`5`)}, nil)
		var failure *InlineFailure
		if !errors.As(err, &failure) || failure.Code != PathInvalidCode {
			t.Fatal("missing declared failure", err)
		}
	}
	q := p.Clone()
	q.Reference.Frame = "other"
	paths, _ := json.Marshal([]navigationpath.Path{p, q})
	d, _ := b.Definition(PathNodePrefix + "join-path")
	if _, err := d.EvaluateInline(context.Background(), map[string]json.RawMessage{"paths": paths}, nil); err == nil {
		t.Fatal("joined incompatible coordinate systems")
	}
}
