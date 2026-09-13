package registryclient

import (
	"context"
)

// Category is the Registry-owned publication directory, independent of local folders.
type Category struct {
	Icon          string `json:"icon"`
	ImageMediaKey string `json:"imageMediaKey"`
	Key           string `json:"key"`
	Name          string `json:"name"`
	ParentKey     string `json:"parentKey"`
	Position      int    `json:"position"`
	Active        bool   `json:"active"`
}

func (client *Client) Categories(ctx context.Context) ([]Category, error) {
	profile, err := client.TaxonomyProfile(ctx, "workflow")
	return profile.Categories, err
}
