package compiler

import (
	"context"
	"errors"
	"fmt"

	"github.com/yottaapp/yotta/internal/automation/inputcoord"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodeadapter"
)

// Counts active and queued branches per operation. The existing scope and
// retained-value budgets also apply; a mailbox holds at most one request.
const maxPendingBranches = 128

type branchCall struct {
	ctx     context.Context
	request nodeadapter.BranchRequest
	ready   chan struct{}
	handle  nodeadapter.BranchHandle
	err     error
}

type branchHandle struct {
	done chan struct{}
	err  error
}

func (h *branchHandle) Done() <-chan struct{} { return h.done }
func (h *branchHandle) Err() error            { return h.err }
func (h *branchHandle) finish(err error) {
	h.err = err
	close(h.done)
}

type branchLane struct {
	coalesce bool
	active   *branchExecution
	queue    []*branchExecution
}

type branchExecution struct {
	cooperative bool
	ctx         context.Context
	stopWake    func() bool
	cancelCause error
	owner       *scheduler
	op          *pendingOperation
	lane        *branchLane
	output      string
	values      map[string]datatype.ValueEnvelope
	bytes       int
	handle      *branchHandle
	scope       *scheduler
	failure     error
}

func (s *scheduler) requestBranch(ctx context.Context, op *pendingOperation, request nodeadapter.BranchRequest) (nodeadapter.BranchHandle, error) {
	waitCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(op.ctx, cancel)
	defer func() { stop(); cancel() }()
	// Envelopes are immutable; copy the caller's map before crossing threads.
	outputs := make(map[string]datatype.ValueEnvelope, len(request.Outputs))
	for id, value := range request.Outputs {
		outputs[id] = value
	}
	request.Outputs = outputs
	call := &branchCall{ctx: ctx, request: request, ready: make(chan struct{})}
	if err := s.waitActive(waitCtx, op, 0, nil); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-op.ctx.Done():
		return nil, op.ctx.Err()
	case op.branchRequests <- call:
	}
	select {
	case s.group.wake <- struct{}{}:
	default:
	}
	if err := s.awaitEvent(waitCtx, op, call.ready, 0); err != nil {
		return nil, err
	}
	return call.handle, call.err
}

// Only the scheduler reads/writes lanes, snapshots and scope trees.
func (s *scheduler) advanceBranches(ctx context.Context, op *pendingOperation) {
	if op.branchRequests == nil || op.closingBranches {
		return
	}
	for _, spec := range op.activation.node.Instruction.Invoke.Branches {
		lane := op.branches[spec.Output]
		pending := lane.queue[:0]
		for _, b := range lane.queue {
			if cause := b.ctx.Err(); cause != nil {
				b.stopWake()
				b.handle.finish(cause)
				op.branchBytes -= b.bytes
				op.branchCount--
			} else {
				pending = append(pending, b)
			}
		}
		clear(lane.queue[len(pending):])
		lane.queue = pending
		if b := lane.active; b != nil && b.ctx.Err() != nil {
			b.cancelCause = b.ctx.Err()
			if err := s.group.cancelScope(ctx, b.scope); err != nil {
				op.branchFailure = errors.Join(op.branchFailure, err)
			}
		}
	}
	if s.paused {
		return
	}
	select {
	case call := <-op.branchRequests:
		call.handle, call.err = s.admitBranch(op, call)
		close(call.ready)
	default:
	}
	// Contract order makes dispatch independent of map iteration order.
	for _, spec := range op.activation.node.Instruction.Invoke.Branches {
		lane := op.branches[spec.Output]
		if lane.active != nil || len(lane.queue) == 0 {
			continue
		}
		branch := lane.queue[0]
		lane.queue[0] = nil
		lane.queue = lane.queue[1:]
		op.branchBytes -= branch.bytes
		lane.active = branch
		if err := s.startBranch(ctx, branch); err != nil {
			s.finishBranch(branch, err)
		}
	}
}

func (s *scheduler) admitBranch(op *pendingOperation, call *branchCall) (nodeadapter.BranchHandle, error) {
	if err := call.ctx.Err(); err != nil {
		return nil, err
	}
	lane := op.branches[call.request.Output]
	if lane == nil {
		return nil, fmt.Errorf("undeclared invocation branch %q", call.request.Output)
	}
	if op.branchFailure != nil {
		return nil, op.branchFailure
	}
	values, leases, err := s.executor.validateOutputs(op.activation.node, call.request.Outputs, op.activation.sessions, s.targets)
	if err != nil {
		return nil, err
	}
	// Branch event snapshots carry durable values. Runtime handle transfer
	// needs a separate borrow lifetime and is deliberately not implicit here.
	if len(leases) != 0 {
		return nil, errors.New("branch snapshots cannot transfer runtime handles")
	}
	if lane.coalesce {
		if lane.active != nil {
			if lane.active.cooperative != call.request.CooperativeInput {
				return nil, errors.New("coalesced branch cannot change input composition")
			}
			return lane.active.handle, nil
		}
		if len(lane.queue) != 0 {
			return lane.queue[0].handle, nil
		}
	}
	if op.branchCount >= maxPendingBranches {
		return nil, errors.New("pending invocation branch budget exceeded")
	}
	size := 0
	for _, value := range values {
		size += len(value.RuntimeArtifact())
	}
	if !s.canRetain(size) {
		return nil, errors.New("branch snapshot retained value budget exceeded")
	}
	branch := &branchExecution{cooperative: call.request.CooperativeInput, ctx: call.ctx, owner: s, op: op, lane: lane, output: call.request.Output, values: values, bytes: size, handle: &branchHandle{done: make(chan struct{})}}
	branch.stopWake = context.AfterFunc(call.ctx, func() {
		select {
		case s.group.wake <- struct{}{}:
		default:
		}
	})
	lane.queue = append(lane.queue, branch)
	op.branchCount++
	op.branchBytes += size
	return branch.handle, nil
}

func (s *scheduler) startBranch(ctx context.Context, b *branchExecution) error {
	a := b.op.activation
	child, err := s.group.spawn(s, s.instructionRoutes(a.node.ID, b.output, nil), func(_ context.Context, cause error) error {
		if b.failure != nil {
			cause = errors.Join(b.failure, withoutCancellation(cause))
		}
		s.finishBranch(b, cause)
		return nil
	})
	if err != nil {
		return err
	}
	b.scope = child
	child.cooperativeInput = b.cooperative
	child.inputOwner = inputcoord.NewOwner()
	if b.cooperative {
		child.inputOwner = inputcoord.NewCooperatingOwner(s.inputOwner)
	}
	child.inputOwner.SetWake(s.group.wake)
	child.branchRoot = b
	child.scopeRole, child.scopeNode = "branch", a.node
	child.transientOutputs = make(map[string]bool, len(s.transientOutputs)+1)
	for id := range s.transientOutputs {
		child.transientOutputs[id] = true
	}
	child.transientOutputs[a.node.ID] = true
	child.clearNodeResult(a.node.ID)
	for port, value := range b.values {
		if err := child.setInstructionOutput(a.node, port, value, a.attempt); err != nil {
			child.done = nil
			child.branchRoot = nil
			return errors.Join(err, s.group.cancelScope(ctx, child))
		}
	}
	b.values = nil
	return nil
}

func (s *scheduler) finishBranch(b *branchExecution, cause error) {
	b.stopWake()
	b.lane.active = nil
	b.op.branchCount--
	failure := cause
	if b.cancelCause != nil {
		failure = withoutCancellation(cause)
		cause = errors.Join(b.cancelCause, failure)
	}
	b.handle.finish(cause)
	if !b.op.closingBranches && failure != nil {
		b.op.branchFailure = errors.Join(b.op.branchFailure, failure)
	}
	select {
	case s.group.wake <- struct{}{}:
	default:
	}
}

func (s *scheduler) closeBranches(ctx context.Context, op *pendingOperation) error {
	op.closingBranches = true
	var err error
	for _, lane := range op.branches {
		for _, b := range lane.queue {
			b.stopWake()
			b.handle.finish(context.Canceled)
			op.branchCount--
		}
		lane.queue = nil
		if lane.active != nil {
			err = errors.Join(err, s.group.cancelScope(ctx, lane.active.scope))
		}
	}
	op.branchBytes = 0
	if op.branchRequests != nil {
		select {
		case call := <-op.branchRequests:
			call.err = context.Canceled
			close(call.ready)
		default:
		}
	}
	return withoutCancellation(err)
}

// Unhandled child failures complete the owning handle before the operation
// unwinds. Existing task/listener semantics outside a branch are unchanged.
func (g *executionGroup) scopeFailure(ctx context.Context, s *scheduler, cause error) error {
	for scope := s; scope != nil; scope = scope.parent {
		if b := scope.branchRoot; b != nil && !scope.stopped {
			b.failure = cause
			return g.cancelScope(ctx, scope)
		}
	}
	return cause
}
