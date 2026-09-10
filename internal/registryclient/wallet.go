package registryclient

import (
	"context"
	"net/url"
)

type WalletAuthorization struct {
	ID        string `json:"id"`
	SiteKey   string `json:"siteKey"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}
type WalletAuthorizationRequest struct {
	Challenge      string `json:"challenge"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type WalletAuthorizationResult struct {
	Authorization WalletAuthorization `json:"authorization"`
	AuthorizeURL  string              `json:"authorizeUrl,omitempty"`
	Token         string              `json:"token,omitempty"`
}
type WalletPayment struct {
	OrderNo     string `json:"orderNo"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
}
type WalletBalance struct {
	Currency     string `json:"currency"`
	BalanceCents int64  `json:"balanceCents"`
}
type WalletPaid struct {
	OrderNo string `json:"orderNo"`
	Status  string `json:"status"`
}

func (c *Client) RequestWalletAuthorization(ctx context.Context, input WalletAuthorizationRequest) (WalletAuthorizationResult, error) {
	var out WalletAuthorizationResult
	err := c.checkoutRequest(ctx, "POST", "/v1/commerce/wallet/authorizations", input, &out)
	return out, err
}
func (c *Client) ExchangeWalletAuthorization(ctx context.Context, id, verifier string) (WalletAuthorizationResult, error) {
	var out WalletAuthorizationResult
	err := c.checkoutRequest(ctx, "POST", "/v1/commerce/wallet/authorizations/"+url.PathEscape(id)+"/exchange", struct {
		Verifier string `json:"verifier"`
	}{verifier}, &out)
	return out, err
}
func (c *Client) WalletBalance(ctx context.Context, grant string) (WalletBalance, error) {
	var out WalletBalance
	err := c.walletRequest(ctx, grant, "GET", "/v1/commerce/wallet", nil, &out)
	return out, err
}
func (c *Client) PayWallet(ctx context.Context, grant string, input WalletPayment) (WalletPaid, error) {
	var out WalletPaid
	err := c.walletRequest(ctx, grant, "POST", "/v1/commerce/wallet/pay", input, &out)
	return out, err
}

func (c *Client) WalletURL(ctx context.Context) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	err := c.checkoutRequest(ctx, "GET", "/v1/commerce/wallet/link", nil, &out)
	return out.URL, err
}
