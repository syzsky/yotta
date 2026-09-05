package nativeoidc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yottaapp/yotta/internal/securestore"
)

type accountStore struct {
	values              map[string]string
	writeErr, deleteErr error
}

func (s *accountStore) Get(key string) (string, error) {
	v, ok := s.values[key]
	if !ok {
		return "", securestore.ErrNotFound
	}
	return v, nil
}
func (s *accountStore) Set(key, value string) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.values[key] = value
	return nil
}
func (s *accountStore) Delete(key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.values, key)
	return nil
}

func TestRememberedSessionRestartsRotatesAndRefreshesProfile(t *testing.T) {
	store := &accountStore{values: map[string]string{}}
	mode, name, picture := "ok", "Reader", "https://example.test/first.png"
	browserCalls := 0
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if mode == "offline" {
			w.WriteHeader(503)
			return
		}
		if mode == "revoked" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		if r.URL.Path == "/token" {
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "refresh_token" {
				t.Error("expected silent refresh")
			}
			_, _ = w.Write([]byte(`{"access_token":"access-not-persisted","refresh_token":"rotated-refresh","expires_in":3600}`))
			return
		}
		_, _ = w.Write([]byte(`{"user_key":"reader","name":"` + name + `","picture":"` + picture + `"}`))
	}))
	defer identity.Close()
	config := Config{AuthorizationEndpoint: identity.URL + "/authorize", TokenEndpoint: identity.URL + "/token", UserinfoEndpoint: identity.URL + "/userinfo", ClientID: "desktop", CallbackAddress: freeAddress(t), Credentials: store, CredentialScope: "profile-A", AccountURL: identity.URL + "/", Browser: browserFunc(func(string) error { browserCalls++; return nil })}
	first, _ := New(config)
	first.refreshToken = "initial-refresh"
	first.persistLocked()
	restarted, _ := New(config)
	profile, err := restarted.RefreshProfile(context.Background())
	if err != nil || profile.UserKey != "reader" || profile.Name != "Reader" || profile.SessionOnly || browserCalls != 0 {
		t.Fatalf("restored profile=%#v,error=%v,browserCalls=%d", profile, err, browserCalls)
	}
	if store.values[first.credentialKey()] != "rotated-refresh" {
		t.Fatal("rotated refresh token not saved")
	}
	for _, value := range store.values {
		if strings.Contains(value, "access-not-persisted") {
			t.Fatal("access token was persisted")
		}
	}
	name, picture = "Changed", "https://example.test/updated.png"
	profile, err = restarted.RefreshProfile(context.Background())
	if err != nil || profile.Name != "Changed" || profile.Picture != picture {
		t.Fatalf("updated profile=%#v,%v", profile, err)
	}
	isolated := config
	isolated.CredentialScope = "profile-B"
	other, _ := New(isolated)
	if _, err := other.Token(context.Background()); !errors.Is(err, ErrAuthenticationRequired) {
		t.Fatalf("profile leaked: %v", err)
	}
	mode = "offline"
	offline, _ := New(config)
	if _, err := offline.RefreshProfile(context.Background()); err == nil || errors.Is(err, ErrAuthenticationRequired) {
		t.Fatalf("offline=%v", err)
	}
	if store.values[first.credentialKey()] != "rotated-refresh" {
		t.Fatal("offline erased saved login")
	}
	mode = "ok"
	if _, err := offline.RefreshProfile(context.Background()); err != nil {
		t.Fatal(err)
	}
	store.deleteErr = errors.New("credential store locked")
	if err := offline.Logout(); !errors.Is(err, ErrCredentialStorage) || offline.Profile().UserKey != "reader" {
		t.Fatalf("failed signout was hidden: %v", err)
	}
	store.deleteErr = nil
	if err := offline.Logout(); err != nil {
		t.Fatal(err)
	}
	afterLogout, _ := New(config)
	if _, err := afterLogout.Token(context.Background()); !errors.Is(err, ErrAuthenticationRequired) {
		t.Fatalf("logout restored credentials: %v", err)
	}
	store.values[first.credentialKey()] = "revoked-refresh"
	mode = "revoked"
	revoked, _ := New(config)
	profile, err = revoked.RefreshProfile(context.Background())
	if err != nil || profile.UserKey != "" || len(store.values) != 0 {
		t.Fatalf("revoked=%#v,%v", profile, err)
	}
}

func TestAccountCenterUsesOnlyConfiguredPublicURL(t *testing.T) {
	var opened string
	s, err := New(Config{AuthorizationEndpoint: "https://id.example/authorize", TokenEndpoint: "https://id.example/token", ClientID: "desktop", CallbackAddress: freeAddress(t), AccountURL: "https://account.example/", Browser: browserFunc(func(value string) error { opened = value; return nil })})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.OpenAccountCenter(); err != nil || opened != "https://account.example/" {
		t.Fatalf("open=%q,%v", opened, err)
	}
}
