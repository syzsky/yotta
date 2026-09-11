package compiler

import (
	"context"
	"testing"
	"time"
)

func TestSignalWaitParksAndReceivesOnlyAfterResume(t *testing.T) {
	root := &scheduler{executor: &Executor{monotonicNow: time.Now, newTimer: func(d time.Duration) (<-chan time.Time, func()) {
		v := time.NewTimer(d)
		return v.C, func() { v.Stop() }
	}}}
	g := newExecutionGroup(root)
	g.operations = newOperationPool()
	op := &pendingOperation{done: make(chan struct{})}
	root.operation = op
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{}, 1)
	result := make(chan error, 1)
	go func() { result <- root.awaitEvent(ctx, op, ready, 100*time.Millisecond) }()
	deadline := time.Now().Add(time.Second)
	for !op.parked.Load() {
		if time.Now().After(deadline) {
			t.Fatal("wait did not park")
		}
		time.Sleep(time.Millisecond)
	}
	if err := g.pauseForDebug(ctx); err != nil {
		t.Fatal(err)
	}
	ready <- struct{}{}
	select {
	case e := <-result:
		t.Fatalf("signal escaped pause: %v", e)
	case <-time.After(150 * time.Millisecond):
	}
	if err := g.resumeForDebug(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("signal did not resume")
	}
}
