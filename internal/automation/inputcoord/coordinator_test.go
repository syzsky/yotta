package inputcoord

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHeldOwnerBlocksConflictsAndAllowsItsOwnCorrections(t *testing.T) {
	var c Coordinator
	owner, other := NewOwner(), NewOwner()
	held, err := c.Acquire(context.Background(), "sendinput", owner)
	if err != nil {
		t.Fatal(err)
	}
	correction, err := c.Acquire(context.Background(), "sendinput", owner)
	if err != nil {
		t.Fatal(err)
	}
	correction.Release()
	ctx, cancel := context.WithCancel(context.Background())
	blocked := make(chan error, 1)
	go func() {
		lease, err := c.Acquire(ctx, "sendinput", other)
		if lease != nil {
			lease.Release()
		}
		blocked <- err
	}()
	cancel()
	if err := <-blocked; !errors.Is(err, context.Canceled) {
		t.Fatalf("wait=%v", err)
	}
	held.Release()
	next, err := c.Acquire(context.Background(), "sendinput", other)
	if err != nil {
		t.Fatal(err)
	}
	next.Release()
	next.Release()
	if len(c.domains) != 0 {
		t.Fatal("idle domain retained")
	}
}

func TestInputDomainHandoffIsFIFOAndAvoidsCycles(t *testing.T) {
	var c Coordinator
	a, b, d := NewOwner(), NewOwner(), NewOwner()
	held, _ := c.Acquire(context.Background(), "window-a", a)
	second, _ := c.Acquire(context.Background(), "window-b", b)
	if lease, err := c.Acquire(context.Background(), "window-b", a); err == nil {
		lease.Release()
		t.Fatal("allowed cross-domain wait cycle")
	}
	second.Release()
	acquired := make(chan *Lease, 1)
	go func() { l, _ := c.Acquire(context.Background(), "window-a", b); acquired <- l }()
	deadline := time.Now().Add(time.Second)
	for {
		c.mu.Lock()
		waiting := len(c.domains["window-a"].queue)
		c.mu.Unlock()
		if waiting == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("waiter did not register")
		}
		time.Sleep(time.Millisecond)
	}
	third := make(chan *Lease, 1)
	go func() { l, _ := c.Acquire(context.Background(), "window-a", d); third <- l }()
	held.Release()
	next := <-acquired
	select {
	case l := <-third:
		l.Release()
		t.Fatal("second waiter overtook holder")
	default:
	}
	next.Release()
	(<-third).Release()
}

func BenchmarkInputCoordination(b *testing.B) {
	var c Coordinator
	owner := NewOwner()
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		lease, err := c.Acquire(ctx, "sendinput", owner)
		if err != nil {
			b.Fatal(err)
		}
		lease.Release()
	}
}

type testHeld struct {
	pauses, resumes int
	failure         error
}

func (*testHeld) InputDomain() string                 { return "desktop" }
func (h *testHeld) PauseInput(context.Context) error  { h.pauses++; return h.failure }
func (h *testHeld) ResumeInput(context.Context) error { h.resumes++; return h.failure }

func TestNestedInputPauseRequiresReleaseAcknowledgement(t *testing.T) {
	owner := NewOwner()
	held := &testHeld{failure: errors.New("release failed")}
	owner.Register(held)
	if err := owner.Pause(context.Background()); err == nil || owner.paused != 0 {
		t.Fatal("failed release acknowledged pause")
	}
	held.failure = nil
	if err := owner.Pause(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := owner.Pause(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := owner.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if held.resumes != 0 {
		t.Fatal("nested pause resumed physical input early")
	}
	if err := owner.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if held.resumes != 1 || owner.NeedsResume() {
		t.Fatal("input did not resume exactly once")
	}
	owner.Unregister(held)
}

func TestFrozenAcquisitionDoesNotDispatchAndCanCancel(t *testing.T) {
	var c Coordinator
	owner := NewOwner()
	owner.Freeze()
	owner.Freeze()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		lease, err := c.Acquire(ctx, "desktop", owner)
		if lease != nil {
			lease.Release()
		}
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for !owner.Waiting() {
		if time.Now().After(deadline) {
			t.Fatal("acquisition did not park")
		}
		time.Sleep(time.Millisecond)
	}
	owner.Thaw()
	select {
	case err := <-done:
		t.Fatalf("nested freeze dispatched: %v", err)
	default:
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if owner.Waiting() {
		t.Fatal("cancelled acquisition retained waiter")
	}
	owner.Thaw()
	lease, err := c.Acquire(context.Background(), "desktop", owner)
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
	if len(c.domains) != 0 {
		t.Fatal("cancelled gate retained domain")
	}
}
