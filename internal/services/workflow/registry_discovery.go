package workflow

import (
	"context"
	"github.com/yottaapp/yotta/internal/registryclient"
)

type RegistryQuery = registryclient.SearchOptions
type RegistryCategory = registryclient.Category

func (s *Service) RegistryFilterCatalog(ctx context.Context) (registryclient.FilterCatalog, error) {
	client, ok := s.registry.(interface {
		FilterCatalog(context.Context) (registryclient.FilterCatalog, error)
	})
	if !ok {
		return registryclient.FilterCatalog{}, unavailable("registry")
	}
	result, err := client.FilterCatalog(ctx)
	if err != nil {
		return registryclient.FilterCatalog{}, registryError("categories", err)
	}
	return result, nil
}

func (s *Service) RegistryCategories(ctx context.Context) ([]RegistryCategory, error) {
	client, ok := s.registry.(interface {
		Categories(context.Context) ([]registryclient.Category, error)
	})
	if !ok {
		return nil, unavailable("registry")
	}
	categories, err := client.Categories(ctx)
	if err != nil {
		return nil, registryError("categories", err)
	}
	return categories, nil
}

type registryDiscovery interface {
	SearchCatalog(context.Context, registryclient.SearchOptions) (registryclient.SearchPage, error)
	WorkflowHistory(context.Context, string) ([]registryclient.WorkflowRelease, error)
}

func (s *Service) DiscoverRegistry(ctx context.Context, query RegistryQuery) (RegistrySearchPageView, error) {
	client, ok := s.registry.(registryDiscovery)
	if !ok {
		return RegistrySearchPageView{}, unavailable("registry")
	}
	page, err := client.SearchCatalog(ctx, query)
	if err != nil {
		return RegistrySearchPageView{}, registryError("search", err)
	}
	result := RegistrySearchPageView{Items: []RegistryWorkflowReleaseView{}, Facets: page.Facets, NextCursor: page.NextCursor}
	for _, item := range page.Items {
		if item.Kind == "workflow" {
			result.Items = append(result.Items, registryReleaseView(item.Workflow))
		}
	}
	return result, nil
}
func (s *Service) RegistryWorkflowHistory(ctx context.Context, id string) ([]RegistryWorkflowReleaseView, error) {
	client, ok := s.registry.(registryDiscovery)
	if !ok {
		return nil, unavailable("registry")
	}
	releases, err := client.WorkflowHistory(ctx, id)
	if err != nil {
		return nil, registryError("history", err)
	}
	result := make([]RegistryWorkflowReleaseView, 0, len(releases))
	for _, release := range releases {
		result = append(result, registryReleaseView(release))
	}
	return result, nil
}
