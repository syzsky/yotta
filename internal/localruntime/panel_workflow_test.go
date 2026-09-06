package localruntime

import (
	"context"
	"github.com/yottaapp/yotta/internal/noderuntime"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/panel"
	wf "github.com/yottaapp/yotta/internal/services/workflow"
	"github.com/yottaapp/yotta/internal/workflow/authoring"
	"github.com/yottaapp/yotta/internal/workflow/schema"
	contract "github.com/yottaapp/yotta/sdk/plugin/panel"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkflowUsesManagedPanelAndLeavesItInteractive(t *testing.T) {
	ctx := context.Background()
	rt, e := Open(ctx, Config{StorageRoot: t.TempDir(), Executable: filepath.Join(t.TempDir(), "Yotta.exe"), WorkflowLog: noderuntime.LogEmitterFunc(func(context.Context, noderuntime.LogEntry) error { return nil })})
	if e != nil {
		t.Fatal(e)
	}
	defer rt.Close(ctx)
	if e = rt.Workflow.Application.Start(ctx); e != nil {
		t.Fatal(e)
	}
	d, e := rt.Panels.Save(panel.Draft{Title: "Independent", Components: []panel.ComponentDraft{{Kind: "toggle", Title: "Choice", Initial: false}}})
	if e != nil {
		t.Fatal(e)
	}
	svc, e := wf.NewService(rt.Workflow.Application)
	if e != nil {
		t.Fatal(e)
	}
	view, e := svc.CreateSource("Use existing panel")
	if e != nil {
		t.Fatal(e)
	}
	commands := []authoring.Command{}
	kinds := []string{"use", "ref-toggle", "write-toggle", "wait", "read-toggle"}
	for i, kind := range kinds {
		commands = append(commands, authoring.Command{Kind: authoring.CommandAddNode, AddNode: &authoring.AddNodeCommand{GraphID: "main", NodeTypeID: nodes.ManagedPanelPrefix + kind, Handle: kind, Position: schema.Position{X: float64(i * 200)}}})
	}
	patch, e := svc.ApplyPatch(view.WorkflowID, view.Revision, commands)
	if e != nil {
		t.Fatal(e)
	}
	ids := map[string]string{}
	for _, n := range patch.GeneratedNodes {
		ids[n.Handle] = n.NodeID
	}
	commands = []authoring.Command{{Kind: authoring.CommandSetTargetDefault, SetTargetDefault: &authoring.SetTargetDefaultCommand{Target: "panel", Slot: d.ID}}, {Kind: authoring.CommandBindValue, BindValue: &authoring.BindValueCommand{GraphID: "main", NodeID: ids["write-toggle"], PortID: "value", Value: true}}}
	from, port := "run-started", "started"
	for _, kind := range kinds {
		if kind != "use" {
			commands = append(commands, authoring.Command{Kind: authoring.CommandSetConfig, SetConfig: &authoring.SetConfigCommand{GraphID: "main", NodeID: ids[kind], FieldID: "component", Value: d.Components[0].ID}})
		}
		commands = append(commands, authoring.Command{Kind: authoring.CommandConnect, Connect: &authoring.EdgeCommand{GraphID: "main", Edge: authoring.PatchEdge{Channel: schema.EdgeExec, From: authoring.PatchEndpoint{NodeID: authoring.PatchNodeReference(from), PortID: port}, To: authoring.PatchEndpoint{NodeID: authoring.PatchNodeReference(ids[kind]), PortID: "in"}}}})
		from = ids[kind]
		port = "completed"
	}
	commands = append(commands, authoring.Command{Kind: authoring.CommandSetConfig, SetConfig: &authoring.SetConfigCommand{GraphID: "main", NodeID: ids["use"], FieldID: "show", Value: false}})
	for _, edge := range []struct{ from, output, to, input string }{{"use", "panel", "ref-toggle", "panel-ref"}, {"ref-toggle", "reference", "write-toggle", "component-ref"}} {
		commands = append(commands, authoring.Command{Kind: authoring.CommandConnect, Connect: &authoring.EdgeCommand{GraphID: "main", Edge: authoring.PatchEdge{Channel: schema.EdgeData, From: authoring.PatchEndpoint{NodeID: authoring.PatchNodeReference(ids[edge.from]), PortID: edge.output}, To: authoring.PatchEndpoint{NodeID: authoring.PatchNodeReference(ids[edge.to]), PortID: edge.input}}}})
	}
	if _, e = svc.ApplyPatch(view.WorkflowID, patch.Source.Revision, commands); e != nil {
		t.Fatal(e)
	}
	started, e := svc.StartRun(view.WorkflowID)
	if e != nil || started.Run == nil {
		t.Fatalf("%+v %v", started, e)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		ready := false
		for _, p := range rt.Panels.List() {
			ready = ready || p.ID == d.ID && p.Waiting > 0
		}
		if ready {
			break
		}
		run, _ := svc.GetRunTimeline(started.Run.RunID)
		if run.Status == "failed" {
			t.Fatalf("%+v", run.Failure)
		}
		if time.Now().After(deadline) {
			t.Fatal("not waiting")
		}
		time.Sleep(5 * time.Millisecond)
	}
	state, e := rt.Panels.Read(d.ID)
	if e != nil || state.Values[d.Components[0].ID] != true {
		t.Fatalf("default panel not used: %+v %v", state, e)
	}
	event := contract.Event{SessionID: state.SessionID, EventID: "choose", ComponentID: d.Components[0].ID, Name: d.Components[0].ID + ".change", Revision: state.ControlRevisions[d.Components[0].ID], Value: false}
	if _, e = rt.Panels.Dispatch(d.ID, event); e != nil {
		t.Fatal(e)
	}
	for {
		run, _ := svc.GetRunTimeline(started.Run.RunID)
		if run.Status == "succeeded" {
			break
		}
		if run.Status == "failed" {
			t.Fatalf("%+v", run.Failure)
		}
		if time.Now().After(deadline) {
			t.Fatal("run did not resume")
		}
		time.Sleep(5 * time.Millisecond)
	}
	state, _ = rt.Panels.Read(d.ID)
	event.EventID = "after-run"
	event.Revision = state.ControlRevisions[event.ComponentID]
	event.Value = true
	if _, e = rt.Panels.Dispatch(d.ID, event); e != nil {
		t.Fatal("finished workflow disabled independent panel", e)
	}
	if len(rt.Panels.List()) != 1 {
		t.Fatal("workflow created another panel")
	}
}
