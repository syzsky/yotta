package installed

import (
	"errors"
	"fmt"
	"sync"

	pkginput "github.com/yottaapp/yotta/pkg/input"
)

// keyClaims belongs to one driver. Its physical backend is used only for keys;
// each claimant keeps its existing backend for pointer operations and cleanup.
type keyClaims struct {
	mu       sync.Mutex
	physical pkginput.Backend
	counts   map[claimedKey]int
	closed   bool
}

type claimedKey struct {
	window pkginput.Handle
	code   uint32
}

type claimedInput struct {
	pkginput.Backend
	keys *keyClaims
	held map[claimedKey]pkginput.Handle
}

func (s *keyClaims) input(backend pkginput.Backend) *claimedInput {
	return &claimedInput{Backend: backend, keys: s, held: make(map[claimedKey]pkginput.Handle)}
}

func (b *claimedInput) key(hwnd pkginput.Handle, code uint32) claimedKey {
	if b.keys.physical.Capabilities().GlobalInput {
		hwnd = 0
	}
	return claimedKey{hwnd, code}
}

func (b *claimedInput) KeyDown(hwnd pkginput.Handle, key string) error {
	return b.KeyDownCode(hwnd, pkginput.VK(key))
}
func (b *claimedInput) KeyUp(hwnd pkginput.Handle, key string) error {
	return b.KeyUpCode(hwnd, pkginput.VK(key))
}
func (b *claimedInput) KeyDownCode(hwnd pkginput.Handle, code uint32) error {
	s := b.keys
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("keyboard driver is closed")
	}
	if code == 0 || code > 255 {
		return fmt.Errorf("invalid virtual key %d", code)
	}
	// Repeated downs refresh held input, but must not accumulate claims.
	if err := s.physical.KeyDownCode(hwnd, code); err != nil {
		return err
	}
	k := b.key(hwnd, code)
	if _, exists := b.held[k]; !exists {
		if s.counts == nil {
			s.counts = make(map[claimedKey]int)
		}
		s.counts[k]++
		b.held[k] = hwnd
	}
	return nil
}
func (b *claimedInput) KeyUpCode(hwnd pkginput.Handle, code uint32) error {
	s := b.keys
	s.mu.Lock()
	defer s.mu.Unlock()
	if code == 0 || code > 255 {
		return fmt.Errorf("invalid virtual key %d", code)
	}
	return b.release(b.key(hwnd, code))
}

// release runs under keys.mu. Failed physical releases retain the claim so
// operation cleanup, Close or driver teardown can retry.
func (b *claimedInput) release(k claimedKey) error {
	hwnd, exists := b.held[k]
	if !exists {
		return nil
	}
	s := b.keys
	if !s.closed && s.counts[k] == 1 {
		if err := s.physical.KeyUpCode(hwnd, k.code); err != nil {
			return err
		}
	}
	delete(b.held, k)
	if s.counts[k] <= 1 {
		delete(s.counts, k)
	} else {
		s.counts[k]--
	}
	return nil
}
func (b *claimedInput) ReleaseAll() error {
	s := b.keys
	s.mu.Lock()
	var err error
	for k := range b.held {
		err = errors.Join(err, b.release(k))
	}
	s.mu.Unlock()
	return errors.Join(err, b.Backend.ReleaseAll())
}
func (b *claimedInput) Close() error {
	if err := b.ReleaseAll(); err != nil {
		return err
	}
	return b.Backend.Close()
}
func (s *keyClaims) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	// PostMessage's own tracking is keyed only by VK. Use our full window/key
	// identities so teardown also releases the same key held in two windows.
	var releaseErr error
	for key := range s.counts {
		releaseErr = errors.Join(releaseErr, s.physical.KeyUpCode(key.window, key.code))
	}
	if releaseErr != nil {
		return releaseErr
	}
	if err := s.physical.ReleaseAll(); err != nil {
		return err
	}
	if err := s.physical.Close(); err != nil {
		return err
	}
	s.closed = true
	clear(s.counts)
	return nil
}
