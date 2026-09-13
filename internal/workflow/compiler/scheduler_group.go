package compiler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/yottaapp/yotta/internal/automation/inputcoord"
	"github.com/yottaapp/yotta/internal/datatype"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/signals"
)

const maxExecutionScopes = 128

// A scope owns its control queue, wait queue and values. The group is the only
// writer of graph state; siblings exchange data through declared Run state.
type executionGroup struct {
	listenerCount  int
	signals        signals.Hub
	root           *scheduler
	nextScopeID    uint64
	lastDebugTasks time.Time
	scopes         []*scheduler
	timers         []*scopeTimer
	cursor         int
	result         ExecutionResult
	resultBytes    int
	operations     *operationPool
	wake           chan struct{}
}

type scopeTimer struct {
	owner  *scheduler
	at     time.Time
	active bool
	fire   func(context.Context) error
}

type scopeClock struct {
	mu       sync.RWMutex
	now      func() time.Time
	pausedAt time.Time
	offset   time.Duration
	changed  chan struct{}
}

func (c *scopeClock) read() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := c.now()
	if !c.pausedAt.IsZero() {
		now = c.pausedAt
	}
	return now.Add(-c.offset)
}

func (s *scheduler) activeNow() time.Time {
	if s.clock == nil {
		return s.executor.monotonicNow()
	}
	return s.clock.read()
}

func newExecutionGroup(root *scheduler) *executionGroup {
	g := &executionGroup{root: root, scopes: []*scheduler{root}, result: emptyExecutionResult(), wake: make(chan struct{}, 1)}
	g.nextScopeID = 1
	root.scopeID, root.scopeRole = 1, "root"
	root.group = g
	root.inputOwner = inputcoord.NewOwner()
	root.inputOwner.SetWake(g.wake)
	root.clock = &scopeClock{now: root.executor.monotonicNow, changed: make(chan struct{})}
	if root.owner != nil {
		root.jobs = newScopeJobs(root.owner.Context(), g.wake)
	}
	return g
}

func emptyExecutionResult() ExecutionResult {
	return ExecutionResult{NodeOutputs: map[string]map[string]datatype.ValueEnvelope{}, attempts: map[string]map[string]int{}}
}

func (g *executionGroup) spawn(parent *scheduler, queue []scheduledInvocation, done func(context.Context, error) error) (*scheduler, error) {
	if len(g.scopes) >= maxExecutionScopes {
		return nil, errors.New("execution scope budget exceeded")
	}
	s := &scheduler{
		group: g, parent: parent, clock: &scopeClock{now: parent.executor.monotonicNow, changed: make(chan struct{})}, done: done,
		runID: parent.runID, workflowID: parent.workflowID, runStartedAt: parent.runStartedAt,
		executor: parent.executor, graph: parent.graph, owner: parent.owner, targets: parent.targets, journal: parent.journal, state: parent.state,
		nodes: parent.nodes, routes: parent.routes, dataConsumers: parent.dataConsumers, volatile: parent.volatile,
		attempts: parent.attempts, evaluating: map[string]bool{}, queue: queue, result: emptyExecutionResult(),
		outputSessions: map[string]map[string]*run.Session{}, control: parent.control,
		transientOutputs: parent.transientOutputs,
	}
	g.nextScopeID++
	s.scopeID = g.nextScopeID
	s.inputOwner = inputcoord.NewOwner()
	if parent.cooperativeInput {
		s.inputOwner = inputcoord.NewCooperatingOwner(parent.inputOwner)
		s.cooperativeInput = true
	}
	s.inputOwner.SetWake(g.wake)
	s.jobs = newScopeJobs(parent.owner.Context(), g.wake)
	for id, outputs := range parent.result.NodeOutputs {
		s.result.NodeOutputs[id] = map[string]datatype.ValueEnvelope{}
		s.result.attempts[id] = map[string]int{}
		for port, value := range outputs {
			s.result.NodeOutputs[id][port] = value
			s.result.attempts[id][port] = parent.result.attempts[id][port]
			s.retainedBytes += len(value.RuntimeArtifact())
		}
	}
	for id, sessions := range parent.outputSessions {
		s.outputSessions[id] = map[string]*run.Session{}
		for port, session := range sessions {
			s.outputSessions[id][port] = session
		}
	}
	if !parent.canRetain(s.retainedBytes) {
		_ = s.jobs.close()
		return nil, errors.New("run retained value budget exceeded")
	}
	g.scopes = append(g.scopes, s)
	return s, nil
}

func (g *executionGroup) schedule(owner *scheduler, at time.Time, fire func(context.Context) error) *scopeTimer {
	t := &scopeTimer{owner: owner, at: at, active: true, fire: fire}
	g.timers = append(g.timers, t)
	return t
}

func (g *executionGroup) merge(s *scheduler) error {
	for id, outputs := range s.result.NodeOutputs {
		if s.transientOutputs[id] {
			continue
		}
		for port, value := range outputs {
			if _, _, runtimeValue := runtimeHandle(value); runtimeValue {
				continue
			}
			attempt := s.result.attempts[id][port]
			if attempt < g.result.attempts[id][port] {
				continue
			}
			if g.result.NodeOutputs[id] == nil {
				g.result.NodeOutputs[id] = map[string]datatype.ValueEnvelope{}
				g.result.attempts[id] = map[string]int{}
			}
			previous := g.result.NodeOutputs[id][port]
			nextBytes := g.resultBytes + len(value.RuntimeArtifact()) - len(previous.RuntimeArtifact())
			if nextBytes > MaxRunRetainedValueBytes {
				return errors.New("run result value budget exceeded")
			}
			g.resultBytes = nextBytes
			g.result.NodeOutputs[id][port] = value
			g.result.attempts[id][port] = attempt
		}
	}
	return nil
}

func (s *scheduler) canRetain(delta int) bool {
	if s.group == nil {
		return s.retainedBytes+delta <= MaxRunRetainedValueBytes
	}
	bytes := s.group.resultBytes + delta
	for _, scope := range s.group.scopes {
		bytes += scope.retainedBytes
		if scope.operation != nil {
			bytes += scope.operation.branchBytes
		}
	}
	return bytes <= MaxRunRetainedValueBytes
}

func (s *scheduler) hasWork() bool {
	return len(s.queue) > 0 || len(s.frames) > 0 || len(s.waits) > 0 || s.keepAlive > 0 || s.operation != nil || s.jobs.active()
}

func (g *executionGroup) retire(ctx context.Context, s *scheduler, cause error) error {
	cause = preserveExecutionFailure(cause, s.journal)
	if s.stopped {
		return nil
	}
	s.stopped = true
	for _, timer := range g.timers {
		if timer.owner == s {
			timer.active = false
		}
	}
	err := errors.Join(s.closeOperation(ctx), s.jobs.close(), s.closeTasks(ctx, cause), s.closeWaits(ctx), s.closeRegions(ctx, cause))
	// Run-owned producers may still be using root leases after its graph drains.
	// The root releases them only after Owner.Wait, just as a single scope does.
	if s != g.root {
		err = errors.Join(err, s.cleanup())
	}
	if cause == nil && err == nil {
		err = g.merge(s)
	}
	for i, scope := range g.scopes {
		if scope == s {
			g.scopes = append(g.scopes[:i], g.scopes[i+1:]...)
			break
		}
	}
	if s.done != nil {
		err = errors.Join(err, s.done(ctx, errors.Join(cause, err)))
	} else if b := s.branchRoot; b != nil && b.lane.active == b {
		// Group shutdown suppresses routing callbacks, but must still settle
		// handles before the owning worker is cancelled and joined.
		b.owner.finishBranch(b, errors.Join(cause, err))
	}
	return err
}

func (g *executionGroup) cancelScope(ctx context.Context, s *scheduler) error {
	if s == nil || s.stopped {
		return nil
	}
	var result error
	for _, child := range append([]*scheduler(nil), g.scopes...) {
		if child.parent == s {
			result = errors.Join(result, g.cancelScope(ctx, child))
		}
	}
	return errors.Join(result, g.retire(ctx, s, context.Canceled))
}

func (g *executionGroup) pause(s *scheduler) {
	if s == nil || s.stopped {
		return
	}
	s.pauseCount++
	s.inputOwner.Freeze()
	if s.pauseCount == 1 {
		s.paused = true
		s.clock.mu.Lock()
		s.clock.pausedAt = s.executor.monotonicNow()
		close(s.clock.changed)
		s.clock.changed = make(chan struct{})
		s.clock.mu.Unlock()
	}
	for _, child := range g.scopes {
		if child.parent == s {
			g.pause(child)
		}
	}
}

func (g *executionGroup) resume(s *scheduler) {
	if s == nil || s.stopped || !s.paused {
		return
	}
	s.pauseCount--
	defer s.inputOwner.Thaw()
	if s.pauseCount == 0 {
		s.clock.mu.Lock()
		duration := s.executor.monotonicNow().Sub(s.clock.pausedAt)
		s.clock.offset += duration
		s.clock.pausedAt = time.Time{}
		close(s.clock.changed)
		s.clock.changed = make(chan struct{})
		s.clock.mu.Unlock()
		for _, wait := range s.waits {
			wait.due = wait.due.Add(duration)
		}
		for _, frame := range s.frames {
			for _, wait := range frame.parentWaits {
				wait.due = wait.due.Add(duration)
			}
		}
		for _, timer := range g.timers {
			if timer.owner == s && timer.active {
				timer.at = timer.at.Add(duration)
			}
		}
		s.paused = false
	}
	for _, child := range g.scopes {
		if child.parent == s {
			g.resume(child)
		}
	}
}

func (g *executionGroup) run(ctx context.Context) (_ ExecutionResult, resultErr error) {
	defer func() {
		defer g.signals.Close()
		resultErr = preserveExecutionFailure(resultErr, g.root.journal)
		for len(g.scopes) > 0 {
			s := g.scopes[len(g.scopes)-1]
			s.done = nil
			resultErr = errors.Join(resultErr, g.retire(ctx, s, resultErr))
		}
		resultErr = errors.Join(resultErr, g.root.cleanup())
		if g.operations != nil {
			close(g.operations.jobs)
			g.operations.workers.Wait()
		}
	}()
	timerWasLast := false
	for len(g.scopes) > 0 {
		g.publishTasks()
		// A last atomic invocation may finish at the same instant its caller
		// cancels. Seal already-drained scopes before testing for more work.
		for _, s := range append([]*scheduler(nil), g.scopes...) {
			if !s.stopped && !s.paused && !s.hasWork() {
				if err := g.retire(ctx, s, nil); err != nil {
					return ExecutionResult{}, err
				}
			}
		}
		if len(g.scopes) == 0 {
			break
		}
		if err := ctx.Err(); err != nil {
			return ExecutionResult{}, err
		}
		pendingOperations := false
		for _, s := range append([]*scheduler(nil), g.scopes...) {
			if s.stopped {
				continue
			}
			if err := s.jobs.failure(); err != nil {
				if err := g.scopeFailure(ctx, s, err); err != nil {
					return ExecutionResult{}, err
				}
				continue
			}
			pendingOperations = pendingOperations || s.jobs.active()
			if s.operation == nil {
				continue
			}
			select {
			case <-s.operation.done:
				if err := s.finishOperation(ctx); err != nil {
					if err := g.scopeFailure(ctx, s, err); err != nil {
						return ExecutionResult{}, err
					}
				}
			default:
				pendingOperations = true
				s.advanceBranches(ctx, s.operation)
			}
		}
		for _, s := range append([]*scheduler(nil), g.scopes...) {
			if s.stopped {
				continue
			}
			pendingOperations = pendingOperations || len(s.listeners) > 0
			for _, listener := range s.listeners {
				if err := listener.advance(ctx); err != nil {
					if err := g.scopeFailure(ctx, s, err); err != nil {
						return ExecutionResult{}, err
					}
					break
				}
			}
			if s.stopped {
				continue
			}
			for _, task := range s.tasks {
				if err := task.startHandler(ctx); err != nil {
					if err := g.scopeFailure(ctx, s, err); err != nil {
						return ExecutionResult{}, err
					}
					break
				}
				pendingOperations = pendingOperations || task.resuming != nil
			}
		}
		now := g.root.executor.monotonicNow()
		var nextScope *scheduler
		var nextTimer *scopeTimer
		var deadline time.Time
		// Remove consumed registrations: a permanent subscription must not grow
		// the timer list once per tick.
		live := g.timers[:0]
		for _, timer := range g.timers {
			if timer.active && !timer.owner.stopped {
				live = append(live, timer)
			}
		}
		g.timers = live
		for _, timer := range g.timers {
			if timer.owner.paused {
				continue
			}
			if deadline.IsZero() || timer.at.Before(deadline) {
				deadline = timer.at
				nextTimer = timer
			}
		}
		// Give ready scopes a turn between timer deliveries, even when a very
		// short period is already overdue again by the next scheduler pass.
		if nextTimer != nil && !deadline.After(now) && !timerWasLast {
			nextTimer.active = false
			if err := nextTimer.fire(ctx); err != nil {
				if err := g.scopeFailure(ctx, nextTimer.owner, err); err != nil {
					return ExecutionResult{}, err
				}
			}
			timerWasLast = true
			continue
		}
		progress := false
		for n := 0; n < len(g.scopes); n++ {
			g.cursor %= len(g.scopes)
			s := g.scopes[g.cursor]
			g.cursor++
			if s.paused {
				continue
			}
			if s.operation != nil {
				continue
			}
			if !s.hasWork() {
				if err := g.retire(ctx, s, nil); err != nil {
					return ExecutionResult{}, err
				}
				progress = true
				break
			}
			if len(s.waits) > 0 && (deadline.IsZero() || s.waits[0].due.Before(deadline)) {
				deadline = s.waits[0].due
				nextScope = s
				nextTimer = nil
			}
			if len(s.queue) > 0 || len(s.frames) > 0 && len(s.waits) == 0 && s.keepAlive == 0 || len(s.waits) > 0 && !s.waits[0].due.After(now) {
				if err := s.step(ctx); err != nil {
					if err := g.scopeFailure(ctx, s, err); err != nil {
						return ExecutionResult{}, err
					}
				}
				progress = true
				break
			}
		}
		timerWasLast = false
		if progress {
			continue
		}
		if pendingOperations {
			if err := g.waitForOperation(ctx, deadline); err != nil {
				return ExecutionResult{}, err
			}
			continue
		}
		if nextScope != nil {
			if err := nextScope.resumeWait(ctx); err != nil {
				if err := g.scopeFailure(ctx, nextScope, err); err != nil {
					return ExecutionResult{}, err
				}
			}
		} else if nextTimer != nil {
			if err := g.root.executor.wait(ctx, max(time.Duration(0), deadline.Sub(now))); err != nil {
				return ExecutionResult{}, err
			}
			nextTimer.active = false
			if err := nextTimer.fire(ctx); err != nil {
				if err := g.scopeFailure(ctx, nextTimer.owner, err); err != nil {
					return ExecutionResult{}, err
				}
			}
		} else {
			return ExecutionResult{}, errors.New("execution scopes have no runnable task or wakeup")
		}
	}
	if err := g.root.owner.Wait(ctx); err != nil {
		return ExecutionResult{}, err
	}
	return g.result, nil
}

func (g *executionGroup) pauseInputs(ctx context.Context, s *scheduler) error {
	if s == nil || s.stopped {
		return nil
	}
	if err := s.inputOwner.Pause(ctx); err != nil {
		return err
	}
	for _, child := range g.scopes {
		if child.parent == s {
			if err := g.pauseInputs(ctx, child); err != nil {
				return err
			}
		}
	}
	return nil
}

// Freeze owners before dispatching a blocking handoff. Workers never traverse
// the scheduler's mutable scope tree.
func (g *executionGroup) inputOwners(s *scheduler) []*inputcoord.Owner {
	if s == nil || s.stopped {
		return nil
	}
	owners := []*inputcoord.Owner{s.inputOwner}
	for _, child := range g.scopes {
		if child.parent == s {
			owners = append(owners, g.inputOwners(child)...)
		}
	}
	return owners
}
