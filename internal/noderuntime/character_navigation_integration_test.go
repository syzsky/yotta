package noderuntime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/httpegress"
	"github.com/yottaapp/yotta/internal/noderuntime"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/resource"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/targetruntime"
	"github.com/yottaapp/yotta/internal/workflow/compiler"
)

type navigationProvider struct {
	mu                   sync.Mutex
	held                 bool
	sampled              time.Time
	readyAt              time.Time
	opened               int
	x, heading           float64
	turns, steps, closed int
}

func (p *navigationProvider) Open(_ context.Context, r resource.ProviderOpenRequest) (any, error) {
	if r.Kind == installed.KindHeldInput && !slices.Equal(r.Operations, installed.HeldInputOperations()) {
		return nil, fmt.Errorf("held input session requires exact operations: %v", r.Operations)
	}
	p.mu.Lock()
	p.opened++
	p.mu.Unlock()
	return r.Kind, nil
}
func (p *navigationProvider) Close(_ context.Context, object any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed++
	if object == installed.KindHeldInput {
		p.held = false
	}
	return nil
}
func (p *navigationProvider) Invoke(_ context.Context, _ any, op string, payload []byte) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch op {
	case httpegress.OperationGet:
		now := time.Now()
		if now.Before(p.readyAt) {
			return artifact.Marshal(httpegress.GetResponse{StatusCode: 200, ContentType: "application/json", Body: `{"valid":false}`})
		}
		if p.held && !p.sampled.IsZero() {
			p.x += now.Sub(p.sampled).Seconds() * 10
		}
		p.sampled = now
		return artifact.Marshal(httpegress.GetResponse{StatusCode: 200, ContentType: "application/json", Body: fmt.Sprintf(`{"valid":true,"x":%v,"y":0,"cameraHeading":%v,"sampleTimeMs":%d}`, p.x, p.heading, time.Now().Add(time.Millisecond).UnixMilli())})
	case installed.OperationTurnView:
		var req installed.TurnViewRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		p.heading += req.Degrees
		p.turns++
	case installed.OperationHoldKeys:
		if math.Abs(p.heading-90) > 5 {
			return nil, fmt.Errorf("held forward before facing target")
		}
		p.held = true
		p.sampled = time.Now()
		p.steps++
	case installed.OperationPressKeys:
		var req installed.PressKeysRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		if math.Abs(p.heading-90) > 5 || len(req.Keys) != 1 || req.Keys[0] != "W" {
			return nil, fmt.Errorf("walked before facing target: %v %+v", p.heading, req)
		}
		p.x += float64(req.DurationMilliseconds) * 0.01
		p.steps++
	default:
		return nil, fmt.Errorf("unexpected operation %s", op)
	}
	return []byte(`{}`), nil
}

func navigationSource(t *testing.T, b nodes.Builtins, id string, config map[string]any, bindings map[string]any) []byte {
	t.Helper()
	start, _ := b.Definition(nodes.RunStartedNodeID)
	node, _ := b.Definition(id)
	ref := func(def nodes.BuiltinDefinition) map[string]any {
		r := def.Contract.NodeRef()
		return map[string]any{"nodeTypeId": r.NodeTypeID, "version": r.Version, "semanticDigest": r.SemanticDigest}
	}
	source := map[string]any{"format": "yotta.workflow", "version": "1", "workflow": map[string]any{"id": "navigation-test", "name": "Navigation test"}, "revision": 0, "entryGraph": "main",
		"graphs": []any{map[string]any{"id": "main", "kind": "main", "nodes": []any{
			map[string]any{"id": "start", "nodeRef": ref(start), "position": map[string]int{"x": 0, "y": 0}, "config": map[string]any{}, "bindings": map[string]any{}},
			map[string]any{"id": "navigation", "nodeRef": ref(node), "position": map[string]int{"x": 100, "y": 0}, "config": config, "bindings": bindings},
		}, "edges": []any{map[string]any{"channel": "exec", "from": map[string]string{"nodeId": "start", "portId": "started"}, "to": map[string]string{"nodeId": "navigation", "portId": "in"}}}, "inputs": []any{}, "outputs": []any{}}},
		"variables": []any{}, "resources": []any{}, "targetProfileDefinitions": []any{}, "credentialRequirements": []any{}, "dependencies": []any{},
	}
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestCharacterMoveConsumesIndependentPositionUpdates(t *testing.T) {
	testCharacterPositionFeed(t, 0)
}

func TestCharacterMoveWaitsForColdPositionSource(t *testing.T) {
	testCharacterPositionFeed(t, 1500*time.Millisecond)
}

func testCharacterPositionFeed(t *testing.T, startupDelay time.Duration) {
	b, err := nodes.Build()
	if err != nil {
		t.Fatal(err)
	}
	bindings := map[string]any{}
	for key, value := range map[string]any{"target-x": 10, "target-y": 0, "tolerance": 0.6, "timeout": 60000, "interval": 100, "slow-distance": 3} {
		bindings[key] = map[string]any{"kind": "value", "value": value}
	}

	raw := navigationPositionFeed(t, b, navigationSource(t, b, nodes.MoveCharacterNodeID, map[string]any{"slot": "game", "position-variable": "position"}, bindings), map[string]any{"axisHeading": 90})
	program := compilePrimitiveProgram(t, b, raw)
	if slots := program.ConfiguredTargetSlots(b.Catalog); !slices.Equal(slots, []string{"game", "position"}) {
		t.Fatalf("resolved navigation target slots: %v", slots)
	}

	p := &navigationProvider{heading: 180, readyAt: time.Now().Add(startupDelay)}
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
	_, owner, journal := admittedExecutionWithConsent(t, b, program, map[string]run.InstalledProvider{}, now, executionProfile(t, b), nil, func(value *run.Store) { store = value })
	defer owner.Close(context.Background())
	adapters, err := noderuntime.Installed(b, testDependencies())
	if err != nil {
		t.Fatal(err)
	}
	result, err := compiler.NewExecutor(b.Catalog, adapters, compiler.ExecutorOptions{Now: func() time.Time { return now }}).RunWithTargets(context.Background(), program, owner, targets, journal)
	if err != nil {
		t.Fatal(err)
	}
	var distance float64
	if err := json.Unmarshal(result.NodeOutputs["navigation"]["distance"].InlineJSON(), &distance); err != nil {
		t.Fatal(err)
	}
	if distance > 0.6 || p.turns != 2 || p.steps == 0 || p.closed != p.opened || p.held {
		t.Fatalf("distance=%v provider=%+v", distance, p)
	}
	actions := 0
	timeline, err := store.TimelineSnapshot(context.Background(), journal.Current().Admission().RunID)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range timeline.Entries {
		if entry.Kind == run.JournalAdapterAction && entry.NodeID == "navigation" {
			actions++
		}
	}
	if actions != 1 {
		t.Fatalf("adapter journal actions=%d", actions)
	}
}

func navigationPositionFeed(t *testing.T, b nodes.Builtins, input []byte, parseConfig map[string]any) []byte {
	t.Helper()
	var source map[string]any
	if err := json.Unmarshal(input, &source); err != nil {
		t.Fatal(err)
	}
	graph := source["graphs"].([]any)[0].(map[string]any)
	makeNode := func(id, kind string, config, bindings map[string]any) map[string]any {
		def, _ := b.Definition(kind)
		for _, port := range def.Contract.Machine().Ports.DataInputs {
			if _, ok := bindings[port.ID]; !ok && port.Default != nil {
				bindings[port.ID] = map[string]any{"kind": "default"}
			}
		}
		return map[string]any{"id": id, "nodeRef": def.Contract.NodeRef(), "position": map[string]int{"x": 0, "y": 0}, "config": config, "bindings": bindings}
	}
	graph["nodes"] = append(graph["nodes"].([]any),
		makeNode("monitor", nodes.MonitorNodeID, map[string]any{}, map[string]any{"interval-milliseconds": map[string]any{"kind": "value", "value": 20}}),
		makeNode("get", nodes.HTTPGetNodeID, map[string]any{"slot": "position"}, map[string]any{"path": map[string]any{"kind": "value", "value": "/v1/position"}}),
		makeNode("parse", nodes.ParseWorldPositionNodeID, parseConfig, map[string]any{}),
		makeNode("write", nodes.StateWriteNodeID, map[string]any{"variable": "position"}, map[string]any{}))
	edge := func(channel, from, out, to, in string) map[string]any {
		return map[string]any{"channel": channel, "from": map[string]string{"nodeId": from, "portId": out}, "to": map[string]string{"nodeId": to, "portId": in}}
	}
	graph["edges"] = []any{edge("exec", "start", "started", "monitor", "in"), edge("exec", "monitor", "main", "navigation", "in"), edge("exec", "monitor", "tick", "get", "in"), edge("exec", "get", "completed", "parse", "in"), edge("data", "get", "body", "parse", "source"), edge("exec", "parse", "done", "write", "in"), edge("data", "parse", "position", "write", "value")}
	source["variables"] = []any{map[string]any{"name": "position", "type": map[string]any{"kind": "ref", "ref": b.WorldPositionType.TypeRef()}, "default": b.WorldPositionType.Authoring().Examples[0]}}
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
