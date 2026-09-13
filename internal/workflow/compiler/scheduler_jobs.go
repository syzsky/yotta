package compiler

import (
	"context"
	"errors"
	"sync"

	"github.com/yottaapp/yotta/internal/automation/inputcoord"
)

// Adapter producers inherit the lifetime of the scope that created them.
// Resource cleanup waits for their cancellation acknowledgement.
type scopeJobs struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	wake     chan struct{}
	count    int
	closing  bool
	finished *sync.Cond
	err      error
}

func newScopeJobs(parent context.Context, wake chan struct{}) *scopeJobs {
	ctx, cancel := context.WithCancel(parent)
	j := &scopeJobs{ctx: ctx, cancel: cancel, wake: wake}
	j.finished = sync.NewCond(&j.mu)
	return j
}

func (j *scopeJobs) active() bool {
	if j == nil {
		return false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.count > 0
}

func (j *scopeJobs) failure() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.err
}

func (s *scheduler) spawnTask(task func(context.Context) error) error {
	if s.jobs == nil {
		return s.owner.Go(task)
	}
	if task == nil {
		return errors.New("scope task is required")
	}
	j := s.jobs
	taskCtx := j.ctx
	if s.cooperativeInput {
		taskCtx = inputcoord.WithOwner(taskCtx, s.inputOwner)
	}
	j.mu.Lock()
	if j.closing || j.ctx.Err() != nil {
		j.mu.Unlock()
		return context.Canceled
	}
	if j.count >= maxExecutionScopes {
		j.mu.Unlock()
		return errors.New("scope producer task budget exceeded")
	}
	j.count++
	j.mu.Unlock()
	finish := func(err error) {
		j.mu.Lock()
		j.count--
		if j.ctx.Err() != nil {
			err = withoutCancellation(err)
		}
		j.err = errors.Join(j.err, err)
		j.finished.Broadcast()
		j.mu.Unlock()
		select {
		case j.wake <- struct{}{}:
		default:
		}
	}
	err := s.owner.Go(func(context.Context) error {
		var resultErr error
		defer func() {
			if recovered := recover(); recovered != nil {
				resultErr = errors.New("scope producer panicked")
			}
			finish(resultErr)
		}()
		resultErr = task(taskCtx)
		// The scheduler observes the failure and preserves its scope ownership.
		return nil
	})
	if err != nil {
		finish(err)
	}
	return err
}

func (j *scopeJobs) close() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	j.closing = true
	j.cancel()
	for j.count > 0 {
		j.finished.Wait()
	}
	err := j.err
	j.mu.Unlock()
	return err
}
