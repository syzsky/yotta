package compiler

import (
	"container/heap"
	"context"
	"errors"
	"time"

	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodecontract"
	run "github.com/yottaapp/yotta/internal/run"
)

const maxPendingWaits = 1024

// waitingActivation retains the invocation's identity and evidence writers
// until completion. Only the scheduler accesses it; timers own no goroutines.
type waitingActivation struct {
	node     programNode
	machine  nodecontract.MachineContract
	attempt  int
	summary  run.RedactedSummary
	sessions map[string]*run.Session
	actions  *adapterActionRecorder
	statuses *statusEmitter
}

type pendingWait struct {
	activation waitingActivation
	request    nodeadapter.WaitRequest
	due        time.Time
	sequence   uint64
}

type waitQueue []*pendingWait

func (q waitQueue) Len() int { return len(q) }
func (q waitQueue) Less(i, j int) bool {
	if q[i].due.Equal(q[j].due) {
		return q[i].sequence < q[j].sequence
	}
	return q[i].due.Before(q[j].due)
}
func (q waitQueue) Swap(i, j int)   { q[i], q[j] = q[j], q[i] }
func (q *waitQueue) Push(value any) { *q = append(*q, value.(*pendingWait)) }
func (q *waitQueue) Pop() any {
	last := len(*q) - 1
	value := (*q)[last]
	(*q)[last] = nil
	*q = (*q)[:last]
	return value
}

func (s *scheduler) addWait(activation waitingActivation, request *nodeadapter.WaitRequest) error {
	if request.Complete == nil || request.Duration < 0 {
		return errors.New("timer continuation has no completion or has a negative duration")
	}
	if s.waitCount >= maxPendingWaits {
		return errors.New("pending timer budget exceeded")
	}
	s.waitCount++
	s.waitSequence++
	heap.Push(&s.waits, &pendingWait{
		activation: activation, request: *request,
		due: s.executor.monotonicNow().Add(request.Duration), sequence: s.waitSequence,
	})
	return nil
}

func (s *scheduler) resumeWait(ctx context.Context) error {
	wait := heap.Pop(&s.waits).(*pendingWait)
	s.waitCount--
	remaining := max(time.Duration(0), wait.due.Sub(s.executor.monotonicNow()))
	err := s.executor.wait(ctx, remaining)
	// Cancellation wins over a successful timer callback when both are ready.
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	outputs, completionErr := wait.request.Complete(ctx, err)
	if ctx.Err() != nil {
		completionErr = errors.Join(completionErr, ctx.Err())
	}
	return s.finishInvocation(ctx, wait.activation, nodeadapter.AdapterResult{ExecOutputs: outputs}, completionErr)
}

// closeWaits consumes every continuation without executing its outgoing edges.
// A region exit can cancel its children while the enclosing Run continues.
func (s *scheduler) closeWaits(ctx context.Context) error {
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	cancel()
	var result error
	for len(s.waits) != 0 {
		wait := heap.Pop(&s.waits).(*pendingWait)
		s.waitCount--
		outputs, err := wait.request.Complete(ctx, context.Canceled)
		err = errors.Join(err, context.Canceled)
		result = errors.Join(result, withoutCancellation(s.finishInvocation(ctx, wait.activation, nodeadapter.AdapterResult{ExecOutputs: outputs}, err)))
	}
	return result
}

// Remove only expected cancellation leaves, retaining journal/cleanup failures
// even when errors.Join also contains context.Canceled.
func withoutCancellation(err error) error {
	if err == nil || err == context.Canceled {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var result error
		for _, child := range joined.Unwrap() {
			result = errors.Join(result, withoutCancellation(child))
		}
		return result
	}
	if wrapped := errors.Unwrap(err); wrapped != nil && errors.Is(err, context.Canceled) {
		return withoutCancellation(wrapped)
	}
	return err
}
