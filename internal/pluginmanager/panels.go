package pluginmanager

import (
	"github.com/yottaapp/yotta/internal/panel"
	contract "github.com/yottaapp/yotta/sdk/plugin/panel"
	"sort"
)

// PanelSources projects enabled signed contributions without probing processes.
// Windows consume this catalog; neither definitions nor providers belong to DOM.
func (m *Manager) PanelSources() []panel.Source {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []panel.Source{}
	if m.store == nil {
		return out
	}
	for _, p := range m.store.List() {
		if !p.Enabled {
			continue
		}
		r, ok := m.records[p.PackageID]
		if !ok {
			continue
		}
		for _, tab := range r.descriptor.Panels {
			for _, c := range r.descriptor.Companions {
				if c.ID != tab.CompanionID {
					continue
				}
				out = append(out, panel.Source{PublisherNamespace: r.manifest.PublisherNamespace(), PackageVersion: r.manifest.PackageVersion(), ID: panel.PluginSourceID(p.PackageID, tab.Definition.ID), OwnerID: p.PackageID, OwnerName: r.descriptor.Name, Generation: string(p.Current), Definition: contract.Clone(tab.Definition), Origin: c.Origin, CompanionID: c.ID, SnapshotPath: tab.SnapshotPath, EventPath: tab.EventPath})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
