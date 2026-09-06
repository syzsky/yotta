// Package panel defines the versioned, transport-neutral extension panel contract.
package panel

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
)

const Format = "yotta.panel/v1"
const Protocol = "yotta.panel-provider/v1"

var iconName = regexp.MustCompile(`^i-tabler-[a-z0-9-]+$`)
var identifier = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$`)

type Field struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}
type Option struct {
	Value    string `json:"value"`
	LabelKey string `json:"labelKey"`
}
type Component struct {
	Icon      string      `json:"icon,omitempty"`
	ID        string      `json:"id"`
	Kind      string      `json:"kind"`
	TitleKey  string      `json:"titleKey"`
	Field     string      `json:"field,omitempty"`
	UnitKey   string      `json:"unitKey,omitempty"`
	Event     string      `json:"event,omitempty"`
	Precision int         `json:"precision,omitempty"`
	Options   []Option    `json:"options,omitempty"`
	Children  []Component `json:"children,omitempty"`
}
type Definition struct {
	Format         string      `json:"format"`
	ID             string      `json:"id"`
	TitleKey       string      `json:"titleKey"`
	DescriptionKey string      `json:"descriptionKey,omitempty"`
	Fields         []Field     `json:"fields"`
	Components     []Component `json:"components"`
}

// Contribution belongs to one signed plugin and uses one declared companion.
type Contribution struct {
	Definition   Definition `json:"definition"`
	CompanionID  string     `json:"companionId"`
	SnapshotPath string     `json:"snapshotPath"`
	EventPath    string     `json:"eventPath"`
}
type Record struct {
	ID         string `json:"id"`
	TimeMs     int64  `json:"timeMs"`
	Level      string `json:"level"`
	Text       string `json:"text,omitempty"`
	MessageKey string `json:"messageKey,omitempty"`
}
type Snapshot struct {
	Protocol         string              `json:"protocol"`
	SessionID        string              `json:"sessionId"`
	Revision         uint64              `json:"revision"`
	Status           string              `json:"status"`
	Values           map[string]any      `json:"values"`
	ControlRevisions map[string]uint64   `json:"controlRevisions"`
	Records          map[string][]Record `json:"records"`
}
type Event struct {
	SessionID   string `json:"sessionId"`
	EventID     string `json:"eventId"`
	ComponentID string `json:"componentId"`
	Name        string `json:"name"`
	Revision    uint64 `json:"revision"`
	Value       any    `json:"value"`
}
type Result struct {
	EventID  string   `json:"eventId"`
	Snapshot Snapshot `json:"snapshot"`
}

func (d Definition) Validate() error {
	if d.Format != Format || !identifier.MatchString(d.ID) || d.TitleKey == "" || len(d.Components) == 0 || len(d.Fields) > 128 {
		return errors.New("invalid panel definition")
	}
	fields := map[string]string{}
	for _, f := range d.Fields {
		if !identifier.MatchString(f.ID) || fields[f.ID] != "" {
			return errors.New("invalid panel field")
		}
		switch f.Kind {
		case "string", "number", "boolean":
		default:
			return errors.New("unsupported panel field")
		}
		fields[f.ID] = f.Kind
	}
	seen := map[string]bool{}
	var visit func([]Component, int) error
	visit = func(items []Component, depth int) error {
		if depth > 4 {
			return errors.New("panel nesting too deep")
		}
		for _, c := range items {
			if c.Icon != "" && (len(c.Icon) > 128 || !iconName.MatchString(c.Icon)) {
				return errors.New("invalid component icon")
			}
			if !identifier.MatchString(c.ID) || seen[c.ID] || len(seen) >= 128 || c.TitleKey == "" {
				return errors.New("invalid component identity")
			}
			seen[c.ID] = true
			expected := ""
			switch c.Kind {
			case "group":
				if len(c.Children) == 0 {
					return errors.New("empty group")
				}
			case "text", "select", "input":
				expected = "string"
			case "number", "progress", "timer":
				expected = "number"
			case "toggle", "status":
				expected = "boolean"
			case "button", "log":
			default:
				return fmt.Errorf("unsupported component: %s", c.Kind)
			}
			if expected != "" && fields[c.Field] != expected {
				return errors.New("component field type mismatch")
			}
			if c.Kind != "group" && len(c.Children) > 0 {
				return errors.New("only groups may contain components")
			}
			interactive := c.Kind == "button" || c.Kind == "select" || c.Kind == "toggle" || c.Kind == "input"
			if interactive != (c.Event != "") || c.Event != "" && !identifier.MatchString(c.Event) {
				return errors.New("invalid component event")
			}
			if c.Precision < 0 || c.Precision > 8 {
				return errors.New("invalid precision")
			}
			if c.Kind == "select" {
				if len(c.Options) == 0 || len(c.Options) > 256 {
					return errors.New("select options required")
				}
				options := map[string]bool{}
				for _, o := range c.Options {
					if o.Value == "" || o.LabelKey == "" || options[o.Value] {
						return errors.New("invalid select option")
					}
					options[o.Value] = true
				}
			} else if len(c.Options) > 0 {
				return errors.New("options only belong to select")
			}
			if err := visit(c.Children, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(d.Components, 0)
}
func (d Definition) MessageKeys() []string {
	keys := []string{d.TitleKey, d.DescriptionKey}
	var walk func([]Component)
	walk = func(items []Component) {
		for _, c := range items {
			keys = append(keys, c.TitleKey, c.UnitKey)
			for _, o := range c.Options {
				keys = append(keys, o.LabelKey)
			}
			walk(c.Children)
		}
	}
	walk(d.Components)
	return keys
}
func (d Definition) Component(id string) (Component, bool) {
	var find func([]Component) (Component, bool)
	find = func(items []Component) (Component, bool) {
		for _, c := range items {
			if c.ID == id {
				return c, true
			}
			if v, ok := find(c.Children); ok {
				return v, true
			}
		}
		return Component{}, false
	}
	return find(d.Components)
}
func (d Definition) ValidateSnapshot(s Snapshot) error {
	if s.Protocol != Protocol || s.SessionID == "" || len(s.SessionID) > 128 || s.Revision == 0 {
		return errors.New("invalid panel session")
	}
	if s.Status != "ready" && s.Status != "waiting" && s.Status != "stale" && s.Status != "unavailable" && s.Status != "ended" {
		return errors.New("invalid panel status")
	}
	if len(s.Values) != len(d.Fields) {
		return errors.New("missing panel fields")
	}
	for _, f := range d.Fields {
		v, ok := s.Values[f.ID]
		if !ok {
			return errors.New("missing panel value")
		}
		switch f.Kind {
		case "number":
			n, ok := v.(float64)
			if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
				return errors.New("invalid panel number")
			}
		case "boolean":
			if _, ok := v.(bool); !ok {
				return errors.New("invalid panel boolean")
			}
		case "string":
			v, ok := v.(string)
			if !ok || len(v) > 16384 {
				return errors.New("invalid panel string")
			}
		}
	}
	for id, records := range s.Records {
		c, ok := d.Component(id)
		if !ok || c.Kind != "log" || len(records) > 2000 {
			return errors.New("invalid panel records")
		}
		seen := map[string]bool{}
		for _, r := range records {
			if r.ID == "" || seen[r.ID] || len(r.Text) > 16384 || r.TimeMs <= 0 {
				return errors.New("invalid panel record")
			}
			seen[r.ID] = true
		}
	}
	return nil
}
func (d Definition) ValidateEvent(e Event) error {
	c, ok := d.Component(e.ComponentID)
	if !ok || c.Event == "" || e.Name != c.Event || e.SessionID == "" || len(e.SessionID) > 128 || e.EventID == "" || len(e.EventID) > 128 {
		return errors.New("invalid panel event")
	}
	switch c.Kind {
	case "button":
		if e.Value != nil {
			return errors.New("button value must be null")
		}
	case "toggle":
		if _, ok := e.Value.(bool); !ok {
			return errors.New("toggle value must be boolean")
		}
	case "input":
		v, ok := e.Value.(string)
		if !ok || len(v) > 4096 {
			return errors.New("invalid input value")
		}
	case "select":
		v, ok := e.Value.(string)
		if !ok {
			return errors.New("invalid selection")
		}
		found := false
		for _, o := range c.Options {
			found = found || o.Value == v
		}
		if !found {
			return errors.New("unknown selection")
		}
	default:
		return errors.New("component is not interactive")
	}
	return nil
}
func Endpoint(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && !strings.ContainsAny(path, "?#\\") && !strings.Contains(path, "..")
}
func Clone[T any](v T) T {
	raw, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(raw, &out)
	return out
}
