package registryclient

import (
	"context"
	"net/http"
	"net/url"
)

type SystemFacetDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type SystemFacetValue struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

type SystemFacet struct {
	ID     string             `json:"id"`
	Source string             `json:"source"`
	Values []SystemFacetValue `json:"values"`
}

type TaxonomyProfile struct {
	Kind         string                  `json:"kind"`
	Revision     uint64                  `json:"revision"`
	Categories   []Category              `json:"categories"`
	Dimensions   []FilterDimension       `json:"dimensions"`
	SystemFacets []SystemFacetDefinition `json:"systemFacets"`
}

func (client *Client) TaxonomyProfile(ctx context.Context, kind string) (TaxonomyProfile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/v1/taxonomy/profiles/"+url.PathEscape(kind), nil)
	if err != nil {
		return TaxonomyProfile{}, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return TaxonomyProfile{}, err
	}
	defer response.Body.Close()
	var profile TaxonomyProfile
	if err := decodeResponse(response, &profile); err != nil {
		return TaxonomyProfile{}, err
	}
	if profile.Categories == nil {
		profile.Categories = []Category{}
	}
	if profile.Dimensions == nil {
		profile.Dimensions = []FilterDimension{}
	}
	if profile.SystemFacets == nil {
		profile.SystemFacets = []SystemFacetDefinition{}
	}
	return profile, nil
}
