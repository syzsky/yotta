//go:build windows

package nativeoidc

import (
	"errors"
	"github.com/yottaapp/yotta/internal/securestore"
	"testing"
)

func TestWindowsCredentialPersistenceUsesIsolatedProfile(t *testing.T) {
	store := securestore.New()
	config := Config{AuthorizationEndpoint: "https://identity.example/authorize", TokenEndpoint: "https://identity.example/token", ClientID: "desktop-test", CallbackAddress: freeAddress(t), Credentials: store, CredentialScope: t.TempDir(), Browser: browserFunc(func(string) error { return nil })}
	first, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	key := first.credentialKey()
	t.Cleanup(func() { _ = store.Delete(key) })
	first.refreshToken = "synthetic-refresh-token"
	first.persistLocked()
	if first.Profile().SessionOnly {
		t.Fatal("Windows credential save failed")
	}
	restarted, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	restarted.restoreLocked()
	if restarted.refreshToken != "synthetic-refresh-token" {
		t.Fatal("Windows credential was not restored")
	}
	if err := restarted.Logout(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(key); !errors.Is(err, securestore.ErrNotFound) {
		t.Fatalf("credential was not removed: %v", err)
	}
}
