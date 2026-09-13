package inputcoord

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCooperatingFamilyRetainsExclusivityUntilLastLease(t *testing.T) {
	var c Coordinator
	parent := NewOwner()
	child := NewCooperatingOwner(parent)
	grandchild := NewCooperatingOwner(child)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if child == parent || grandchild == child {
		t.Fatal("shared lifecycle owner")
	}
	leases := make([]*Lease, 0, 3)
	for _, owner := range []*Owner{parent, child, grandchild} {
		lease, err := c.Acquire(ctx, "desktop", owner)
		if err != nil {
			t.Fatal(err)
		}
		leases = append(leases, lease)
		defer lease.Release()
	}
	leases[0].Release()
	leases[1].Release()
	blocked, stop := context.WithTimeout(ctx, 10*time.Millisecond)
	lease, err := c.Acquire(blocked, "desktop", NewOwner())
	stop()
	if lease != nil {
		lease.Release()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unrelated owner entered live family: %v", err)
	}
	leases[2].Release()
	next, err := c.Acquire(ctx, "desktop", NewOwner())
	if err != nil {
		t.Fatal(err)
	}
	next.Release()
	if len(c.domains) != 0 {
		t.Fatal("family leaked arbitration domain")
	}
	if IsCooperative(ctx) || IsCooperative(WithOwner(ctx, parent)) || IsCooperative(WithOwner(ctx, NewCooperatingOwner(nil))) {
		t.Fatal("independent owner marked cooperative")
	}
	if !IsCooperative(WithOwner(ctx, child)) || !IsCooperative(WithOwner(ctx, grandchild)) {
		t.Fatal("cooperation lost in descendant")
	}
}

func TestCooperatingOwnerFreezeCancelAndHeldLifecycleAreIndependent(t *testing.T) {
	var c Coordinator
	parent := NewOwner()
	child := NewCooperatingOwner(parent)
	parentHeld, childHeld := &testHeld{}, &testHeld{}
	parent.Register(parentHeld)
	child.Register(childHeld)
	defer parent.Unregister(parentHeld)
	defer child.Unregister(childHeld)
	parentWake, childWake := make(chan struct{}, 1), make(chan struct{}, 1)
	parent.SetWake(parentWake)
	child.SetWake(childWake)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	held, err := c.Acquire(ctx, "desktop", parent)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	<-parentWake
	child.Freeze()
	blocked, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		lease, err := c.Acquire(blocked, "desktop", child)
		if lease != nil {
			lease.Release()
		}
		done <- err
	}()
	select {
	case <-childWake:
	case <-ctx.Done():
		t.Fatal("child did not signal its own waiter")
	}
	if !child.Waiting() || parent.Waiting() {
		t.Fatal("waiting state crossed lifecycle boundary")
	}
	select {
	case <-parentWake:
		t.Fatal("child woke parent owner")
	default:
	}
	correction, err := c.Acquire(ctx, "desktop", parent)
	if err != nil {
		t.Fatal(err)
	}
	correction.Release()
	if err := child.Pause(ctx); err != nil {
		t.Fatal(err)
	}
	if parentHeld.pauses != 0 || childHeld.pauses != 1 {
		t.Fatal("child pause affected parent held state")
	}
	stop()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("frozen child cancel: %v", err)
	}
	if child.Waiting() {
		t.Fatal("canceled child leaked waiting count")
	}
	child.Thaw()
	if err := child.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	if parentHeld.resumes != 0 || childHeld.resumes != 1 {
		t.Fatal("child resume affected parent held state")
	}
	// Freezing the parent also leaves a cooperating child's acquisition independent.
	parent.Freeze()
	resumed, err := c.Acquire(ctx, "desktop", child)
	if err != nil {
		t.Fatal(err)
	}
	resumed.Release()
	parent.Thaw()
}

func TestCooperatingFamilyAvoidsCrossDomainCycles(t *testing.T) {
	var c Coordinator
	parent, other := NewOwner(), NewOwner()
	child := NewCooperatingOwner(parent)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	first, err := c.Acquire(ctx, "a", parent)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	second, err := c.Acquire(ctx, "b", other)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	lease, err := c.Acquire(ctx, "b", child)
	if lease != nil {
		lease.Release()
	}
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("family cycle was not rejected immediately: %v", err)
	}
}

func TestCooperatingQueuedFamilyHandoffDoesNotStrandChild(t *testing.T) {
	var c Coordinator
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	blocker, err := c.Acquire(ctx, "desktop", NewOwner())
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Release()
	parent := NewOwner()
	child := NewCooperatingOwner(parent)
	third := NewOwner()
	type acquired struct {
		lease *Lease
		err   error
	}
	start := func(owner *Owner, queued int) chan acquired {
		done := make(chan acquired, 1)
		go func() { lease, err := c.Acquire(ctx, "desktop", owner); done <- acquired{lease, err} }()
		for {
			c.mu.Lock()
			n := len(c.domains["desktop"].queue)
			c.mu.Unlock()
			if n == queued {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("waiter did not queue")
			case <-time.After(time.Millisecond):
			}
		}
		return done
	}
	parentDone := start(parent, 1)
	thirdDone := start(third, 2)
	childDone := start(child, 3)
	blocker.Release()
	parentResult := <-parentDone
	if parentResult.err != nil {
		t.Fatal(parentResult.err)
	}
	defer parentResult.lease.Release()
	childResult := <-childDone
	if childResult.err != nil {
		t.Fatalf("queued child deadlocked behind live parent: %v", childResult.err)
	}
	defer childResult.lease.Release()
	select {
	case result := <-thirdDone:
		if result.lease != nil {
			result.lease.Release()
		}
		t.Fatal("unrelated family overtook active parent")
	default:
	}
	parentResult.lease.Release()
	childResult.lease.Release()
	thirdResult := <-thirdDone
	if thirdResult.err != nil {
		t.Fatal(thirdResult.err)
	}
	thirdResult.lease.Release()
	if len(c.domains) != 0 {
		t.Fatal("queued family leaked domain")
	}
}
