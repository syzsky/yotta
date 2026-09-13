// Package pathrecording adapts configured position-source HTTP observations for
// authoring. Recording and movement consume the same navigationpath observation.
package pathrecording

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/hotkey"
	"github.com/yottaapp/yotta/internal/navigationpath"
)

type Service struct {
	client  *http.Client
	mu      sync.Mutex
	wg      sync.WaitGroup
	closed  bool
	cancel  context.CancelFunc
	session string
	emit    func(string, any)
	hotkeys *hotkey.HotkeyRegistry
	prepare func(context.Context, string) error
}

func NewService() *Service { return &Service{client: &http.Client{Timeout: 2 * time.Second}} }

func NewDesktopService(keys *hotkey.HotkeyRegistry, emit func(string, any), prepare ...func(context.Context, string) error) *Service {
	s := NewService()
	s.hotkeys = keys
	s.emit = emit
	if len(prepare) > 0 {
		s.prepare = prepare[0]
	}
	return s
}

type Sample struct {
	Reference    navigationpath.Reference `json:"reference"`
	Point        navigationpath.Point     `json:"point"`
	Epoch        string                   `json:"epoch"`
	Sequence     int64                    `json:"sequence"`
	SampleTimeMs int64                    `json:"sampleTimeMs"`
	Recovery     string                   `json:"recovery"`
}

func (s *Service) Sample(endpoint string) (Sample, error) {
	if s.prepare != nil {
		if err := s.prepare(context.Background(), endpoint); err != nil {
			return Sample{}, err
		}
	}
	return s.waitForPosition(context.Background(), endpoint, 10*time.Second)
}

// A responsive companion can still be waiting for the game's first observation.
// Wait for fresh data, never turn a stale observation into a recorded point.
func (s *Service) waitForPosition(parent context.Context, endpoint string, timeout time.Duration) (Sample, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	for {
		value, err := s.sample(ctx, endpoint)
		if parent.Err() != nil {
			return Sample{}, parent.Err()
		}
		if ctx.Err() != nil {
			return Sample{}, apperr.NewRetryable("path.position_wait_timeout", nil)
		}
		if err == nil || apperr.From(err).ID != "path.position_unavailable" {
			return value, err
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			if parent.Err() != nil {
				return Sample{}, parent.Err()
			}
			return Sample{}, apperr.NewRetryable("path.position_wait_timeout", nil)
		case <-timer.C:
		}
	}
}

func (s *Service) sample(parent context.Context, endpoint string) (Sample, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return Sample{}, apperr.New("path.source_invalid", nil)
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Sample{}, apperr.New("path.source_invalid", nil)
	}
	response, err := s.client.Do(req)
	if err != nil {
		return Sample{}, fmt.Errorf("%w: %v", apperr.NewRetryable("path.source_unavailable", nil), err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return Sample{}, apperr.New("path.source_endpoint_missing", nil)
	}
	if response.StatusCode != http.StatusOK {
		return Sample{}, apperr.NewRetryable("path.source_unavailable", nil)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil {
		return Sample{}, fmt.Errorf("%w: %v", apperr.NewRetryable("path.source_unavailable", nil), err)
	}
	if len(raw) > 65536 {
		return Sample{}, apperr.New("path.source_invalid", nil)
	}
	o, err := navigationpath.Observe(raw, time.Now(), 500*time.Millisecond)
	if errors.Is(err, navigationpath.ErrReference) {
		return Sample{}, apperr.New("path.reference_undeclared", nil)
	}
	if errors.Is(err, navigationpath.ErrUnavailable) {
		return Sample{}, apperr.NewRetryable("path.position_unavailable", nil)
	}
	if err != nil {
		return Sample{}, fmt.Errorf("%w: %v", apperr.New("path.source_invalid", nil), err)
	}
	return Sample{Reference: o.Reference, Point: o.Point, Epoch: o.Epoch, Sequence: o.Sequence, SampleTimeMs: o.SampleTimeMs, Recovery: o.Recovery}, nil
}
