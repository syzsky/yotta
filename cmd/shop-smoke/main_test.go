package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCaptureProxyPreservesAuthorizationRequest(t *testing.T) {
	handler := captureProxy("http://account.test")
	request := httptest.NewRequest(http.MethodGet, "/oauth2/authorize?state=s&code_challenge=c", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusFound || response.Header().Get("Location") != "http://account.test/oauth2/authorize?state=s&code_challenge=c" {
		t.Fatalf("authorization redirect = %d %q", response.Code, response.Header().Get("Location"))
	}
	latest := httptest.NewRecorder()
	handler.ServeHTTP(latest, httptest.NewRequest(http.MethodGet, "/latest", nil))
	if strings.TrimSpace(latest.Body.String()) != response.Header().Get("Location") {
		t.Fatalf("latest authorization = %q", latest.Body.String())
	}
}
