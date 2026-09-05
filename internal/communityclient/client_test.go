package communityclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReplyPaginationIsPublicAndPreservesCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workflows/wf/reviews/parent/replies" || r.URL.Query().Get("cursor") != "20" || r.Header.Get("Authorization") != "" {
			t.Errorf("request=%s, auth=%t", r.URL, r.Header.Get("Authorization") != "")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"reply","author":{"userKey":"reader","name":"Reader"},"content":"Last reply","createdAt":"2026-09-05T00:00:00Z"}]}`))
	}))
	defer server.Close()
	client, err := New(server.URL, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.Replies(context.Background(), "wf", "parent", "20")
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != "reply" || page.NextCursor != "" {
		t.Fatalf("page=%#v,%v", page, err)
	}
}

func TestFailurePreservesServerCorrelation(t *testing.T) {
	const id = "8968a822-0ce3-4c91-9a55-104a1f75b7bc"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"hub.review.invalid","operationId":"` + id + `"}`))
	}))
	defer server.Close()
	client, _ := New(server.URL, nil, true)
	_, err := client.List(context.Background(), "wf", "")
	var problem Problem
	if !errors.As(err, &problem) || problem.Code != "hub.review.invalid" || problem.CorrelationID() != id {
		t.Fatalf("problem=%v", err)
	}
}
