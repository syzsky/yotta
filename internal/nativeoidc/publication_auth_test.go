package nativeoidc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenNeverOpensBrowserForMissingOrExpiredSession(t *testing.T) {
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "invalid_grant", 401) }))
	defer identity.Close()
	for _, expired := range []bool{false, true} {
		browserCalls := 0
		s, err := New(Config{AuthorizationEndpoint: identity.URL + "/authorize", TokenEndpoint: identity.URL + "/token", ClientID: "test", CallbackAddress: freeAddress(t), LoginTimeout: time.Millisecond, Browser: browserFunc(func(string) error { browserCalls++; return nil })})
		if err != nil {
			t.Fatal(err)
		}
		if expired {
			s.token = "expired"
			s.refreshToken = "revoked"
			s.expiresAt = time.Now().Add(-time.Minute)
		}
		if _, err := s.Token(context.Background()); !errors.Is(err, ErrAuthenticationRequired) {
			t.Fatalf("expected authentication problem, got %v", err)
		}
		if browserCalls != 0 {
			t.Fatalf("API token request launched browser (expired=%v)", expired)
		}
	}
}
