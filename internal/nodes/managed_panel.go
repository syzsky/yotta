package nodes

import (
	"encoding/json"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodecontract"
)

const ManagedPanelPrefix = "https://schemas.yotta.dev/nodes/panels/"

var ManagedPanelKinds = []string{"use", "show", "read-text", "read-number", "read-toggle", "write-text", "write-number", "write-toggle", "log", "wait", "ref-text", "ref-number", "ref-toggle", "ref-event", "ref-log"}
var ManagedPanelIDs = func() []string {
	out := []string{}
	for _, k := range ManagedPanelKinds {
		out = append(out, ManagedPanelPrefix+k)
	}
	return out
}()

func sealPanelTypes(stringRef datatype.TypeRef) ([]datatype.Definition, map[string]datatype.TypeRef, error) {
	out := []datatype.Definition{}
	refs := map[string]datatype.TypeRef{}
	panelSchema := map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string", "minLength": 1, "maxLength": 256}, "generation": map[string]any{"type": "string", "minLength": 1, "maxLength": 256}}, "required": []string{"id", "generation"}, "additionalProperties": false}
	for _, kind := range []string{"panel", "string", "number", "boolean", "event", "log"} {
		id := "https://schemas.yotta.dev/types/panel/" + kind + "/v1"
		schemaID := id + "/schema"
		shape := map[string]any{}
		if kind == "panel" {
			for k, v := range panelSchema {
				shape[k] = v
			}
		} else {
			shape = map[string]any{"type": "object", "properties": map[string]any{"panel": panelSchema, "id": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}, "kind": map[string]any{"const": kind}}, "required": []string{"panel", "id", "kind"}, "additionalProperties": false}
		}
		shape["$id"] = schemaID
		shape["$schema"] = datatype.JSONSchemaDialect
		raw, _ := json.Marshal(shape)
		fields := []datatype.StructureField{{ID: "id", JSONKey: "id", Type: datatype.RefExpression(stringRef)}}
		if kind == "panel" {
			fields = append(fields, datatype.StructureField{ID: "generation", JSONKey: "generation", Type: datatype.RefExpression(stringRef)})
		} else {
			fields = append(fields, datatype.StructureField{ID: "panel", JSONKey: "panel", Type: datatype.RefExpression(refs["panel"])}, datatype.StructureField{ID: "kind", JSONKey: "kind", Type: datatype.RefExpression(stringRef)})
		}
		d, err := datatype.SealDefinition(datatype.DefinitionDraft{Structure: &datatype.StructureSpec{BreakNodeTypeID: "https://schemas.yotta.dev/nodes/panel-reference/break-" + kind, Fields: fields}, TypeID: id, SchemaDialect: datatype.JSONSchemaDialect, SchemaRoot: schemaID, SchemaBundle: []datatype.SchemaResource{{ID: schemaID, Schema: raw}}, Representations: []datatype.RepresentationSpec{{Kind: datatype.RepresentationInlineJSON, Codec: datatype.CodecJCSV1}}, Traits: []datatype.Trait{datatype.TraitEquatable, datatype.TraitObservable}, Authoring: datatype.Authoring{BreakTitleKey: "type.panel." + kind + ".break_title", BreakDescriptionKey: "type.panel." + kind + ".description", TitleKey: "type.panel." + kind + ".title", DescriptionKey: "type.panel." + kind + ".description", Color: "#14b8a6", Icon: "layout-dashboard"}})
		if err != nil {
			return nil, nil, err
		}
		out = append(out, d)
		refs[kind] = d.TypeRef()
	}
	return out, refs, nil
}
func defineManagedPanelNodes(values, refs map[string]datatype.TypeRef) ([]BuiltinDefinition, error) {
	out := []BuiltinDefinition{}
	for _, kind := range ManagedPanelKinds {
		id := ManagedPanelPrefix + kind
		schemaID := id + "/config"
		prefix := "node.managed_panel." + kind
		properties := map[string]any{"panel": map[string]any{"type": "string", "maxLength": 256, "x-yotta-title-key": "node.managed_panel.config.panel", "x-yotta-editor-adapter": "panel-reference"}}
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
		if componentKind != "" {
			properties["component"] = map[string]any{"type": "string", "maxLength": 128, "x-yotta-title-key": "node.managed_panel.config.component", "x-yotta-editor-adapter": "panel-component-" + componentKind}
		}
		if kind == "use" {
			properties["show"] = map[string]any{"type": "boolean", "default": true, "x-yotta-title-key": "node.managed_panel.config.show"}
		}
		if kind == "wait" {
			properties["timeoutMs"] = map[string]any{"type": "integer", "minimum": 1, "maximum": 86400000, "default": 30000, "x-yotta-title-key": "node.panel.config.timeout"}
		}
		raw, _ := json.Marshal(map[string]any{"$id": schemaID, "$schema": datatype.JSONSchemaDialect, "type": "object", "properties": properties, "additionalProperties": false})
		inputs := []nodecontract.DataInputPort{{ID: "panel-ref", Type: datatype.RefExpression(refs["panel"]), Required: false}}
		outputs := []nodecontract.DataOutputPort{}
		if componentKind != "" {
			inputs = append(inputs, nodecontract.DataInputPort{ID: "component-ref", Type: datatype.RefExpression(refs[componentKind]), Required: false})
		}
		switch kind {
		case "use", "show":
			outputs = append(outputs, nodecontract.DataOutputPort{ID: "panel", Type: datatype.RefExpression(refs["panel"])})
		case "read-text", "read-number", "read-toggle":
			outputs = append(outputs, nodecontract.DataOutputPort{ID: "value", Type: datatype.RefExpression(values[componentKind])})
		case "write-text", "write-number", "write-toggle":
			inputs = append(inputs, nodecontract.DataInputPort{ID: "value", Type: datatype.RefExpression(values[componentKind]), Required: true})
		case "log":
			inputs = append(inputs, nodecontract.DataInputPort{ID: "value", Type: datatype.RefExpression(values["string"]), Required: true})
		case "wait":
			outputs = append(outputs, nodecontract.DataOutputPort{ID: "value", Type: datatype.RefExpression(values["json"])}, nodecontract.DataOutputPort{ID: "component", Type: datatype.RefExpression(values["string"])})
		default:
			outputs = append(outputs, nodecontract.DataOutputPort{ID: "reference", Type: datatype.RefExpression(refs[componentKind])})
		}
		hints := dataPortHints(prefix, inputs, outputs, nil)
		for i := range hints {
			if hints[i].ID == "panel-ref" || hints[i].ID == "component-ref" {
				hints[i].Group = "advanced"
			}
		}
		c, err := nodecontract.Seal(nodecontract.Draft{NodeTypeID: id, Version: "1.0.0", ConfigSchemaRoot: schemaID, ConfigSchemaBundle: []datatype.SchemaResource{{ID: schemaID, Schema: raw}}, Ports: nodecontract.PortSet{DataInputs: inputs, DataOutputs: outputs, ExecInputs: signalList("in"), ExecOutputs: signalList("completed"), ErrorOutputs: signalList("failed")}, Execution: nodecontract.ExecutionSpec{Class: nodecontract.ExecutionEffect, Effects: []nodecontract.EffectID{nodecontract.EffectID(PanelEffect(kind))}, Determinism: nodecontract.Recorded, Evaluation: nodecontract.EvaluationPush, Cache: nodecontract.CacheNone, Retry: nodecontract.RetryNever, Cancellation: nodecontract.CancellationCooperative, Timeout: nodecontract.TimeoutNone}, Instruction: nodecontract.Invoke(), Errors: panelErrors(), ConfiguredTargets: []nodecontract.ConfiguredTargetSpec{{ID: "panel", TargetSlot: "panel", SlotConfigKey: "panel", TargetKinds: []string{"panel"}}}, ImplementationABI: []nodecontract.ABIRequirement{{Kind: nodecontract.ABIBuiltin, Version: "v1"}}, Authoring: nodecontract.Authoring{TitleKey: prefix + ".title", DescriptionKey: prefix + ".description", Category: "panel", Icon: "layout-dashboard", Tags: []string{"panel", "面板"}, Ports: hints}})
		if err != nil {
			return nil, err
		}
		d, err := defineBuiltin(c, "panels."+kind, "v1", "managed-panel-"+kind+"/v1", nil)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
