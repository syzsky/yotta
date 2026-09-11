package pluginmanager

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/httpegress"
	"github.com/yottaapp/yotta/internal/services"
	"github.com/yottaapp/yotta/internal/targetruntime"
	"github.com/yottaapp/yotta/sdk/plugin/packaging"
)

func TestRunServicesPrepareOnlyReferencedOwnedCompanion(t *testing.T) {
	var probes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probes.Add(1)
		_, _ = w.Write([]byte(`{"protocol":"test/v1","status":"ready"}`))
	}))
	defer server.Close()
	root := t.TempDir()
	app, err := services.OpenApp(filepath.Join(root, "settings.json"), filepath.Join(root, "logs"), nil, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(filepath.Join(root, "store"), nil, app, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	c := packaging.Companion{ID: "capture", Name: "Capture", Executable: "bin/helper.exe", ApplicationSlot: "capture-app", NetworkSlot: "position", Origin: server.URL, HealthPath: "/health", StopPath: "/stop", Protocol: "test/v1"}
	if err := m.Import(buildArchive(t, root, "1.0.0", key, c)); err != nil {
		t.Fatal(err)
	}
	profile, err := httpegress.SealProfile(httpegress.ProfileDraft{Origin: server.URL, ResponseByteLimit: 65536, TimeoutMilliseconds: 1000})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := httpegress.NewProvider(profile)
	if err != nil {
		t.Fatal(err)
	}
	makeTargets := func(origin string) targetruntime.Snapshot {
		r := m.records[testID]
		exe, args := companionPaths(c, r.root)
		snapshot, err := targetruntime.NewSnapshot([]targetruntime.Installation{
			{Slot: "position", TargetID: "position", Provider: provider, Configuration: targetruntime.Configuration{Origin: origin}},
			{Slot: "capture-app", TargetID: "capture-app", Provider: provider, Configuration: targetruntime.Configuration{Executable: exe, Arguments: args}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	targets := makeTargets(server.URL)
	ids, err := m.prepareRunServices(context.Background(), []string{"unrelated"}, targets)
	if err != nil || len(ids) != 0 || probes.Load() != 0 {
		t.Fatalf("unrelated ids=%v probes=%v err=%v", ids, probes.Load(), err)
	}
	for range 2 {
		ids, err = m.prepareRunServices(context.Background(), []string{"position", "position"}, targets)
		if err != nil || len(ids) != 1 || ids[0] != testID {
			t.Fatalf("owned ids=%v err=%v", ids, err)
		}
	}
	if probes.Load() != 2 {
		t.Fatalf("duplicate preparations: %d", probes.Load())
	}
	_, _, err = app.MutateSettings(func(s *services.Settings) error { s.Network.HTTPOrigins[0].Origin = "http://127.0.0.1:1"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	ids, err = m.prepareRunServices(context.Background(), []string{"position"}, targets)
	if err != nil || len(ids) != 1 || probes.Load() != 3 {
		t.Fatalf("leased configuration changed with live settings: %v %v", ids, err)
	}
	_, _, err = app.MutateSettings(func(s *services.Settings) error { s.Network.HTTPOrigins[0].Origin = server.URL; return nil })
	if err != nil {
		t.Fatal(err)
	}
	ids, err = m.prepareRunServices(context.Background(), []string{"position"}, makeTargets("http://127.0.0.1:1"))
	if err != nil || len(ids) != 0 || probes.Load() != 3 {
		t.Fatalf("custom snapshot started live companion: %v %v", ids, err)
	}
	// Disable the package without invoking its external stop endpoint.
	if _, err := m.store.Disable(testID); err != nil {
		t.Fatal(err)
	}
	_, err = m.prepareRunServices(context.Background(), []string{"position"}, targets)
	if err == nil || apperr.Project(err).ID != "plugins.disabled" {
		t.Fatalf("disabled companion=%v", err)
	}
}
