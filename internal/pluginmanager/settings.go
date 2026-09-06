package pluginmanager

import (
	"reflect"
	"slices"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/services"
	"github.com/yottaapp/yotta/sdk/plugin/packaging"
)

func removeOwned(settings *services.Settings, r record) {
	for _, c := range r.descriptor.Companions {
		exe, args := companionPaths(c, r.root)
		owned := false
		settings.Applications.Profiles = slices.DeleteFunc(settings.Applications.Profiles, func(p services.InstalledApplicationSettings) bool {
			match := p.Slot == c.ApplicationSlot && p.Executable == exe && reflect.DeepEqual(p.Arguments, args)
			owned = owned || match
			return match
		})
		if owned {
			settings.Network.HTTPOrigins = slices.DeleteFunc(settings.Network.HTTPOrigins, func(p services.HTTPOriginSettings) bool { return p.Slot == c.NetworkSlot && p.Origin == c.Origin })
		}
	}
}
func (m *Manager) configure(d packaging.Descriptor, root string, old record, validateOnly bool) error {
	if m.app == nil {
		if len(d.Companions) > 0 {
			return apperr.New("plugins.configuration_unavailable", nil)
		}
		return nil
	}
	mutate := func(settings *services.Settings) error {
		removeOwned(settings, old)
		for _, c := range d.Companions {
			for _, p := range settings.Network.HTTPOrigins {
				if p.Slot == c.NetworkSlot {
					return apperr.New("plugins.target_conflict", nil)
				}
			}
			for _, p := range settings.Applications.Profiles {
				if p.Slot == c.ApplicationSlot {
					return apperr.New("plugins.target_conflict", nil)
				}
			}
			exe, args := companionPaths(c, root)
			settings.Network.HTTPOrigins = append(settings.Network.HTTPOrigins, services.HTTPOriginSettings{Slot: c.NetworkSlot, Label: c.Name, Origin: c.Origin, ResponseByteLimit: 65536, TimeoutMilliseconds: 1000})
			settings.Applications.Profiles = append(settings.Applications.Profiles, services.InstalledApplicationSettings{Slot: c.ApplicationSlot, Label: c.Name, Executable: exe, Arguments: args})
		}
		return settings.Validate()
	}
	if validateOnly {
		return wrapIf("plugins.configuration_invalid", mutate(m.app.Settings().Clone()))
	}
	_, _, err := m.app.MutateSettings(mutate)
	return wrapIf("plugins.configuration_invalid", err)
}
func (m *Manager) removeTargets(r record) error {
	if m.app == nil {
		return nil
	}
	_, _, err := m.app.MutateSettings(func(s *services.Settings) error { removeOwned(s, r); return nil })
	return err
}
