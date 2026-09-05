package runprepare

import (
	"context"
	"github.com/yottaapp/yotta/internal/workflow/schema"
	"testing"
)

func TestTemplatePreviewUsesRuntimeVariantPolicy(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	small, large := putPattern(t, store, 30, 15), putPattern(t, store, 40, 20)
	planner, _ := New(store)
	variants := []schema.ImageResourceVariant{
		{ID: "1080p", Resolution: [2]int{1920, 1080}, BBox: [4]int{0, 0, 30, 15}, Blob: small},
		{ID: "2k", Resolution: [2]int{2560, 1440}, BBox: [4]int{0, 0, 40, 20}, Blob: large},
	}
	for _, candidates := range [][]schema.ImageResourceVariant{variants, variants[:1]} {
		runtime, err := planner.Prepare(ctx, sourceArtifact(t, candidates), map[string][2]int{"game": {2560, 1440}})
		if err != nil {
			t.Fatal(err)
		}
		preview, release, err := planner.PrepareTemplate(ctx, candidates, [2]int{2560, 1440})
		if err != nil {
			t.Fatal(err)
		}
		if preview != runtime.Overrides[0].Blob {
			t.Fatal("preview and runtime selected different template content")
		}
		release()
		runtime.Release()
	}
}
