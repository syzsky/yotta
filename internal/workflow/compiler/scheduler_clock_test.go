package compiler

import (
	"context"
	"sync"
	"testing"
	"time"
)

type testRuntimeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers map[*testRuntimeTimer]struct{}
}
type testRuntimeTimer struct {
	due     time.Time
	channel chan time.Time
}

func (c *testRuntimeClock) read() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *testRuntimeClock) timer(d time.Duration) (<-chan time.Time, func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timers == nil {
		c.timers = map[*testRuntimeTimer]struct{}{}
	}
	timer := &testRuntimeTimer{due: c.now.Add(d), channel: make(chan time.Time, 1)}
	if d <= 0 {
		timer.channel <- c.now
	} else {
		c.timers[timer] = struct{}{}
	}
	return timer.channel, func() { c.mu.Lock(); delete(c.timers, timer); c.mu.Unlock() }
}
func (c *testRuntimeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	for timer := range c.timers {
		if !timer.due.After(c.now) {
			timer.channel <- c.now
			delete(c.timers, timer)
		}
	}
}
func (c *testRuntimeClock) pending() int { c.mu.Lock(); defer c.mu.Unlock(); return len(c.timers) }
func awaitClockCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("clock operation did not reach its checkpoint")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestAsyncWaitRetainsThreeSecondsAfterEightSecondPause(t *testing.T) {
	clock := &testRuntimeClock{now: time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)}
	scope := &scheduler{executor: &Executor{monotonicNow: clock.read, newTimer: clock.timer}}
	group := newExecutionGroup(scope)
	group.operations = newOperationPool()
	op := &pendingOperation{}
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { done <- scope.waitActive(ctx, op, 5*time.Second, nil) }()
	awaitClockCondition(t, func() bool { return clock.pending() == 1 })
	clock.advance(2 * time.Second)
	group.pause(scope)
	awaitClockCondition(t, func() bool { return op.parked.Load() })
	clock.advance(8 * time.Second)
	group.resume(scope)
	awaitClockCondition(t, func() bool { return clock.pending() == 1 })
	clock.advance(2 * time.Second)
	select {
	case err := <-done:
		t.Fatalf("lost remaining time: %v", err)
	default:
	}
	clock.advance(time.Second)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("remaining three seconds did not finish")
	}
	if clock.pending() != 0 {
		t.Fatal("completed wait retained timer")
	}
}
