package noderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/panel"
	"github.com/yottaapp/yotta/internal/problem"
	"strings"
	"time"
)

func managedPanelAdapter(builtins nodes.Builtins, service *panel.Service, id string) nodeadapter.Adapter {
	kind := strings.TrimPrefix(id, nodes.ManagedPanelPrefix)
	return func(ctx context.Context, inv nodeadapter.Invocation) (_ nodeadapter.AdapterResult, runErr error) {
		defer func() {
			runErr = errors.Join(runErr, recordAdapterOutcome(ctx, inv, nodeadapter.AdapterAction{EffectID: nodes.PanelEffect(kind), Action: "panels." + kind, SummaryCode: "panels." + kind}, "panels.node_failed", runErr))
		}()
		selected, _ := inv.Config["panel"].(string)
		component, _ := inv.Config["component"].(string)
		fail := func(err error) (nodeadapter.AdapterResult, error) {
			if errors.Is(err, context.Canceled) {
				return nodeadapter.AdapterResult{}, err
			}
			f := &nodeadapter.NodeFailure{Code: "panels.node_failed", Output: "failed", Cause: err}
			var reason panel.RunError
			if errors.As(err, &reason) {
				f.Code = "panels." + string(reason)
				f.Params = problem.Must(map[string]any{"panel": selected, "component": component})
			}
			return nodeadapter.AdapterResult{}, f
		}
		if service == nil {
			return fail(panel.ErrPanelUnavailable)
		}
		var ref panel.Reference
		var err error
		if input, ok := inv.Inputs["panel-ref"]; ok {
			err = json.Unmarshal(input.InlineJSON(), &ref)
			if err == nil {
				var current panel.Reference
				current, err = service.Resolve(ref.ID)
				if err == nil && current.Generation != ref.Generation {
					err = panel.ErrPanelEnded
				}
			}
		} else if _, wired := inv.Inputs["component-ref"]; !wired {
			ref, err = service.Resolve(selected)
		}
		if err != nil {
			return fail(err)
		}
		var cr panel.ComponentReference
		componentKind := ""
		switch kind {
		case "read-text", "write-text", "ref-text":
			componentKind = "string"
		case "read-number", "write-number", "ref-number":
			componentKind = "number"
		case "read-toggle", "write-toggle", "ref-toggle":
			componentKind = "boolean"
		case "log", "ref-log":
			componentKind = "log"
		case "wait", "ref-event":
			componentKind = "event"
		}
		if input, ok := inv.Inputs["component-ref"]; ok {
			if err = json.Unmarshal(input.InlineJSON(), &cr); err != nil {
				return fail(err)
			}
			ref = cr.Panel
			component = cr.ID
			if cr.Kind != componentKind {
				return fail(panel.ErrInvalidValue)
			}
		}
		selected = ref.ID
		if componentKind != "" && !(kind == "wait" && component == "") {
			cr, err = service.ResolveComponent(ref, component, componentKind)
			if err != nil {
				return fail(err)
			}
		}
		result := nodeadapter.AdapterResult{ExecOutputs: []string{"completed"}, Outputs: map[string]datatype.ValueEnvelope{}}
		output := func(port string, v any) error {
			sealed, e := sealStateOutput(builtins, inv, port, v)
			if e == nil {
				result.Outputs[port] = sealed
			}
			return e
		}
		switch {
		case kind == "use" || kind == "show":
			show := true
			if configured, ok := inv.Config["show"].(bool); ok {
				show = configured
			}
			if kind == "show" || show {
				if err = service.Show(ref.ID); err != nil {
					return fail(panel.ErrPanelUnavailable)
				}
			}
			err = output("panel", ref)
		case strings.HasPrefix(kind, "ref-"):
			err = output("reference", cr)
		case strings.HasPrefix(kind, "read-"):
			var v any
			v, err = service.Value(cr)
			if err == nil {
				err = output("value", v)
			}
		case kind == "wait":
			duration := int64(30000)
			if v, ok := inv.Config["timeoutMs"]; ok {
				raw, _ := json.Marshal(v)
				if err = json.Unmarshal(raw, &duration); err != nil {
					return fail(panel.ErrInvalidValue)
				}
			}
			if duration < 1 || duration > 86400000 {
				return fail(panel.ErrInvalidValue)
			}
			wait, cancel := context.WithTimeout(ctx, time.Duration(duration)*time.Millisecond)
			defer cancel()
			var e panel.Interaction
			e, err = service.Wait(wait, ref, component)
			if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
				return nodeadapter.AdapterResult{}, &nodeadapter.NodeFailure{Code: "panels.wait_timeout", Output: "failed", Cause: errors.New("panel interaction wait elapsed")}
			}
			if err == nil {
				if err = output("value", e.Value); err == nil {
					err = output("component", e.ComponentID)
				}
			}
		default:
			var v any
			input, ok := inv.Inputs["value"]
			if !ok {
				return fail(panel.ErrInvalidValue)
			}
			if err = json.Unmarshal(input.InlineJSON(), &v); err == nil {
				err = service.Write(cr, v)
				if err == nil {
					service.MarkUpdated(ref.ID, inv.RunID)
				}
			}
		}
		if err != nil {
			return fail(err)
		}
		return result, nil
	}
}
