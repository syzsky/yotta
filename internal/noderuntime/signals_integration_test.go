package noderuntime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/noderuntime"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/panel"
	"github.com/yottaapp/yotta/internal/signals"
	"github.com/yottaapp/yotta/internal/workflow/compiler"
	panelcontract "github.com/yottaapp/yotta/sdk/plugin/panel"
	"reflect"
	"sync"
	"testing"
	"time"
)

func signalSource(t *testing.T, b nodes.Builtins) []byte {
	t.Helper()
	var source map[string]any
	json.Unmarshal(navigationSource(t, b, nodes.SignalPrefix+"listen", map[string]any{}, map[string]any{}), &source)
	graph := source["graphs"].([]any)[0].(map[string]any)
	makeNode := func(id, kind string, values map[string]any) map[string]any {
		def, _ := b.Definition(kind)
		bindings := map[string]any{}
		for _, port := range def.Contract.Machine().Ports.DataInputs {
			if v, ok := values[port.ID]; ok {
				bindings[port.ID] = map[string]any{"kind": "value", "value": v}
			} else if port.Default != nil {
				bindings[port.ID] = map[string]any{"kind": "default"}
			}
		}
		return map[string]any{"id": id, "nodeRef": def.Contract.NodeRef(), "position": map[string]int{"x": 0, "y": 0}, "config": map[string]any{}, "bindings": bindings}
	}
	graph["nodes"] = []any{makeNode("start", nodes.RunStartedNodeID, nil), makeNode("listen", nodes.SignalPrefix+"listen", nil), makeNode("one", nodes.SignalPrefix+"send", map[string]any{"value": 1}), makeNode("two", nodes.SignalPrefix+"send", map[string]any{"value": 2}), makeNode("three", nodes.SignalPrefix+"send", map[string]any{"value": 3}), makeNode("wait", nodes.SignalPrefix+"wait", map[string]any{"name": "done"}), makeNode("delay", nodes.DelayNodeID, map[string]any{"duration-milliseconds": 10}), makeNode("ack", nodes.SignalPrefix+"send", map[string]any{"name": "ack"})}
	edge := func(channel, from, out, to, in string) map[string]any {
		return map[string]any{"channel": channel, "from": map[string]string{"nodeId": from, "portId": out}, "to": map[string]string{"nodeId": to, "portId": in}}
	}
	graph["edges"] = []any{edge("exec", "start", "started", "listen", "in"), edge("exec", "listen", "main", "one", "in"), edge("exec", "one", "completed", "two", "in"), edge("exec", "two", "completed", "three", "in"), edge("exec", "three", "completed", "wait", "in"), edge("exec", "listen", "event", "delay", "in"), edge("exec", "delay", "done", "ack", "in"), edge("data", "listen", "value", "ack", "value")}
	// The handler consumes this edge rather than the send node's default payload.
	delete(graph["nodes"].([]any)[7].(map[string]any)["bindings"].(map[string]any), "value")
	raw, e := json.Marshal(source)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func TestSignalListenerSeriallyDrainsBurstAndClosesWithMain(t *testing.T) {
	testSignalListener(t, false)
}
func TestSignalListenerStopCancelsItsWaitingMain(t *testing.T) { testSignalListener(t, true) }
func testSignalListener(t *testing.T, stop bool) {
	b, e := nodes.Build()
	if e != nil {
		t.Fatal(e)
	}
	raw := signalSource(t, b)
	if stop {
		var source map[string]any
		json.Unmarshal(raw, &source)
		graph := source["graphs"].([]any)[0].(map[string]any)
		graph["edges"] = append(graph["edges"].([]any), map[string]any{"channel": "exec", "from": map[string]string{"nodeId": "ack", "portId": "completed"}, "to": map[string]string{"nodeId": "listen", "portId": "stop"}})
		raw, _ = json.Marshal(source)
	}
	program := compilePrimitiveProgram(t, b, raw)
	_, owner, journal := admittedExecution(t, b, program, nil, time.Now().UTC())
	defer owner.Close(context.Background())
	adapters, e := noderuntime.Installed(b, testDependencies())
	if e != nil {
		t.Fatal(e)
	}
	seen := []int{}
	entry := adapters["signals.send"]
	original := entry.Run
	entry.Run = func(ctx context.Context, inv nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
		if inv.NodeID == "ack" {
			var value int
			json.Unmarshal(inv.Inputs["value"].InlineJSON(), &value)
			seen = append(seen, value)
			if len(seen) == 3 {
				inv.Signals.Publish("done", signals.Message{Value: json.RawMessage(`null`)})
			}
		}
		return original(ctx, inv)
	}
	adapters["signals.send"] = entry
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, e = compiler.NewExecutor(b.Catalog, adapters, compiler.ExecutorOptions{}).Run(ctx, program, owner, journal)
	if e != nil {
		t.Fatal(e)
	}
	expected := []int{1, 2, 3}
	if stop {
		expected = []int{1}
	}
	if !reflect.DeepEqual(seen, expected) {
		t.Fatal(seen)
	}
}

func TestPanelListenerBroadcastsEachClickToTwoRuns(t *testing.T) {
	b, e := nodes.Build()
	if e != nil {
		t.Fatal(e)
	}
	service := panel.New(nil)
	defer service.Close()
	draft, e := service.Save(panel.Draft{Title: "Signals", Components: []panel.ComponentDraft{{Kind: "button", Title: "Next"}}})
	if e != nil {
		t.Fatal(e)
	}
	var source map[string]any
	json.Unmarshal(signalSource(t, b), &source)
	graph := source["graphs"].([]any)[0].(map[string]any)
	listener := graph["nodes"].([]any)[1].(map[string]any)
	def, _ := b.Definition(nodes.ManagedPanelPrefix + "listen")
	listener["nodeRef"] = def.Contract.NodeRef()
	listener["config"] = map[string]any{"panel": draft.ID, "component": draft.Components[0].ID}
	listener["bindings"] = map[string]any{}
	raw, _ := json.Marshal(source)
	program := compilePrimitiveProgram(t, b, raw)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	counts := [2]int{}
	for runIndex := 0; runIndex < 2; runIndex++ {
		_, owner, journal := admittedExecution(t, b, program, nil, time.Now().UTC())
		deps := testDependencies()
		deps.Panels = service
		adapters, e := noderuntime.Installed(b, deps)
		if e != nil {
			t.Fatal(e)
		}
		entry := adapters["signals.send"]
		original := entry.Run
		entry.Run = func(ctx context.Context, inv nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
			if inv.NodeID == "ack" {
				counts[runIndex]++
				if counts[runIndex] == 3 {
					inv.Signals.Publish("done", signals.Message{Value: json.RawMessage(`null`)})
				}
			}
			return original(ctx, inv)
		}
		adapters["signals.send"] = entry
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer owner.Close(context.Background())
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := compiler.NewExecutor(b.Catalog, adapters, compiler.ExecutorOptions{}).Run(ctx, program, owner, journal)
			results <- err
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		sources := service.List()
		if len(sources) == 1 && sources[0].Waiting == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("both runs did not subscribe")
		}
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 3; i++ {
		snapshot, e := service.Read(draft.ID)
		if e != nil {
			t.Fatal(e)
		}
		_, e = service.Dispatch(draft.ID, panelcontract.Event{SessionID: snapshot.SessionID, EventID: fmt.Sprintf("click-%d", i), ComponentID: draft.Components[0].ID, Name: draft.Components[0].ID + ".change", Revision: snapshot.ControlRevisions[draft.Components[0].ID], Value: nil})
		if e != nil {
			t.Fatal(e)
		}
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if counts != [2]int{3, 3} {
		t.Fatal(counts)
	}
	if service.List()[0].Waiting != 0 {
		t.Fatal("subscriptions leaked after both runs ended")
	}
}

func TestSignalWaitTimeoutIsFailureRatherThanRunCancellation(t *testing.T) {
	b, e := nodes.Build()
	if e != nil {
		t.Fatal(e)
	}
	raw := navigationSource(t, b, nodes.SignalPrefix+"wait", map[string]any{"timeoutMs": 10}, map[string]any{"name": map[string]any{"kind": "default"}})
	program := compilePrimitiveProgram(t, b, raw)
	_, owner, journal := admittedExecution(t, b, program, nil, time.Now().UTC())
	defer owner.Close(context.Background())
	adapters, e := noderuntime.Installed(b, testDependencies())
	if e != nil {
		t.Fatal(e)
	}
	_, e = compiler.NewExecutor(b.Catalog, adapters, compiler.ExecutorOptions{}).Run(context.Background(), program, owner, journal)
	if e == nil || string(journal.Current().Status()) != "failed" {
		t.Fatalf("status=%s err=%v", journal.Current().Status(), e)
	}
	found := false
	for _, entry := range journal.Current().Journal() {
		if entry.ErrorCode == "signals.wait_timeout" {
			found = true
		}
	}
	if !found {
		t.Fatal("timeout failure code not retained")
	}
}
