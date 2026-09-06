// Command plugin-management-smoke exercises an externally built plugin through
// installation, profile reopening, authoring, and the production Run path.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yottaapp/yotta/internal/localruntime"
	"github.com/yottaapp/yotta/internal/noderuntime"
	"github.com/yottaapp/yotta/internal/services/workflow"
	"github.com/yottaapp/yotta/internal/workflow/authoring"
	"github.com/yottaapp/yotta/internal/workflow/schema"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	archive := flag.String("archive", "", "external .ynp archive")
	root := flag.String("root", "", "isolated profile root")
	executable := flag.String("host", "bin/Yotta.exe", "production host executable")
	flag.Parse()
	if *archive == "" || *root == "" {
		return fmt.Errorf("archive and isolated root required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	exe, err := filepath.Abs(*executable)
	if err != nil {
		return err
	}
	config := localruntime.Config{StorageRoot: *root, Executable: exe, WorkflowLog: noderuntime.LogEmitterFunc(func(context.Context, noderuntime.LogEntry) error { return nil })}
	first, err := localruntime.Open(ctx, config)
	if err != nil {
		return err
	}
	err = first.Plugins.Import(*archive)
	closeErr := first.Close(context.Background())
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	current, err := localruntime.Open(ctx, config)
	if err != nil {
		return err
	}
	defer current.Close(context.Background())
	if err = current.Workflow.Application.Start(ctx); err != nil {
		return err
	}
	list, err := current.Plugins.List()
	if err != nil || len(list) == 0 {
		return fmt.Errorf("installed plugin missing: %v", err)
	}
	if !list[0].Loaded || !list[0].Enabled || list[0].RestartRequired {
		return fmt.Errorf("installed plugin was not loaded")
	}
	deps := current.Workflow.Application.NodePackageDependencies()
	if len(deps) == 0 || len(deps[0].NodeRefs) == 0 {
		return fmt.Errorf("package authoring dependencies unavailable")
	}
	service, err := workflow.NewService(current.Workflow.Application)
	if err != nil {
		return err
	}
	view, err := service.CreateSource("Plugin workflow example")
	if err != nil {
		return err
	}
	var source schema.WorkflowSource
	if err = json.Unmarshal([]byte(view.SourceJSON), &source); err != nil {
		return err
	}
	firstNode := source.Graphs[0].Nodes[0].ID
	patched, err := service.ApplyPatch(view.WorkflowID, view.Revision, []authoring.Command{
		{Kind: authoring.CommandAddNode, AddNode: &authoring.AddNodeCommand{GraphID: source.EntryGraph, NodeTypeID: deps[0].NodeRefs[0].NodeTypeID, Handle: "plugin", Position: schema.Position{X: 500, Y: 160}}},
		{Kind: authoring.CommandConnect, Connect: &authoring.EdgeCommand{GraphID: source.EntryGraph, Edge: authoring.PatchEdge{Channel: schema.EdgeExec, From: authoring.PatchEndpoint{NodeID: authoring.PatchNodeReference(firstNode), PortID: "started"}, To: authoring.PatchEndpoint{NodeID: "$plugin", PortID: "in"}}}},
	})
	if err != nil {
		return err
	}
	if err = json.Unmarshal([]byte(patched.Source.SourceJSON), &source); err != nil {
		return err
	}
	if len(source.Dependencies) == 0 {
		return fmt.Errorf("package dependency was not persisted")
	}
	started, err := service.StartRun(view.WorkflowID)
	if err != nil {
		return err
	}
	if started.Run == nil {
		return fmt.Errorf("run rejected: %+v", started)
	}
	for {
		run, err := service.GetRunTimeline(started.Run.RunID)
		if err != nil {
			return err
		}
		switch run.Status {
		case "succeeded":
			fmt.Printf("Workflow %s Run %s succeeded\n", view.WorkflowID, run.RunID)
			return nil
		case "failed", "cancelled":
			return fmt.Errorf("run %s: %+v", run.Status, run.Failure)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
