package panel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/yottaapp/yotta/internal/signals"
	contract "github.com/yottaapp/yotta/sdk/plugin/panel"
	"time"
)

func Open(c Catalog, path string) (*Service, error) {
	m, err := openManaged(path)
	if err != nil {
		return nil, err
	}
	s := New(c)
	s.managed = m
	return s, nil
}
func (s *Service) Save(d Draft) (Draft, error) {
	saved, err := s.managed.save(d)
	if err != nil {
		return saved, err
	}
	ref, e := s.Resolve(saved.ID)
	if e == nil {
		for _, component := range s.events.Subscribers(saved.ID) {
			if component != "" {
				if _, e := s.ResolveComponent(ref, component, "event"); e != nil {
					s.events.Invalidate(saved.ID, component, false)
				}
			}
		}
	}
	return saved, nil
}
func (s *Service) Edit(id string) (Draft, error) { return s.managed.draft(id) }
func (s *Service) Delete(id string, revision uint64) error {
	if err := s.managed.delete(id, revision); err != nil {
		return err
	}
	s.events.Invalidate(id, "", true)
	return nil
}
func (s *Service) SetPresenter(show func(string) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.show = show
}
func (s *Service) Selected() string { s.mu.Lock(); defer s.mu.Unlock(); return s.selected }
func (s *Service) Show(id string) error {
	source, err := s.source(id)
	if err != nil {
		return err
	}
	id = source.ID
	s.mu.Lock()
	show := s.show
	s.selected = id
	s.mu.Unlock()
	if show == nil {
		return problem("panels.presentation_unavailable", nil)
	}
	return show(id)
}
func (s *Service) Resolve(id string) (Reference, error) {
	p, err := s.source(id)
	if err != nil {
		return Reference{}, ErrPanelMissing
	}
	return Reference{ID: p.ID, Generation: p.Generation}, nil
}
func (s *Service) ResolveComponent(ref Reference, id, kind string) (ComponentReference, error) {
	p, err := s.checkReference(ref)
	if err != nil {
		return ComponentReference{}, err
	}
	c, ok := p.Definition.Component(id)
	if !ok {
		return ComponentReference{}, ErrComponentMissing
	}
	if !componentMatches(p.Definition, c, kind) {
		return ComponentReference{}, ErrInvalidValue
	}
	return ComponentReference{Panel: ref, ID: id, Kind: kind}, nil
}
func componentMatches(d contract.Definition, c contract.Component, kind string) bool {
	if kind == "event" {
		return c.Event != ""
	}
	if kind == "log" {
		return c.Kind == "log"
	}
	for _, f := range d.Fields {
		if f.ID == c.Field {
			return f.Kind == kind
		}
	}
	return false
}
func (s *Service) checkReference(ref Reference) (Source, error) {
	p, err := s.source(ref.ID)
	if err != nil {
		return p, ErrPanelMissing
	}
	if p.Generation != ref.Generation {
		return p, ErrPanelEnded
	}
	return p, nil
}
func (s *Service) Value(ref ComponentReference) (any, error) {
	p, err := s.checkReference(ref.Panel)
	if err != nil {
		return nil, err
	}
	c, ok := p.Definition.Component(ref.ID)
	if !ok {
		return nil, ErrComponentMissing
	}
	if !componentMatches(p.Definition, c, ref.Kind) {
		return nil, ErrInvalidValue
	}
	snapshot, err := s.Read(ref.Panel.ID)
	if err != nil {
		return nil, err
	}
	value, ok := snapshot.Values[c.Field]
	if !ok {
		return nil, ErrComponentNoValue
	}
	return value, nil
}
func (s *Service) Write(ref ComponentReference, value any) error {
	p, err := s.checkReference(ref.Panel)
	if err != nil {
		return err
	}
	c, ok := p.Definition.Component(ref.ID)
	if !ok {
		return ErrComponentMissing
	}
	if !componentMatches(p.Definition, c, ref.Kind) {
		return ErrInvalidValue
	}
	if p.Managed {
		return s.managed.write(ref.Panel, ref.ID, value, ref.Kind == "log")
	}
	if c.Event == "" {
		return ErrComponentReadOnly
	}
	state, err := s.Read(ref.Panel.ID)
	if err != nil {
		return err
	}
	_, err = s.dispatch(ref.Panel.ID, contract.Event{SessionID: state.SessionID, EventID: uuid.NewString(), ComponentID: c.ID, Name: c.Event, Revision: state.ControlRevisions[c.ID], Value: value}, false)
	return err
}
func (s *Service) publish(id string, e contract.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := id + "/" + e.SessionID + "/" + e.EventID
	if s.received[key] {
		return
	}
	s.received[key] = true
	s.receivedOrder = append(s.receivedOrder, key)
	if len(s.receivedOrder) > 1024 {
		delete(s.received, s.receivedOrder[0])
		s.receivedOrder = s.receivedOrder[1:]
	}
	raw, _ := json.Marshal(e.Value)
	s.events.Publish(id, signals.Message{Name: e.Name, SourceID: e.ComponentID, EventID: e.EventID, Value: raw})
}

// Each waiting workflow receives the interaction independently. Ending a Run
// cancels its context and releases only its own subscription.
func (s *Service) Wait(ctx context.Context, ref Reference, component string) (Interaction, error) {
	p, err := s.checkReference(ref)
	if err != nil {
		return Interaction{}, err
	}
	if component != "" {
		c, ok := p.Definition.Component(component)
		if !ok {
			return Interaction{}, ErrComponentMissing
		}
		if c.Event == "" {
			return Interaction{}, ErrComponentNotInteractive
		}
	}
	sub, err := s.Subscribe(ref, component, 1)
	if err != nil {
		return Interaction{}, err
	}
	defer sub.Close()
	for {
		select {
		case <-ctx.Done():
			return Interaction{}, ctx.Err()
		case <-sub.Ready():
			event, ok, err := sub.Poll()
			if err != nil {
				return Interaction{}, err
			}
			if ok {
				return event, nil
			}
		}
	}
}

type Subscription struct {
	service *Service
	ref     Reference
	source  *signals.Subscription
}

func (s *Service) Subscribe(ref Reference, component string, capacity int) (*Subscription, error) {
	p, err := s.checkReference(ref)
	if err != nil {
		return nil, err
	}
	if component != "" {
		c, ok := p.Definition.Component(component)
		if !ok {
			return nil, ErrComponentMissing
		}
		if c.Event == "" {
			return nil, ErrComponentNotInteractive
		}
	}
	sub, err := s.events.Subscribe(ref.ID, component, capacity)
	if err != nil {
		return nil, ErrPanelCapacity
	}
	return &Subscription{service: s, ref: ref, source: sub}, nil
}
func (s *Subscription) Ready() <-chan struct{} { return s.source.Ready() }
func (s *Subscription) Close()                 { s.source.Close() }
func (s *Subscription) Poll() (Interaction, bool, error) {
	m, ok, err := s.source.Poll()
	if !ok && err == nil {
		return Interaction{}, false, nil
	}
	if _, e := s.service.checkReference(s.ref); e != nil {
		return Interaction{}, false, e
	}
	if errors.Is(err, signals.ErrOverflow) {
		err = ErrPanelQueueFull
	}
	if errors.Is(err, signals.ErrClosed) {
		err = ErrPanelUnavailable
	}
	if err != nil || !ok {
		return Interaction{}, false, err
	}
	var value any
	if err = json.Unmarshal(m.Value, &value); err != nil {
		return Interaction{}, false, ErrInvalidValue
	}
	return Interaction{ComponentID: m.SourceID, EventID: m.EventID, Name: m.Name, Value: value}, true, nil
}

func (s *Service) MarkUpdated(id, runID string) {
	s.managed.mu.Lock()
	defer s.managed.mu.Unlock()
	if p := s.managed.panels[id]; p != nil {
		p.source.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		p.source.LastRunID = runID
		p.source.LastRunStatus = "running"
	}
}
func (s *Service) EndRun(runID, status string) {
	s.managed.mu.Lock()
	defer s.managed.mu.Unlock()
	for _, p := range s.managed.panels {
		if p.source.LastRunID == runID {
			p.source.LastRunStatus = status
		}
	}
}
