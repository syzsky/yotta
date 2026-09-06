// Package plugins exposes plugin management through the desktop RPC boundary.
package plugins

import (
	"github.com/yottaapp/yotta/internal/pluginmanager"
	"github.com/yottaapp/yotta/internal/storage"
)

type Service struct {
	manager       *pluginmanager.Manager
	roots         storage.Roots
	openDirectory func(string) error
}

func NewService(manager *pluginmanager.Manager, roots storage.Roots) *Service {
	return &Service{manager: manager, roots: roots, openDirectory: openDirectory}
}

func (s *Service) List() ([]pluginmanager.View, error)      { return s.manager.List() }
func (s *Service) Messages() map[string]map[string]string   { return s.manager.Messages() }
func (s *Service) Import(path string) error                 { return s.manager.Import(path) }
func (s *Service) SetEnabled(id string, enabled bool) error { return s.manager.SetEnabled(id, enabled) }
func (s *Service) Uninstall(id string) error                { return s.manager.Uninstall(id) }
func (s *Service) Rollback(id string) error                 { return s.manager.Rollback(id) }
func (s *Service) Control(id, companionID string, start bool) error {
	return s.manager.Control(id, companionID, start)
}

func (s *Service) NeedsRestart() bool { return s.manager.NeedsRestart() }

func (s *Service) Batch(action string, ids []string) ([]pluginmanager.BatchResult, error) {
	return s.manager.Batch(action, ids)
}
