package compiler

import (
	"context"
	"time"
)

// Debug pause freezes the whole Run, including work in other scopes. Waiting
// for a breakpoint must not leave a background movement operation holding W.
func (g *executionGroup) pauseForDebug(ctx context.Context) error {
	g.pause(g.root)
	for !g.quiescent(g.root) {
		for _, scope := range append([]*scheduler(nil), g.scopes...) {
			if scope.operation != nil {
				select {
				case <-scope.operation.done:
					if err := scope.finishOperation(ctx); err != nil {
						return err
					}
				default:
				}
			}
			for _, task := range scope.tasks {
				if task.resuming != nil {
					if err := task.startHandler(ctx); err != nil {
						return err
					}
				}
			}
		}
		if g.quiescent(g.root) {
			break
		}
		if err := g.waitForOperation(ctx, time.Time{}); err != nil {
			return err
		}
	}
	if err := g.pauseInputs(ctx, g.root); err != nil {
		return err
	}
	if g.root.control != nil {
		g.root.control.updateTasks(g.taskViews())
	}
	return nil
}
func (g *executionGroup) resumeForDebug(ctx context.Context) error {
	for _, owner := range g.inputOwners(g.root) {
		if err := owner.Resume(ctx); err != nil {
			return err
		}
	}
	g.resume(g.root)
	if g.root.control != nil {
		g.root.control.updateTasks(g.taskViews())
	}
	return nil
}

func (g *executionGroup) taskViews() []DebugTaskView {
	tasks := make([]DebugTaskView, 0, len(g.scopes))
	for _, scope := range g.scopes {
		task := DebugTaskView{ID: scope.scopeID, Role: scope.scopeRole, Status: "running"}
		if scope.parent != nil {
			task.ParentID = scope.parent.scopeID
		}
		for parent := scope.parent; parent != nil; parent = parent.parent {
			task.Depth++
		}
		node := scope.scopeNode
		switch {
		case scope.operation != nil:
			node = scope.operation.activation.node
			task.Status = "waiting_result"
		case len(scope.queue) > 0:
			node = scope.nodes[scope.queue[0].nodeID]
		case len(scope.waits) > 0:
			node = scope.waits[0].activation.node
			task.Status = "waiting"
		case scope.keepAlive > 0:
			task.Status = "monitoring"
		}
		if scope.paused {
			task.Status = "paused"
			if !g.quiescent(scope) {
				task.Status = "pausing"
			}
		}
		task.NodeID = node.SourceNodeID
		task.GraphPath = append([]string(nil), node.GraphPath...)
		tasks = append(tasks, task)
	}
	return tasks
}
func (g *executionGroup) publishTasks() {
	if g.root.control == nil {
		return
	}
	now := g.root.executor.monotonicNow()
	if !g.lastDebugTasks.IsZero() && now.Sub(g.lastDebugTasks) < 100*time.Millisecond {
		return
	}
	g.lastDebugTasks = now
	g.root.control.updateTasks(g.taskViews())
}
