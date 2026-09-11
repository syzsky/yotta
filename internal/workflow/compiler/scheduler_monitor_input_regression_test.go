package compiler_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/automation/inputcoord"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/workflow/compiler"
)

type monitorRegressionHeld struct {
	coordinator *inputcoord.Coordinator
	owner       *inputcoord.Owner
	lease       *inputcoord.Lease
	active      atomic.Bool
}

func (*monitorRegressionHeld) InputDomain() string { return "monitor-pause" }
func (h *monitorRegressionHeld) PauseInput(context.Context) error {
	h.active.Store(false)
	h.lease.Release()
	return nil
}
func (h *monitorRegressionHeld) ResumeInput(ctx context.Context) error {
	lease, err := h.coordinator.Acquire(ctx, h.InputDomain(), h.owner)
	if err != nil {
		return err
	}
	h.lease = lease
	h.active.Store(true)
	return nil
}

func TestMonitorInterruptParksContenderBeforeReleasingHeldInput(t *testing.T) {
	b := schedulerBuiltins(t)
	var source map[string]any
	if err := json.Unmarshal(timerBranchSource(t, b, true, false), &source); err != nil {
		t.Fatal(err)
	}
	graph := source["graphs"].([]any)[0].(map[string]any)
	makeNode := func(id, typ string, bindings map[string]any) map[string]any {
		return map[string]any{"id": id, "nodeRef": schedulerNodeRef(t, b, typ), "position": map[string]int{"x": 0, "y": 0}, "config": map[string]any{}, "bindings": bindings}
	}
	literal := func(v int) map[string]any { return map[string]any{"kind": "value", "value": v} }
	graph["nodes"] = []any{
		makeNode("started", nodes.RunStartedNodeID, map[string]any{}),
		makeNode("outer", nodes.MonitorNodeID, map[string]any{"count": literal(1), "interval-milliseconds": literal(1)}),
		makeNode("inner", nodes.MonitorNodeID, map[string]any{"count": literal(1), "interval-milliseconds": literal(1)}),
	}
	for _, id := range []string{"holder", "contender", "observer", "handler"} {
		graph["nodes"] = append(graph["nodes"].([]any), makeNode(id, nodes.DelayNodeID, map[string]any{"duration-milliseconds": literal(0)}))
	}
	edge := func(from, out, to, in string) map[string]any {
		return map[string]any{"channel": "exec", "from": map[string]string{"nodeId": from, "portId": out}, "to": map[string]string{"nodeId": to, "portId": in}}
	}
	graph["edges"] = []any{edge("started", "started", "outer", "in"), edge("outer", "main", "inner", "in"), edge("inner", "main", "holder", "in"), edge("inner", "tick", "contender", "in"), edge("outer", "tick", "observer", "in"), edge("observer", "done", "outer", "interrupt"), edge("outer", "handler", "handler", "in")}
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	program := compileSchedulerInstructionProgram(t, b, raw)
	coordinator := &inputcoord.Coordinator{}
	held := &monitorRegressionHeld{coordinator: coordinator}
	heldReady := make(chan struct{})
	contenderOwner := make(chan *inputcoord.Owner, 1)
	var contenderDispatched, handled atomic.Bool
	runtime := prepareSchedulerInstructionRuntime(t, b, program, compiler.ExecutorOptions{}, func(adapters map[string]nodeadapter.InstalledAdapter) {
		entry := adapters["control.delay"]
		original := entry.Run
		entry.Blocking, entry.PauseAtWait = true, true
		entry.Run = func(ctx context.Context, i nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
			switch i.NodeID {
			case "holder":
				held.owner = inputcoord.FromContext(ctx)
				lease, err := coordinator.Acquire(ctx, held.InputDomain(), held.owner)
				if err != nil {
					return nodeadapter.AdapterResult{}, err
				}
				held.lease = lease
				held.active.Store(true)
				held.owner.Register(held)
				close(heldReady)
				defer func() { held.owner.Unregister(held); held.active.Store(false); held.lease.Release() }()
				for !handled.Load() {
					if err := i.Wait(ctx, 10*time.Millisecond); err != nil {
						return nodeadapter.AdapterResult{}, err
					}
				}
			case "contender":
				select {
				case <-heldReady:
				case <-ctx.Done():
					return nodeadapter.AdapterResult{}, ctx.Err()
				}
				owner := inputcoord.FromContext(ctx)
				contenderOwner <- owner
				lease, err := coordinator.Acquire(ctx, held.InputDomain(), owner)
				if err != nil {
					return nodeadapter.AdapterResult{}, err
				}
				defer lease.Release()
				contenderDispatched.Store(true)
			case "observer":
				var owner *inputcoord.Owner
				select {
				case owner = <-contenderOwner:
				case <-ctx.Done():
					return nodeadapter.AdapterResult{}, ctx.Err()
				}
				for !owner.Waiting() {
					select {
					case <-ctx.Done():
						return nodeadapter.AdapterResult{}, ctx.Err()
					case <-time.After(time.Millisecond):
					}
				}
			case "handler":
				if held.active.Load() || contenderDispatched.Load() {
					return nodeadapter.AdapterResult{}, errors.New("handler received input before suspended branches released/parked")
				}
				lease, err := coordinator.Acquire(ctx, held.InputDomain(), inputcoord.FromContext(ctx))
				if err != nil {
					return nodeadapter.AdapterResult{}, err
				}
				lease.Release()
				handled.Store(true)
			}
			return original(ctx, i)
		}
		adapters["control.delay"] = entry
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := runtime.executor.Run(ctx, program, runtime.owner, runtime.journal); err != nil {
		t.Fatal(err)
	}
	if !handled.Load() {
		t.Fatal("monitor handler never ran")
	}
	if held.active.Load() {
		t.Fatal("completed Run retained held input")
	}
	lease, err := coordinator.Acquire(ctx, held.InputDomain(), inputcoord.NewOwner())
	if err != nil {
		t.Fatal("completed Run retained input domain", err)
	}
	lease.Release()
}
