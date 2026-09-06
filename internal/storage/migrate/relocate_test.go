package migrate

import (
	"bytes"
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/services"
	"github.com/yottaapp/yotta/internal/storage"
	"github.com/yottaapp/yotta/internal/storage/catalog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func relocationFixture(t *testing.T) (string, string, []byte) {
	t.Helper()
	base := t.TempDir()
	old := filepath.Join(base, "Yotta", "Yotta")
	next := filepath.Join(base, "yueli", "Yotta")
	p, err := storage.Open(context.Background(), storage.OpenOptions{Root: old})
	if err != nil {
		t.Fatal(err)
	}
	foundation, err := catalog.Open(context.Background(), p.Roots)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("unchanged canonical workflow fixture")
	hash, _ := artifact.Sum("yotta/test/workflow-source/v1", raw)
	now := time.Now().UTC()
	err = foundation.Workflows().Commit(context.Background(), -1, catalog.WorkflowSourceRecord{WorkflowID: "workflow-one", Name: "Example", Revision: 0, Hash: hash, Format: "yotta.workflow", Version: "1", Artifact: raw, CreatedAt: now, UpdatedAt: now}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = foundation.Close(); err != nil {
		t.Fatal(err)
	}
	store, settings, err := services.OpenSettingsStore(p.Roots.SettingsFile())
	if err != nil {
		t.Fatal(err)
	}
	settings.Applications.Profiles = append(settings.Applications.Profiles, services.InstalledApplicationSettings{Slot: "plugin", Label: "Plugin", Executable: filepath.Join(old, "packages", "collector.exe"), Arguments: []string{"--core=" + filepath.Join(old, "packages", "core.exe"), "unchanged"}})
	if err = store.Save(settings); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(p.Roots.Objects, "resource"), []byte("immutable resource"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = p.Close(); err != nil {
		t.Fatal(err)
	}
	return old, next, raw
}

func TestRelocationPreservesDatabasesResourcesSettingsAndAccountIdentity(t *testing.T) {
	old, next, original := relocationFixture(t)
	report, err := RelocateProfile(context.Background(), old, next)
	if err != nil {
		t.Fatal(err)
	}
	if report.SettingsPaths != 2 || report.Bytes == 0 || len(report.Files) == 0 {
		t.Fatalf("report=%+v", report)
	}
	if _, err = os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old profile not archived: %v", err)
	}
	if _, err = os.Stat(filepath.Join(report.Backup, "root.json")); err != nil {
		t.Fatal("backup missing", err)
	}
	roots, _ := storage.Resolve(next)
	_, settings, err := services.OpenSettingsStore(roots.SettingsFile())
	if err != nil {
		t.Fatal(err)
	}
	if settings.Applications.Profiles[0].Executable != filepath.Join(next, "packages", "collector.exe") || settings.Applications.Profiles[0].Arguments[0] != "--core="+filepath.Join(next, "packages", "core.exe") {
		t.Fatal("settings paths not relocated")
	}
	scope, err := storage.CredentialScope(roots)
	if err != nil || scope != filepath.Join(old, "data") {
		t.Fatalf("credential scope changed: %q %v", scope, err)
	}
	resource, err := os.ReadFile(filepath.Join(roots.Objects, "resource"))
	if err != nil || string(resource) != "immutable resource" {
		t.Fatal("resource bytes changed", err)
	}
	p, err := storage.Open(context.Background(), storage.OpenOptions{Root: next})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	f, err := catalog.Open(context.Background(), roots)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	record, found, err := f.Workflows().Get(context.Background(), "workflow-one")
	if err != nil || !found || !bytes.Equal(record.Artifact, original) {
		t.Fatal("workflow lost or changed", err)
	}
	if _, err = RelocateProfile(context.Background(), report.Backup, next); err == nil {
		t.Fatal("existing destination overwritten")
	}
}

func TestRelocationDoesNotModifySourceOnCancelInvalidSettingsOrBusyWriter(t *testing.T) {
	for _, mode := range []string{"cancel", "settings", "busy"} {
		t.Run(mode, func(t *testing.T) {
			old, next, _ := relocationFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel" {
				cancel()
			}
			if mode == "settings" {
				if err := os.WriteFile(filepath.Join(old, "config", "settings.json"), []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "busy" {
				held, err := storage.Open(ctx, storage.OpenOptions{Root: old})
				if err != nil {
					t.Fatal(err)
				}
				defer held.Close()
			}
			if _, err := RelocateProfile(ctx, old, next); err == nil {
				t.Fatal("invalid relocation succeeded")
			}
			if _, err := os.Stat(filepath.Join(old, "root.json")); err != nil {
				t.Fatal("source removed after failure", err)
			}
			if _, err := os.Stat(next); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed relocation published target", err)
			}
		})
	}
}

func TestRebaseOnlyChangesRootedPaths(t *testing.T) {
	old := filepath.Join(t.TempDir(), "old")
	next := filepath.Join(filepath.Dir(old), "new")
	for _, value := range []string{"https://example.test/old", old + "-other/file", "note " + old} {
		if _, changed := rebasePath(value, old, next); changed {
			t.Fatalf("non-path changed: %q", value)
		}
	}
	for _, value := range []string{old, filepath.Join(old, "file"), "--path=" + filepath.Join(old, "file")} {
		if _, changed := rebasePath(value, old, next); !changed {
			t.Fatalf("rooted path unchanged: %q", value)
		}
	}
}

func TestExplicitRootAndEnvironmentSkipDefaultRelocation(t *testing.T) {
	t.Setenv(storage.EnvironmentRoot, t.TempDir())
	if err := relocateDefault(context.Background(), Options{}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(storage.EnvironmentRoot, "")
	if err := relocateDefault(context.Background(), Options{Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultRelocationIsIdempotentAndDoesNotMergeExistingProfiles(t *testing.T) {
	old, next, _ := relocationFixture(t)
	if err := relocateDefaultRoots(context.Background(), Options{MaxRuns: 8}, old, next); err != nil {
		t.Fatal(err)
	}
	if err := relocateDefaultRoots(context.Background(), Options{MaxRuns: 8}, old, next); err != nil {
		t.Fatal(err)
	}
	newOnly := filepath.Join(next, "documents", "keep.txt")
	if err := os.WriteFile(newOnly, []byte("new data"), 0600); err != nil {
		t.Fatal(err)
	}
	recreated, err := storage.Open(context.Background(), storage.OpenOptions{Root: old})
	if err != nil {
		t.Fatal(err)
	}
	recreated.Close()
	if err = relocateDefaultRoots(context.Background(), Options{MaxRuns: 8}, old, next); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(newOnly)
	if err != nil || string(got) != "new data" {
		t.Fatal("existing profile changed", err)
	}
	if _, err = os.Stat(filepath.Join(old, "root.json")); err != nil {
		t.Fatal("existing secondary profile was moved", err)
	}
}
