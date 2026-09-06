package panel

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yottaapp/yotta/internal/durablefs"
	contract "github.com/yottaapp/yotta/sdk/plugin/panel"
)

// Reference identifies a panel definition, not the workflow that happens to use it.
type Reference struct {
	ID         string `json:"id"`
	Generation string `json:"generation"`
}
type ComponentReference struct {
	Panel Reference `json:"panel"`
	ID    string    `json:"id"`
	Kind  string    `json:"kind"`
}
type ComponentDraft struct {
	Icon    string   `json:"icon,omitempty"`
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Title   string   `json:"title"`
	Initial any      `json:"initial"`
	Options []string `json:"options,omitempty"`
}
type Draft struct {
	ID         string           `json:"id"`
	Revision   uint64           `json:"revision"`
	Title      string           `json:"title"`
	Components []ComponentDraft `json:"components"`
}
type managedPanel struct {
	draft    Draft
	source   Source
	snapshot contract.Snapshot
	receipts map[string]contract.Event
	order    []string
}
type managedStore struct {
	imports map[string]importedPanel
	mu      sync.Mutex
	path    string
	panels  map[string]*managedPanel
	closed  bool
}
type managerFile struct {
	Imports map[string]importedPanel `json:"imports,omitempty"`
	Format  string                   `json:"format"`
	Panels  []Draft                  `json:"panels"`
}

const managerFormat = "yotta.panel-manager/v1"

func openManaged(path string) (*managedStore, error) {
	m := &managedStore{path: path, panels: map[string]*managedPanel{}}
	if path == "" {
		return m, nil
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, problem("panels.load_failed", err)
	}
	var file managerFile
	if json.Unmarshal(raw, &file) != nil || file.Format != managerFormat || len(file.Panels) > 128 {
		return nil, problem("panels.invalid_definition", nil)
	}
	for _, d := range file.Panels {
		if d.ID == "" || d.Revision == 0 || m.panels[d.ID] != nil {
			return nil, problem("panels.invalid_definition", nil)
		}
		p, e := makeManaged(d)
		if e != nil {
			return nil, e
		}
		m.panels[d.ID] = p
	}
	m.imports = file.Imports
	return m, nil
}
func makeManaged(d Draft) (*managedPanel, error) {
	if strings.TrimSpace(d.Title) == "" || len(d.Title) > 128 || len(d.Components) == 0 || len(d.Components) > 128 {
		return nil, problem("panels.invalid_definition", nil)
	}
	def := contract.Definition{Format: contract.Format, ID: d.ID, TitleKey: d.Title, Fields: []contract.Field{}, Components: []contract.Component{}}
	snapshot := contract.Snapshot{Protocol: contract.Protocol, SessionID: uuid.NewString(), Revision: 1, Status: "ready", Values: map[string]any{}, ControlRevisions: map[string]uint64{}, Records: map[string][]contract.Record{}}
	for _, item := range d.Components {
		c := contract.Component{ID: item.ID, Kind: item.Kind, Icon: item.Icon, TitleKey: item.Title, Precision: 2}
		kind := ""
		switch item.Kind {
		case "text", "input", "select":
			kind = "string"
		case "number", "progress", "timer":
			kind = "number"
		case "toggle", "status":
			kind = "boolean"
		case "button", "log":
		default:
			return nil, problem("panels.invalid_definition", nil)
		}
		if kind != "" {
			c.Field = c.ID
			def.Fields = append(def.Fields, contract.Field{ID: c.ID, Kind: kind})
			v := item.Initial
			if v == nil {
				switch kind {
				case "string":
					v = ""
				case "number":
					v = float64(0)
				case "boolean":
					v = false
				}
			}
			snapshot.Values[c.ID] = v
		}
		if item.Kind == "select" || item.Kind == "input" || item.Kind == "toggle" || item.Kind == "button" {
			c.Event = c.ID + ".change"
			snapshot.ControlRevisions[c.ID] = 0
		}
		if item.Kind == "log" {
			snapshot.Records[c.ID] = []contract.Record{}
		}
		for _, o := range item.Options {
			c.Options = append(c.Options, contract.Option{Value: o, LabelKey: o})
		}
		if item.Kind == "select" && (snapshot.Values[c.ID] == nil || snapshot.Values[c.ID] == "") && len(c.Options) > 0 {
			snapshot.Values[c.ID] = c.Options[0].Value
		}
		def.Components = append(def.Components, c)
	}
	// Only complete definitions are published; unsaved empty drafts stay in the editor.
	if len(def.Components) > 0 {
		if err := def.Validate(); err != nil {
			return nil, problem("panels.invalid_definition", err)
		}
		if err := def.ValidateSnapshot(snapshot); err != nil {
			return nil, problem("panels.invalid_definition", err)
		}
	}
	return &managedPanel{draft: contract.Clone(d), source: Source{Managed: true, ID: d.ID, OwnerID: "user", OwnerName: "panels.user_owned", Generation: strconv.FormatUint(d.Revision, 10), Status: "ready", Definition: def}, snapshot: snapshot, receipts: map[string]contract.Event{}}, nil
}
func (m *managedStore) persist(next map[string]*managedPanel) error {
	if m.path == "" {
		return nil
	}
	file := managerFile{Format: managerFormat, Panels: []Draft{}, Imports: m.imports}
	for _, p := range next {
		file.Panels = append(file.Panels, p.draft)
	}
	sort.Slice(file.Panels, func(i, j int) bool { return file.Panels[i].ID < file.Panels[j].ID })
	raw, err := json.Marshal(file)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(m.path), 0700); err != nil {
		return err
	}
	return durablefs.WriteFile(m.path, raw, 0600)
}
func (m *managedStore) save(d Draft) (Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Draft{}, problem("panels.closed", nil)
	}
	creating := d.ID == ""
	if d.ID == "" {
		if len(m.panels) >= 128 {
			return Draft{}, problem("panels.capacity", nil)
		}
		d.ID = "panel-" + uuid.NewString()
	}
	old := m.panels[d.ID]
	if !creating && old == nil {
		return Draft{}, problem("panels.not_found", nil)
	}
	if old == nil && d.Revision != 0 || old != nil && old.draft.Revision != d.Revision {
		return Draft{}, problem("panels.changed", nil)
	}
	for i := range d.Components {
		if d.Components[i].ID == "" {
			d.Components[i].ID = "c-" + uuid.NewString()
		}
	}
	d.Revision++
	next, err := makeManaged(d)
	if err != nil {
		return Draft{}, err
	}
	carryManagedState(old, next)
	all := make(map[string]*managedPanel, len(m.panels)+1)
	for id, p := range m.panels {
		all[id] = p
	}
	all[d.ID] = next
	err = m.persist(all)
	if err != nil && !durablefs.Committed(err) {
		return Draft{}, problem("panels.save_failed", err)
	}
	m.panels = all
	if err != nil {
		return d, problem("panels.save_failed", err)
	}
	return contract.Clone(d), nil
}
func (m *managedStore) delete(id string, revision uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return problem("panels.closed", nil)
	}
	old := m.panels[id]
	if old == nil {
		return problem("panels.not_found", nil)
	}
	if old.draft.Revision != revision {
		return problem("panels.changed", nil)
	}
	all := make(map[string]*managedPanel, len(m.panels))
	for key, p := range m.panels {
		if key != id {
			all[key] = p
		}
	}
	err := m.persist(all)
	if err != nil && !durablefs.Committed(err) {
		return problem("panels.save_failed", err)
	}
	m.panels = all
	if err != nil {
		return problem("panels.save_failed", err)
	}
	return nil
}
func (m *managedStore) list() []Source {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Source{}
	for _, p := range m.panels {
		out = append(out, contract.Clone(p.source))
	}
	return out
}
func (m *managedStore) draft(id string) (Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.panels[id]
	if p == nil {
		return Draft{}, problem("panels.not_found", nil)
	}
	return contract.Clone(p.draft), nil
}
func (m *managedStore) read(id string) (contract.Snapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.panels[id]
	if p == nil {
		return contract.Snapshot{}, false
	}
	return contract.Clone(p.snapshot), true
}
func (m *managedStore) dispatch(id string, e contract.Event) (contract.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.panels[id]
	if p == nil {
		return contract.Result{}, problem("panels.not_found", nil)
	}
	if m.closed || e.SessionID != p.snapshot.SessionID {
		return contract.Result{}, problem("panels.changed", nil)
	}
	if err := p.source.Definition.ValidateEvent(e); err != nil {
		return contract.Result{}, problem("panels.invalid_event", err)
	}
	if previous, ok := p.receipts[e.EventID]; ok {
		a, _ := json.Marshal(previous)
		b, _ := json.Marshal(e)
		if string(a) != string(b) {
			return contract.Result{}, problem("panels.changed", nil)
		}
		return contract.Result{EventID: e.EventID, Snapshot: contract.Clone(p.snapshot)}, nil
	}
	if e.Revision != p.snapshot.ControlRevisions[e.ComponentID] {
		return contract.Result{}, problem("panels.changed", nil)
	}
	c, _ := p.source.Definition.Component(e.ComponentID)
	if c.Field != "" {
		p.snapshot.Values[c.Field] = e.Value
	}
	p.snapshot.ControlRevisions[c.ID]++
	p.snapshot.Revision++
	p.receipts[e.EventID] = e
	p.order = append(p.order, e.EventID)
	if len(p.order) > 512 {
		delete(p.receipts, p.order[0])
		p.order = p.order[1:]
	}
	return contract.Result{EventID: e.EventID, Snapshot: contract.Clone(p.snapshot)}, nil
}
func (m *managedStore) write(ref Reference, component string, value any, appendLog bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.panels[ref.ID]
	if p == nil {
		return ErrPanelMissing
	}
	if m.closed || p.source.Generation != ref.Generation {
		return ErrPanelEnded
	}
	c, ok := p.source.Definition.Component(component)
	if !ok {
		return ErrComponentMissing
	}
	next := contract.Clone(p.snapshot)
	if appendLog {
		text, ok := value.(string)
		if !ok || c.Kind != "log" || len(text) > 16384 {
			return ErrInvalidValue
		}
		records := append(next.Records[c.ID], contract.Record{ID: uuid.NewString(), TimeMs: time.Now().UnixMilli(), Level: "info", Text: text})
		if len(records) > 200 {
			records = records[len(records)-200:]
		}
		next.Records[c.ID] = records
	} else {
		if c.Field == "" {
			return ErrComponentNoValue
		}
		if c.Kind == "select" {
			found := false
			for _, o := range c.Options {
				found = found || value == o.Value
			}
			if !found {
				return ErrInvalidValue
			}
		}
		next.Values[c.Field] = value
		if err := p.source.Definition.ValidateSnapshot(next); err != nil {
			return errors.Join(ErrInvalidValue, err)
		}
		if c.Event != "" {
			next.ControlRevisions[c.ID]++
		}
	}
	next.Revision++
	p.snapshot = next
	return nil
}
func (m *managedStore) close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.closed = true
}

func carryManagedState(old, next *managedPanel) {
	d := next.draft
	if old != nil {
		next.source.Generation = old.source.Generation
		for _, c := range d.Components {
			for _, previous := range old.draft.Components {
				if c.ID == previous.ID && c.Kind == previous.Kind && reflect.DeepEqual(c.Initial, previous.Initial) {
					candidate := contract.Clone(next.snapshot)
					if value, ok := old.snapshot.Values[c.ID]; ok {
						candidate.Values[c.ID] = value
					}
					if records, ok := old.snapshot.Records[c.ID]; ok {
						candidate.Records[c.ID] = records
					}
					if next.source.Definition.ValidateSnapshot(candidate) == nil {
						valid := true
						if c.Kind == "select" {
							valid = false
							for _, o := range c.Options {
								valid = valid || candidate.Values[c.ID] == o
							}
						}
						if valid {
							next.snapshot = candidate
						}
					}
				}
			}
		}
		next.source.UpdatedAt = old.source.UpdatedAt
		next.source.LastRunID = old.source.LastRunID
		next.source.LastRunStatus = old.source.LastRunStatus
	}
}
