package nodes

import (
	"encoding/json"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodecontract"
)

const PanelNodePrefix = "https://schemas.yotta.dev/nodes/panel/"

var PanelKinds = []string{"create", "end", "number", "text", "status", "select", "toggle", "input", "button", "log", "read-text", "read-number", "read-toggle", "write-text", "write-number", "write-toggle", "wait"}
var PanelNodeIDs = func() []string {
	out := []string{}
	for _, kind := range PanelKinds {
		out = append(out, PanelNodePrefix+kind)
	}
	return out
}()

func PanelEffect(kind string) string {
	return "https://schemas.yotta.dev/effects/panel/" + kind + "/v1"
}

func definePanelNodes(refs map[string]datatype.TypeRef) ([]BuiltinDefinition, error) {
	out := []BuiltinDefinition{}
	for _, kind := range PanelKinds {
		id := PanelNodePrefix + kind
		schemaID := id + "/config"
		text := func(defaultValue, title string) map[string]any {
			return map[string]any{"type": "string", "default": defaultValue, "maxLength": 128, "x-yotta-title-key": title}
		}
		properties := map[string]any{"panel": text("main", "node.panel.config.panel"), "component": text("value", "node.panel.config.component"), "title": text("值", "node.panel.config.title")}
		properties["panel"].(map[string]any)["pattern"] = `^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$`
		properties["component"].(map[string]any)["pattern"] = `^[a-zA-Z][a-zA-Z0-9_.-]{0,99}$`
		if kind == "read-text" || kind == "read-number" || kind == "read-toggle" || kind == "write-text" || kind == "write-number" || kind == "write-toggle" {
			delete(properties, "title")
		}
		if kind == "end" {
			delete(properties, "component")
			delete(properties, "title")
		}
		if kind == "create" {
			delete(properties, "component")
			properties["title"] = text("运行信息", "node.panel.config.title")
		}
		if kind == "select" {
			properties["choices"] = map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}, "minItems": 1, "maxItems": 128, "uniqueItems": true, "default": []string{"继续", "停止"}, "x-yotta-title-key": "node.panel.config.choices", "x-yotta-editor-adapter": "panel-choices"}
		}
		if kind == "wait" {
			delete(properties, "title")
			properties["component"] = text("", "node.panel.config.event_component")
			properties["timeoutMs"] = map[string]any{"type": "integer", "minimum": 1, "maximum": 86400000, "default": 30000, "x-yotta-title-key": "node.panel.config.timeout"}
		}
		raw, _ := json.Marshal(map[string]any{"$id": schemaID, "$schema": datatype.JSONSchemaDialect, "type": "object", "properties": properties, "additionalProperties": false})
		inputs := []nodecontract.DataInputPort{}
		outputs := []nodecontract.DataOutputPort{}
		inputKind := ""
		switch kind {
		case "number", "write-number":
			inputKind = "number"
		case "text", "select", "input", "log", "write-text":
			inputKind = "string"
		case "status", "toggle", "write-toggle":
			inputKind = "boolean"
		}
		if inputKind != "" {
			value := json.RawMessage(`""`)
			if inputKind == "number" {
				value = json.RawMessage(`0`)
			}
			if inputKind == "boolean" {
				value = json.RawMessage(`false`)
			}
			write := kind == "write-text" || kind == "write-number" || kind == "write-toggle"
			initial := &value
			if write {
				initial = nil
			}
			inputs = append(inputs, nodecontract.DataInputPort{ID: "value", Type: datatype.RefExpression(refs[inputKind]), Required: write, Default: initial})
		}
		outputKind := ""
		switch kind {
		case "read-text":
			outputKind = "string"
		case "read-number":
			outputKind = "number"
		case "read-toggle":
			outputKind = "boolean"
		case "wait":
			outputKind = "json"
		}
		if outputKind != "" {
			outputs = append(outputs, nodecontract.DataOutputPort{ID: "value", Type: datatype.RefExpression(refs[outputKind])})
		}
		if kind == "wait" {
			outputs = append(outputs, nodecontract.DataOutputPort{ID: "component", Type: datatype.RefExpression(refs["string"])}, nodecontract.DataOutputPort{ID: "event", Type: datatype.RefExpression(refs["string"])})
		}
		prefix := "node.panel." + kind
		c, err := nodecontract.Seal(nodecontract.Draft{NodeTypeID: id, Version: "1.2.0", ConfigSchemaRoot: schemaID, ConfigSchemaBundle: []datatype.SchemaResource{{ID: schemaID, Schema: raw}},
			Ports:       nodecontract.PortSet{DataInputs: inputs, DataOutputs: outputs, ExecInputs: signalList("in"), ExecOutputs: signalList("completed"), ErrorOutputs: signalList("failed")},
			Execution:   nodecontract.ExecutionSpec{Class: nodecontract.ExecutionEffect, Effects: []nodecontract.EffectID{nodecontract.EffectID(PanelEffect(kind))}, Determinism: nodecontract.Recorded, Evaluation: nodecontract.EvaluationPush, Cache: nodecontract.CacheNone, Retry: nodecontract.RetryNever, Cancellation: nodecontract.CancellationCooperative, Timeout: nodecontract.TimeoutNone},
			Instruction: nodecontract.Invoke(), Errors: panelErrors(), ImplementationABI: []nodecontract.ABIRequirement{{Kind: nodecontract.ABIBuiltin, Version: "v1"}},
			Authoring: nodecontract.Authoring{TitleKey: prefix + ".title", DescriptionKey: prefix + ".description", Category: "panel-legacy", Icon: "layout-dashboard", Tags: []string{"panel", "面板", "交互"}, Ports: dataPortHints(prefix, inputs, outputs, nil)},
		})
		if err != nil {
			return nil, err
		}
		d, err := defineBuiltin(c, "panel."+kind, "v1", "run-scoped-panel-"+kind+"/v1", nil)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// panelErrors declares recoverable causes as durable node evidence.
func panelErrors() []nodecontract.ErrorSpec {
	out := []nodecontract.ErrorSpec{{Code: "panels.workflow_creation_retired", Category: "panels"}, {Code: "panels.node_failed", Category: "panels"}, {Code: "panels.wait_timeout", Category: "panels", RetryHint: true}}
	for _, reason := range []string{"panel_missing", "component_missing", "component_conflict", "component_read_only", "component_no_value", "component_not_interactive", "invalid_value", "invalid_definition", "panel_ended", "capacity", "unavailable"} {
		out = append(out, nodecontract.ErrorSpec{Code: "panels." + reason, Category: "panels", Params: []nodecontract.ProblemParamSpec{{Name: "panel", Type: nodecontract.ProblemParamString, Required: true}, {Name: "component", Type: nodecontract.ProblemParamString, Required: true}}})
	}
	return out
}
