package noderuntime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/blob"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/httpegress"
	"github.com/yottaapp/yotta/internal/navigationpath"
	"github.com/yottaapp/yotta/internal/noderuntime"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/resource"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/targetruntime"
	"github.com/yottaapp/yotta/internal/workflow/compiler"
	"github.com/yottaapp/yotta/sdk/plugin/positionsource"
)

func TestPathActionsAndRecovery(t *testing.T) {
	for _, scenario := range []string{"moving", "marker", "marker-pause", "recover", "recovery-exhausted", "release-failure"} {
		t.Run(scenario, func(t *testing.T) { testPathFollower(t, scenario, false) })
	}
}
func TestFollowPathCompilesAndRunsOrderedABC(t *testing.T) { testFollowPath(t, "arrived") }
func TestFollowPathFailureProgressAndInputRelease(t *testing.T) {
	for _, scenario := range []string{"reference", "height", "stale", "epoch", "timeout", "stuck", "cancel"} {
		t.Run(scenario, func(t *testing.T) { testFollowPath(t, scenario) })
	}
}
func TestFollowSavedPathCancellationReleasesInput(t *testing.T) { testPathFollower(t, "cancel", true) }
func TestFollowSavedPathReadsLargeRouteOnce(t *testing.T)       { testPathFollower(t, "arrived", true) }
func testFollowPath(t *testing.T, scenario string)              { testPathFollower(t, scenario, false) }
func testPathFollower(t *testing.T, scenario string, saved bool) {
	b, err := nodes.Build()
	if err != nil {
		t.Fatal(err)
	}
	path := navigationpath.Path{Version: 1, Reference: navigationpath.Reference{Kind: "world", Frame: "test/world", Unit: "test-raw", AxisHeading: 90, AxisSign: 1}, Points: []navigationpath.Point{{ID: "a", X: 0}, {ID: "b", X: 2}, {ID: "c", X: 4}}}

	timeout := 60000
	expectedLast := 2
	if scenario != "arrived" {
		expectedLast = -1
		path.Points = []navigationpath.Point{{ID: "a", X: 10000}}
	}
	if scenario == "reference" {
		path.Reference.Frame = "other"
	}
	if scenario == "height" {
		z := 5.0
		path.Points[0].X = 0
		path.Points[0].Z = &z
	}
	if scenario == "timeout" {
		timeout = 250
	}
	actions := scenario == "moving" || scenario == "marker" || scenario == "marker-pause" || scenario == "recover" || scenario == "recovery-exhausted" || scenario == "release-failure"
	if actions {
		path.Points = []navigationpath.Point{{ID: "a", X: 0}, {ID: "b", Name: "jump", X: 10}, {ID: "c", X: 20}}
		expectedLast = 2
		if scenario == "recovery-exhausted" {
			expectedLast = 0
		}
	}
	bindings := map[string]any{}
	for k, v := range map[string]any{"path": path, "start": 0, "end": -1, "tolerance": 0.6, "height-tolerance": 1, "timeout": timeout, "interval": 100, "slow-distance": 3} {
		bindings[k] = map[string]any{"kind": "value", "value": v}
	}
	nodeID := nodes.FollowPathNodeID
	var blobReader *countedPathReader
	providers := map[string]run.InstalledProvider{}
	if saved {
		points := make([]navigationpath.Point, 20000)
		for n := range points {
			points[n] = navigationpath.Point{ID: fmt.Sprintf("point-%d", n), Name: strings.Repeat("route", 16), X: 10000}
		}
		copy(points[len(points)-3:], path.Points)
		path.Points = points
		encoded, err := json.Marshal(path)
		if err != nil || len(encoded) <= datatype.MaxInlineValueBytes {
			t.Fatalf("long route must exceed inline budget: %d %v", len(encoded), err)
		}
		store, err := blob.Open(t.TempDir(), blob.Limits{MaxBlobBytes: navigationpath.MaxEncodedBytes, MaxTotalBytes: 2 * navigationpath.MaxEncodedBytes})
		if err != nil {
			t.Fatal(err)
		}
		ref, err := store.Put(context.Background(), navigationpath.MediaType, bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		provider, err := blob.NewProvider(store, blob.ProviderLimits{MaxChunkBytes: 64 << 10, QueueCapacity: 4})
		if err != nil {
			t.Fatal(err)
		}
		blobReader = &countedPathReader{Provider: provider}
		providers[blob.ProviderID] = run.InstalledProvider{ArtifactDigest: blobProviderDigest(t), ABI: blob.ProviderABI, Provider: blobReader}
		delete(bindings, "path")
		bindings["asset"] = map[string]any{"kind": "blob", "blob": ref}
		bindings["start"] = map[string]any{"kind": "value", "value": len(points) - 3}
		nodeID, expectedLast = nodes.FollowSavedPathNodeID, len(points)-1
	}
	raw := navigationPositionFeed(t, b, navigationSource(t, b, nodeID, map[string]any{"slot": "game", "position-variable": "position"}, bindings), map[string]any{})
	var source map[string]any
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	graph := source["graphs"].([]any)[0].(map[string]any)
	nodeList := []any{}
	for _, n := range graph["nodes"].([]any) {
		if n.(map[string]any)["id"] != "parse" {
			nodeList = append(nodeList, n)
		}
	}
	graph["nodes"] = nodeList
	edges := []any{}
	for _, e := range graph["edges"].([]any) {
		edge := e.(map[string]any)
		from, to := edge["from"].(map[string]any), edge["to"].(map[string]any)
		if from["nodeId"] == "parse" {
			continue
		}
		if to["nodeId"] == "parse" {
			to["nodeId"] = "write"
			if edge["channel"] == "data" {
				to["portId"] = "value"
			}
		}
		edges = append(edges, edge)
	}
	graph["edges"] = edges
	source["variables"] = []any{map[string]any{"name": "position", "type": map[string]any{"kind": "ref", "ref": b.StringType.TypeRef()}, "default": ""}}
	raw, err = json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	if actions {
		def, _ := b.Definition(nodes.PressKeysNodeID)
		graph["nodes"] = append(graph["nodes"].([]any), map[string]any{"id": "path-action", "nodeRef": def.Contract.NodeRef(), "position": map[string]int{"x": 0, "y": 0}, "config": map[string]any{"slot": "game"}, "bindings": map[string]any{"keys": map[string]any{"kind": "value", "value": []string{"F"}}, "hold-duration": map[string]any{"kind": "value", "value": 20}}})
		port := "marker"
		if scenario == "moving" {
			port = "moving"
		}
		if scenario == "recover" || scenario == "recovery-exhausted" || scenario == "release-failure" {
			port = "recover"
		}
		graph["edges"] = append(graph["edges"].([]any), map[string]any{"channel": "exec", "from": map[string]string{"nodeId": "navigation", "portId": port}, "to": map[string]string{"nodeId": "path-action", "portId": "in"}})
		for _, n := range graph["nodes"].([]any) {
			node := n.(map[string]any)
			if node["id"] == "navigation" {
				config := node["config"].(map[string]any)
				config["recovery-attempts"] = 2
				if scenario == "marker-pause" {
					config["marker-mode"] = "pause"
				}
			}
		}
		raw, err = json.Marshal(source)
		if err != nil {
			t.Fatal(err)
		}
	}
	program := compilePrimitiveProgram(t, b, raw)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &followProvider{navigationProvider: &navigationProvider{heading: 90, positionSource: true, allowReturn: true}, scenario: scenario, cancel: cancel}
	snapshot, err := targetruntime.NewSnapshot([]targetruntime.Installation{{Slot: "game", TargetID: "test/game", Provider: p}, {Slot: "position", TargetID: "test/position", Provider: p}})
	if err != nil {
		t.Fatal(err)
	}
	targets, err := snapshot.NewRun()
	if err != nil {
		t.Fatal(err)
	}
	defer targets.Close(context.Background())
	now := time.Now().UTC()
	var store *run.Store
	_, owner, journal := admittedExecutionWithConsent(t, b, program, providers, now, executionProfile(t, b), nil, func(value *run.Store) { store = value })
	defer owner.Close(context.Background())
	adapters, err := noderuntime.Installed(b, testDependencies())
	if err != nil {
		t.Fatal(err)
	}
	result, err := compiler.NewExecutor(b.Catalog, adapters, compiler.ExecutorOptions{Now: func() time.Time { return now }}).RunWithTargets(ctx, program, owner, targets, journal)
	if scenario == "release-failure" {
		if err == nil || !strings.Contains(err.Error(), "fixture release failed") || p.actionCount != 0 {
			t.Fatalf("release failure was handled/retried: err=%v actions=%d", err, p.actionCount)
		}
		return
	}
	if scenario == "cancel" {
		if !errors.Is(err, context.Canceled) || p.held || p.closed != p.opened || p.steps == 0 {
			t.Fatalf("cancellation: %v provider=%+v", err, p.navigationProvider)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}

	timeline, err := store.TimelineSnapshot(context.Background(), journal.Current().Admission().RunID)
	if err != nil {
		t.Fatal(err)
	}
	expectedOutcome := map[string]string{"arrived": "arrived", "reference": "reference_mismatch", "height": "height_mismatch", "stale": "unavailable", "epoch": "reference_mismatch", "timeout": "timeout", "stuck": "stuck", "moving": "arrived", "marker": "arrived", "marker-pause": "arrived", "recover": "arrived", "recovery-exhausted": "stuck"}[scenario]
	routed := false
	for _, entry := range timeline.Entries {
		if entry.NodeID == "navigation" && entry.StatusCode == nodes.PathProgressStatus && entry.Summary.Counters["path_"+expectedOutcome] == 1 {
			routed = true
		}
	}
	if !routed {
		output := map[string]json.RawMessage{}
		for key, value := range result.NodeOutputs["navigation"] {
			output[key] = value.InlineJSON()
		}
		encoded, _ := json.Marshal(output)
		t.Fatalf("missing durable %s outcome: outputs=%s provider=%+v", expectedOutcome, encoded, p.navigationProvider)
	}
	var last int
	var id string
	if err := json.Unmarshal(result.NodeOutputs["navigation"]["last-index"].InlineJSON(), &last); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(result.NodeOutputs["navigation"]["point-id"].InlineJSON(), &id); err != nil {
		t.Fatal(err)
	}
	expectedID := "a"
	if scenario == "arrived" || (actions && scenario != "recovery-exhausted") {
		expectedID = "c"
	}
	if scenario == "recovery-exhausted" {
		expectedID = "b"
	}
	if actions {
		if p.actionCount == 0 {
			t.Fatal("branch never executed")
		}
		if (scenario == "marker" || scenario == "marker-pause" || scenario == "recover") && p.actionCount != 1 {
			t.Fatalf("action count=%d", p.actionCount)
		}
		if scenario == "recovery-exhausted" && p.actionCount != 2 {
			t.Fatalf("retry count=%d", p.actionCount)
		}
		if scenario == "moving" && !p.actionWhileHeld {
			t.Fatal("moving action did not overlap forward hold")
		}
		if (scenario == "marker-pause" || scenario == "recover" || scenario == "recovery-exhausted") && p.actionWhileHeld {
			t.Fatal("paused action ran while navigation held forward")
		}
	}
	if saved && blobReader.opens != 1 {
		t.Fatalf("route re-read %d times", blobReader.opens)
	}
	if last != expectedLast || id != expectedID || (scenario == "arrived" && p.steps == 0) || p.held || p.closed != p.opened {
		t.Fatalf("last=%d id=%s provider=%+v", last, id, p)
	}
}

// The provider supplies synthetic failures after real adapter input has started.
type followProvider struct {
	*navigationProvider
	scenario        string
	cancel          context.CancelFunc
	actionCount     int
	actionWhileHeld bool
}

func (p *followProvider) Close(ctx context.Context, object any) error {
	err := p.navigationProvider.Close(ctx, object)
	if object == installed.KindHeldInput && p.scenario == "release-failure" {
		return errors.Join(err, errors.New("fixture release failed"))
	}
	return err
}

func (p *followProvider) Invoke(ctx context.Context, object any, op string, payload []byte) ([]byte, error) {
	if op == installed.OperationPressKeys {
		var request installed.PressKeysRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, err
		}
		if len(request.Keys) == 1 && request.Keys[0] == "F" {
			p.mu.Lock()
			p.actionCount++
			p.actionWhileHeld = p.actionWhileHeld || p.held
			if p.scenario == "recover" {
				p.x = 10
				p.sampled = time.Now()
			}
			p.mu.Unlock()
			return []byte(`{}`), nil
		}
	}
	raw, err := p.navigationProvider.Invoke(ctx, object, op, payload)
	if err != nil {
		return raw, err
	}
	if op == installed.OperationHoldKeys && p.scenario == "cancel" {
		p.cancel()
	}
	if op != httpegress.OperationGet {
		return raw, nil
	}
	var response httpegress.GetResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	var snapshot positionsource.Snapshot
	if err := json.Unmarshal([]byte(response.Body), &snapshot); err != nil {
		return nil, err
	}
	p.mu.Lock()
	steps := p.steps
	actionCount := p.actionCount
	p.mu.Unlock()
	if p.scenario == "height" {
		snapshot.Capabilities = append(snapshot.Capabilities, positionsource.Altitude)
		altitude := snapshot.Observations[positionsource.Position]
		altitude.Value = json.RawMessage(`0`)
		snapshot.Observations[positionsource.Altitude] = altitude
	}
	if p.scenario == "recover" && actionCount == 0 {
		position := snapshot.Observations[positionsource.Position]
		var point positionsource.Point
		if err := json.Unmarshal(position.Value, &point); err != nil {
			return nil, err
		}
		if point.X > 10 {
			point.X = 10
		}
		position.Value, _ = json.Marshal(point)
		snapshot.Observations[positionsource.Position] = position
	}
	if p.scenario == "stuck" || p.scenario == "recovery-exhausted" || p.scenario == "release-failure" {
		position := snapshot.Observations[positionsource.Position]
		position.Value = json.RawMessage(`{"x":0,"y":0}`)
		snapshot.Observations[positionsource.Position] = position
	}
	if steps > 0 && p.scenario == "epoch" {
		snapshot.Epoch = "new-session"
	}
	if steps > 0 && p.scenario == "stale" {
		position := snapshot.Observations[positionsource.Position]
		position.SampleAgeMs = 2000
		snapshot.Observations[positionsource.Position] = position
	}
	raw, err = json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	response.Body = string(raw)
	return artifact.Marshal(response)
}

// Count opens at the real Blob provider boundary, not inside the follower loop.
type countedPathReader struct {
	*blob.Provider
	opens int
}

func (p *countedPathReader) Open(ctx context.Context, request resource.ProviderOpenRequest) (any, error) {
	p.opens++
	return p.Provider.Open(ctx, request)
}
