package panel

import (
	"encoding/json"
	contract "github.com/yottaapp/yotta/sdk/plugin/panel"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type catalogStub struct {
	mu      sync.Mutex
	sources []Source
	starts  int
}

func (c *catalogStub) PanelSources() []Source {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Source(nil), c.sources...)
}
func (c *catalogStub) Control(string, string, bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.starts++
	return nil
}
func fixture() Source {
	return Source{ID: "plugin#tab", OwnerID: "plugin", Generation: "one", SnapshotPath: "/state", EventPath: "/event", Definition: contract.Definition{Format: contract.Format, ID: "tab", TitleKey: "plugin.tab", Fields: []contract.Field{{ID: "x", Kind: "number"}}, Components: []contract.Component{{ID: "x", Kind: "number", Field: "x", TitleKey: "plugin.x"}, {ID: "clear", Kind: "button", Event: "clear", TitleKey: "plugin.clear"}}}}
}
func TestProviderRoundTripAndDisabledSource(t *testing.T) {
	p := fixture()
	snap := contract.Snapshot{Protocol: contract.Protocol, SessionID: "s", Revision: 1, Status: "ready", Values: map[string]any{"x": float64(23)}}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			calls++
			var event contract.Event
			_ = json.NewDecoder(r.Body).Decode(&event)
			_ = json.NewEncoder(w).Encode(contract.Result{EventID: event.EventID, Snapshot: snap})
			return
		}
		_ = json.NewEncoder(w).Encode(snap)
	}))
	defer server.Close()
	p.Origin = server.URL
	c := &catalogStub{sources: []Source{p}}
	s := New(c)
	defer s.Close()
	got, err := s.Read(p.ID)
	if err != nil || got.Values["x"] != float64(23) {
		t.Fatal(got, err)
	}
	_, err = s.Dispatch(p.ID, contract.Event{SessionID: "s", EventID: "event", ComponentID: "clear", Name: "clear"})
	if err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	_, err = s.Dispatch(p.ID, contract.Event{SessionID: "s", EventID: "bad", ComponentID: "x", Name: "clear"})
	if err == nil || calls != 1 {
		t.Fatal("read-only control caused request")
	}
	c.mu.Lock()
	c.sources = nil
	c.mu.Unlock()
	if _, err = s.Read(p.ID); err == nil {
		t.Fatal("disabled contribution still readable")
	}
	if c.starts != 0 {
		t.Fatal("healthy provider was relaunched")
	}
}
func TestIncompatibleEndpointAndStaleEventAreNotRetried(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(409) }))
	defer server.Close()
	p := fixture()
	p.Origin = server.URL
	c := &catalogStub{sources: []Source{p}}
	s := New(c)
	defer s.Close()
	if _, e := s.Read(p.ID); e == nil {
		t.Fatal("invalid provider accepted")
	}
	if _, e := s.Dispatch(p.ID, contract.Event{SessionID: "old", EventID: "e", ComponentID: "clear", Name: "clear"}); e == nil {
		t.Fatal("stale event accepted")
	}
	if calls != 2 || c.starts != 0 {
		t.Fatal("operation was retried", calls, c.starts)
	}
}
func TestCloseCancelsOutstandingRead(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	p := fixture()
	p.Origin = server.URL
	s := New(&catalogStub{sources: []Source{p}})
	done := make(chan struct{})
	go func() { defer close(done); _, _ = s.Read(p.ID) }()
	<-started
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if _, err := s.Read(p.ID); err == nil {
		t.Fatal("closed service allowed read")
	}
}
