package registryclient

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

type PaymentMethod struct {
	Provider string `json:"provider"`
	Label    string `json:"label"`
	Method   string `json:"method"`
	Enabled  bool   `json:"enabled"`
}

type CheckoutInput struct {
	IdempotencyKey string `json:"idempotencyKey"`
	Provider       string `json:"provider"`
}

type CheckoutPayment struct {
	PaymentSession *PaymentSession `json:"paymentSession,omitempty"`
	OrderNo        string          `json:"orderNo"`
	AmountCents    int64           `json:"amountCents"`
	Currency       string          `json:"currency"`
	Provider       string          `json:"provider"`
	Method         string          `json:"method"`
	PayURL         string          `json:"payUrl"`
	QRCode         string          `json:"qrCode"`
}

type PaymentSession struct {
	SessionID string `json:"sessionId"`
	OrderNo   string `json:"orderNo"`
	PayURL    string `json:"payUrl"`
	ExpiresAt string `json:"expiresAt"`
}

type PaymentView struct {
	CanChooseMethod bool   `json:"canChooseMethod"`
	OrderNo         string `json:"orderNo"`
	Title           string `json:"title"`
	AmountCents     int64  `json:"amountCents"`
	Currency        string `json:"currency"`
	Provider        string `json:"provider"`
	Status          string `json:"status"`
	OrderStatus     string `json:"orderStatus"`
	DeliveryState   string `json:"deliveryState"`
	ExpiresAt       string `json:"expiresAt"`
	CanConfirm      bool   `json:"canConfirm"`
	CanCancel       bool   `json:"canCancel"`
	PayURL          string `json:"payUrl"`
	QRCode          string `json:"qrCode"`
}

func (client *Client) NativePayment(ctx context.Context, order string, confirm bool) (PaymentView, error) {
	var result PaymentView
	action := "payment-view"
	if confirm {
		action = "confirm"
	}
	err := client.checkoutRequest(ctx, "POST", "/v1/commerce/checkouts/"+url.PathEscape(order)+"/"+action, struct{}{}, &result)
	return result, err
}

func (client *Client) RenewPaymentSession(ctx context.Context, order string) (PaymentSession, error) {
	var result PaymentSession
	err := client.checkoutRequest(ctx, "POST", "/v1/commerce/checkouts/"+url.PathEscape(order)+"/payment-session", struct{}{}, &result)
	return result, err
}

type CheckoutStatus struct {
	OrderNo       string `json:"orderNo"`
	Status        string `json:"status"`
	DeliveryState string `json:"deliveryState"`
}

func (client *Client) checkoutRequest(ctx context.Context, method, path string, body, result any) error {
	return client.walletRequest(ctx, "", method, path, body, result)
}

func (client *Client) walletRequest(ctx context.Context, grant, method, path string, body, result any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	r, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if err = client.authorizeDelivery(r); err != nil {
		return err
	}
	if r.Header.Get("Authorization") == "" {
		return ErrAuthenticationRequired
	}
	r.Header.Set("Content-Type", "application/json")
	if grant != "" {
		r.Header.Set("X-Commerce-Wallet-Grant", grant)
	}
	response, err := client.http.Do(r)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return decodeResponse(response, result)
}

func (client *Client) PaymentMethods(ctx context.Context) ([]PaymentMethod, error) {
	var result struct {
		Items           []PaymentMethod `json:"items"`
		CentralCheckout bool            `json:"centralCheckout"`
	}
	err := client.checkoutRequest(ctx, "GET", "/v1/commerce/payment-methods", nil, &result)
	if err == nil && result.CentralCheckout {
		found := false
		for _, item := range result.Items {
			if item.Provider == "alipay" {
				found = true
			}
		}
		if !found {
			result.Items = append([]PaymentMethod{{Provider: "alipay", Method: "central", Enabled: true}}, result.Items...)
		}
	}
	return result.Items, err
}

func (client *Client) Checkout(ctx context.Context, workflow string, input CheckoutInput) (CheckoutPayment, error) {
	var result CheckoutPayment
	err := client.checkoutRequest(ctx, "POST", "/v1/workflows/"+url.PathEscape(workflow)+"/checkout", input, &result)
	return result, err
}

func (client *Client) CheckoutState(ctx context.Context, order, action string) (CheckoutStatus, error) {
	var result CheckoutStatus
	method := "GET"
	if action != "status" {
		method = "POST"
	}
	err := client.checkoutRequest(ctx, method, "/v1/commerce/checkouts/"+url.PathEscape(order)+"/"+url.PathEscape(action), struct{}{}, &result)
	return result, err
}
