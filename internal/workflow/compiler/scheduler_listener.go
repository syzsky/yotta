package compiler

import (
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodeadapter"
)

type listenerActivation struct {
	owner      *scheduler
	activation waitingActivation
	request    *nodeadapter.SubscriptionRequest
	latest     map[string]datatype.ValueEnvelope
	body, main *scheduler
	cancel     context.CancelFunc
	done       chan struct{}
	closed     bool
}

func (s *scheduler) listenerSignal(ctx context.Context, id string, trigger *nodeadapter.SignalTrigger) (bool, error) {
	node, ok := s.nodes[id]
	if !ok || node.Instruction.Invoke == nil || node.Instruction.Invoke.Subscription == nil {
		return false, nil
	}
	if trigger != nil && trigger.InputPort == node.Instruction.Invoke.Subscription.StopInput {
		for owner := s; owner != nil; owner = owner.parent {
			if l := owner.listeners[id]; l != nil {
				return true, l.close(ctx, nil, true)
			}
		}
		return true, errors.New("listener stop has no active subscription")
	}
	if s.listeners[id] != nil {
		return true, errors.New("listener is already active in this scope")
	}
	return false, nil
}
func (s *scheduler) addListener(ctx context.Context, a waitingActivation, r *nodeadapter.SubscriptionRequest) error {
	if s.group == nil || a.node.Instruction.Invoke == nil || a.node.Instruction.Invoke.Subscription == nil || r.Ready == nil || r.Poll == nil || r.Close == nil {
		return errors.New("invalid subscription continuation")
	}
	if s.group.listenerCount >= maxExecutionScopes {
		return errors.New("subscription budget exceeded")
	}
	watch, cancel := context.WithCancel(ctx)
	l := &listenerActivation{owner: s, activation: a, request: r, latest: r.Initial, cancel: cancel, done: make(chan struct{})}
	if s.listeners == nil {
		s.listeners = map[string]*listenerActivation{}
	}
	s.listeners[a.node.ID] = l
	s.keepAlive++
	s.group.listenerCount++
	go func() {
		defer close(l.done)
		for {
			select {
			case <-watch.Done():
				return
			case <-r.Ready:
				select {
				case s.group.wake <- struct{}{}:
				default:
				}
			}
		}
	}()
	v := a.node.Instruction.Invoke.Subscription
	routes := s.instructionRoutes(a.node.ID, v.MainOutput, nil)
	if len(routes) > 0 {
		main, err := s.group.spawn(s, routes, func(ctx context.Context, cause error) error {
			if l.closed {
				return nil
			}
			return l.close(ctx, cause, cause == nil)
		})
		if err != nil {
			l.cancel()
			<-l.done
			delete(s.listeners, a.node.ID)
			s.keepAlive--
			s.group.listenerCount--
			return err
		}
		l.main = main
		main.scopeRole, main.scopeNode = "main", a.node
	}
	return nil
}
func (l *listenerActivation) advance(ctx context.Context) error {
	if l.closed || l.owner.paused || l.body != nil {
		return nil
	}
	outputs, ok, err := l.request.Poll()
	if err != nil {
		return l.close(ctx, err, false)
	}
	if !ok {
		return nil
	}
	s, a := l.owner, l.activation
	sealed, leases, err := s.executor.validateOutputs(a.node, outputs, a.sessions, s.targets)
	if err != nil {
		return l.close(ctx, err, false)
	}
	s.clearNodeResult(a.node.ID)
	size := 0
	for _, value := range sealed {
		size += len(value.RuntimeArtifact())
	}
	s.owned = append(s.owned, leases...)
	if !s.canRetain(size) {
		return l.close(ctx, errors.New("subscription output budget exceeded"), false)
	}
	s.retainedBytes += size
	s.result.NodeOutputs[a.node.ID] = sealed
	s.result.attempts[a.node.ID] = map[string]int{}
	for port := range sealed {
		s.result.attempts[a.node.ID][port] = a.attempt
	}
	l.latest = sealed
	body, err := s.group.spawn(s, s.instructionRoutes(a.node.ID, a.node.Instruction.Invoke.Subscription.EventOutput, nil), func(ctx context.Context, cause error) error {
		l.body = nil
		if l.closed {
			return nil
		}
		if cause != nil {
			return l.close(ctx, cause, false)
		}
		return nil
	})
	if err != nil {
		return l.close(ctx, err, false)
	}
	l.body = body
	body.scopeRole, body.scopeNode = "event", a.node
	return nil
}
func (l *listenerActivation) close(ctx context.Context, cause error, completed bool) error {
	if l.closed {
		return nil
	}
	l.closed = true
	l.cancel()
	<-l.done
	var err error
	for _, child := range []*scheduler{l.body, l.main} {
		if child != nil {
			child.done = nil
			err = errors.Join(err, l.owner.group.cancelScope(ctx, child))
		}
	}
	delete(l.owner.listeners, l.activation.node.ID)
	l.owner.keepAlive--
	l.owner.group.listenerCount--
	if !completed && cause == nil {
		cause = context.Canceled
	}
	cause = errors.Join(cause, err)
	cause = errors.Join(cause, l.request.Close(ctx, cause))
	outcome := nodeadapter.AdapterResult{Outputs: l.latest}
	if completed && cause == nil {
		outcome.ExecOutputs = []string{l.activation.node.Instruction.Invoke.Subscription.CompletedOutput}
	}
	l.owner.clearNodeResult(l.activation.node.ID)
	return l.owner.finishInvocation(ctx, l.activation, outcome, cause)
}
