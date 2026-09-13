package pathrecording

import (
	"context"
	"time"

	"github.com/yottaapp/yotta/internal/apperr"
)

type SampleEvent struct {
	Session string           `json:"session"`
	Sample  *Sample          `json:"sample,omitempty"`
	Problem *apperr.Envelope `json:"problem,omitempty"`
}

// StartWatching owns the sampling clock outside the WebView, so foreground
// games and minimised editor windows do not throttle waypoint acquisition.
func (s *Service) StartWatching(session, endpoint string) error {
	if session == "" || len(session) > 128 || s.emit == nil {
		return apperr.New("path.source_invalid", nil)
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return apperr.NewRetryable("path.source_unavailable", nil)
	}
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.session = cancel, session
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		if s.prepare != nil {
			if err := s.prepare(ctx, endpoint); err != nil {
				if ctx.Err() == nil {
					problem := apperr.From(err)
					s.emit("path:sample", SampleEvent{Session: session, Problem: &problem})
				}
				return
			}
		}
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		first := true
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			var sample Sample
			var err error
			if first {
				sample, err = s.waitForPosition(ctx, endpoint, 10*time.Second)
				first = false
			} else {
				sample, err = s.sample(ctx, endpoint)
			}
			if ctx.Err() != nil {
				return
			}
			event := SampleEvent{Session: session, Sample: &sample}
			if err != nil {
				problem := apperr.From(err)
				event.Sample = nil
				event.Problem = &problem
			}
			s.emit("path:sample", event)
			if err != nil {
				return
			}
		}
	}()
	return nil
}

// StopCurrent cancels sampling when the native recording window closes.
//wails:ignore
func (s *Service) StopCurrent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.session = ""
}

func (s *Service) StopWatching(session string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == session && s.cancel != nil {
		s.cancel()
		s.cancel = nil
		s.session = ""
	}
}

// SetMarkHotkey preserves the existing RPC while delegating to the durable
// hotkey-center entry. The editor no longer maintains a transient binding.
func (s *Service) SetMarkHotkey(chord string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return apperr.New("hotkey.registration_failed", nil)
	}
	if s.hotkeys == nil || s.emit == nil {
		return apperr.New("hotkey.registration_failed", nil)
	}
	return s.hotkeys.Update("paths.mark", chord)
}

func Shutdown(ctx context.Context, s *Service) error {
	s.mu.Lock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}
	return nil
}
