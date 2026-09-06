// Package panel owns extension panel sessions independently of any window.
package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/yottaapp/yotta/internal/apperr"
	contract "github.com/yottaapp/yotta/sdk/plugin/panel"
)

type Source struct {
	PublisherNamespace string              `json:"-"`
	PackageVersion     string              `json:"-"`
	WaitingComponents  []string            `json:"waitingComponents"`
	Waiting            int                 `json:"waiting"`
	UpdatedAt          string              `json:"updatedAt,omitempty"`
	LastRunID          string              `json:"lastRunId,omitempty"`
	LastRunStatus      string              `json:"lastRunStatus,omitempty"`
	Managed            bool                `json:"managed"`
	ID                 string              `json:"id"`
	OwnerID            string              `json:"ownerId"`
	OwnerName          string              `json:"ownerName"`
	Generation         string              `json:"generation"`
	Status             string              `json:"status,omitempty"`
	Definition         contract.Definition `json:"definition"`
	Origin             string              `json:"-"`
	CompanionID        string              `json:"-"`
	SnapshotPath       string              `json:"-"`
	EventPath          string              `json:"-"`
}
type Catalog interface {
	PanelSources() []Source
	Control(string, string, bool) error
}
type Service struct {
	catalog       Catalog
	client        *http.Client
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
	closed        bool
	active        sync.WaitGroup
	managed       *managedStore
	show          func(string) error
	selected      string
	listeners     map[string]map[chan Interaction]string
	received      map[string]bool
	receivedOrder []string
}

func New(catalog Catalog) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	m, _ := openManaged("")
	return &Service{managed: m, received: map[string]bool{}, listeners: map[string]map[chan Interaction]string{}, catalog: catalog, ctx: ctx, cancel: cancel, client: &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func problem(id string, cause error) error {
	if cause == nil {
		return apperr.New(id, nil)
	}
	return fmt.Errorf("%w: %v", apperr.New(id, nil), cause)
}
func (s *Service) begin() (context.Context, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, nil, problem("panels.closed", nil)
	}
	s.active.Add(1)
	return s.ctx, s.active.Done, nil
}
func (s *Service) Close() error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.managed.close()
	s.mu.Unlock()
	s.active.Wait()
	s.client.CloseIdleConnections()
	return nil
}
func (s *Service) SetCatalog(c Catalog) { s.mu.Lock(); defer s.mu.Unlock(); s.catalog = c }
func (s *Service) List() []Source {
	s.mu.Lock()
	c := s.catalog
	s.mu.Unlock()
	out := s.managed.list()
	if c != nil {
		out = append(out, c.PanelSources()...)
	}
	s.mu.Lock()
	for i := range out {
		out[i].Waiting = len(s.listeners[out[i].ID])
		unique := map[string]bool{}
		out[i].WaitingComponents = []string{}
		for _, component := range s.listeners[out[i].ID] {
			if !unique[component] {
				unique[component] = true
				out[i].WaitingComponents = append(out[i].WaitingComponents, component)
			}
		}
		sort.Strings(out[i].WaitingComponents)
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Managed != out[j].Managed {
			return out[i].Managed
		}
		if out[i].Definition.TitleKey != out[j].Definition.TitleKey {
			return out[i].Definition.TitleKey < out[j].Definition.TitleKey
		}
		return out[i].ID < out[j].ID
	})
	return out
}
func (s *Service) source(id string) (Source, error) {
	for _, p := range s.List() {
		if p.ID == id || !p.Managed && p.OwnerID+"#"+p.Definition.ID == id {
			return p, nil
		}
	}
	return Source{}, problem("panels.not_found", nil)
}
func (s *Service) current(p Source) error {
	now, err := s.source(p.ID)
	if err != nil {
		return err
	}
	if now.Generation != p.Generation {
		return problem("panels.changed", nil)
	}
	return nil
}
func (s *Service) request(ctx context.Context, p Source, path string, event *contract.Event) ([]byte, int, error) {
	method := http.MethodGet
	var body io.Reader
	if event != nil {
		method = http.MethodPost
		raw, err := json.Marshal(event)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.Origin+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if len(raw) > 2<<20 {
		return nil, response.StatusCode, errors.New("panel response too large")
	}
	return raw, response.StatusCode, err
}
func (s *Service) Read(id string) (contract.Snapshot, error) {
	if state, ok := s.managed.read(id); ok {
		return state, nil
	}
	ctx, done, err := s.begin()
	if err != nil {
		return contract.Snapshot{}, err
	}
	defer done()
	p, err := s.source(id)
	if err != nil {
		return contract.Snapshot{}, err
	}
	raw, status, err := s.request(ctx, p, p.SnapshotPath, nil)
	// Only a failed connection may start a companion. Incompatible HTTP services
	// are not replaced; startup uses the plugin manager's existing lifecycle.
	var networkError *net.OpError
	if err != nil && errors.As(err, &networkError) && networkError.Op == "dial" && ctx.Err() == nil {
		if e := s.catalog.Control(p.OwnerID, p.CompanionID, true); e != nil {
			return contract.Snapshot{}, problem("panels.source_unavailable", e)
		}
		raw, status, err = s.request(ctx, p, p.SnapshotPath, nil)
	}
	if err != nil {
		return contract.Snapshot{}, problem("panels.source_unavailable", err)
	}
	var snapshot contract.Snapshot
	if status != 200 || json.Unmarshal(raw, &snapshot) != nil {
		return snapshot, problem("panels.invalid_data", nil)
	}
	if err = p.Definition.ValidateSnapshot(snapshot); err != nil {
		return snapshot, problem("panels.invalid_data", err)
	}
	if err = s.current(p); err != nil {
		return contract.Snapshot{}, err
	}
	return snapshot, nil
}
func (s *Service) Dispatch(id string, event contract.Event) (contract.Result, error) {
	return s.dispatch(id, event, true)
}
func (s *Service) dispatch(id string, event contract.Event, notify bool) (result contract.Result, dispatchErr error) {
	defer func() {
		if dispatchErr == nil && notify {
			s.publish(id, event)
		}
	}()
	if _, ok := s.managed.read(id); ok {
		return s.managed.dispatch(id, event)
	}
	ctx, done, err := s.begin()
	if err != nil {
		return contract.Result{}, err
	}
	defer done()
	p, err := s.source(id)
	if err != nil {
		return contract.Result{}, err
	}
	if err = p.Definition.ValidateEvent(event); err != nil {
		return contract.Result{}, problem("panels.invalid_event", err)
	}
	raw, status, err := s.request(ctx, p, p.EventPath, &event)
	if err != nil {
		return contract.Result{}, problem("panels.result_unknown", err)
	}
	if status == 409 {
		return contract.Result{}, problem("panels.changed", nil)
	}
	if status != 200 {
		return contract.Result{}, problem("panels.action_failed", nil)
	}
	if json.Unmarshal(raw, &result) != nil || result.EventID != event.EventID || result.Snapshot.SessionID != event.SessionID {
		return result, problem("panels.invalid_data", nil)
	}
	if err = p.Definition.ValidateSnapshot(result.Snapshot); err != nil {
		return result, problem("panels.invalid_data", err)
	}
	if err = s.current(p); err != nil {
		return contract.Result{}, err
	}
	return result, nil
}
