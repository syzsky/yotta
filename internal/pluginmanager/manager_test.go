package pluginmanager

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"github.com/rs/zerolog"
	"github.com/yottaapp/yotta/internal/services"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/nodepackage"
	"github.com/yottaapp/yotta/sdk/plugin/authoring"
	"github.com/yottaapp/yotta/sdk/plugin/packaging"
)

const testNamespace = "https://plugins.example.test/example"
const testID = testNamespace + "/packages/example/v1"

func buildArchive(t *testing.T, root, version string, key ed25519.PrivateKey, companions ...packaging.Companion) string {
	t.Helper()
	schemaID := testNamespace + "/config"
	contract, err := authoring.Seal(authoring.Draft{NodeTypeID: testNamespace + "/nodes/example", Version: "1.0.0", ConfigSchemaRoot: schemaID, ConfigSchemaBundle: []authoring.SchemaResource{{ID: schemaID, Schema: json.RawMessage(`{"$id":"` + schemaID + `","$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false}`)}}, Ports: authoring.PortSet{ExecInputs: []authoring.SignalPort{{ID: "in"}}, ExecOutputs: []authoring.SignalPort{{ID: "done"}}}, Execution: authoring.ExecutionSpec{Class: authoring.ExecutionEffect, Effects: []authoring.EffectID{testNamespace + "/effects/read/v1"}, Determinism: authoring.Recorded, Evaluation: authoring.EvaluationPush, Cache: authoring.CacheNone, Retry: authoring.RetryNever, Cancellation: authoring.CancellationCooperative, Timeout: authoring.TimeoutRequired}, Instruction: authoring.Invoke(), ImplementationABI: []authoring.ABIRequirement{{Kind: authoring.ABIProcess, Version: "v1"}}, Authoring: authoring.Authoring{TitleKey: "plugin.example.title"}})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, version+".ynp")
	_, err = packaging.Write(out, packaging.Build{Namespace: testNamespace, PackageID: testID, Version: version, Descriptor: packaging.Descriptor{Companions: companions, Name: "Example plugin", Messages: map[string]map[string]string{"en": {"plugin.example.title": "Example"}, "zh": {"plugin.example.title": "示例"}}}, Nodes: []packaging.Node{{Contract: contract, PayloadPath: "bin/node.exe", Entrypoint: "example", OperatingSystems: []string{"windows"}, Architectures: []string{"amd64"}}}, Files: map[string][]byte{"bin/node.exe": []byte("node " + version), "bin/helper.exe": []byte("helper " + version)}}, key)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestImportLifecycleRetainsStateAndRejectsPublisherReplacement(t *testing.T) {
	root := t.TempDir()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	m, err := New(filepath.Join(root, "store"), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	first := buildArchive(t, root, "1.0.0", key)
	if err = m.Import(first); err != nil {
		t.Fatal(err)
	}
	views, err := m.List()
	if err != nil || len(views) != 1 || !views[0].Enabled || views[0].Loaded || !views[0].RestartRequired {
		t.Fatalf("views=%+v err=%v", views, err)
	}
	if m.Messages()["zh"]["plugin.example.title"] != "示例" {
		t.Fatal("messages missing")
	}
	if err = m.SetEnabled(testID, false); err != nil {
		t.Fatal(err)
	}
	reopened, err := nodepackage.OpenStore(context.Background(), filepath.Join(root, "store"))
	if err != nil || reopened.List()[0].Enabled {
		t.Fatal("disabled state not persisted", err)
	}
	second := buildArchive(t, root, "1.1.0", key)
	if err = m.Import(second); err != nil {
		t.Fatal(err)
	}
	if err = m.Import(second); err != nil {
		t.Fatal(err)
	}
	if err = m.Rollback(testID); err != nil {
		t.Fatal(err)
	}
	views, _ = m.List()
	if views[0].Version != "1.0.0" {
		t.Fatal("rollback did not restore version")
	}
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	changed := buildArchive(t, root, "1.2.0", other)
	if e := m.Import(changed); e == nil || apperr.Project(e).ID != "plugins.publisher_changed" {
		t.Fatalf("publisher replacement=%v", e)
	}
	views, _ = m.List()
	if views[0].Version != "1.0.0" {
		t.Fatal("failed update damaged previous install")
	}
	if err = m.Uninstall(testID); err != nil {
		t.Fatal(err)
	}
	views, _ = m.List()
	if len(views) != 0 {
		t.Fatal("uninstall did not remove package")
	}
}
func TestTamperedCompanionDoesNotChangeExistingInstallation(t *testing.T) {
	root := t.TempDir()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	m, _ := New(filepath.Join(root, "store"), nil, nil, nil)
	archive := buildArchive(t, root, "1.0.0", key)
	if err := m.Import(archive); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	f, err := os.Create(filepath.Join(root, "tampered.ynp"))
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(f)
	for _, entry := range reader.File {
		r, _ := entry.Open()
		w, _ := writer.Create(entry.Name)
		if entry.Name == "bin/helper.exe" {
			_, _ = w.Write([]byte("modified"))
		} else {
			_, _ = io.Copy(w, r)
		}
		r.Close()
	}
	writer.Close()
	f.Close()
	err = m.Import(filepath.Join(root, "tampered.ynp"))
	if err == nil || apperr.Project(err).ID != "plugins.invalid_package" {
		t.Fatalf("tampered=%v", err)
	}
	if strings.Contains(string(apperr.Marshal(err)), root) {
		t.Fatal("private path leaked in RPC error")
	}
	views, _ := m.List()
	if len(views) != 1 || views[0].Version != "1.0.0" {
		t.Fatal("existing install changed")
	}
}

func TestFailedRollbackPreservesEnabledReleaseAndUserConfiguration(t *testing.T) {
	root := t.TempDir()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	app, err := services.OpenApp(filepath.Join(root, "settings.json"), filepath.Join(root, "logs"), nil, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	companion := packaging.Companion{ID: "capture", Name: "Capture", Executable: "bin/helper.exe", ApplicationSlot: "test-capture", NetworkSlot: "test-data", Origin: "http://127.0.0.1:1", HealthPath: "/health", StopPath: "/stop", Protocol: "test/v1"}
	m, _ := New(filepath.Join(root, "store"), nil, app, nil)
	for _, version := range []string{"1.0.0", "1.1.0"} {
		archive := buildArchive(t, root, version, key, companion)
		if err = m.Import(archive); err != nil {
			t.Fatal(err)
		}
	}
	custom := filepath.Join(root, "user-program.exe")
	_, _, err = app.MutateSettings(func(s *services.Settings) error { s.Applications.Profiles[0].Executable = custom; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Rollback(testID); err == nil {
		t.Fatal("conflicting user configuration was overwritten")
	}
	views, _ := m.List()
	if views[0].Version != "1.1.0" || !views[0].Enabled {
		t.Fatal("failed rollback changed the active release", views)
	}
	if app.Settings().Applications.Profiles[0].Executable != custom {
		t.Fatal("user configuration lost")
	}
}

func TestRestartOnlyRequiredForEnabledUnavailableGenerations(t *testing.T) {
	root := t.TempDir()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	m, _ := New(filepath.Join(root, "store"), nil, nil, nil)
	archive := buildArchive(t, root, "1.0.0", key)
	if err := m.Import(archive); err != nil {
		t.Fatal(err)
	}
	if !m.NeedsRestart() {
		t.Fatal("new package was reported as loaded")
	}
	m.loaded[testID] = m.store.List()[0].Current
	if m.NeedsRestart() {
		t.Fatal("matching process catalog requires restart")
	}
	if err := m.SetEnabled(testID, false); err != nil {
		t.Fatal(err)
	}
	if m.NeedsRestart() {
		t.Fatal("disable is already effective and must not require restart")
	}
	raw := []byte(`{"dependencies":[{"packageId":"` + testID + `"}]}`)
	if _, err := m.checkSource(raw); err == nil || apperr.Project(err).ID != "plugins.disabled" {
		t.Fatal("disabled package was not blocked", err)
	}
	if err := m.SetEnabled(testID, true); err != nil {
		t.Fatal(err)
	}
	if m.NeedsRestart() {
		t.Fatal("enabling the cached version must be immediate")
	}
	if err := m.Uninstall(testID); err != nil {
		t.Fatal(err)
	}
	if m.NeedsRestart() {
		t.Fatal("removed package incorrectly requires restart")
	}
	if err := m.Import(archive); err != nil {
		t.Fatal(err)
	}
	if m.NeedsRestart() {
		t.Fatal("exact reinstallation left a stale restart warning")
	}
}

func TestBatchRetainsPartialResultsAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	m, _ := New(filepath.Join(root, "store"), nil, nil, nil)
	archive := buildArchive(t, root, "1.0.0", key)
	if err := m.Import(archive); err != nil {
		t.Fatal(err)
	}
	results, err := m.Batch("disable", []string{"missing", testID, testID})
	if err != nil || len(results) != 2 {
		t.Fatalf("batch=%+v %v", results, err)
	}
	if results[0].Succeeded || results[0].Problem == nil || results[0].Problem.ID != "plugins.not_found" || results[0].Problem.OperationID == "" {
		t.Fatal("failure lost its structured evidence", results[0])
	}
	if !results[1].Succeeded {
		t.Fatal("one failure prevented the next item")
	}
	views, _ := m.List()
	if views[0].Enabled {
		t.Fatal("successful disable did not persist")
	}
	if _, err = m.Batch("invalid", []string{testID}); err == nil {
		t.Fatal("unknown action accepted")
	}
	results, err = m.Batch("uninstall", []string{testID, "missing"})
	if err != nil || len(results) != 2 || !results[0].Succeeded || results[1].Succeeded {
		t.Fatalf("uninstall=%+v %v", results, err)
	}
}
