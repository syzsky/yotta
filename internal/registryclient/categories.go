package registryclient

import (
	"context"
	"net/http"
)

// Category is the Registry-owned publication directory, independent of local folders.
type Category struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	ParentKey string `json:"parentKey"`
	Active    bool   `json:"active"`
}

func (client *Client) Categories(ctx context.Context) ([]Category, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/v1/categories", nil)
	if err != nil {
		return nil, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var page struct {
		Items []Category `json:"items"`
	}
	if err := decodeResponse(response, &page); err != nil {
		return nil, err
	}
	if page.Items == nil {
		return []Category{}, nil
	}
	return page.Items, nil
}
