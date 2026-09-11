package registryclient

import (
	"context"
	"net/http"
)

type FilterValue struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentId"`
	Active   bool   `json:"active"`
	Position int    `json:"position"`
}
type FilterDimension struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Icon        string        `json:"icon"`
	Active      bool          `json:"active"`
	Position    int           `json:"position"`
	MinValues   int           `json:"minValues"`
	MaxValues   int           `json:"maxValues"`
	MaxDepth    int           `json:"maxDepth"`
	LeafOnly    bool          `json:"leafOnly"`
	Values      []FilterValue `json:"values"`
}
type FilterCatalog struct {
	Revision   uint64            `json:"revision"`
	Dimensions []FilterDimension `json:"dimensions"`
}

func (client *Client) FilterCatalog(ctx context.Context) (FilterCatalog, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/v1/filter-catalog", nil)
	if err != nil {
		return FilterCatalog{}, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return FilterCatalog{}, err
	}
	defer response.Body.Close()
	var result FilterCatalog
	err = decodeResponse(response, &result)
	return result, err
}
