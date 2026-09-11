package compiler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/automation/inputcoord"
)

type pauseRegressionHeld struct {
	lease  *inputcoord.Lease
	paused atomic.Bool
}

func (*pauseRegressionHeld) InputDomain() string { return "pause-regression" }
func (h *pauseRegressionHeld) PauseInput(context.Context) error {
	h.lease.Release()
	h.paused.Store(true)
	return nil
}
func (*pauseRegressionHeld) ResumeInput(context.Context) error { return nil }

// A held input in one scope must not prevent the whole Run from pausing when
// another scope is awaiting that same physical domain. The pending adapter
// reaches its cooperative pause point only after its input acquisition returns.
func TestDebugPauseReleasesHolderBeforeWaitingForInputContender(t *testing.T) {
	root := &scheduler{executor: &Executor{monotonicNow: time.Now}}
	g := newExecutionGroup(root)
	g.operations = newOperationPool()
	child := &scheduler{executor: root.executor, group: g, parent: root,
		clock:      &scopeClock{now: time.Now, changed: make(chan struct{})},
		inputOwner: inputcoord.NewOwner()}
	child.inputOwner.SetWake(g.wake)
	g.scopes = append(g.scopes, child)
	coordinator := &inputcoord.Coordinator{}
	lease, err := coordinator.Acquire(context.Background(), "pause-regression", root.inputOwner)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	held := &pauseRegressionHeld{lease: lease}
	root.inputOwner.Register(held)
	opCtx, cancelOperation := context.WithCancel(context.Background())
	op := &pendingOperation{done: make(chan struct{})}
	child.operation = op
	started := make(chan struct{})
	acquired := make(chan struct{})
	go func() {
		defer close(op.done)
		close(started)
		contenderLease, err := coordinator.Acquire(opCtx, "pause-regression", child.inputOwner)
		if err != nil {
			return
		}
		defer contenderLease.Release()
		close(acquired)
		_ = child.waitActive(opCtx, op, time.Hour, nil)
	}()
	defer func() { cancelOperation(); <-op.done }()
	<-started
	pauseCtx, cancelPause := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelPause()
	if err := g.pauseForDebug(pauseCtx); err != nil {
		t.Fatalf("pause could not hand input to the pending scope: %v; holder released=%v", err, held.paused.Load())
	}
	if !held.paused.Load() {
		t.Fatal("debug pause retained physical input")
	}
	select {
	case <-acquired:
		t.Fatal("contender dispatched physical input while paused")
	default:
	}
	if !child.inputOwner.Waiting() {
		t.Fatal("debug pause returned before the contender parked")
	}
	if err := g.resumeForDebug(pauseCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-acquired:
	case <-pauseCtx.Done():
		t.Fatal("contender did not resume")
	}
}
