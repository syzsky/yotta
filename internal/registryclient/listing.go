package registryclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Listing struct {
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
	query := url.Values{"q": {options.Search}, "category": {options.Category}, "tag": {options.Tag}, "sort": {options.Sort}, "cursor": {options.Cursor}}
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
		client.resolveImages(&page.Items[i].Workflow)
	}
	return page, nil
}
func (client *Client) WorkflowHistory(ctx context.Context, id string) ([]WorkflowRelease, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/v1/workflows/"+url.PathEscape(id)+"/releases", nil)
	if err != nil {
		return nil, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var page struct {
		Items []WorkflowRelease `json:"items"`
	}
	if err := decodeResponse(response, &page); err != nil {
		return nil, err
	}
	for i := range page.Items {
		client.resolveImages(&page.Items[i])
	}
	return page.Items, nil
}
func (client *Client) resolveImages(release *WorkflowRelease) {
	base, _ := url.Parse(client.baseURL)
	for i := range release.Screenshots {
		ref, err := url.Parse(release.Screenshots[i].URL)
		if err == nil {
			release.Screenshots[i].URL = base.ResolveReference(ref).String()
		}
	}
	if release.Listing.Tags == nil {
		release.Listing.Tags = []string{}
	}
}
