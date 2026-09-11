package noderuntime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	x, heading           float64
	turns, steps, closed int
}

func (p *navigationProvider) Open(_ context.Context, r resource.ProviderOpenRequest) (any, error) {
	return r.Kind, nil
}
func (p *navigationProvider) Close(context.Context, any) error { p.closed++; return nil }
func (p *navigationProvider) Invoke(_ context.Context, _ any, op string, payload []byte) ([]byte, error) {
	switch op {
	case httpegress.OperationGet:
		return artifact.Marshal(httpegress.GetResponse{StatusCode: 200, ContentType: "application/json", Body: fmt.Sprintf(`{"valid":true,"x":%v,"y":0,"cameraHeading":%v,"sampleTimeMs":%d}`, p.x, p.heading, time.Now().Add(time.Millisecond).UnixMilli())})
	case installed.OperationTurnView:
		var req installed.TurnViewRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		p.heading += req.Degrees
		p.turns++
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

func TestCharacterMoveCompilesAndUsesFreshConfiguredSource(t *testing.T) {
	b, err := nodes.Build()
	if err != nil {
		t.Fatal(err)
	}
	bindings := map[string]any{}
	for key, value := range map[string]any{"target-x": 10, "target-y": 0, "tolerance": 0.6, "timeout": 10000, "pulse": 200} {
		bindings[key] = map[string]any{"kind": "value", "value": value}
	}
	program := compilePrimitiveProgram(t, b, navigationSource(t, b, nodes.MoveCharacterNodeID, map[string]any{"slot": "game", "source": "position", "axisHeading": 90}, bindings))
	p := &navigationProvider{heading: 180}
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
	_, owner, journal := admittedExecution(t, b, program, map[string]run.InstalledProvider{}, now)
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
	if distance > 0.6 || p.turns != 2 || p.steps == 0 || p.closed != 3 {
		t.Fatalf("distance=%v provider=%+v", distance, p)
	}
	actions := 0
	for _, entry := range journal.Current().Journal() {
		if entry.Kind == run.JournalAdapterAction && entry.NodeID == "navigation" {
			actions++
		}
	}
	if actions != 1 {
		t.Fatalf("adapter journal actions=%d", actions)
	}
}
