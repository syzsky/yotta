package pluginhost

import (
	"context"
	"encoding/json"
	"github.com/yottaapp/yotta/internal/httpegress"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodecontract"
	"github.com/yottaapp/yotta/internal/pluginprotocol"
	"github.com/yottaapp/yotta/internal/targetruntime"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPluginConfiguredHTTPUsesHostTargetAndReleasesIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"damage":123}`))
	}))
	defer server.Close()
	profile, err := httpegress.SealProfile(httpegress.ProfileDraft{Origin: server.URL, ResponseByteLimit: 4096, TimeoutMilliseconds: 1000})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := httpegress.NewProvider(profile)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := targetruntime.NewSnapshot([]targetruntime.Installation{{Slot: "data", TargetID: "data", Provider: provider}})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := snapshot.NewRun()
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(context.Background())
	s := &processSession{ctx: context.Background(), invocation: nodeadapter.Invocation{Config: map[string]any{"slot": "data"}, Targets: runtime}, targetSpecs: []nodecontract.ConfiguredTargetSpec{{ID: "network", SlotConfigKey: "slot"}}}
	open := s.open(&pluginprotocol.HostOpenRequest{RequestId: "open", RequirementId: "network", Operations: []string{"get"}, ConfigJson: []byte(`{"config":{},"kind":"network/http-session"}`)}).GetHostOpenResponse()
	if open.Failure != nil {
		t.Fatal(open.Failure)
	}
	response := s.invoke(&pluginprotocol.HostInvokeRequest{RequestId: "read", RequirementId: "network", HandleJson: open.HandleJson, Operation: "get", Payload: []byte(`{"path":"/","query":{}}`)}).GetHostInvokeResponse()
	if response.Failure != nil {
		t.Fatal(response.Failure)
	}
	var result httpegress.GetResponse
	if json.Unmarshal(response.Payload, &result) != nil || result.Body != `{"damage":123}` {
		t.Fatalf("response=%s", response.Payload)
	}
	if s.drop(&pluginprotocol.HostDropRequest{RequestId: "drop", RequirementId: "network", HandleJson: open.HandleJson}).GetHostDropResponse().Failure != nil {
		t.Fatal("target drop failed")
	}
	if s.open(&pluginprotocol.HostOpenRequest{RequestId: "unknown", RequirementId: "other", Operations: []string{"get"}, ConfigJson: []byte(`{}`)}).GetHostOpenResponse().Failure == nil {
		t.Fatal("undeclared target accepted")
	}
}
