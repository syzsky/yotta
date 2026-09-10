package workflow

import (
	"errors"
	"github.com/yottaapp/yotta/internal/securestore"
	"testing"
)

type walletMemory map[string]string

func (m walletMemory) Get(k string) (string, error) {
	v, ok := m[k]
	if !ok {
		return "", securestore.ErrNotFound
	}
	return v, nil
}
func (m walletMemory) Set(k, v string) error { m[k] = v; return nil }
func (m walletMemory) Delete(k string) error { delete(m, k); return nil }
func TestWalletCredentialsRemainScopedAndPendingExchangeSurvivesRestart(t *testing.T) {
	store := walletMemory{}
	first := &walletSession{store: store, scope: "profile-a|issuer-a|registry-a"}
	pending := walletSaved{ID: "authorization", Verifier: "private-proof", Key: "same-intent", ExpiresAt: "2030-01-01T00:00:00Z"}
	if e := first.save("buyer-a", pending); e != nil {
		t.Fatal(e)
	}
	restart := &walletSession{store: store, scope: first.scope}
	got, e := restart.read("buyer-a")
	if e != nil || got != pending {
		t.Fatal("pending intent was not restored")
	}
	for _, other := range []struct{ scope, user string }{{first.scope, "buyer-b"}, {"profile-b|issuer-a|registry-a", "buyer-a"}, {"profile-a|issuer-b|registry-b", "buyer-a"}} {
		s := &walletSession{store: store, scope: other.scope}
		v, e := s.read(other.user)
		if e != nil || v.Token != "" || v.Verifier != "" {
			t.Error("credential crossed account or environment")
		}
	}
	if e := restart.save("buyer-a", walletSaved{Token: "active-delegation"}); e != nil {
		t.Fatal(e)
	}
	got, e = first.read("buyer-a")
	if e != nil || got.Verifier != "" || got.Key != "" || got.Token != "active-delegation" {
		t.Fatal("activation must remove exchange proof")
	}
}
func TestWalletErrorStorageAndBalanceHaveActionableProblems(t *testing.T) {
	if walletError(nil) != nil {
		t.Fatal("nil failure")
	}
	err := walletError(securestore.ErrUnavailable)
	if err == nil || !errors.Is(err, securestore.ErrUnavailable) {
		t.Fatal("storage cause missing")
	}
}
