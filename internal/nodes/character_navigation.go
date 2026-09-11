package nodes

import (
	"encoding/json"
	"fmt"

	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/capability"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodecontract"
)

const (
	TurnViewNodeID           = "https://schemas.yotta.dev/nodes/automation/turn-view"
	MoveCharacterNodeID      = "https://schemas.yotta.dev/nodes/automation/move-character-to"
	TurnFindTemplateNodeID   = "https://schemas.yotta.dev/nodes/automation/turn-find-template"
	NavigationFailedCode     = "automation.navigation_failed"
	NavigationWaitingStatus  = "automation.navigation.waiting"
	NavigationTimeoutStatus  = "automation.navigation.timeout"
	NavigationFinishedStatus = "automation.navigation.finished"
)

func NavigationEffect(id string) string {
	switch id {
	case TurnViewNodeID:
		return "https://schemas.yotta.dev/effects/automation/turn-view/v1"
	case MoveCharacterNodeID:
		return "https://schemas.yotta.dev/effects/automation/move-character-to/v1"
	default:
		return "https://schemas.yotta.dev/effects/automation/turn-find-template/v1"
	}
}

func defineNavigationNodes(t automationTemplateTypes, blobRead capability.Definition, positionRef datatype.TypeRef) ([]BuiltinDefinition, error) {
	num, dur := datatype.RefExpression(t.numberRef), datatype.RefExpression(t.durationRef)
	input := func(id string, typ datatype.TypeExpression, value string) nodecontract.DataInputPort {
		return nodecontract.DataInputPort{ID: id, Type: typ, Required: true, Default: rawDefault(value)}
	}
	var definitions []BuiltinDefinition
	for _, id := range []string{TurnViewNodeID, MoveCharacterNodeID, TurnFindTemplateNodeID} {
		name, key := "turn-view", "node.navigation.turn"
		inputs := []nodecontract.DataInputPort{input("angle", num, "15"), input("duration", dur, "150")}
		outputs := []nodecontract.DataOutputPort{}
		exits := signalList("completed")
		props := map[string]any{"slot": map[string]any{"type": "string", "minLength": 1, "maxLength": 128, "pattern": "^[a-z][a-z0-9]*(?:[-_][a-z0-9]+)*$", "x-yotta-title-key": "node.automation.config.slot.title"}}
		required := []string{"slot"}
		targets := automationTargetSpec("input-target", installed.TargetKindDesktopWindow)
		var caps []capability.Requirement
		var stateAccesses []nodecontract.StateAccessSpec
		version := BuiltinNodeVersion
		implementationVersion := "v1"
		if id == MoveCharacterNodeID {
			name, key = "move-character-to", "node.navigation.move"
			inputs = []nodecontract.DataInputPort{input("target-x", num, "0"), input("target-y", num, "0"), input("tolerance", num, "20"), input("timeout", dur, "30000"), input("interval", dur, "100"), input("slow-distance", num, "100")}
			outputs = []nodecontract.DataOutputPort{{ID: "x", Type: num}, {ID: "y", Type: num}, {ID: "distance", Type: num}}
			exits = signalList("arrived", "timeout", "stuck", "unavailable")

			version = "2.0.0"
			implementationVersion = "v2"
			props["position-variable"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 128, "x-yotta-control": "state-variable", "x-yotta-title-key": "node.navigation.config.positionVariable"}
			required = append(required, "position-variable")
			props["forwardKey"] = map[string]any{"type": "string", "default": "W", "x-yotta-title-key": "node.navigation.config.forwardKey"}
			props["turnSign"] = map[string]any{"type": "integer", "enum": []int{-1, 1}, "default": 1, "x-yotta-title-key": "node.navigation.config.turnSign"}
			stateAccesses = []nodecontract.StateAccessSpec{{ID: "position", SlotConfigKey: "position-variable", Type: datatype.RefExpression(positionRef), Mode: nodecontract.StateRead}}

		}
		if id == TurnFindTemplateNodeID {
			name, key = "turn-find-template", "node.navigation.search"
			inputs = []nodecontract.DataInputPort{
				{ID: "template", Type: datatype.RefExpression(t.imageRef), Required: true},
				input("region", datatype.RefExpression(t.regionRef), `{"x":0,"y":0,"width":1,"height":1,"unit":"ratio"}`),
				input("threshold", num, "0.85"), input("step", num, "10"), input("max-angle", num, "360"), input("settle", dur, "250"), input("timeout", dur, "30000"),
			}
			outputs = []nodecontract.DataOutputPort{{ID: "matched", Type: datatype.RefExpression(t.booleanRef)}, {ID: "score", Type: num}, {ID: "center", Type: datatype.RefExpression(t.pointRef)}, {ID: "bounds", Type: datatype.RefExpression(t.regionRef)}, {ID: "angle", Type: num}}
			exits = signalList("found", "not-found", "timeout")
			targets = append(targets, automationTargetSpec("capture-target", installed.TargetKindDesktopWindow)...)
			caps = []capability.Requirement{requirement(blobRead, "blob-read", []string{"read-range"}, "blob-store")}
		}
		schemaID := id + "/config"
		schema, err := json.Marshal(map[string]any{"$id": schemaID, "$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": props, "required": required, "additionalProperties": false})
		if err != nil {
			return nil, err
		}
		errors := automationTemplateErrors(true)
		errors = append(errors, nodecontract.ErrorSpec{Code: NavigationFailedCode, Category: "automation"})
		contract, err := nodecontract.Seal(nodecontract.Draft{Version: version, NodeTypeID: id, ConfigSchemaRoot: schemaID, ConfigSchemaBundle: []datatype.SchemaResource{{ID: schemaID, Schema: schema}},
			Ports:     nodecontract.PortSet{DataInputs: inputs, DataOutputs: outputs, ExecInputs: signalList("in"), ExecOutputs: exits, ErrorOutputs: signalList("failed")},
			Execution: automationEffectExecution(NavigationEffect(id)), Instruction: nodecontract.Invoke(), ConfiguredTargets: targets, StateAccesses: stateAccesses, CapabilityRequirements: caps, Errors: errors,
			StatusEvents:      []nodecontract.StatusEventSpec{{Code: NavigationWaitingStatus, Category: nodecontract.StatusWaiting}, {Code: NavigationTimeoutStatus, Category: nodecontract.StatusProgress}, {Code: NavigationFinishedStatus, Category: nodecontract.StatusProgress}},
			ImplementationABI: []nodecontract.ABIRequirement{{Kind: nodecontract.ABIBuiltin, Version: "v1"}},
			Authoring:         nodecontract.Authoring{TitleKey: key + ".title", DescriptionKey: key + ".description", Category: "automation", Tags: []string{"game", "navigation", "character", "turn"}, Icon: "route", Ports: dataPortHints(key, inputs, outputs, map[string]string{"template": "template-image"})},
		})
		if err != nil {
			return nil, fmt.Errorf("navigation %s: %w", name, err)
		}
		def, err := defineBuiltin(contract, "automation."+name, implementationVersion, "configured-target/closed-loop-navigation/v1", nil)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, def)
	}
	return definitions, nil
}
