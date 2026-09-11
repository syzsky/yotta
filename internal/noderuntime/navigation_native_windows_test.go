//go:build windows

package noderuntime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/blob"
	"github.com/yottaapp/yotta/internal/httpegress"
	"github.com/yottaapp/yotta/internal/noderuntime"
	"github.com/yottaapp/yotta/internal/nodes"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/targetruntime"
	"github.com/yottaapp/yotta/internal/workflow/compiler"
)

// TestNavigationWorkflowDesktopSmoke runs the real compiler, configured providers
// and executor. It is opt-in and never reads or changes the user's catalog/settings.
func TestNavigationWorkflowDesktopSmoke(t *testing.T) {
	profilePath, requestPath := os.Getenv("YOTTA_NAVIGATION_SMOKE_PROFILE"), os.Getenv("YOTTA_NAVIGATION_SMOKE_REQUEST")
	if profilePath == "" || requestPath == "" {
		t.Skip("requires an explicit desktop profile and bounded workflow request")
	}
	profileRaw, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	var profile installed.DesktopProfilePayload
	if err := json.Unmarshal(profileRaw, &profile); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	var req struct {
		NodeID   string         `json:"nodeId"`
		Origin   string         `json:"origin"`
		Config   map[string]any `json:"config"`
		Inputs   map[string]any `json:"inputs"`
		Template string         `json:"template"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if req.NodeID != nodes.MoveCharacterNodeID && req.NodeID != nodes.TurnFindTemplateNodeID {
		t.Fatal("unsupported smoke node")
	}
	if timeout, ok := req.Inputs["timeout"].(float64); !ok || timeout <= 0 || timeout > 15000 {
		t.Fatal("smoke requires timeout <=15000 ms")
	}
	b, err := nodes.Build()
	if err != nil {
		t.Fatal(err)
	}
	bindings := map[string]any{}
	for key, value := range req.Inputs {
		bindings[key] = map[string]any{"kind": "value", "value": value}
	}
	providers := map[string]run.InstalledProvider{}
	if req.Template != "" {
		data, err := os.ReadFile(req.Template)
		if err != nil {
			t.Fatal(err)
		}
		store, err := blob.Open(t.TempDir(), blob.Limits{MaxBlobBytes: 8 << 20, MaxTotalBytes: 16 << 20})
		if err != nil {
			t.Fatal(err)
		}
		ref, err := store.Put(context.Background(), "image/png", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		provider, err := blob.NewProvider(store, blob.ProviderLimits{MaxChunkBytes: 64 << 10, QueueCapacity: 4})
		if err != nil {
			t.Fatal(err)
		}
		providers[blob.ProviderID] = run.InstalledProvider{ArtifactDigest: blobProviderDigest(t), ABI: blob.ProviderABI, Provider: provider}
		bindings["template"] = map[string]any{"kind": "blob", "blob": ref}
	}
	program := compilePrimitiveProgram(t, b, navigationSource(t, b, req.NodeID, req.Config, bindings))
	installations, err := installed.Install([]installed.InstallationDraft{{Slot: "game", Label: "Navigation smoke", Profile: installed.NewDesktopProfileDraft(profile)}})
	if err != nil {
		t.Fatal(err)
	}
	defer installations.Close()
	entry := installations.Entries()[0]
	entries := []targetruntime.Installation{{Slot: "game", TargetID: entry.TargetID, Provider: entry.Provider}}
	if req.Origin != "" {
		profile, err := httpegress.SealProfile(httpegress.ProfileDraft{Origin: req.Origin, ResponseByteLimit: 65536, TimeoutMilliseconds: 1000})
		if err != nil {
			t.Fatal(err)
		}
		provider, err := httpegress.NewProvider(profile)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, targetruntime.Installation{Slot: "position", TargetID: "http-target/navigation-smoke", Provider: provider})
	}
	snapshot, err := targetruntime.NewSnapshot(entries)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := snapshot.NewRun()
	if err != nil {
		t.Fatal(err)
	}
	defer targets.Close(context.Background())
	now := time.Now().UTC()
	_, owner, journal := admittedExecution(t, b, program, providers, now)
	defer owner.Close(context.Background())
	adapters, err := noderuntime.Installed(b, testDependencies())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := compiler.NewExecutor(b.Catalog, adapters, compiler.ExecutorOptions{Now: func() time.Time { return time.Now().UTC() }}).RunWithTargets(ctx, program, owner, targets, journal)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range result.NodeOutputs["navigation"] {
		t.Logf("%s=%s", key, value.InlineJSON())
	}
	if req.NodeID == nodes.MoveCharacterNodeID {
		var remaining float64
		if err := json.Unmarshal(result.NodeOutputs["navigation"]["distance"].InlineJSON(), &remaining); err != nil {
			t.Fatal(err)
		}
		if remaining > req.Inputs["tolerance"].(float64) {
			t.Fatalf("destination not reached: distance=%v", remaining)
		}
	}
}
