package workflowbundle

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/yottaapp/yotta/internal/blob"
	"github.com/yottaapp/yotta/internal/panel"
	"github.com/yottaapp/yotta/internal/workflow/schema"
)

func TestBundleIncludesPanelAndInstallsIndependentBindings(t *testing.T) {
	ctx := context.Background()
	store, blobs := openTestWorkspace(t)
	panels, _ := panel.Open(nil, filepath.Join(t.TempDir(), "panels.json"))
	defer panels.Close()
	d, err := panels.Save(panel.Draft{Title: "Dashboard", Components: []panel.ComponentDraft{{Kind: "number", Title: "Value", Initial: float64(42)}}})
	if err != nil {
		t.Fatal(err)
	}
	var source schema.WorkflowSource
	if err := json.Unmarshal(testSource("publisher", "With panel", blob.BlobRef{}), &source); err != nil {
		t.Fatal(err)
	}
	if err := schema.SetTargetDefault(&source, "panel", d.ID); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(source)
	original, err := store.Save(ctx, raw, -1)
	if err != nil {
		t.Fatal(err)
	}
	exporter, _ := New(testSourceRepository{store}, blobs, panels)
	archive := filepath.Join(t.TempDir(), "panel.yotta-workflow")
	result, err := exporter.Export(ctx, original.WorkflowID(), archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Info.Panels) != 1 || result.Info.Panels[0].Managed.Components[0].Initial != float64(42) {
		t.Fatal("definition missing")
	}
	entries := readTestZip(t, archive)
	var manifest Manifest
	_ = json.Unmarshal(entries[ManifestPath], &manifest)
	if manifest.Version != 3 {
		t.Fatal("panel bundle has no version fence")
	}
	manifest.Version = 2
	entries[ManifestPath], _ = json.Marshal(manifest)
	invalid := filepath.Join(t.TempDir(), "invalid.yotta-workflow")
	writeTestZip(t, invalid, entries)
	if _, err := exporter.Inspect(ctx, invalid); err == nil {
		t.Fatal("old format silently accepted panel resources")
	}
	consumerStore, consumerBlobs := openTestWorkspace(t)
	consumerPanels, _ := panel.Open(nil, "")
	defer consumerPanels.Close()
	importer, _ := New(testSourceRepository{consumerStore}, consumerBlobs, consumerPanels)
	installed, err := importer.Import(ctx, ImportRequest{Path: archive, Mode: ImportRegistry})
	if err != nil {
		t.Fatal(err)
	}
	var local schema.WorkflowSource
	_ = json.Unmarshal(installed.Source.Artifact(), &local)
	localID, _ := schema.TargetDefaultSlot(local, "panel")
	if localID == d.ID {
		t.Fatal("retained publisher-local identity")
	}
	definition, err := consumerPanels.Edit(localID)
	if err != nil || definition.Components[0].ID != d.Components[0].ID {
		t.Fatal("component binding lost", err)
	}
	clone, err := importer.Import(ctx, ImportRequest{Path: archive, Mode: ImportCopy})
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(clone.Source.Artifact(), &local)
	cloneID, _ := schema.TargetDefaultSlot(local, "panel")
	if cloneID == localID {
		t.Fatal("cloned workflow shared installation")
	}
	if _, _, err := importer.ExportBytes(ctx, installed.Source.WorkflowID()); err != nil {
		t.Fatal("installed workflow cannot be re-exported", err)
	}
	if err := panels.Delete(d.ID, d.Revision); err != nil {
		t.Fatal(err)
	}
	if _, _, err := exporter.ExportBytes(ctx, original.WorkflowID()); err == nil {
		t.Fatal("missing panel silently exported")
	}
}
