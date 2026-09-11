package noderuntime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/blob"
	"github.com/yottaapp/yotta/internal/noderuntime"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/resource"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/workflow/compiler"
)

type turningTemplateProvider struct {
	templateAutomationProvider
	turns  []float64
	reveal *image.RGBA
}

func (p *turningTemplateProvider) Open(ctx context.Context, r resource.ProviderOpenRequest) (any, error) {
	if r.Kind == installed.KindInput {
		return installed.OperationTurnView, nil
	}
	return p.templateAutomationProvider.Open(ctx, r)
}
func (p *turningTemplateProvider) Invoke(ctx context.Context, object any, op string, payload []byte) ([]byte, error) {
	if op == installed.OperationTurnView {
		var req installed.TurnViewRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		p.turns = append(p.turns, req.Degrees)
		if len(p.turns) == 2 && p.reveal != nil {
			p.frame = p.reveal
		}
		return []byte(`{}`), nil
	}
	return p.templateAutomationProvider.Invoke(ctx, object, op, payload)
}

func TestTurnSearchObservesBeforeTurningAndRespectsSweepLimit(t *testing.T) {
	b, err := nodes.Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, found := range []bool{true, false} {
		t.Run(map[bool]string{true: "found", false: "exhausted"}[found], func(t *testing.T) {
			template := image.NewRGBA(image.Rect(0, 0, 8, 8))
			for y := range 8 {
				for x := range 8 {
					template.SetRGBA(x, y, color.RGBA{R: uint8(x * 25), G: uint8(y * 23), B: uint8((x + y) * 12), A: 255})
				}
			}
			frame := image.NewRGBA(image.Rect(0, 0, 40, 30))
			draw.Draw(frame, image.Rect(10, 10, 18, 18), template, image.Point{}, draw.Src)
			store, err := blob.Open(t.TempDir(), blob.Limits{MaxBlobBytes: 1 << 20, MaxTotalBytes: 2 << 20})
			if err != nil {
				t.Fatal(err)
			}
			ref, err := store.Put(context.Background(), "image/png", bytes.NewReader(encodeVisionPNG(t, template)))
			if err != nil {
				t.Fatal(err)
			}
			blobProvider, err := blob.NewProvider(store, blob.ProviderLimits{MaxChunkBytes: 64 << 10, QueueCapacity: 4})
			if err != nil {
				t.Fatal(err)
			}
			bindings := map[string]any{"template": map[string]any{"kind": "blob", "blob": ref}}
			for k, v := range map[string]any{"region": map[string]any{"x": 0, "y": 0, "width": 1, "height": 1, "unit": "ratio"}, "step": 7, "max-angle": 10, "settle": 0, "timeout": 5000, "threshold": 0.95} {
				bindings[k] = map[string]any{"kind": "value", "value": v}
			}
			program := compilePrimitiveProgram(t, b, navigationSource(t, b, nodes.TurnFindTemplateNodeID, map[string]any{"slot": "game"}, bindings))
			p := &turningTemplateProvider{templateAutomationProvider: templateAutomationProvider{frame: image.NewRGBA(image.Rect(0, 0, 40, 30))}}
			if found {
				p.reveal = frame
			}
			targets := configuredTargetRun(t, "game", "automation-target/search", p)
			now := time.Now().UTC()
			_, owner, journal := admittedExecution(t, b, program, map[string]run.InstalledProvider{blob.ProviderID: {ArtifactDigest: blobProviderDigest(t), ABI: blob.ProviderABI, Provider: blobProvider}}, now)
			defer owner.Close(context.Background())
			adapters, err := noderuntime.Installed(b, testDependencies())
			if err != nil {
				t.Fatal(err)
			}
			result, err := compiler.NewExecutor(b.Catalog, adapters, compiler.ExecutorOptions{Now: func() time.Time { return now }}).RunWithTargets(context.Background(), program, owner, targets, journal)
			if err != nil {
				t.Fatal(err)
			}
			var matched bool
			if err := json.Unmarshal(result.NodeOutputs["navigation"]["matched"].InlineJSON(), &matched); err != nil {
				t.Fatal(err)
			}
			if matched != found || p.captures != 3 || len(p.turns) != 2 || p.turns[0] != 7 || p.turns[1] != 3 || p.closed != 2 {
				t.Fatalf("matched=%v captures=%d turns=%v closed=%d", matched, p.captures, p.turns, p.closed)
			}
		})
	}
}
