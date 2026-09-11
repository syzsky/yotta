package nodes

import (
	"encoding/json"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodecontract"
)

const SignalPrefix = "https://schemas.yotta.dev/nodes/signals/"

var SignalKinds = []string{"send", "wait", "listen"}

func subscriptionInstruction() nodecontract.InstructionSpec {
	v := nodecontract.Invoke()
	v.Invoke.Subscription = &nodecontract.SubscriptionInstruction{StopInput: "stop", EventOutput: "event", MainOutput: "main", CompletedOutput: "completed"}
	return v
}
func defineSignalNodes(refs map[string]datatype.TypeRef) ([]BuiltinDefinition, error) {
	out := []BuiltinDefinition{}
	for _, kind := range SignalKinds {
		id := SignalPrefix + kind
		prefix := "node.signal." + kind
		schemaID := id + "/config"
		properties := map[string]any{}
		if kind == "wait" {
			properties["timeoutMs"] = map[string]any{"type": "integer", "minimum": 0, "maximum": 86400000, "default": 0, "x-yotta-title-key": "node.signal.timeout"}
		}
		raw, _ := json.Marshal(map[string]any{"$id": schemaID, "$schema": datatype.JSONSchemaDialect, "type": "object", "properties": properties, "additionalProperties": false})
		nameDefault, valueDefault := json.RawMessage(`"continue"`), json.RawMessage(`{}`)
		inputs := []nodecontract.DataInputPort{{ID: "name", Type: datatype.RefExpression(refs["string"]), Required: true, Default: &nameDefault}}
		outputs := []nodecontract.DataOutputPort{}
		if kind == "send" {
			inputs = append(inputs, nodecontract.DataInputPort{ID: "value", Type: datatype.RefExpression(refs["json"]), Required: true, Default: &valueDefault})
		} else {
			outputs = append(outputs, nodecontract.DataOutputPort{ID: "value", Type: datatype.RefExpression(refs["json"])}, nodecontract.DataOutputPort{ID: "event-id", Type: datatype.RefExpression(refs["string"])})
		}
		instruction := nodecontract.Invoke()
		execInputs, execOutputs := signalList("in"), signalList("completed")
		if kind == "listen" {
			instruction = subscriptionInstruction()
			execInputs = signalList("in", "stop")
			execOutputs = signalList("event", "main", "completed")
		}
		effect := "https://schemas.yotta.dev/effects/signals/" + kind + "/v1"
		c, err := nodecontract.Seal(nodecontract.Draft{NodeTypeID: id, Version: "1.0.0", ConfigSchemaRoot: schemaID, ConfigSchemaBundle: []datatype.SchemaResource{{ID: schemaID, Schema: raw}}, Ports: nodecontract.PortSet{DataInputs: inputs, DataOutputs: outputs, ExecInputs: execInputs, ExecOutputs: execOutputs, ErrorOutputs: signalList("failed")}, Execution: effectExecution(nodecontract.EffectID(effect)), Instruction: instruction, Errors: []nodecontract.ErrorSpec{{Code: "signals.failed", Category: "signals"}, {Code: "signals.queue_full", Category: "signals"}, {Code: "signals.wait_timeout", Category: "signals", RetryHint: true}}, ImplementationABI: []nodecontract.ABIRequirement{{Kind: nodecontract.ABIBuiltin, Version: "v1"}}, Authoring: nodecontract.Authoring{TitleKey: prefix + ".title", DescriptionKey: prefix + ".description", Category: "event", Icon: "broadcast", Tags: []string{"signal", "event", "信号"}, Ports: dataPortHints(prefix, inputs, outputs, nil)}})
		if err != nil {
			return nil, err
		}
		d, err := defineBuiltin(c, "signals."+kind, "v1", "signals-"+kind+"/v1", nil)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
