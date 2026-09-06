package panel

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/artifact"

	"github.com/yottaapp/yotta/internal/durablefs"
	contract "github.com/yottaapp/yotta/sdk/plugin/panel"
)

// PortableResource carries configuration, never a live provider or session.
type PortableResource struct {
	ID      string             `json:"id"`
	Title   string             `json:"title"`
	Managed *Draft             `json:"managed,omitempty"`
	Plugin  *PluginRequirement `json:"plugin,omitempty"`
}

type PluginRequirement struct {
	PublisherNamespace string              `json:"publisherNamespace"`
	PackageID          string              `json:"packageId"`
	PackageVersion     string              `json:"packageVersion"`
	ManifestDigest     string              `json:"manifestDigest"`
	Definition         contract.Definition `json:"definition"`
}

type importedPanel struct {
	WorkflowID string `json:"workflowId"`
	ResourceID string `json:"resourceId"`
	LocalID    string `json:"localId"`
	Baseline   Draft  `json:"baseline"`
}

func ValidatePortable(resource PortableResource) error {
	if resource.ID == "" || len(resource.ID) > 256 || resource.Title == "" || len(resource.Title) > 128 || (resource.Managed == nil) == (resource.Plugin == nil) {
		return problem("panels.invalid_definition", nil)
	}
	if resource.Managed != nil {
		if resource.Managed.ID != resource.ID || resource.Managed.Revision != 1 || resource.Managed.Title != resource.Title {
			return problem("panels.invalid_definition", nil)
		}
		_, err := makeManaged(*resource.Managed)
		return err
	}
	p := resource.Plugin
	if p.PackageID == "" || p.PackageVersion == "" || p.PublisherNamespace == "" || !artifact.Digest(p.ManifestDigest).Valid() || resource.ID != PluginSourceID(p.PackageID, p.Definition.ID) {
		return problem("panels.invalid_definition", nil)
	}
	return p.Definition.Validate()
}

func (s *Service) ExportResource(id string) (PortableResource, error) {
	source, err := s.source(id)
	if err != nil {
		return PortableResource{}, problem("panels.portable_missing", err)
	}
	out := PortableResource{ID: source.ID, Title: source.Definition.TitleKey}
	if source.Managed {
		d, err := s.Edit(source.ID)
		if err != nil {
			return out, err
		}
		d.Revision = 1
		out.Managed = &d
		out.Title = d.Title
	} else {
		out.Plugin = &PluginRequirement{PublisherNamespace: source.PublisherNamespace, PackageID: source.OwnerID, PackageVersion: source.PackageVersion, ManifestDigest: source.Generation, Definition: contract.Clone(source.Definition)}
	}
	return out, ValidatePortable(out)
}

// ImportResources installs a batch and publishes its consumer as one operation.
// A crash between writes can leave a reusable panel, never a broken consumer.
func (s *Service) ImportResources(ctx context.Context, workflowID string, resources []PortableResource, publish func(map[string]string) error) error {
	mapping := map[string]string{}
	for _, r := range resources {
		if err := ValidatePortable(r); err != nil {
			return err
		}
		if r.Plugin != nil {
			src, err := s.source(r.ID)
			if err != nil || src.Generation != r.Plugin.ManifestDigest || src.OwnerID != r.Plugin.PackageID || src.PackageVersion != r.Plugin.PackageVersion || src.PublisherNamespace != r.Plugin.PublisherNamespace {
				return apperr.New("panels.plugin_required", map[string]any{"plugin": r.Title, "version": r.Plugin.PackageVersion})
			}
			if !reflect.DeepEqual(src.Definition, r.Plugin.Definition) {
				return problem("panels.portable_conflict", nil)
			}
			mapping[r.ID] = src.ID
		}
	}
	m := s.managed
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return problem("panels.closed", nil)
	}
	next := make(map[string]*managedPanel, len(m.panels))
	for id, p := range m.panels {
		next[id] = p
	}
	imports := contract.Clone(m.imports)
	if imports == nil {
		imports = map[string]importedPanel{}
	}
	for _, r := range resources {
		if r.Managed == nil {
			continue
		}
		key := workflowID + "\x00" + r.ID
		localID := fmt.Sprintf("panel-%x", sha256.Sum256([]byte(key)))
		previous, exists := imports[key]
		if exists {
			localID = previous.LocalID
		}
		incoming := contract.Clone(*r.Managed)
		incoming.ID = localID
		current := next[localID]
		merged := incoming
		if current != nil {
			if !exists {
				return problem("panels.portable_conflict", nil)
			}
			var err error
			merged, err = mergeImported(previous.Baseline, current.draft, incoming)
			if err != nil {
				return err
			}
			merged.Revision = current.draft.Revision + 1
		}
		prepared, err := makeManaged(merged)
		if err != nil {
			return problem("panels.portable_conflict", err)
		}
		if current != nil && reflect.DeepEqual(withoutRevision(current.draft), withoutRevision(merged)) {
			prepared = current
		} else if current != nil {
			carryManagedState(current, prepared)
		}
		next[localID] = prepared
		imports[key] = importedPanel{WorkflowID: workflowID, ResourceID: r.ID, LocalID: localID, Baseline: incoming}
		mapping[r.ID] = localID
	}
	if len(next) > 128 {
		return problem("panels.capacity", nil)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	oldImports := m.imports
	m.imports = imports
	if err := m.persist(next); err != nil {
		m.imports = oldImports
		if durablefs.Committed(err) {
			_ = m.persist(m.panels)
		}
		return problem("panels.save_failed", err)
	}
	if err := publish(mapping); err != nil {
		m.imports = oldImports
		if rollback := m.persist(m.panels); rollback != nil {
			return problem("panels.save_failed", rollback)
		}
		return err
	}
	m.panels = next
	return nil
}

func PortableComponentMatches(r PortableResource, id, kind string) bool {
	var definition contract.Definition
	if r.Managed != nil {
		p, err := makeManaged(*r.Managed)
		if err != nil {
			return false
		}
		definition = p.source.Definition
	} else {
		definition = r.Plugin.Definition
	}
	c, ok := definition.Component(id)
	return ok && componentMatches(definition, c, kind)
}

func withoutRevision(d Draft) Draft { d.Revision = 0; return d }

func mergeImported(base, local, incoming Draft) (Draft, error) {
	out := contract.Clone(incoming)
	if local.Title != base.Title {
		out.Title = local.Title
	}
	b := map[string]ComponentDraft{}
	l := map[string]ComponentDraft{}
	n := map[string]ComponentDraft{}
	for _, c := range base.Components {
		b[c.ID] = c
	}
	for _, c := range local.Components {
		l[c.ID] = c
	}
	for _, c := range incoming.Components {
		old, hadBase := b[c.ID]
		cur, hasLocal := l[c.ID]
		if hasLocal && hadBase {
			if cur.Kind != c.Kind && (cur.Kind != old.Kind || !reflect.DeepEqual(cur.Initial, old.Initial) || !reflect.DeepEqual(cur.Options, old.Options)) {
				return Draft{}, problem("panels.portable_conflict", nil)
			}
			if cur.Title != old.Title {
				c.Title = cur.Title
			}
			if cur.Icon != old.Icon {
				c.Icon = cur.Icon
			}
			if !reflect.DeepEqual(cur.Initial, old.Initial) {
				c.Initial = cur.Initial
			}
			if !reflect.DeepEqual(cur.Options, old.Options) {
				c.Options = cur.Options
			}
		}
		n[c.ID] = c
	}
	for _, c := range local.Components {
		if _, ok := n[c.ID]; !ok {
			if old, existed := b[c.ID]; !existed || !reflect.DeepEqual(old, c) {
				n[c.ID] = c
			}
		}
	}
	ids := func(d Draft) []string {
		var out []string
		for _, c := range d.Components {
			out = append(out, c.ID)
		}
		return out
	}
	order := ids(incoming)
	if !reflect.DeepEqual(ids(local), ids(base)) {
		order = ids(local)
	}
	out.Components = nil
	appendID := func(id string) {
		if c, ok := n[id]; ok {
			out.Components = append(out.Components, c)
			delete(n, id)
		}
	}
	for _, id := range order {
		appendID(id)
	}
	for _, c := range incoming.Components {
		appendID(c.ID)
	}
	keys := []string{}
	for id := range n {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		appendID(id)
	}
	return out, nil
}
