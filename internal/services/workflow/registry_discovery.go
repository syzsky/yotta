package workflow

import (
	"context"
	"github.com/yottaapp/yotta/internal/registryclient"
)

type RegistryQuery = registryclient.SearchOptions
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
