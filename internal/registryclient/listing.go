package registryclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Listing struct {
	FilterValues []string `json:"filterValues,omitempty"`
	Icon         string   `json:"icon,omitempty"`
	Category     string   `json:"category,omitempty"`
	Tags         []string `json:"tags"`
	Description  string   `json:"description,omitempty"`
	Instructions string   `json:"instructions,omitempty"`
}

type DependencySummary struct {
	PackageID      string `json:"packageId"`
	PackageVersion string `json:"packageVersion"`
}
type BundleFacts struct {
	PanelCount         int   `json:"panelCount"`
	Verified           bool  `json:"verified"`
	ResourceCount      int   `json:"resourceCount"`
	TargetProfileCount int   `json:"targetProfileCount"`
	CredentialCount    int   `json:"credentialCount"`
	DependencyCount    int   `json:"dependencyCount"`
	BlobBytes          int64 `json:"blobBytes"`
}
type Facets struct {
	Categories []string `json:"categories"`
	Tags       []string `json:"tags"`
}
type SearchOptions struct {
	Kinds              []string `json:"kinds,omitempty"`
	FilterValues       []string `json:"filterValues,omitempty"`
	Selection          string   `json:"selection,omitempty"`
	IncludeDescendants bool     `json:"includeDescendants,omitempty"`
	WorkflowIDs        []string `json:"workflowIds,omitempty"`
	Search             string   `json:"search"`
	Category           string   `json:"category"`
	Tag                string   `json:"tag"`
	Sort               string   `json:"sort"`
	Cursor             string   `json:"cursor"`
	Limit              int      `json:"limit"`
}

func (client *Client) SearchCatalog(ctx context.Context, options SearchOptions) (SearchPage, error) {
	query := url.Values{"q": {options.Search}, "selection": {options.Selection}, "category": {options.Category}, "tag": {options.Tag}, "sort": {options.Sort}, "cursor": {options.Cursor}}
	if len(options.Kinds) > 0 {
		query.Set("kinds", strings.Join(options.Kinds, ","))
	}
	for _, id := range options.FilterValues {
		query.Add("filterValue", id)
	}
	if options.IncludeDescendants {
		query.Set("includeDescendants", "true")
	}
	if len(options.WorkflowIDs) > 0 {
		query.Set("workflowIds", strings.Join(options.WorkflowIDs, ","))
	}
	if options.Limit > 0 {
		query.Set("limit", strconv.Itoa(options.Limit))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/v1/catalog/search?"+query.Encode(), nil)
	if err != nil {
		return SearchPage{}, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return SearchPage{}, err
	}
	defer response.Body.Close()
	var page SearchPage
	if err := decodeResponse(response, &page); err != nil {
		return SearchPage{}, err
	}
	for i := range page.Items {
		switch page.Items[i].Kind {
		case "workflow":
			client.resolveImages(&page.Items[i].Workflow)
		case "node-pack":
			normalizeNodePackRelease(&page.Items[i].NodePack)
		case "node":
			if page.Items[i].Node.Inputs == nil {
				page.Items[i].Node.Inputs = []PortProjection{}
			}
			if page.Items[i].Node.Outputs == nil {
				page.Items[i].Node.Outputs = []PortProjection{}
			}
		}
	}
	return page, nil
}
func (client *Client) resolveImages(release *WorkflowRelease) {
	if release.Listing.Tags == nil {
		release.Listing.Tags = []string{}
	}
}

func normalizeNodePackRelease(release *NodePackRelease) {
	if release.Listing.Tags == nil {
		release.Listing.Tags = []string{}
	}
	if release.Listing.FilterValues == nil {
		release.Listing.FilterValues = []string{}
	}
	if release.Nodes == nil {
		release.Nodes = []NodeProjection{}
	}
	if release.Variants == nil {
		release.Variants = []RuntimeVariant{}
	}
	for index := range release.Nodes {
		if release.Nodes[index].Inputs == nil {
			release.Nodes[index].Inputs = []PortProjection{}
		}
		if release.Nodes[index].Outputs == nil {
			release.Nodes[index].Outputs = []PortProjection{}
		}
	}
}
