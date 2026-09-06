package plugins

import (
	"fmt"
	"github.com/yottaapp/yotta/internal/apperr"
	"os"
)

type Location struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

func (s *Service) Locations(id string) ([]Location, error) {
	result := []Location{}
	if id != "" {
		path, err := s.manager.InstallationDirectory(id)
		if err != nil {
			return nil, err
		}
		result = append(result, Location{Kind: "plugin", Path: path})
	}
	result = append(result, Location{Kind: "root", Path: s.roots.Root}, Location{Kind: "packages", Path: s.roots.Packages}, Location{Kind: "config", Path: s.roots.Config}, Location{Kind: "catalog", Path: s.roots.Catalog}, Location{Kind: "state", Path: s.roots.State}, Location{Kind: "objects", Path: s.roots.Objects}, Location{Kind: "logs", Path: s.roots.Logs})
	return result, nil
}

// OpenLocation accepts a location identity, never an arbitrary caller path.
func (s *Service) OpenLocation(kind, id string) error {
	locations, err := s.Locations(id)
	if err != nil {
		return err
	}
	for _, location := range locations {
		if location.Kind != kind {
			continue
		}
		info, err := os.Stat(location.Path)
		if err != nil || !info.IsDir() {
			return apperr.New("plugins.location_unavailable", nil)
		}
		if err = s.openDirectory(location.Path); err != nil {
			return fmt.Errorf("%w: %v", apperr.New("plugins.location_open_failed", nil), err)
		}
		return nil
	}
	return apperr.New("plugins.location_unavailable", nil)
}
