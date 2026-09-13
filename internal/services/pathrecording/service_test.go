package pathrecording

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/sdk/plugin/positionsource"
)

func sampleSnapshot() positionsource.Snapshot {
	return positionsource.Snapshot{Descriptor: positionsource.Descriptor{Protocol: positionsource.Protocol, Source: "synthetic", Capabilities: []positionsource.Capability{positionsource.Position}, Frame: positionsource.Frame{ID: "world", Unit: "m", AxisSign: 1, Kind: "world", Recovery: "stable"}}, Epoch: "epoch", Observations: map[positionsource.Capability]positionsource.Observation{positionsource.Position: {Status: "tracking", Value: json.RawMessage(`{"x":1,"y":2}`), SampleTimeMs: time.Now().UnixMilli(), Sequence: 1}}}
}

func TestSourceSamplesWithoutRequiringHeadingAndProjectsFailures(t *testing.T) {
	for _, scenario := range []string{"ok", "invalid", "undeclared", "stale", "status", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			snapshot := sampleSnapshot()
			if scenario == "undeclared" {
				snapshot.Frame.Kind = ""
				snapshot.Frame.Recovery = ""
			}
			if scenario == "stale" {
				o := snapshot.Observations[positionsource.Position]
				o.SampleAgeMs = 2000
				snapshot.Observations[positionsource.Position] = o
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario == "missing" {
					w.WriteHeader(404)
					return
				}
				if scenario == "status" {
					w.WriteHeader(503)
					return
				}
				if scenario == "invalid" {
					_, _ = w.Write([]byte("private-provider-body"))
					return
				}
				_ = json.NewEncoder(w).Encode(snapshot)
			}))
			defer server.Close()
			got, err := NewService().sample(context.Background(), server.URL)
			if scenario == "ok" {
				if err != nil || got.Point.X != 1 || got.Point.Y != 2 || got.Point.Z != nil || got.Reference.Kind != "world" {
					t.Fatalf("sample: %+v %v", got, err)
				}
				return
			}
			id := map[string]string{"invalid": "path.source_invalid", "undeclared": "path.reference_undeclared", "stale": "path.position_unavailable", "status": "path.source_unavailable", "missing": "path.source_endpoint_missing"}[scenario]
			envelope := apperr.From(err)
			if envelope.ID != id || envelope.Category != apperr.CategoryDomain || envelope.Retryable != (scenario == "stale" || scenario == "status") || envelope.OperationID == "" {
				t.Fatalf("problem: %+v", envelope)
			}
			encoded := string(apperr.Marshal(err))
			if strings.Contains(encoded, server.URL) || strings.Contains(encoded, "private-provider-body") {
				t.Fatal(encoded)
			}
		})
	}
	for _, url := range []string{"file:///private", "http://user:secret@localhost", "not a URL"} {
		if _, err := NewService().Sample(url); apperr.From(err).ID != "path.source_invalid" {
			t.Fatal(err)
		}
	}
}

func TestWatchShutdownCancelsOwnedHTTPRequest(t *testing.T) {
	started := make(chan struct{})
	ended := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(ended) }))
	defer server.Close()
	events := make(chan any, 2)
	s := NewDesktopService(nil, func(_ string, event any) { events <- event })
	if err := s.StartWatching("test-session", server.URL); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("watch did not request")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := Shutdown(ctx, s); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ended:
	case <-ctx.Done():
		t.Fatal("HTTP request outlived shutdown")
	}
	select {
	case event := <-events:
		t.Fatalf("emitted after cancellation: %v", event)
	default:
	}
	if err := s.StartWatching("later", server.URL); apperr.From(err).ID != "path.source_unavailable" {
		t.Fatal(err)
	}
}

func TestRecordingPreparesSourceWithoutOpeningPanel(t *testing.T) {
	for _, watch := range []bool{false, true} {
		t.Run(fmt.Sprint(watch), func(t *testing.T) {
			var ready atomic.Bool
			var preparations atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !ready.Load() {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode(sampleSnapshot())
			}))
			defer server.Close()
			events := make(chan SampleEvent, 8)
			s := NewDesktopService(nil, func(_ string, e any) { events <- e.(SampleEvent) }, func(ctx context.Context, endpoint string) error {
				if endpoint != server.URL {
					t.Errorf("endpoint=%s", endpoint)
				}
				preparations.Add(1)
				ready.Store(true)
				return nil
			})
			defer Shutdown(context.Background(), s)
			if !watch {
				got, err := s.Sample(server.URL)
				if err != nil || got.Point.X != 1 {
					t.Fatalf("cold sample=%+v %v", got, err)
				}
			} else {
				if err := s.StartWatching("cold", server.URL); err != nil {
					t.Fatal(err)
				}
				for range 2 {
					select {
					case e := <-events:
						if e.Problem != nil || e.Sample == nil || e.Sample.Point.X != 1 {
							t.Fatalf("cold watch=%+v", e)
						}
					case <-time.After(3 * time.Second):
						t.Fatal("no sample")
					}
				}
			}
			if preparations.Load() != 1 {
				t.Fatalf("prepare repeated: %d", preparations.Load())
			}
		})
	}
}

func TestRecordingPreservesCompanionStartFailure(t *testing.T) {
	events := make(chan SampleEvent, 1)
	s := NewDesktopService(nil, func(_ string, e any) { events <- e.(SampleEvent) }, func(context.Context, string) error { return apperr.New("plugins.start_failed", nil) })
	defer Shutdown(context.Background(), s)
	if _, err := s.Sample("http://127.0.0.1:1"); apperr.From(err).ID != "plugins.start_failed" {
		t.Fatal(err)
	}
	if err := s.StartWatching("failure", "http://127.0.0.1:1"); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-events:
		if e.Problem == nil || e.Problem.ID != "plugins.start_failed" {
			t.Fatalf("%+v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("no failure")
	}
}

func TestWatchWaitsForFirstPositionAfterServiceIsHealthy(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snapshot := sampleSnapshot()
		if requests.Add(1) < 3 {
			snapshot.Observations[positionsource.Position] = positionsource.Observation{Status: "waiting", SampleAgeMs: -1}
		}
		_ = json.NewEncoder(w).Encode(snapshot)
	}))
	defer server.Close()
	events := make(chan SampleEvent, 8)
	s := NewDesktopService(nil, func(_ string, e any) { events <- e.(SampleEvent) }, func(context.Context, string) error { return nil })
	defer Shutdown(context.Background(), s)
	if err := s.StartWatching("warming", server.URL); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-events:
		if e.Problem != nil || e.Sample == nil {
			t.Fatalf("healthy process without first coordinate stopped recording: %+v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("did not receive first position")
	}
}

func TestFirstPositionWaitIsBoundedAndCancelable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snapshot := sampleSnapshot()
		snapshot.Observations[positionsource.Position] = positionsource.Observation{Status: "waiting", SampleAgeMs: -1}
		_ = json.NewEncoder(w).Encode(snapshot)
	}))
	defer server.Close()
	s := NewService()
	if _, err := s.waitForPosition(context.Background(), server.URL, 50*time.Millisecond); apperr.From(err).ID != "path.position_wait_timeout" {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.waitForPosition(ctx, server.URL, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
