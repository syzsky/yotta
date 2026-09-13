package pluginmanager

import (
	"context"
	"net/url"
	"slices"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/services"
)

// PrepareAuthoringEndpoint starts an installed companion for a tool, independent
// of its optional panel. A repointed user configuration is not plugin-owned.
func (m *Manager) PrepareAuthoringEndpoint(ctx context.Context, endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return apperr.New("path.source_invalid", nil)
	}
	origin := u.Scheme + "://" + u.Host
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.store == nil || m.app == nil {
		return nil
	}
	settings := m.app.Settings()
	for _, p := range m.store.List() {
		r, ok := m.records[p.PackageID]
		if !ok {
			continue
		}
		for _, c := range r.descriptor.Companions {
			if c.Origin != origin {
				continue
			}
			exe, args := companionPaths(c, r.root)
			networkMatches := slices.ContainsFunc(settings.Network.HTTPOrigins, func(n services.HTTPOriginSettings) bool { return n.Slot == c.NetworkSlot && n.Origin == c.Origin })
			applicationMatches := slices.ContainsFunc(settings.Applications.Profiles, func(a services.InstalledApplicationSettings) bool {
				return a.Slot == c.ApplicationSlot && a.Executable == exe && slices.Equal(a.Arguments, args)
			})
			if networkMatches && applicationMatches {
				return m.controlLocked(ctx, p.PackageID, c.ID, true)
			}
		}
	}
	return nil
}
