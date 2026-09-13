package noderuntime

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/navigationpath"
)

// One input writer belongs to the path invocation. Replacing a demand never queues old turns.
type pathSteeringStream struct {
	mu     sync.Mutex
	rate   float64
	until  time.Time
	err    error
	cancel context.CancelFunc
	done   chan struct{}
}

func (d *pathDriver) SetSteering(ctx context.Context, rate float64) error {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || math.Abs(rate) > 180 {
		return errors.New("invalid path steering rate")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s := &d.stream
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.rate = rate
	s.until = time.UnixMilli(d.observation.SampleTimeMs).Add(500 * time.Millisecond)
	if s.done == nil && rate != 0 {
		runCtx, cancel := context.WithCancel(ctx)
		s.cancel = cancel
		s.done = make(chan struct{})
		go d.runSteering(runCtx, s.done)
	}
	return nil
}
func (d *pathDriver) runSteering(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	previous := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			dt := now.Sub(previous)
			previous = now
			s := &d.stream
			s.mu.Lock()
			rate, until := s.rate, s.until
			s.mu.Unlock()
			if rate == 0 {
				continue
			}
			var err error
			if now.After(until) {
				err = navigationpath.ErrUnavailable
			} else if dt > 100*time.Millisecond {
				err = errors.New("path input scheduling stalled")
			} else {
				err = navInvoke(ctx, d.i, d.turn, installed.OperationTurnView, installed.TurnViewRequest{Degrees: rate * dt.Seconds(), DurationMilliseconds: 0})
			}
			if err != nil {
				if ctx.Err() != nil {
					err = pathCancellationRemainder(err)
				}
				s.mu.Lock()
				s.err = err
				s.mu.Unlock()
				return
			}
		}
	}
}
func (s *pathSteeringStream) failure() error { s.mu.Lock(); defer s.mu.Unlock(); return s.err }
func (s *pathSteeringStream) stop() error {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.rate = 0
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.err
	s.err = nil
	s.cancel = nil
	s.done = nil
	return err
}
func (d *pathDriver) Wait(ctx context.Context, duration time.Duration) error {
	if err := d.stream.failure(); err != nil {
		return err
	}
	if d.i.WaitWithPause == nil {
		return d.i.Wait(ctx, duration)
	}
	paused := false
	err := d.i.WaitWithPause(ctx, duration, func(cleanup context.Context) error { paused = true; return d.StopForward(cleanup) })
	if paused {
		d.freshAfter = time.Now().UnixMilli() + 1
		d.inputReset = true
	}
	return err
}
