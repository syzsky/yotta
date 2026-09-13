// Package plugins exposes plugin management through the desktop RPC boundary.
package plugins

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/pluginmanager"
	"github.com/yottaapp/yotta/internal/registryclient"
	"github.com/yottaapp/yotta/internal/storage"
	"github.com/yottaapp/yotta/pkg/version"
)

type Registry interface {
	SearchCatalog(context.Context, registryclient.SearchOptions) (registryclient.SearchPage, error)
	CreateNodePackInstallPlan(context.Context, string, registryclient.Environment, []registryclient.InstalledNodePack) (registryclient.InstallPlan, error)
	DownloadArtifact(context.Context, string) ([]byte, error)
}

type RegistrySearchPage struct {
	Total      int64                            `json:"total"`
	Items      []registryclient.NodePackRelease `json:"items"`
	Facets     registryclient.Facets            `json:"facets"`
	NextCursor string                           `json:"nextCursor,omitempty"`
}

type RegistryInstallResult struct {
	Plugin           pluginmanager.View `json:"plugin"`
	AlreadyInstalled bool               `json:"alreadyInstalled"`
	PlanID           string             `json:"planId"`
}

type Service struct {
	manager       *pluginmanager.Manager
	roots         storage.Roots
	registry      Registry
	openDirectory func(string) error
}

func NewService(manager *pluginmanager.Manager, roots storage.Roots, registry ...Registry) *Service {
	service := &Service{manager: manager, roots: roots, openDirectory: openDirectory}
	if len(registry) > 0 {
		service.registry = registry[0]
	}
	return service
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

func (s *Service) DiscoverRegistry(ctx context.Context, options registryclient.SearchOptions) (RegistrySearchPage, error) {
	if s.registry == nil {
		return RegistrySearchPage{}, apperr.NewRetryable("plugins.market_unavailable", nil)
	}
	options.Kinds = []string{"node-pack"}
	page, err := s.registry.SearchCatalog(ctx, options)
	if err != nil {
		return RegistrySearchPage{}, pluginMarketError("plugins.market_unavailable", err, true)
	}
	result := RegistrySearchPage{Items: []registryclient.NodePackRelease{}, Facets: page.Facets, NextCursor: page.NextCursor, Total: page.Total}
	for _, item := range page.Items {
		if item.Kind == "node-pack" {
			result.Items = append(result.Items, item.NodePack)
		}
	}
	return result, nil
}

func (s *Service) RegistryTaxonomy(ctx context.Context) (registryclient.TaxonomyProfile, error) {
	client, ok := s.registry.(interface {
		TaxonomyProfile(context.Context, string) (registryclient.TaxonomyProfile, error)
	})
	if !ok {
		return registryclient.TaxonomyProfile{}, apperr.NewRetryable("plugins.market_unavailable", nil)
	}
	profile, err := client.TaxonomyProfile(ctx, "node-pack")
	if err != nil {
		return registryclient.TaxonomyProfile{}, pluginMarketError("plugins.market_unavailable", err, true)
	}
	return profile, nil
}

func (s *Service) InstallRegistry(ctx context.Context, releaseID string) (RegistryInstallResult, error) {
	if s.registry == nil {
		return RegistryInstallResult{}, apperr.NewRetryable("plugins.market_unavailable", nil)
	}
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return RegistryInstallResult{}, apperr.New("plugins.market_invalid_release", nil)
	}
	installed := s.manager.InstalledNodePacks()
	registryInstalled := make([]registryclient.InstalledNodePack, 0, len(installed))
	for _, item := range installed {
		registryInstalled = append(registryInstalled, registryclient.InstalledNodePack{
			PackageID: item.PackageID, PackageVersion: item.PackageVersion, ManifestDigest: item.ManifestDigest,
		})
	}
	plan, err := s.registry.CreateNodePackInstallPlan(ctx, releaseID, registryclient.Environment{
		YottaVersion: registryclient.NormalizeEnvironmentVersion(version.Version), OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH,
	}, registryInstalled)
	if err != nil {
		return RegistryInstallResult{}, pluginMarketError("plugins.market_install_failed", err, true)
	}
	if len(plan.IncompatibleRequirements) > 0 {
		return RegistryInstallResult{}, apperr.New("plugins.market_incompatible", map[string]any{"reason": userActionText(plan.IncompatibleRequirements)})
	}
	if len(plan.RuntimeConfigurationNeeded) > 0 {
		return RegistryInstallResult{}, apperr.New("plugins.market_runtime_required", map[string]any{"reason": userActionText(plan.RuntimeConfigurationNeeded)})
	}
	if len(plan.NodePacksToInstall) == 0 && len(plan.Updates) == 0 {
		if len(plan.NodePacksAlreadySatisfied) != 1 {
			return RegistryInstallResult{}, apperr.New("plugins.market_install_failed", nil)
		}
		view, ok := s.pluginView(plan.NodePacksAlreadySatisfied[0].PackageID)
		if !ok {
			return RegistryInstallResult{}, apperr.New("plugins.market_install_failed", nil)
		}
		return RegistryInstallResult{Plugin: view, AlreadyInstalled: true, PlanID: plan.PlanID}, nil
	}
	planned := append(append([]registryclient.PlannedNodePack{}, plan.NodePacksToInstall...), plan.Updates...)
	if len(planned) != 1 || strings.TrimSpace(planned[0].SelectedVariant.ArtifactDigest) == "" {
		return RegistryInstallResult{}, apperr.New("plugins.market_install_failed", nil)
	}
	content, err := s.registry.DownloadArtifact(ctx, planned[0].SelectedVariant.ArtifactDigest)
	if err != nil {
		return RegistryInstallResult{}, pluginMarketError("plugins.market_install_failed", err, true)
	}
	if err := os.MkdirAll(s.roots.Temp, 0o700); err != nil {
		return RegistryInstallResult{}, fmt.Errorf("%w: %v", apperr.New("plugins.market_install_failed", nil), err)
	}
	archive, err := os.CreateTemp(s.roots.Temp, "registry-node-pack-*.ynp")
	if err != nil {
		return RegistryInstallResult{}, fmt.Errorf("%w: %v", apperr.New("plugins.market_install_failed", nil), err)
	}
	archivePath := filepath.Clean(archive.Name())
	defer os.Remove(archivePath)
	writeErr := archive.Chmod(0o600)
	if writeErr == nil {
		_, writeErr = archive.Write(content)
	}
	if writeErr == nil {
		writeErr = archive.Sync()
	}
	closeErr := archive.Close()
	if writeErr != nil || closeErr != nil {
		return RegistryInstallResult{}, fmt.Errorf("%w: %v", apperr.New("plugins.market_install_failed", nil), errors.Join(writeErr, closeErr))
	}
	if err := s.manager.Import(archivePath); err != nil {
		return RegistryInstallResult{}, err
	}
	view, ok := s.pluginView(planned[0].PackageID)
	if !ok {
		return RegistryInstallResult{}, apperr.New("plugins.market_install_failed", nil)
	}
	return RegistryInstallResult{Plugin: view, PlanID: plan.PlanID}, nil
}

func (s *Service) pluginView(packageID string) (pluginmanager.View, bool) {
	views, err := s.manager.List()
	if err != nil {
		return pluginmanager.View{}, false
	}
	for _, view := range views {
		if view.ID == packageID {
			return view, true
		}
	}
	return pluginmanager.View{}, false
}

func userActionText(actions []registryclient.UserAction) string {
	parts := make([]string, 0, len(actions))
	for _, action := range actions {
		if text := strings.TrimSpace(action.Description); text != "" {
			parts = append(parts, text)
		} else if text := strings.TrimSpace(action.Title); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "；")
}

func pluginMarketError(id string, cause error, retryable bool) error {
	if errors.Is(cause, context.Canceled) {
		id, retryable = "plugins.market_cancelled", false
	} else if errors.Is(cause, context.DeadlineExceeded) {
		id, retryable = "plugins.market_timeout", true
	}
	var problem registryclient.Problem
	params := map[string]any{}
	if errors.As(cause, &problem) && problem.OperationID != "" {
		params["operationId"] = problem.OperationID
	}
	var projected error
	if retryable {
		projected = apperr.NewRetryable(id, params)
	} else {
		projected = apperr.New(id, params)
	}
	return fmt.Errorf("%w: %w", projected, cause)
}
