// Package inputcoord coordinates conflicting physical input. It does not grant
// permission: configured targets remain direct calls. Ownership lasts through
// held input and is handed over only after physical release is acknowledged.
package inputcoord

import (
	"context"
	"errors"
	"sort"
	"sync"
)

type ownerKey struct{}
type resumeOwnerKey struct{}

type Held interface {
	InputDomain() string
	PauseInput(context.Context) error
	ResumeInput(context.Context) error
}

type Owner struct {
	mu      sync.Mutex
	held    map[Held]struct{}
	paused  int
	frozen  int
	waiting int
	changed chan struct{}
	wake    chan<- struct{}
}

func NewOwner() *Owner { return &Owner{changed: make(chan struct{})} }

// SetWake is configured once before the scope starts any input operation.
func (o *Owner) SetWake(wake chan<- struct{}) { o.wake = wake }
func (o *Owner) notify() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

// Freeze closes the acquisition gate before the scheduler waits for in-flight
// effects. An acquisition which has not returned cannot send physical input.
func (o *Owner) Freeze() { o.mu.Lock(); o.frozen++; o.mu.Unlock() }
func (o *Owner) Thaw() {
	o.mu.Lock()
	if o.frozen > 0 {
		o.frozen--
		if o.frozen == 0 {
			close(o.changed)
			o.changed = make(chan struct{})
		}
	}
	o.mu.Unlock()
}
func (o *Owner) Waiting() bool { o.mu.Lock(); defer o.mu.Unlock(); return o.waiting > 0 }
func WithOwner(ctx context.Context, owner *Owner) context.Context {
	return context.WithValue(ctx, ownerKey{}, owner)
}
func FromContext(ctx context.Context) *Owner {
	if owner, ok := ctx.Value(ownerKey{}).(*Owner); ok && owner != nil {
		return owner
	}
	return NewOwner()
}
func (o *Owner) Register(held Held) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.held == nil {
		o.held = map[Held]struct{}{}
	}
	o.held[held] = struct{}{}
}
func (o *Owner) Unregister(held Held) { o.mu.Lock(); delete(o.held, held); o.mu.Unlock() }
func (o *Owner) snapshot() []Held {
	o.mu.Lock()
	held := make([]Held, 0, len(o.held))
	for h := range o.held {
		held = append(held, h)
	}
	o.mu.Unlock()
	sort.Slice(held, func(i, j int) bool { return held[i].InputDomain() < held[j].InputDomain() })
	return held
}
func (o *Owner) Pause(ctx context.Context) error {
	o.mu.Lock()
	if o.paused > 0 {
		o.paused++
		o.mu.Unlock()
		return nil
	}
	o.mu.Unlock()
	for _, held := range o.snapshot() {
		if err := held.PauseInput(ctx); err != nil {
			return err
		}
	}
	o.mu.Lock()
	o.paused++
	o.mu.Unlock()
	return nil
}
func (o *Owner) Resume(ctx context.Context) error {
	o.mu.Lock()
	if o.paused != 1 {
		if o.paused > 1 {
			o.paused--
		}
		o.mu.Unlock()
		return nil
	}
	o.mu.Unlock()
	// Restoring existing held intent precedes thawing normal acquisitions.
	resumeCtx := context.WithValue(ctx, resumeOwnerKey{}, o)
	for _, held := range o.snapshot() {
		if err := held.ResumeInput(resumeCtx); err != nil {
			return err
		}
	}
	o.mu.Lock()
	o.paused = 0
	o.mu.Unlock()
	return nil
}

type Coordinator struct {
	mu      sync.Mutex
	domains map[string]*domain
}
type domain struct {
	owner *Owner
	count int
	queue []*waiter
}
type waiter struct {
	owner   *Owner
	ready   chan struct{}
	granted bool
}
type Lease struct {
	coordinator *Coordinator
	key         string
	domain      *domain
	once        sync.Once
}

// Acquire is reentrant for one execution scope. Other scopes queue FIFO; a
// cancelled waiter cannot retain the physical input domain or block its queue.
func (c *Coordinator) Acquire(ctx context.Context, key string, owner *Owner) (*Lease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if owner == nil {
		owner = FromContext(ctx)
	}
	bypass := ctx.Value(resumeOwnerKey{}) == owner
	owner.mu.Lock()
	owner.waiting++
	owner.mu.Unlock()
	owner.notify()
	waiting := true
	defer func() {
		if waiting {
			owner.mu.Lock()
			owner.waiting--
			owner.mu.Unlock()
		}
	}()
	for {
		owner.mu.Lock()
		frozen, changed := owner.frozen > 0 && !bypass, owner.changed
		owner.mu.Unlock()
		if frozen {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-changed:
			}
			continue
		}
		lease, err := c.acquire(ctx, key, owner)
		if err != nil {
			return nil, err
		}
		owner.mu.Lock()
		if owner.frozen > 0 && !bypass {
			owner.mu.Unlock()
			lease.Release()
			continue
		}
		// Commit dispatch under the same lock as Freeze. Once freeze returns,
		// the scheduler sees either a parked acquisition or an active effect.
		owner.waiting--
		waiting = false
		owner.mu.Unlock()
		return lease, nil
	}
}

func (c *Coordinator) acquire(ctx context.Context, key string, owner *Owner) (*Lease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.domains == nil {
		c.domains = map[string]*domain{}
	}
	d := c.domains[key]
	if d == nil {
		d = &domain{}
		c.domains[key] = d
	}
	lease := &Lease{coordinator: c, key: key, domain: d}
	if d.owner == nil || d.owner == owner {
		d.owner = owner
		d.count++
		c.mu.Unlock()
		return lease, nil
	}
	// A scope already holding another domain must not wait in a cycle.
	for _, held := range c.domains {
		if held.owner == owner {
			c.mu.Unlock()
			return nil, errors.New("conflicting input domains are already held")
		}
	}
	w := &waiter{owner: owner, ready: make(chan struct{})}
	d.queue = append(d.queue, w)
	c.mu.Unlock()
	select {
	case <-w.ready:
		if err := ctx.Err(); err != nil {
			lease.Release()
			return nil, err
		}
		return lease, nil
	case <-ctx.Done():
		c.mu.Lock()
		if w.granted {
			c.releaseLocked(key, d)
		} else {
			for n, pending := range d.queue {
				if pending == w {
					d.queue = append(d.queue[:n], d.queue[n+1:]...)
					break
				}
			}
		}
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() { c := l.coordinator; c.mu.Lock(); c.releaseLocked(l.key, l.domain); c.mu.Unlock() })
}
func (c *Coordinator) releaseLocked(key string, d *domain) {
	d.count--
	if d.count > 0 {
		return
	}
	d.owner = nil
	if len(d.queue) > 0 {
		w := d.queue[0]
		d.queue = d.queue[1:]
		d.owner = w.owner
		d.count = 1
		w.granted = true
		close(w.ready)
	} else {
		delete(c.domains, key)
	}
}

func (o *Owner) NeedsResume() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.paused == 1 && len(o.held) > 0
}
