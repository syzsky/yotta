package pluginmanager

import (
	"context"
	"github.com/yottaapp/yotta/internal/targetruntime"

	"slices"

	"github.com/yottaapp/yotta/internal/apperr"
)

// prepareRunServices is called with the application's plugin read lock held.
// Only matching installed target configurations belong to a companion; a user
// repointing a slot must not accidentally launch its former service.
func (m *Manager) prepareRunServices(ctx context.Context, slots []string, targets targetruntime.Snapshot) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(slots) == 0 || m.store == nil {
		return nil, nil
	}
	ids := []string{}
	for _, p := range m.store.List() {
		r, ok := m.records[p.PackageID]
		if !ok {
			continue
		}
		for _, c := range r.descriptor.Companions {
			if !slices.Contains(slots, c.NetworkSlot) {
				continue
			}
			exe, args := companionPaths(c, r.root)
			network := targets.Configuration(c.NetworkSlot)
			application := targets.Configuration(c.ApplicationSlot)
			networkMatches := network.Origin == c.Origin
			applicationMatches := application.Executable == exe && slices.Equal(application.Arguments, args)
			if !networkMatches || !applicationMatches {
				continue
			}
			if !p.Enabled {
				return nil, apperr.New("plugins.disabled", nil)
			}
			if err := m.controlLocked(ctx, p.PackageID, c.ID, true); err != nil {
				return nil, err
			}
			if !slices.Contains(ids, p.PackageID) {
				ids = append(ids, p.PackageID)
			}
		}
	}
	slices.Sort(ids)
	return ids, nil
}
