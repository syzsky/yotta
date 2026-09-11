package compiler

import (
	"context"
	"errors"
	"time"

	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodecontract"
	run "github.com/yottaapp/yotta/internal/run"
)

type taskActivation struct {
	owner               *scheduler
	node                programNode
	attempt             int
	summary             run.RedactedSummary
	interval            time.Duration
	limit, index        int64
	main, body, handler *scheduler
	timer               *scopeTimer
	resuming            *pendingOperation
	closed              bool
	interrupted         bool
	missed              int64
	lastOverrun         time.Time
}

func (s *scheduler) enterTask(ctx context.Context, node programNode, trigger *nodeadapter.SignalTrigger) error {
	v := node.Instruction.Task
	if v == nil || trigger == nil || s.group == nil {
		return errors.New("task requires a routed signal and execution group")
	}
	if trigger.InputPort != v.EntryInput {
		for owner := s; owner != nil; owner = owner.parent {
			if task := owner.tasks[node.ID]; task != nil && !task.closed {
				switch trigger.InputPort {
				case v.StopInput:
					return task.close(ctx, nil, true)
				case v.InterruptInput:
					return task.interrupt(ctx)
				}
			}
		}
		return errors.New("task signal has no active owning task")
	}
	if s.tasks[node.ID] != nil {
		return errors.New("task entry is already active in this scope")
	}
	inputs, err := s.resolveInputs(ctx, node, nil, map[string]bool{})
	if err != nil {
		return err
	}
	interval, err := decodeInstructionInteger(inputs[v.IntervalInput])
	if err != nil || interval < 1 || interval > 86_400_000 {
		return errors.Join(errors.New("task interval must be between 1 and 86400000 milliseconds"), err)
	}
	count, err := decodeInstructionInteger(inputs[v.CountInput])
	if err != nil || count < 0 {
		return errors.Join(errors.New("task count must be nonnegative"), err)
	}
	if err := s.debugCheckpoint(ctx, node, s.attempts[node.ID]+1, inputs); err != nil {
		return err
	}
	attempt, summary, err := s.beginInstruction(ctx, node)
	if err != nil {
		return err
	}
	task := &taskActivation{owner: s, node: node, attempt: attempt, summary: summary, interval: time.Duration(interval) * time.Millisecond, limit: count}
	if s.tasks == nil {
		s.tasks = map[string]*taskActivation{}
	}
	s.tasks[node.ID] = task
	s.keepAlive++
	if v.MainOutput != "" {
		task.main, err = s.group.spawn(s, s.instructionRoutes(node.ID, v.MainOutput, nil), func(ctx context.Context, cause error) error {
			if task.closed {
				return nil
			}
			return task.close(ctx, cause, cause == nil)
		})
		if err != nil {
			return err
		}
		task.main.scopeRole, task.main.scopeNode = "main", node
	}
	if err := task.status(ctx, nodecontract.TaskStartedStatusID); err != nil {
		return err
	}
	task.arm(s.executor.monotonicNow())
	return nil
}

func (t *taskActivation) arm(at time.Time) {
	t.timer = t.owner.group.schedule(t.owner, at, func(ctx context.Context) error {
		if t.closed {
			return nil
		}
		// Coalesce missed ticks. At most one observation body is in flight;
		// handlers suspend observations until the interrupted task resumes.
		if t.body == nil && t.handler == nil && !t.interrupted {
			if t.limit > 0 && t.index >= t.limit {
				if t.main == nil {
					return t.close(ctx, nil, true)
				}
				return nil
			}
			v := t.node.Instruction.Task
			if err := t.owner.setInstructionIntegerOutput(t.node, v.IndexOutput, t.index, t.attempt); err != nil {
				return err
			}
			t.index++
			body, err := t.owner.group.spawn(t.owner, t.owner.instructionRoutes(t.node.ID, v.BodyOutput, nil), func(ctx context.Context, cause error) error {
				t.body = nil
				if t.closed {
					return nil
				}
				if cause != nil {
					return cause
				}
				if t.main == nil && t.limit > 0 && t.index >= t.limit {
					return t.close(ctx, nil, true)
				}
				return nil
			})
			if err != nil {
				return err
			}
			body.scopeRole, body.scopeNode = "tick", t.node
			t.body = body
		} else if t.body != nil && !t.interrupted && t.handler == nil {
			t.missed++
			now := t.owner.activeNow()
			if t.lastOverrun.IsZero() || now.Sub(t.lastOverrun) >= time.Second {
				if err := t.status(ctx, nodecontract.TaskOverrunStatusID); err != nil {
					return err
				}
				t.lastOverrun = now
			}
		}
		next := at.Add(t.interval)
		now := t.owner.executor.monotonicNow()
		if !next.After(now) {
			next = now.Add(t.interval)
		}
		t.arm(next)
		return nil
	})
}

func (t *taskActivation) interrupt(ctx context.Context) error {
	if t.main == nil || t.node.Instruction.Task.HandlerOutput == "" {
		return errors.New("task has no interruptible main branch")
	}
	if t.handler != nil || t.interrupted {
		return nil
	}
	g := t.owner.group
	g.pause(t.main)
	t.interrupted = true
	if err := t.status(ctx, nodecontract.TaskPausingStatusID); err != nil {
		return err
	}
	if t.body != nil {
		t.body.done = nil
		if err := g.cancelScope(ctx, t.body); err != nil {
			return err
		}
		t.body = nil
	}
	return t.startHandler(ctx)
}

func (t *taskActivation) startHandler(ctx context.Context) error {
	if t.resuming != nil {
		select {
		case <-t.resuming.done:
			op := t.resuming
			t.resuming = nil
			op.cancel()
			if op.err != nil {
				return op.err
			}
			t.owner.group.resume(t.main)
			t.interrupted = false
			return t.status(ctx, nodecontract.TaskResumedStatusID)
		default:
			return nil
		}
	}
	if t.closed || t.owner.paused || !t.interrupted || t.handler != nil || !t.owner.group.quiescent(t.main) {
		return nil
	}
	g := t.owner.group
	if err := g.pauseInputs(ctx, t.main); err != nil {
		return err
	}
	if err := t.status(ctx, nodecontract.TaskPausedStatusID); err != nil {
		return err
	}
	handler, err := g.spawn(t.owner, t.owner.instructionRoutes(t.node.ID, t.node.Instruction.Task.HandlerOutput, nil), func(ctx context.Context, cause error) error {
		t.handler = nil
		if !t.closed && cause == nil {
			return t.beginResume(ctx)
		}
		return cause
	})
	if handler != nil {
		handler.scopeRole, handler.scopeNode = "handler", t.node
	}
	t.handler = handler
	return err
}

func (t *taskActivation) close(ctx context.Context, cause error, completed bool) error {
	if t.closed {
		return nil
	}
	t.closed = true
	if t.timer != nil {
		t.timer.active = false
	}
	var err error
	if t.resuming != nil {
		t.resuming.cancel()
		<-t.resuming.done
		err = withoutCancellation(t.resuming.err)
		t.resuming = nil
	}
	for _, child := range []*scheduler{t.main, t.body, t.handler} {
		if child != nil {
			child.done = nil
			err = errors.Join(err, t.owner.group.cancelScope(ctx, child))
		}
	}
	delete(t.owner.tasks, t.node.ID)
	t.owner.keepAlive--
	if completed && err == nil {
		err = t.owner.finishInstruction(ctx, t.node, t.attempt, t.summary)
		if err == nil {
			t.owner.enqueueInstructionOutput(t.node.ID, t.node.Instruction.Task.CompletedOutput, nil)
		}
	} else {
		err = errors.Join(err, t.owner.closeInterruptedInstruction(ctx, t.node, t.attempt, t.summary, cause))
	}
	return errors.Join(cause, err)
}

func (s *scheduler) closeTasks(ctx context.Context, cause error) error {
	var result error
	for _, listener := range s.listeners {
		result = errors.Join(result, listener.close(ctx, cause, false))
	}
	for _, task := range s.tasks {
		result = errors.Join(result, task.close(ctx, cause, false))
	}
	return withoutCancellation(result)
}

func (t *taskActivation) status(ctx context.Context, code string) error {
	return t.owner.recordInstructionStatus(ctx, t.node, t.attempt, code, map[string]int64{"ticks": t.index, "skipped_ticks": t.missed, "interval_ms": t.interval.Milliseconds()}, nil)
}

func (t *taskActivation) beginResume(ctx context.Context) error {
	g := t.owner.group
	owners := g.inputOwners(t.main)
	needsInput := false
	for _, owner := range owners {
		needsInput = needsInput || owner.NeedsResume()
	}
	resume := func(ctx context.Context) error {
		for _, owner := range owners {
			if err := owner.Resume(ctx); err != nil {
				return err
			}
		}
		return nil
	}
	if !needsInput {
		if err := resume(ctx); err != nil {
			return err
		}
		g.resume(t.main)
		t.interrupted = false
		return t.status(ctx, nodecontract.TaskResumedStatusID)
	}
	if g.operations == nil {
		g.operations = newOperationPool()
	}
	opCtx, cancel := context.WithCancel(ctx)
	op := &pendingOperation{ctx: opCtx, cancel: cancel, done: make(chan struct{}), run: func(ctx context.Context, _ nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
		return nodeadapter.AdapterResult{}, resume(ctx)
	}}
	t.resuming = op
	g.operations.submit(op)
	return nil
}
