package compiler

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/nodeadapter"
	run "github.com/yottaapp/yotta/internal/run"
)

func TestTimerQueueOrdersDeadlinesAndBoundsAdmission(t *testing.T) {
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	s := &scheduler{executor: &Executor{monotonicNow: func() time.Time { return now }}}
	complete := func(context.Context, error) ([]string, error) { return nil, nil }
	for _, duration := range []time.Duration{100, 10, 10} {
		if err := s.addWait(waitingActivation{}, &nodeadapter.WaitRequest{Duration: duration, Complete: complete}); err != nil {
			t.Fatal(err)
		}
	}
	for _, sequence := range []uint64{2, 3, 1} {
		if got := heap.Pop(&s.waits).(*pendingWait).sequence; got != sequence {
			t.Fatalf("timer sequence = %d, want %d", got, sequence)
		}
	}
	s.waitCount = 0
	for range maxPendingWaits {
		if err := s.addWait(waitingActivation{}, &nodeadapter.WaitRequest{Complete: complete}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.addWait(waitingActivation{}, &nodeadapter.WaitRequest{Complete: complete}); err == nil {
		t.Fatal("accepted an unbounded timer queue")
	}
	if s.waitCount != maxPendingWaits || len(s.waits) != maxPendingWaits {
		t.Fatal("rejected timer changed queue ownership")
	}
	for _, request := range []*nodeadapter.WaitRequest{{}, {Duration: -1, Complete: complete}} {
		if err := s.addWait(waitingActivation{}, request); err == nil {
			t.Fatal("accepted an invalid continuation")
		}
	}
}

func TestCancellationFilteringPreservesCleanupFailures(t *testing.T) {
	failure := errors.New("journal write failed")
	combined := fmt.Errorf("close invocation: %w", errors.Join(context.Canceled, failure))
	if err := withoutCancellation(combined); !errors.Is(err, failure) || errors.Is(err, context.Canceled) {
		t.Fatalf("filtered error = %v", err)
	}
	if err := withoutCancellation(fmt.Errorf("cancel: %w", context.Canceled)); err != nil {
		t.Fatalf("expected cancellation was retained: %v", err)
	}
	if err := withoutCancellation(context.DeadlineExceeded); err != context.DeadlineExceeded {
		t.Fatal("cleanup timeout was hidden")
	}
}

func TestTimerBudgetClosesRejectedInvocation(t *testing.T) {
	catalog, contracts, locks := schedulerCatalogForTest(t)
	program := compileSchedulerProgram(t, catalog, contracts, false)
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	owner, journal := admittedSchedulerExecution(t, catalog, program, now)
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	completions := 0
	adapters := schedulerAdapters(locks, map[string]Adapter{
		"source": func(context.Context, Invocation) (AdapterResult, error) {
			return AdapterResult{Wait: &nodeadapter.WaitRequest{Complete: func(_ context.Context, err error) ([]string, error) {
				completions++
				if err == nil {
					t.Fatal("rejected timer reported successful completion")
				}
				return nil, err
			}}}, nil
		},
	})
	executor := NewExecutor(catalog, adapters, ExecutorOptions{Now: func() time.Time { return now }})
	state, err := newRunState(nil, catalog, executor.now)
	if err != nil {
		t.Fatal(err)
	}
	s := newScheduler(executor, &program.state.document.Body.Graphs[0], owner, nil, journal, state)
	s.waitCount = maxPendingWaits
	if err := s.invoke(context.Background(), "source", nil, map[string]bool{}); err == nil {
		t.Fatal("timer admission unexpectedly succeeded")
	}
	facts := journal.Current().Journal()
	if completions != 1 || len(s.waits) != 0 || facts[len(facts)-1].AttemptOutcome != run.AttemptFailed {
		t.Fatalf("completions=%d pending=%d facts=%v", completions, len(s.waits), facts)
	}
}
