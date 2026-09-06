package panel

import (
	"context"
	"github.com/google/uuid"
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
func (s *Service) Save(d Draft) (Draft, error)             { return s.managed.save(d) }
func (s *Service) Edit(id string) (Draft, error)           { return s.managed.draft(id) }
func (s *Service) Delete(id string, revision uint64) error { return s.managed.delete(id, revision) }
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
	for ch, component := range s.listeners[id] {
		if component == "" || component == e.ComponentID {
			select {
			case ch <- Interaction{ComponentID: e.ComponentID, EventID: e.EventID, Name: e.Name, Value: e.Value}:
			default:
			}
		}
	}
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
	ch := make(chan Interaction, 1)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Interaction{}, ErrPanelUnavailable
	}
	if s.listeners[ref.ID] == nil {
		s.listeners[ref.ID] = map[chan Interaction]string{}
	}
	s.listeners[ref.ID][ch] = component
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.listeners[ref.ID], ch)
		if len(s.listeners[ref.ID]) == 0 {
			delete(s.listeners, ref.ID)
		}
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return Interaction{}, ctx.Err()
		case <-s.ctx.Done():
			return Interaction{}, ErrPanelUnavailable
		case e := <-ch:
			if _, err := s.checkReference(ref); err != nil {
				return Interaction{}, err
			}
			return e, nil
		case <-ticker.C:
			current, err := s.checkReference(ref)
			if err != nil {
				return Interaction{}, err
			}
			if component != "" {
				c, ok := current.Definition.Component(component)
				if !ok {
					return Interaction{}, ErrComponentMissing
				}
				if c.Event == "" {
					return Interaction{}, ErrComponentNotInteractive
				}
			}
		}
	}
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
