package registryclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckoutUsesBuyerTokenAndPreservesPurchaseIntent(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer buyer-token" {
			t.Error("missing current buyer")
		}
		if r.URL.Path == "/v1/workflows/work-1/checkout" {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 2 || body["idempotencyKey"] != "same-intent" || body["provider"] != "test" {
				t.Fatalf("unexpected checkout body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"orderNo":"order-1","amountCents":100,"currency":"CNY","method":"native_qr","qrCode":"provider-payload"}`))
			return
		}
		if r.URL.Path != "/v1/commerce/checkouts/order-1/status" || r.Method != "GET" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"orderNo":"order-1","status":"paid","deliveryState":"granted"}`))
	}))
	defer server.Close()
	client := mustClient(t, server.URL, staticToken("buyer-token"))
	for range 2 {
		out, err := client.Checkout(context.Background(), "work-1", CheckoutInput{IdempotencyKey: "same-intent", Provider: "test"})
		if err != nil || out.QRCode != "provider-payload" || out.OrderNo != "order-1" {
			t.Fatalf("checkout %+v %v", out, err)
		}
	}
	state, err := client.CheckoutState(context.Background(), "order-1", "status")
	if err != nil || state.Status != "paid" {
		t.Fatalf("status %+v %v", state, err)
	}
	guest := mustClient(t, server.URL, nil)
	if _, err = guest.Checkout(context.Background(), "work-1", CheckoutInput{}); !errors.Is(err, ErrAuthenticationRequired) {
		t.Fatalf("guest error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("unauthenticated checkout reached provider: %d", calls)
	}
}
