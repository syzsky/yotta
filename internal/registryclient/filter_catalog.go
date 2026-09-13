package registryclient

import (
	"context"
)

type FilterValue struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ParentID   string `json:"parentId"`
	Active     bool   `json:"active"`
	Position   int    `json:"position"`
	Assignable *bool  `json:"assignable,omitempty"`
}
type FilterDimension struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Description      string        `json:"description"`
	Icon             string        `json:"icon"`
	Active           bool          `json:"active"`
	Position         int           `json:"position"`
	MinValues        int           `json:"minValues"`
	MaxValues        int           `json:"maxValues"`
	MaxDepth         int           `json:"maxDepth"`
	LeafOnly         bool          `json:"leafOnly"`
	Values           []FilterValue `json:"values"`
	AuthorVisible    *bool         `json:"authorVisible,omitempty"`
	DiscoveryVisible *bool         `json:"discoveryVisible,omitempty"`
}
type FilterCatalog struct {
	Revision   uint64            `json:"revision"`
	Dimensions []FilterDimension `json:"dimensions"`
}

func (client *Client) FilterCatalog(ctx context.Context) (FilterCatalog, error) {
	profile, err := client.TaxonomyProfile(ctx, "workflow")
	return FilterCatalog{Revision: profile.Revision, Dimensions: profile.Dimensions}, err
}
