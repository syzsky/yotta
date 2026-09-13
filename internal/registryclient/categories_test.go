package registryclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCategoriesUsesServerDirectoryAndPreservesFailure(t *testing.T) {
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/taxonomy/profiles/workflow" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if fail {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":503,"code":"registry.unavailable"}`))
			return
		}
		_, _ = w.Write([]byte(`{"kind":"workflow","categories":[{"key":"utilities","name":"实用工具","active":true},{"key":"retired","name":"旧分类","active":false}]}`))
	}))
	defer server.Close()
	client := mustClient(t, server.URL, nil)
	items, err := client.Categories(context.Background())
	if err != nil || len(items) != 2 || items[0].Key != "utilities" || !items[0].Active || items[1].Active {
		t.Fatalf("categories=%+v err=%v", items, err)
	}
	fail = true
	_, err = client.Categories(context.Background())
	var problem Problem
	if !errors.As(err, &problem) || problem.Code != "registry.unavailable" {
		t.Fatalf("lost directory failure: %v", err)
	}
}
