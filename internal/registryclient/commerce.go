package registryclient

import (
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/nativeoidc"
	"net/http"
	"net/url"
	"strings"
)

// Free downloads remain anonymous; a signed-in buyer's current token accompanies
// every protected request. Refresh failures are not mistaken for a guest session.
func (client *Client) authorizeDelivery(request *http.Request) error {
	if client.tokens == nil {
		return nil
	}
	token, err := client.tokens.Token(request.Context())
	if errors.Is(err, ErrAuthenticationRequired) || errors.Is(err, nativeoidc.ErrAuthenticationRequired) {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) != "" {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	return nil
}

type CommerceView struct {
	PriceCents  int64  `json:"priceCents"`
	Currency    string `json:"currency"`
	Available   bool   `json:"available"`
	PurchaseURL string `json:"purchaseUrl"`
	Entitled    bool   `json:"entitled"`
}

func (client *Client) Commerce(ctx context.Context, workflowID string) (CommerceView, error) {
	var view CommerceView
	for _, part := range []string{"sales", "access"} {
		r, err := http.NewRequestWithContext(ctx, "GET", client.baseURL+"/v1/workflows/"+url.PathEscape(workflowID)+"/"+part, nil)
		if err != nil {
			return view, err
		}
		if err = client.authorizeDelivery(r); err != nil {
			return view, err
		}
		response, err := client.http.Do(r)
		if err != nil {
			return view, err
		}
		// Older Registry releases have no commerce endpoint. Keep their existing
		// install journey; the server remains authoritative for file access.
		if part == "sales" && response.StatusCode == http.StatusNotFound {
			response.Body.Close()
			return CommerceView{Available: true}, nil
		}
		err = decodeResponse(response, &view)
		response.Body.Close()
		if err != nil {
			return view, err
		}
	}
	return view, nil
}
