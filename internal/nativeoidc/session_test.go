package nativeoidc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestTokenCompletesAuthorizationCodePKCES256AndReusesToken(t *testing.T) {
	var challenge, redirectURI string
	tokenCalls := 0
	identity := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/token" {
			http.NotFound(response, request)
			return
		}
		tokenCalls++
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("client_secret") != "" {
			t.Fatalf("public client sent a secret: %#v", request.Form)
		}
		if request.Form.Get("grant_type") == "refresh_token" {
			if request.Form.Get("refresh_token") != "refresh-one" {
				t.Fatalf("refresh form = %#v", request.Form)
			}
			_ = json.NewEncoder(response).Encode(map[string]any{"access_token": "registry-token-two", "refresh_token": "refresh-two", "expires_in": 600})
			return
		}
		digest := sha256.Sum256([]byte(request.Form.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(digest[:]) != challenge || request.Form.Get("redirect_uri") != redirectURI {
			t.Fatalf("token form = %#v", request.Form)
		}
		_ = json.NewEncoder(response).Encode(map[string]any{"access_token": "registry-token", "refresh_token": "refresh-one", "expires_in": 600})
	}))
	defer identity.Close()
	callbackAddress := freeAddress(t)
	browserCalls := 0
	browser := browserFunc(func(raw string) error {
		browserCalls++
		authorization, err := url.Parse(raw)
		if err != nil {
			return err
		}
		challenge = authorization.Query().Get("code_challenge")
		redirectURI = authorization.Query().Get("redirect_uri")
		if authorization.Query().Get("code_challenge_method") != "S256" || authorization.Query().Get("client_id") != "yotta-desktop" {
			t.Fatalf("authorization query = %s", authorization.RawQuery)
		}
		go func() {
			callback, _ := url.Parse(redirectURI)
			query := callback.Query()
			query.Set("state", authorization.Query().Get("state"))
			query.Set("code", "authorization-code")
			callback.RawQuery = query.Encode()
			for attempt := 0; attempt < 20; attempt++ {
				if response, err := http.Get(callback.String()); err == nil {
					_ = response.Body.Close()
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
		return nil
	})
	session, err := New(Config{
		AuthorizationEndpoint: identity.URL + "/authorize", TokenEndpoint: identity.URL + "/token",
		ClientID: "yotta-desktop", Audience: "urn:yueli:registry:yotta",
		Scopes: []string{"openid", "profile", "offline_access"}, CallbackAddress: callbackAddress,
		Browser: browser, HTTPClient: identity.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := session.Login(context.Background())
	if err != nil || first != "registry-token" {
		t.Fatalf("Token = %q, %v", first, err)
	}
	second, err := session.Token(context.Background())
	if err != nil || second != first || tokenCalls != 1 {
		t.Fatalf("reused Token = %q, calls = %d, error = %v", second, tokenCalls, err)
	}
	session.expiresAt = time.Now()
	refreshed, err := session.Token(context.Background())
	if err != nil || refreshed != "registry-token-two" || tokenCalls != 2 || browserCalls != 1 {
		t.Fatalf("refreshed Token = %q, token calls = %d, browser calls = %d, error = %v", refreshed, tokenCalls, browserCalls, err)
	}
}

func TestTokenCancellationStopsLogin(t *testing.T) {
	address := freeAddress(t)
	session, err := New(Config{
		AuthorizationEndpoint: "https://identity.example.test/authorize",
		TokenEndpoint:         "https://identity.example.test/token", ClientID: "yotta-desktop",
		CallbackAddress: address, Browser: browserFunc(func(string) error { return nil }),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := session.Login(ctx); err != context.Canceled {
		t.Fatalf("Token error = %v", err)
	}
}

func TestAbandonedBrowserLoginHasDeadline(t *testing.T) {
	session, err := New(Config{AuthorizationEndpoint: "https://identity.example/authorize", TokenEndpoint: "https://identity.example/token", ClientID: "yotta-desktop", CallbackAddress: freeAddress(t), Browser: browserFunc(func(string) error { return nil }), LoginTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := session.Login(context.Background()); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("abandoned browser login never settled")
	}
}

type browserFunc func(string) error

func (function browserFunc) OpenURL(raw string) error { return function(raw) }

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}
