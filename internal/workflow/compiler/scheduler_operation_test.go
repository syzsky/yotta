package compiler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/nodeadapter"
	run "github.com/yottaapp/yotta/internal/run"
)

func TestBlockingAdapterCompletesBeforeSuccessor(t *testing.T) {
	catalog, contracts, locks := schedulerCatalogForTest(t)
	program := compileSchedulerProgram(t, catalog, contracts, false)
	owner, journal := admittedSchedulerExecution(t, catalog, program, time.Now().UTC())
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	completed := false
	adapters := schedulerAdapters(locks, map[string]Adapter{
		"source": func(ctx context.Context, _ Invocation) (AdapterResult, error) {
			completed = true
			return AdapterResult{ExecOutputs: []string{"right"}}, nil
		},
		"right": func(context.Context, Invocation) (AdapterResult, error) {
			if !completed {
				t.Error("successor ran before producer completion")
			}
			return AdapterResult{}, nil
		},
	})
	entry := adapters[locks["source"].Entrypoint]
	entry.Blocking = true
	adapters[locks["source"].Entrypoint] = entry
	executor := NewExecutor(catalog, adapters, ExecutorOptions{})
	if _, err := executor.Run(context.Background(), program, owner, journal); err != nil {
		t.Fatal(err)
	}
	if journal.Current().Status() != run.StatusSucceeded {
		t.Fatal("asynchronous attempt was not sealed")
	}
}

func TestOperationPoolDoesNotStarveHandlersBehindParkedTasks(t *testing.T) {
	p := newOperationPool()
	release := make(chan struct{})
	defer func() { close(release); close(p.jobs); p.workers.Wait() }()
	for range maxExecutionScopes - 1 {
		p.submit(&pendingOperation{ctx: context.Background(), done: make(chan struct{}), run: func(context.Context, nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
			<-release
			return nodeadapter.AdapterResult{}, nil
		}})
	}
	handler := &pendingOperation{ctx: context.Background(), done: make(chan struct{}), run: func(context.Context, nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
		return nodeadapter.AdapterResult{}, nil
	}}
	p.submit(handler)
	select {
	case <-handler.done:
	case <-time.After(5 * time.Second):
		t.Fatal("parked tasks starved their handler")
	}
	if p.count > operationWorkers {
		t.Fatal("worker budget exceeded")
	}
}

func TestPausedWaitFreezesTimeAndAcknowledgesBeforeResume(t *testing.T) {
	s := &scheduler{executor: &Executor{monotonicNow: time.Now}}
	g := newExecutionGroup(s)
	g.operations = newOperationPool()
	op := &pendingOperation{}
	s.operation = op
	g.pause(s)
	before := s.activeNow()
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { done <- s.waitActive(ctx, op, 10*time.Millisecond, nil) }()
	select {
	case <-g.operations.wake:
	case <-time.After(time.Second):
		t.Fatal("pause was not acknowledged")
	}
	if !g.quiescent(s) {
		t.Fatal("acknowledged scope is not quiescent")
	}
	if !s.activeNow().Equal(before) {
		t.Fatal("paused clock advanced")
	}
	select {
	case <-done:
		t.Fatal("paused wait completed")
	default:
	}
	g.resume(s)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not resume")
	}
}

func TestAsyncCommittedCompletionIsPreservedDuringCancellation(t *testing.T) {
	catalog, contracts, locks := schedulerCatalogForTest(t)
	program := compileSchedulerProgram(t, catalog, contracts, false)
	owner, journal := admittedSchedulerExecution(t, catalog, program, time.Now().UTC())
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	adapters := schedulerAdapters(locks, map[string]Adapter{
		"source": func(context.Context, Invocation) (AdapterResult, error) {
			calls++
			cancel()
			return AdapterResult{ExecOutputs: []string{"right"}}, nil
		},
		"right": func(context.Context, Invocation) (AdapterResult, error) {
			t.Error("cancelled scope dispatched successor")
			return AdapterResult{}, nil
		},
	})
	entry := adapters[locks["source"].Entrypoint]
	entry.Blocking = true
	adapters[locks["source"].Entrypoint] = entry
	_, err := NewExecutor(catalog, adapters, ExecutorOptions{}).Run(ctx, program, owner, journal)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("result=%v", err)
	}
	completed := 0
	for _, entry := range journal.Current().Journal() {
		if entry.NodeID == "source" && entry.AttemptOutcome == run.AttemptSucceeded {
			completed++
		}
	}
	if calls != 1 || completed != 1 || journal.Current().Status() != run.StatusCancelled {
		t.Fatalf("calls=%d completed=%d status=%s", calls, completed, journal.Current().Status())
	}
}

func TestNestedPauseKeepsClockFrozenUntilLastResume(t *testing.T) {
	now := time.Now()
	scope := &scheduler{executor: &Executor{monotonicNow: func() time.Time { return now }}}
	group := newExecutionGroup(scope)
	group.pause(scope)
	group.pause(scope)
	now = now.Add(5 * time.Second)
	group.resume(scope)
	if !scope.paused || !scope.activeNow().Equal(now.Add(-5*time.Second)) {
		t.Fatal("outer resume released an independently paused task")
	}
	now = now.Add(3 * time.Second)
	group.resume(scope)
	if scope.paused || !scope.activeNow().Equal(now.Add(-8*time.Second)) {
		t.Fatal("nested pause was counted twice or did not resume")
	}
}

type blockingResumeInput struct{ release <-chan struct{} }

func (*blockingResumeInput) InputDomain() string              { return "desktop" }
func (*blockingResumeInput) PauseInput(context.Context) error { return nil }
func (h *blockingResumeInput) ResumeInput(ctx context.Context) error {
	select {
	case <-h.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestInputResumeDoesNotBlockScopeScheduler(t *testing.T) {
	scope := &scheduler{executor: &Executor{monotonicNow: time.Now}}
	group := newExecutionGroup(scope)
	release := make(chan struct{})
	scope.inputOwner.Register(&blockingResumeInput{release: release})
	group.pause(scope)
	if err := group.pauseInputs(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	task := &taskActivation{owner: scope, main: scope, interrupted: true}
	if err := task.beginResume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if task.resuming == nil || !scope.paused {
		t.Fatal("input reacquisition blocked or resumed main prematurely")
	}
	// A different runnable scope can now release the domain that this handoff
	// is waiting for. No scheduler lock or graph traversal runs on the worker.
	close(release)
	select {
	case <-task.resuming.done:
	case <-time.After(time.Second):
		t.Fatal("handoff did not wake")
	}
	if task.resuming.err != nil {
		t.Fatal(task.resuming.err)
	}
	task.resuming.cancel()
	close(group.operations.jobs)
	group.operations.workers.Wait()
}
