package registryclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuyerTokenAccompaniesInstallAndBothDownloads(t *testing.T) {
	bundle := []byte("paid workflow bundle")
	sum := sha256.Sum256(bundle)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer buyer-token" {
			t.Error("buyer token missing")
			w.WriteHeader(403)
			return
		}
		if strings.HasSuffix(r.URL.Path, "install-plans") {
			_, _ = w.Write([]byte(`{"planId":"plan"}`))
			return
		}
		_, _ = w.Write(bundle)
	}))
	defer server.Close()
	client := mustClient(t, server.URL, staticToken("buyer-token"))
	if _, err := client.CreateInstallPlan(context.Background(), "release", Environment{}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.DownloadWorkflow(context.Background(), "release", digest); err != nil {
		t.Fatal(err)
	}
	if _, err := client.DownloadArtifact(context.Background(), digest); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyRegistryStillAllowsFreeInstallWithoutCommerce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"code":"registry.not_found","status":404}`))
	}))
	defer server.Close()
	client := mustClient(t, server.URL, nil)
	view, err := client.Commerce(context.Background(), "old-free-workflow")
	if err != nil || !view.Available || view.PriceCents != 0 {
		t.Fatalf("legacy view=%+v err=%v", view, err)
	}
}
