package workflowbundle

import (
	"context"
	"sort"
	"strings"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/panel"
	"github.com/yottaapp/yotta/internal/workflow/schema"
)

const panelNodePrefix = "https://schemas.yotta.dev/nodes/panels/"

func panelProblem(id string) error { return apperr.New("panels."+id, nil) }

func panelSelections(source schema.WorkflowSource) []string {
	ids := map[string]bool{}
	for _, d := range source.TargetDefaults {
		if d.Target == "panel" && d.Slot != "" {
			ids[d.Slot] = true
		}
	}
	for _, g := range source.Graphs {
		for _, n := range g.Nodes {
			if strings.HasPrefix(n.NodeRef.NodeTypeID, panelNodePrefix) {
				if id, _ := n.Config["panel"].(string); id != "" {
					ids[id] = true
				}
			}
		}
	}
	out := []string{}
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (m *Manager) collectPanels(source schema.WorkflowSource) ([]panel.PortableResource, error) {
	out := []panel.PortableResource{}
	for _, id := range panelSelections(source) {
		if m.panels == nil {
			return nil, panelProblem("portable_missing")
		}
		resource, err := m.panels.ExportResource(id)
		if err != nil {
			return nil, err
		}
		// Keep the exact source identity; legacy plugin spelling is normalized in the source separately.
		if resource.ID != id {
			return nil, panelProblem("portable_missing")
		}
		out = append(out, resource)
	}
	return out, validatePanelReferences(source, out)
}

func validatePanelReferences(source schema.WorkflowSource, resources []panel.PortableResource) error {
	index := map[string]panel.PortableResource{}
	for _, r := range resources {
		index[r.ID] = r
	}
	for _, id := range panelSelections(source) {
		if _, ok := index[id]; !ok {
			return panelProblem("portable_missing")
		}
	}
	if len(index) != len(panelSelections(source)) {
		return panelProblem("invalid_definition")
	}
	defaultID, _ := schema.TargetDefaultSlot(source, "panel")
	for _, g := range source.Graphs {
		for _, n := range g.Nodes {
			if !strings.HasPrefix(n.NodeRef.NodeTypeID, panelNodePrefix) {
				continue
			}
			if _, wired := n.Bindings["panel-ref"]; wired {
				continue
			}
			if _, wired := n.Bindings["component-ref"]; wired {
				continue
			}
			id, _ := n.Config["panel"].(string)
			if id == "" {
				id = defaultID
			}
			// Data-edge references are supplied by another panel node, not a local selection.
			wired := false
			for _, edge := range g.Edges {
				if edge.To.NodeID == n.ID && (edge.To.PortID == "panel-ref" || edge.To.PortID == "component-ref") {
					wired = true
				}
			}
			if wired {
				continue
			}
			r, ok := index[id]
			if !ok {
				return panelProblem("portable_missing")
			}
			component, _ := n.Config["component"].(string)
			if component == "" {
				continue
			}
			kind := ""
			suffix := strings.TrimPrefix(n.NodeRef.NodeTypeID, panelNodePrefix)
			switch {
			case strings.HasSuffix(suffix, "text"):
				kind = "string"
			case strings.HasSuffix(suffix, "number"):
				kind = "number"
			case strings.HasSuffix(suffix, "toggle"):
				kind = "boolean"
			case suffix == "wait" || suffix == "ref-event":
				kind = "event"
			case suffix == "log" || suffix == "ref-log":
				kind = "log"
			}
			if !panel.PortableComponentMatches(r, component, kind) {
				return panelProblem("portable_component_missing")
			}
		}
	}
	return nil
}

func rewritePanels(source *schema.WorkflowSource, mapping map[string]string) {
	for i := range source.TargetDefaults {
		d := &source.TargetDefaults[i]
		if d.Target == "panel" {
			if id, ok := mapping[d.Slot]; ok {
				d.Slot = id
			}
		}
	}
	for gi := range source.Graphs {
		for ni := range source.Graphs[gi].Nodes {
			n := &source.Graphs[gi].Nodes[ni]
			if strings.HasPrefix(n.NodeRef.NodeTypeID, panelNodePrefix) {
				if old, ok := n.Config["panel"].(string); ok {
					if id, ok := mapping[old]; ok {
						n.Config["panel"] = id
					}
				}
			}
		}
	}
}

// Preview uses exactly the collection and validation rules used for export.
func (m *Manager) Preview(ctx context.Context, workflowID string) (Info, error) {
	p, err := m.prepareExport(ctx, workflowID, nil)
	return p.info, err
}
