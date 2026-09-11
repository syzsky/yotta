package nodes

import (
	"encoding/json"
	"fmt"

	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodecontract"
)

const WorldPositionTypeID = "https://schemas.yotta.dev/types/navigation/world-position/v1"
const ParseWorldPositionNodeID = "https://schemas.yotta.dev/nodes/navigation/parse-position"
const MakeWorldPositionNodeID = "https://schemas.yotta.dev/nodes/navigation/make-position"
const PositionObservedEffectID = "https://schemas.yotta.dev/effects/navigation/observe-position/v1"

func sealWorldPositionType(t primitiveTypes) (datatype.Definition, error) {
	fields := []datatype.StructureField{}
	for _, key := range []string{"x", "y", "heading", "axisHeading", "axisSign", "frame", "unit", "valid", "receivedAt", "sampleAt", "sequence", "epoch"} {
		ref := t.numberRef
		switch key {
		case "frame", "unit", "epoch":
			ref = t.stringRef
		case "valid":
			ref = t.booleanRef
		case "axisSign", "receivedAt", "sampleAt", "sequence":
			ref = t.integerRef
		}
		id := map[string]string{"axisHeading": "axis-heading", "axisSign": "axis-sign", "receivedAt": "received-at", "sampleAt": "sample-at"}[key]
		if id == "" {
			id = key
		}
		fields = append(fields, datatype.StructureField{ID: id, JSONKey: key, Type: datatype.RefExpression(ref)})
	}
	return sealStructuredTypeWithStructure(WorldPositionTypeID, json.RawMessage(fmt.Sprintf(`{
	"$id":%q,"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,
	"properties":{
	"x":{"type":"number"},"y":{"type":"number"},"heading":{"type":"number"},
	"frame":{"type":"string","minLength":1,"maxLength":128},"unit":{"type":"string","minLength":1,"maxLength":32},
	"axisHeading":{"type":"number"},"axisSign":{"type":"integer","enum":[-1,1]},"valid":{"type":"boolean"},
	"receivedAt":{"type":"integer","minimum":0},"sampleAt":{"type":"integer","minimum":0},
	"sequence":{"type":"integer","minimum":0},"epoch":{"type":"string","maxLength":128}},
	"required":["x","y","heading","frame","unit","axisHeading","axisSign","valid","receivedAt","sampleAt","sequence","epoch"]
	}`, WorldPositionTypeID+"/schema")), datatype.Authoring{TitleKey: "type.navigation.worldPosition.title", DescriptionKey: "type.navigation.worldPosition.description", Color: "#22d3ee", Icon: "map-pin", BreakTitleKey: "node.navigation.breakPosition.title", BreakDescriptionKey: "node.navigation.breakPosition.description", Examples: []json.RawMessage{json.RawMessage(`{"x":0,"y":0,"heading":0,"frame":"world","unit":"world","axisHeading":0,"axisSign":1,"valid":false,"receivedAt":0,"sampleAt":0,"sequence":0,"epoch":""}`)}}, &datatype.StructureSpec{BreakNodeTypeID: "https://schemas.yotta.dev/nodes/navigation/break-position", Fields: fields})
}

func defineWorldPositionNode(t primitiveTypes, pose datatype.TypeRef, parse bool) (BuiltinDefinition, error) {
	id, key, entrypoint := MakeWorldPositionNodeID, "node.navigation.position", "navigation.make-position"
	if parse {
		id, key, entrypoint = ParseWorldPositionNodeID, "node.navigation.parsePosition", "navigation.parse-position"
	}
	number := datatype.RefExpression(t.numberRef)
	inputs := []nodecontract.DataInputPort{}
	for _, key := range []string{"x", "y", "heading"} {
		inputs = append(inputs, nodecontract.DataInputPort{ID: key, Type: number, Required: true, Default: rawDefault("0")})
	}
	inputs = append(inputs, nodecontract.DataInputPort{ID: "valid", Type: datatype.RefExpression(t.booleanRef), Required: true, Default: rawDefault("true")})
	for _, key := range []string{"sample-at", "sequence"} {
		inputs = append(inputs, nodecontract.DataInputPort{ID: key, Type: datatype.RefExpression(t.integerRef), Required: true, Default: rawDefault("0")})
	}
	outputs := []nodecontract.DataOutputPort{{ID: "position", Type: datatype.RefExpression(pose)}}

	if parse {
		inputs = []nodecontract.DataInputPort{{ID: "source", Type: datatype.RefExpression(t.stringRef), Required: true}}
	}
	props := map[string]any{
		"frame":       map[string]any{"type": "string", "minLength": 1, "maxLength": 128, "default": "world"},
		"unit":        map[string]any{"type": "string", "minLength": 1, "maxLength": 32, "default": "world"},
		"axisHeading": map[string]any{"type": "number", "default": 0},
		"axisSign":    map[string]any{"type": "integer", "enum": []int{-1, 1}, "default": 1},
		"epoch":       map[string]any{"type": "string", "maxLength": 128, "default": ""},
	}
	if parse {
		for field, value := range map[string]string{"xField": "x", "yField": "y", "headingField": "cameraHeading", "validField": "valid", "timeField": "sampleTimeMs"} {
			props[field] = map[string]any{"type": "string", "minLength": 1, "maxLength": 128, "default": value}
		}
	}
	for field, prop := range props {
		prop.(map[string]any)["x-yotta-title-key"] = "node.navigation.config." + field
	}
	schema, err := json.Marshal(map[string]any{"$id": id + "/config", "$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "additionalProperties": false, "properties": props, "required": []string{}})
	if err != nil {
		return BuiltinDefinition{}, err
	}
	contract, err := nodecontract.Seal(nodecontract.Draft{NodeTypeID: id, Version: BuiltinNodeVersion, ConfigSchemaRoot: id + "/config", ConfigSchemaBundle: []datatype.SchemaResource{{ID: id + "/config", Schema: schema}},

		Ports:     nodecontract.PortSet{DataInputs: inputs, DataOutputs: outputs, ExecInputs: signalList("in"), ExecOutputs: signalList("done"), ErrorOutputs: signalList("failed")},
		Execution: effectExecution(PositionObservedEffectID), Instruction: nodecontract.Invoke(), Errors: []nodecontract.ErrorSpec{{Code: NavigationFailedCode, Category: "navigation"}},
		ImplementationABI: []nodecontract.ABIRequirement{{Kind: nodecontract.ABIBuiltin, Version: "v1"}},
		Authoring:         nodecontract.Authoring{TitleKey: key + ".title", DescriptionKey: key + ".description", Category: "automation", Tags: []string{"coordinates", "position", "navigation"}, Icon: "map-pin", Ports: dataPortHints(key, inputs, outputs, nil)},
	})
	if err != nil {
		return BuiltinDefinition{}, err
	}
	return defineBuiltin(contract, entrypoint, "v1", "source-independent-world-position/v1", nil)
}
