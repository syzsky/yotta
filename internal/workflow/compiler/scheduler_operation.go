package compiler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yottaapp/yotta/internal/automation/inputcoord"
	"github.com/yottaapp/yotta/internal/nodeadapter"
)

// Blocking operations may be parked at an explicit pause boundary. Give every
// admitted scope a worker slot so parked parents cannot starve their handlers.
// Workers are started on demand and reused for the rest of the Run.
const operationWorkers = maxExecutionScopes

var errInputsPending = errors.New("input producer is pending")

type pendingOperation struct {
	activation waitingActivation
	ctx        context.Context
	cancel     context.CancelFunc
	run        nodeadapter.Adapter
	invocation nodeadapter.Invocation
	done       chan struct{}
	outcome    nodeadapter.AdapterResult
	err        error
	parked     atomic.Bool
}

type operationPool struct {
	jobs    chan *pendingOperation
	wake    chan struct{}
	workers sync.WaitGroup
	count   int
	active  atomic.Int64
}

func newOperationPool() *operationPool {
	p := &operationPool{jobs: make(chan *pendingOperation, maxExecutionScopes), wake: make(chan struct{}, 1)}
	return p
}

func (p *operationPool) submit(op *pendingOperation) {
	active := p.active.Add(1)
	if int64(p.count) < active && p.count < operationWorkers {
		p.count++
		p.workers.Add(1)
		go func() {
			defer p.workers.Done()
			for op := range p.jobs {
				op.outcome, op.err = invokeOperation(op)
				p.active.Add(-1)
				close(op.done)
				select {
				case p.wake <- struct{}{}:
				default:
				}
			}
		}()
	}
	p.jobs <- op
}

func invokeOperation(op *pendingOperation) (result nodeadapter.AdapterResult, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("adapter operation panicked: %v", failure)
		}
	}()
	return op.run(op.ctx, op.invocation)
}

func (s *scheduler) startOperation(ctx context.Context, activation waitingActivation, adapter nodeadapter.InstalledAdapter, invocation nodeadapter.Invocation) error {
	if s.operation != nil {
		return errors.New("scope already has an active operation")
	}
	if s.group.operations == nil {
		s.group.operations = newOperationPool()
	}
	operationCtx, cancel := context.WithCancel(inputcoord.WithOwner(ctx, s.inputOwner))
	op := &pendingOperation{activation: activation, ctx: operationCtx, cancel: cancel, run: adapter.Run, invocation: invocation, done: make(chan struct{})}
	if !adapter.PauseAtWait {
		op.invocation.MonotonicNow = s.executor.monotonicNow
	}
	op.invocation.WaitWithPause = func(ctx context.Context, d time.Duration, cleanup func(context.Context) error) error {
		return s.waitActive(ctx, op, d, cleanup)
	}
	if adapter.PauseAtWait {
		op.invocation.Wait = func(ctx context.Context, d time.Duration) error { return s.waitActive(ctx, op, d, nil) }
	}
	op.invocation.Await = func(ctx context.Context, ready <-chan struct{}, duration time.Duration) error {
		return s.awaitEvent(ctx, op, ready, duration)
	}
	s.operation = op
	// At most one operation belongs to each of the bounded live scopes.
	s.group.operations.submit(op)
	return nil
}

func (s *scheduler) finishOperation(ctx context.Context) error {
	op := s.operation
	s.operation = nil
	defer op.cancel()
	return s.completeInvocation(ctx, op.activation, op.outcome, op.err)
}

func (s *scheduler) closeOperation(ctx context.Context) error {
	if s.operation == nil {
		return nil
	}
	op := s.operation
	op.cancel()
	<-op.done
	// An atomic effect which finished as cancellation arrived keeps its fact;
	// its successor remains in this retiring scope and is never dispatched.
	return withoutCancellation(s.finishOperation(ctx))
}

func (g *executionGroup) waitForOperation(ctx context.Context, deadline time.Time) error {
	var operationWake <-chan struct{}
	if g.operations != nil {
		operationWake = g.operations.wake
	}
	if deadline.IsZero() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-operationWake:
			return nil
		case <-g.wake:
			return nil
		}
	}
	timer, stop := g.root.executor.timer(max(time.Duration(0), deadline.Sub(g.root.executor.monotonicNow())))
	defer stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-operationWake:
		return nil
	case <-g.wake:
		return nil
	case <-timer:
		return nil
	}
}

func (s *scheduler) waitActive(ctx context.Context, op *pendingOperation, duration time.Duration, beforePause func(context.Context) error) error {
	deadline := s.activeNow().Add(duration)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.clock.mu.RLock()
		paused, changed := !s.clock.pausedAt.IsZero(), s.clock.changed
		s.clock.mu.RUnlock()
		if paused {
			if beforePause != nil {
				if err := beforePause(context.WithoutCancel(ctx)); err != nil {
					return err
				}
			}
			op.parked.Store(true)
			select {
			case s.group.operations.wake <- struct{}{}:
			default:
			}
			select {
			case <-ctx.Done():
				op.parked.Store(false)
				return ctx.Err()
			case <-changed:
				op.parked.Store(false)
				continue
			}
		}
		remaining := deadline.Sub(s.activeNow())
		if remaining <= 0 {
			return nil
		}
		timer, stop := s.executor.timer(remaining)
		select {
		case <-ctx.Done():
			stop()
			return ctx.Err()
		case <-changed:
			stop()
		case <-timer:
		}
		stop()
	}
}

func (g *executionGroup) quiescent(s *scheduler) bool {
	for _, task := range s.tasks {
		if task.resuming != nil {
			return false
		}
	}
	if s.operation != nil && !s.operation.parked.Load() && !s.inputOwner.Waiting() {
		return false
	}
	for _, child := range g.scopes {
		if child.parent == s && !g.quiescent(child) {
			return false
		}
	}
	return true
}

// Awaiting an external event is a safe pause boundary. The ready event stays
// pending until the scope resumes; the timeout consumes only active time.
func (s *scheduler) awaitEvent(ctx context.Context, op *pendingOperation, ready <-chan struct{}, duration time.Duration) error {
	deadline := s.activeNow().Add(duration)
	for {
		if err := s.waitActive(ctx, op, 0, nil); err != nil {
			return err
		}
		s.clock.mu.RLock()
		changed := s.clock.changed
		s.clock.mu.RUnlock()
		var timer <-chan time.Time
		stop := func() {}
		if duration > 0 {
			remaining := deadline.Sub(s.activeNow())
			if remaining <= 0 {
				return context.DeadlineExceeded
			}
			timer, stop = s.executor.timer(remaining)
		}
		op.parked.Store(true)
		select {
		case s.group.operations.wake <- struct{}{}:
		default:
		}
		var err error
		received := false
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-ready:
			received = true
		case <-changed:
		case <-timer:
		}
		stop()
		op.parked.Store(false)
		if err != nil {
			return err
		}
		if received {
			return s.waitActive(ctx, op, 0, nil)
		}
	}
}
