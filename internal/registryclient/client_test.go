package registryclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublishWorkflowStreamsBundleAndUsesToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer user-token" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Idempotency-Key") != "workflow-release-key" {
			t.Fatalf("Idempotency-Key = %q", request.Header.Get("Idempotency-Key"))
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		file, _, err := request.FormFile("bundle")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		bundle, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if string(bundle) != "bundle-content" || request.FormValue("title") != "整理照片" {
			t.Fatalf("bundle = %q, title = %q", bundle, request.FormValue("title"))
		}
		if request.FormValue("releaseNotes") != "新增批量整理。" || !strings.Contains(request.FormValue("examples"), "整理相册") {
			t.Fatalf("release notes = %q, examples = %q", request.FormValue("releaseNotes"), request.FormValue("examples"))
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"releaseId":"release-1","workflowId":"workflow","releaseVersion":"1.0.0","title":"整理照片","creator":{"userKey":"TestA123"},"availability":"active"}`))
	}))
	defer server.Close()
	client := mustClient(t, server.URL, staticToken("user-token"))
	release, err := client.PublishWorkflow(context.Background(), PublishRequest{
		Bundle: strings.NewReader("bundle-content"), IdempotencyKey: "workflow-release-key", ReleaseVersion: "1.0.0",
		Title: "整理照片", Summary: "自动整理照片。", ReleaseNotes: "新增批量整理。",
		Examples: []Example{{Title: "整理相册", Description: "按日期整理。"}},
	})
	if err != nil || release.ReleaseID != "release-1" || release.Creator.UserKey != "TestA123" {
		t.Fatalf("PublishWorkflow = %#v, %v", release, err)
	}
}

func TestSearchUsesPublicCatalogAndDecodesProjection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("q") != "照片" || request.URL.Query().Get("limit") != "10" {
			t.Fatalf("query = %s", request.URL.RawQuery)
		}
		_, _ = response.Write([]byte(`{"items":[{"kind":"workflow","workflow":{"releaseId":"release-1","title":"整理照片","creator":{"userKey":"TestA123"}}}]}`))
	}))
	defer server.Close()
	client := mustClient(t, server.URL, nil)
	page, err := client.Search(context.Background(), "照片", 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].Workflow.Title != "整理照片" {
		t.Fatalf("Search = %#v, %v", page, err)
	}
}

func TestProblemKeepsStableCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusConflict)
		_, _ = response.Write([]byte(`{"type":"https://registry.example/problems/conflict","title":"版本冲突","status":409,"code":"registry.release_version_conflict","detail":"请提高版本号。"}`))
	}))
	defer server.Close()
	client := mustClient(t, server.URL, staticToken("token"))
	_, err := client.PublishWorkflow(context.Background(), PublishRequest{Bundle: strings.NewReader("bundle"), IdempotencyKey: "workflow-release-key"})
	var problem Problem
	if !errors.As(err, &problem) || problem.Code != "registry.release_version_conflict" {
		t.Fatalf("error = %#v", err)
	}
}

func TestSearchPropagatesCancellation(t *testing.T) {
	client := mustClient(t, "http://127.0.0.1:1", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Search(ctx, "照片", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("Search error = %v", err)
	}
}

func mustClient(t *testing.T, baseURL string, tokens TokenSource) *Client {
	t.Helper()
	client, err := New(Options{BaseURL: baseURL, Tokens: tokens, AllowLoopbackHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

type staticToken string

func (token staticToken) Token(context.Context) (string, error) { return string(token), nil }
