// Package signals provides bounded, non-replaying broadcast subscriptions.
package signals

import (
	"encoding/json"
	"errors"
	"sync"
)

var ErrOverflow = errors.New("signal queue full")
var ErrClosed = errors.New("signal subscription closed")

const Capacity = 64
const MaxQueueBytes = 1 << 20

type Message struct {
	Name     string
	SourceID string
	EventID  string
	Value    json.RawMessage
}

type Hub struct {
	mu          sync.Mutex
	subscribers map[*Subscription]bool
	closed      bool
}

type Subscription struct {
	bytes         int
	hub           *Hub
	topic, source string
	queue         []Message
	capacity      int
	ready         chan struct{}
	err           error
}

func (h *Hub) Subscribe(topic, source string, capacity int) (*Subscription, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrClosed
	}
	if capacity < 1 || capacity > Capacity {
		return nil, ErrOverflow
	}
	if len(h.subscribers) >= 1024 {
		return nil, ErrOverflow
	}
	s := &Subscription{hub: h, topic: topic, source: source, capacity: capacity, ready: make(chan struct{}, 1)}
	if h.subscribers == nil {
		h.subscribers = map[*Subscription]bool{}
	}
	h.subscribers[s] = true
	return s, nil
}

func (h *Hub) Publish(topic string, message Message) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	accepted := 0
	for s := range h.subscribers {
		if s.topic != topic || s.source != "" && s.source != message.SourceID {
			continue
		}
		if len(s.queue) >= s.capacity || s.bytes+len(message.Value) > MaxQueueBytes {
			s.err = ErrOverflow
			s.queue = nil
			s.bytes = 0
			delete(h.subscribers, s)
		} else {
			copy := message
			copy.Value = append(json.RawMessage(nil), message.Value...)
			s.queue = append(s.queue, copy)
			s.bytes += len(copy.Value)
			if s.capacity == 1 {
				delete(h.subscribers, s)
			}
			accepted++
		}
		s.notify()
	}
	return accepted
}
func (s *Subscription) notify() {
	select {
	case s.ready <- struct{}{}:
	default:
	}
}
func (s *Subscription) Ready() <-chan struct{} { return s.ready }
func (s *Subscription) Poll() (Message, bool, error) {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	if s.err != nil {
		return Message{}, false, s.err
	}
	if len(s.queue) == 0 {
		return Message{}, false, nil
	}
	m := s.queue[0]
	s.bytes -= len(m.Value)
	s.queue[0] = Message{}
	s.queue = s.queue[1:]
	if len(s.queue) > 0 {
		s.notify()
	}
	return m, true, nil
}
func (s *Subscription) Close() {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	if s.err == nil {
		s.err = ErrClosed
	}
	s.queue = nil
	s.bytes = 0
	delete(s.hub.subscribers, s)
	s.notify()
}
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for s := range h.subscribers {
		s.err = ErrClosed
		s.queue = nil
		s.bytes = 0
		s.notify()
		delete(h.subscribers, s)
	}
}
func (h *Hub) Subscribers(topic string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := []string{}
	for s := range h.subscribers {
		if s.topic == topic {
			out = append(out, s.source)
		}
	}
	return out
}

// Invalidate wakes subscribers whose source has been removed or replaced.
func (h *Hub) Invalidate(topic, source string, all bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subscribers {
		if s.topic == topic && (all || s.source == source) {
			s.err = ErrClosed
			s.queue = nil
			s.bytes = 0
			delete(h.subscribers, s)
			s.notify()
		}
	}
}

func (h *Hub) ContinuousSubscribers(topic string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := []string{}
	for s := range h.subscribers {
		if s.topic == topic && s.capacity > 1 {
			out = append(out, s.source)
		}
	}
	return out
}
