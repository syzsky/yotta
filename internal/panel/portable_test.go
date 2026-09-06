package panel

import (
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/apperr"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableInstallationMergeIsolationAndRecovery(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "panels.json")
	s, err := Open(nil, path)
	if err != nil {
		t.Fatal(err)
	}
	resource := PortableResource{ID: "panel-source", Title: "Dashboard", Managed: &Draft{ID: "panel-source", Revision: 1, Title: "Dashboard", Components: []ComponentDraft{{ID: "value", Kind: "number", Title: "Value", Initial: float64(1), Icon: "i-tabler-star"}, {ID: "unused", Kind: "text", Title: "Old", Initial: ""}}}}
	install := func(workflow string, r PortableResource) string {
		t.Helper()
		var id string
		err := s.ImportResources(ctx, workflow, []PortableResource{r}, func(m map[string]string) error { id = m[r.ID]; return nil })
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	id := install("consumer", resource)
	clone := install("copy", resource)
	if id == clone {
		t.Fatal("clones share panel instances")
	}
	local, _ := s.Edit(id)
	local.Title = "My dashboard"
	local.Components[0].Icon = "i-tabler-bolt"
	local.Components[1].Title = "Keep me"
	if _, err := s.Save(local); err != nil {
		t.Fatal(err)
	}
	resource.Managed.Components = []ComponentDraft{{ID: "value", Kind: "number", Title: "Updated", Initial: float64(2), Icon: "i-tabler-target"}, {ID: "added", Kind: "toggle", Title: "New", Initial: false}}
	if updated := install("consumer", resource); updated != id {
		t.Fatal("update changed identity")
	}
	got, _ := s.Edit(id)
	if got.Title != "My dashboard" || got.Components[0].Title != "Updated" || got.Components[0].Icon != "i-tabler-bolt" || got.Components[0].Initial != float64(2) || len(got.Components) != 3 {
		t.Fatalf("merge=%+v", got)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(nil, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if again := install("consumer", resource); again != id || len(s.List()) != 2 {
		t.Fatal("reopen/reinstall duplicated panels")
	}
	before, _ := s.Edit(id)
	resource.Managed.Components[0].Title = "Should roll back"
	failure := errors.New("source CAS failed")
	err = s.ImportResources(ctx, "consumer", []PortableResource{resource}, func(map[string]string) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	after, _ := s.Edit(id)
	if after.Revision != before.Revision || after.Components[0].Title != before.Components[0].Title {
		t.Fatal("failed publication changed panel")
	}
	resource.Managed.Components[0].Kind = "text"
	resource.Managed.Components[0].Initial = "text"
	local, _ = s.Edit(id)
	local.Components[0].Initial = float64(123)
	if _, err := s.Save(local); err != nil {
		t.Fatal(err)
	}
	called := false
	err = s.ImportResources(ctx, "consumer", []PortableResource{resource}, func(map[string]string) error { called = true; return nil })
	if err == nil || called {
		t.Fatal("incompatible update published")
	}
}

func TestPortablePluginRequiresExactInstalledContributionWithoutStartingProvider(t *testing.T) {
	source := fixture()
	source.OwnerID = "https://example.test/packages/telemetry"
	source.PublisherNamespace = "https://example.test"
	source.PackageVersion = "1.2.3"
	source.Generation = "sha256:" + strings.Repeat("a", 64)
	source.ID = PluginSourceID(source.OwnerID, source.Definition.ID)
	catalog := &catalogStub{sources: []Source{source}}
	publisher := New(catalog)
	defer publisher.Close()
	resource, err := publisher.ExportResource(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resource.Managed != nil || resource.Plugin == nil || catalog.starts != 0 {
		t.Fatal("plugin was copied or started")
	}
	consumer := New(nil)
	defer consumer.Close()
	called := false
	err = consumer.ImportResources(context.Background(), "consumer", []PortableResource{resource}, func(map[string]string) error { called = true; return nil })
	if apperr.From(err).ID != "panels.plugin_required" || called {
		t.Fatal("missing dependency accepted", err)
	}
	consumer.SetCatalog(catalog)
	err = consumer.ImportResources(context.Background(), "consumer", []PortableResource{resource}, func(mapping map[string]string) error {
		called = true
		if mapping[source.ID] != source.ID {
			t.Fatal("plugin binding changed")
		}
		return nil
	})
	if err != nil || !called || catalog.starts != 0 {
		t.Fatal("plugin import did not remain declarative", err)
	}
	catalog.sources[0].Generation = "sha256:" + strings.Repeat("b", 64)
	if err := consumer.ImportResources(context.Background(), "consumer", []PortableResource{resource}, func(map[string]string) error { return nil }); apperr.From(err).ID != "panels.plugin_required" {
		t.Fatal("wrong version accepted", err)
	}
}

func TestPortableExportExcludesLiveValues(t *testing.T) {
	s, _ := Open(nil, "")
	defer s.Close()
	d, err := s.Save(Draft{Title: "Dashboard", Components: []ComponentDraft{{Title: "Value", Kind: "number", Initial: float64(7)}}})
	if err != nil {
		t.Fatal(err)
	}
	ref, _ := s.Resolve(d.ID)
	component, _ := s.ResolveComponent(ref, d.Components[0].ID, "number")
	if err := s.Write(component, float64(99)); err != nil {
		t.Fatal(err)
	}
	resource, err := s.ExportResource(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resource.Managed.Components[0].Initial != float64(7) || resource.Managed.Revision != 1 {
		t.Fatal("live state leaked into portable definition")
	}
}
