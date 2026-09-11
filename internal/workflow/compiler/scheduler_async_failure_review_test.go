package compiler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/workflow/compiler"
)

func TestProducerFailureStopsPermanentPeriodicScope(t *testing.T) {
	b := schedulerBuiltins(t)
	var source map[string]any
	if err := json.Unmarshal(timerBranchSource(t, b, true, false), &source); err != nil {
		t.Fatal(err)
	}
	graph := source["graphs"].([]any)[0].(map[string]any)
	var selected []any
	for _, raw := range graph["nodes"].([]any) {
		node := raw.(map[string]any)
		switch node["id"] {
		case "started", "body":
			selected = append(selected, node)
		case "outer":
			node["nodeRef"] = schedulerNodeRef(t, b, nodes.PeriodicNodeID)
			node["bindings"] = map[string]any{"count": map[string]any{"kind": "value", "value": 0}, "interval-milliseconds": map[string]any{"kind": "value", "value": 10}}
			selected = append(selected, node)
		}
	}
	edge := func(from, out, to, in string) map[string]any {
		return map[string]any{"channel": "exec", "from": map[string]string{"nodeId": from, "portId": out}, "to": map[string]string{"nodeId": to, "portId": in}}
	}
	graph["nodes"] = selected
	graph["edges"] = []any{edge("started", "started", "body", "in"), edge("body", "done", "outer", "in")}
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	program := compileSchedulerInstructionProgram(t, b, raw)
	producerFailure := errors.New("test producer failed")
	producerFinished := make(chan struct{})
	runtime := prepareSchedulerInstructionRuntime(t, b, program, compiler.ExecutorOptions{}, func(adapters map[string]nodeadapter.InstalledAdapter) {
		entry := adapters["control.delay"]
		original := entry.Run
		entry.Run = func(ctx context.Context, invocation nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
			err := invocation.Spawn(func(context.Context) error {
				defer close(producerFinished)
				return producerFailure
			})
			if err != nil {
				return nodeadapter.AdapterResult{}, err
			}
			return original(ctx, invocation)
		}
		adapters["control.delay"] = entry
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = runtime.executor.Run(ctx, program, runtime.owner, runtime.journal)
	<-producerFinished
	if ctx.Err() != nil {
		t.Fatalf("producer failure was not delivered until the external deadline: %v", err)
	}
	if !errors.Is(err, producerFailure) || runtime.journal.Current().Status() != run.StatusFailed {
		t.Fatalf("error=%v status=%v; want producer failure and failed Run", err, runtime.journal.Current().Status())
	}
}

func TestFailureAttributionSurvivesCancellationBeyondJournalTail(t *testing.T) {
	b := schedulerBuiltins(t)
	var source map[string]any
	if err := json.Unmarshal(timerBranchSource(t, b, false, false), &source); err != nil {
		t.Fatal(err)
	}
	graph := source["graphs"].([]any)[0].(map[string]any)
	var selected []any
	for _, raw := range graph["nodes"].([]any) {
		if raw.(map[string]any)["id"] == "started" {
			selected = append(selected, raw)
		}
	}
	var edges []any
	for i := 0; i < run.JournalSegmentEntries+2; i++ {
		id, duration := fmt.Sprintf("pending-%d", i), 1000
		if i == 0 {
			id, duration = "failed", 1
		}
		selected = append(selected, map[string]any{
			"id": id, "nodeRef": schedulerNodeRef(t, b, nodes.DelayNodeID), "position": map[string]int{"x": i, "y": 0}, "config": map[string]any{},
			"bindings": map[string]any{"duration-milliseconds": map[string]any{"kind": "value", "value": duration}},
		})
		edges = append(edges, map[string]any{"channel": "exec", "from": map[string]string{"nodeId": "started", "portId": "started"}, "to": map[string]string{"nodeId": id, "portId": "in"}})
	}
	graph["nodes"], graph["edges"] = selected, edges
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	program := compileSchedulerInstructionProgram(t, b, raw)
	now := time.Date(2026, 7, 17, 2, 0, 0, 0, time.UTC)
	runtime := prepareSchedulerInstructionRuntime(t, b, program, compiler.ExecutorOptions{
		Now: func() time.Time { return now }, MonotonicNow: func() time.Time { return now },
		Wait: func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil },
	}, func(adapters map[string]nodeadapter.InstalledAdapter) {
		entry := adapters["control.delay"]
		original := entry.Run
		entry.Run = func(ctx context.Context, invocation nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
			result, err := original(ctx, invocation)
			if err == nil && result.Wait != nil && result.Wait.Duration == time.Millisecond {
				complete := result.Wait.Complete
				result.Wait.Complete = func(ctx context.Context, cause error) ([]string, error) {
					if cause == nil {
						cause = errors.New("test timer failed")
					}
					return complete(ctx, cause)
				}
			}
			return result, err
		}
		adapters["control.delay"] = entry
	})
	_, err = runtime.executor.Run(context.Background(), program, runtime.owner, runtime.journal)
	if err == nil {
		t.Fatal("expected timer failure")
	}
	failure, ok := runtime.journal.Current().Failure()
	if !ok || failure.Code != nodes.DelayFailedCode || failure.NodeID != "failed" || failure.Attempt != 1 || failure.Category != run.ErrorCategoryAdapter {
		t.Fatalf("failure attribution lost during teardown: %+v", failure)
	}
}
