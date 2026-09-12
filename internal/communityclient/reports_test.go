package communityclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type reportTestToken struct{}

func (reportTestToken) Token(context.Context) (string, error) { return "test-report-token", nil }
func TestReportWireBodyAndIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-report-token" {
			t.Error("missing current user authorization")
		}
		if r.URL.Path != "/v1/workflows/work/reports" || r.Method != "POST" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		var input map[string]any
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if len(input) != 4 || input["id"] != "request-id" {
			t.Errorf("wire payload=%#v", input)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write([]byte(`{"id":"request-id","workflowId":"work","state":"pending"}`))
	}))
	defer server.Close()
	client, err := New(server.URL, reportTestToken{}, true)
	if err != nil {
		t.Fatal(err)
	}
	report, err := client.SubmitReport(context.Background(), ReportDraft{ID: "request-id", WorkflowID: "work", Reason: "outdated"})
	if err != nil || report.ID != "request-id" || report.State != "pending" {
		t.Fatalf("report=%#v %v", report, err)
	}
}
