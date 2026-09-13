package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/navigationpath"
	"github.com/yottaapp/yotta/internal/nodecontract"
)

const (
	PathTypeID          = "https://schemas.yotta.dev/types/navigation/path/v1"
	PathPointTypeID     = "https://schemas.yotta.dev/types/navigation/path-point/v1"
	PathReferenceTypeID = "https://schemas.yotta.dev/types/navigation/path-reference/v1"
	PathNodePrefix      = "https://schemas.yotta.dev/nodes/navigation/"
	PathInvalidCode     = "path.invalid"
	ReadPathNodeID      = PathNodePrefix + "read-path"
	ReadPathEffectID    = "https://schemas.yotta.dev/effects/navigation/read-path/v1"
	PathAssetTypeID     = "https://schemas.yotta.dev/types/navigation/path-asset/v1"
)

func sealPathTypes(t primitiveTypes) ([]datatype.Definition, error) {
	heightID := "https://schemas.yotta.dev/types/navigation/path-height/v1"
	height, err := sealStructuredType(heightID, json.RawMessage(fmt.Sprintf(`{"$id":%q,"$schema":"https://json-schema.org/draft/2020-12/schema","type":["number","null"]}`, heightID+"/schema")), datatype.Authoring{TitleKey: "type.navigation.path-height.title", DescriptionKey: "type.navigation.path-height.description", Color: "#22d3ee", Icon: "ruler", Examples: []json.RawMessage{json.RawMessage(`null`)}})
	if err != nil {
		return nil, err
	}
	str := map[string]any{"type": "string"}
	number := map[string]any{"type": "number"}
	refProps := map[string]any{"kind": map[string]any{"type": "string", "enum": []string{"world", "local"}}, "frame": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}, "unit": map[string]any{"type": "string", "minLength": 1, "maxLength": 32}, "axisHeading": map[string]any{"type": "number", "minimum": 0, "exclusiveMaximum": 360}, "axisSign": map[string]any{"type": "integer", "enum": []int{-1, 1}}, "map": str, "floor": str}
	pointProps := map[string]any{"id": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}, "name": str, "x": number, "y": number, "z": map[string]any{"type": []string{"number", "null"}}}
	object := func(props map[string]any, required []string) map[string]any {
		return map[string]any{"type": "object", "additionalProperties": false, "properties": props, "required": required}
	}
	refSchema := object(refProps, []string{"kind", "frame", "unit", "axisHeading", "axisSign", "map", "floor"})
	pointSchema := object(pointProps, []string{"id", "name", "x", "y", "z"})
	seal := func(id, key string, schema map[string]any, fields []datatype.StructureField, example string) (datatype.Definition, error) {
		schema["$id"] = id + "/schema"
		schema["$schema"] = datatype.JSONSchemaDialect
		raw, err := json.Marshal(schema)
		if err != nil {
			return datatype.Definition{}, err
		}
		return sealStructuredTypeWithStructure(id, raw, datatype.Authoring{TitleKey: "type.navigation." + key + ".title", DescriptionKey: "type.navigation." + key + ".description", Color: "#22d3ee", Icon: "route", BreakTitleKey: "type.navigation." + key + ".breakTitle", BreakDescriptionKey: "type.navigation." + key + ".description", Examples: []json.RawMessage{json.RawMessage(example)}}, &datatype.StructureSpec{BreakNodeTypeID: PathNodePrefix + "break-" + key, Fields: fields})
	}
	field := func(id, key string, ref datatype.TypeRef) datatype.StructureField {
		return datatype.StructureField{ID: id, JSONKey: key, Type: datatype.RefExpression(ref)}
	}
	r, err := seal(PathReferenceTypeID, "path-reference", refSchema, []datatype.StructureField{field("kind", "kind", t.stringRef), field("frame", "frame", t.stringRef), field("unit", "unit", t.stringRef), field("axis-heading", "axisHeading", t.numberRef), field("axis-sign", "axisSign", t.integerRef), field("map", "map", t.stringRef), field("floor", "floor", t.stringRef)}, `{"kind":"world","frame":"world","unit":"world","axisHeading":0,"axisSign":1,"map":"","floor":""}`)
	if err != nil {
		return nil, err
	}
	p, err := seal(PathPointTypeID, "path-point", pointSchema, []datatype.StructureField{field("id", "id", t.stringRef), field("x", "x", t.numberRef), field("y", "y", t.numberRef), field("name", "name", t.stringRef), field("z", "z", height.TypeRef())}, `{"id":"point-1","name":"","x":0,"y":0,"z":null}`)
	if err != nil {
		return nil, err
	}
	// Nested schemas are embedded without resource IDs; each type remains an
	// independently valid document in the canonical Catalog.
	delete(refSchema, "$id")
	delete(refSchema, "$schema")
	delete(pointSchema, "$id")
	delete(pointSchema, "$schema")
	pathSchema := object(map[string]any{"version": map[string]any{"type": "integer", "const": 1}, "reference": refSchema, "points": map[string]any{"type": "array", "items": pointSchema}}, []string{"version", "reference", "points"})
	v, err := seal(PathTypeID, "path", pathSchema, []datatype.StructureField{field("version", "version", t.integerRef), field("reference", "reference", r.TypeRef()), {ID: "points", JSONKey: "points", Type: datatype.ListExpression(datatype.RefExpression(p.TypeRef()))}}, `{"version":1,"reference":{"kind":"world","frame":"world","unit":"world","axisHeading":0,"axisSign":1,"map":"","floor":""},"points":[]}`)
	if err != nil {
		return nil, err
	}
	asset, err := datatype.SealDefinition(datatype.DefinitionDraft{
		TypeID: PathAssetTypeID, SchemaDialect: datatype.JSONSchemaDialect, SchemaRoot: PathAssetTypeID + "/schema",
		SchemaBundle:    []datatype.SchemaResource{{ID: PathAssetTypeID + "/schema", Schema: json.RawMessage(fmt.Sprintf(`{"$id":%q,"$schema":"https://json-schema.org/draft/2020-12/schema"}`, PathAssetTypeID+"/schema"))}},
		Representations: []datatype.RepresentationSpec{{Kind: datatype.RepresentationBlobRef, Codec: datatype.CodecBlobRefV1}},
		Authoring:       datatype.Authoring{TitleKey: "type.navigation.pathAsset.title", DescriptionKey: "type.navigation.pathAsset.description", Color: "#22d3ee", Icon: "route"},
	})
	if err != nil {
		return nil, err
	}
	return []datatype.Definition{r, p, v, height, asset}, nil
}

func definePathNodes(t primitiveTypes, types []datatype.Definition) ([]BuiltinDefinition, error) {
	reference, point, path := datatype.RefExpression(types[0].TypeRef()), datatype.RefExpression(types[1].TypeRef()), datatype.RefExpression(types[2].TypeRef())
	integer := datatype.RefExpression(t.integerRef)
	input := func(id string, typ datatype.TypeExpression) nodecontract.DataInputPort {
		return nodecontract.DataInputPort{ID: id, Type: typ, Required: true}
	}
	result := []BuiltinDefinition{}
	for _, op := range []string{"make-path", "path-point", "slice-path", "reverse-path", "join-path", "align-path"} {
		inputs := []nodecontract.DataInputPort{input("path", path)}
		output := nodecontract.DataOutputPort{ID: "result", Type: path}
		switch op {
		case "make-path":
			inputs = []nodecontract.DataInputPort{input("reference", reference), input("points", datatype.ListExpression(point))}
		case "path-point":
			inputs = append(inputs, input("index", integer))
			output = nodecontract.DataOutputPort{ID: "point", Type: point}
		case "slice-path":
			inputs = append(inputs, input("start", integer), input("end", integer))
		case "align-path":
			inputs = append(inputs, input("reference", reference), input("origin", point), input("angle", datatype.RefExpression(t.numberRef)))
		case "join-path":
			inputs = []nodecontract.DataInputPort{input("paths", datatype.ListExpression(path))}
		}
		id := PathNodePrefix + op
		key := "node.navigation." + op
		contract, err := nodecontract.Seal(nodecontract.Draft{NodeTypeID: id, Version: BuiltinNodeVersion, ConfigSchemaRoot: id + "/config", ConfigSchemaBundle: emptyConfigSchema(id + "/config"), Ports: nodecontract.PortSet{DataInputs: inputs, DataOutputs: []nodecontract.DataOutputPort{output}}, Execution: pureDataExecution(), Instruction: nodecontract.Invoke(), Errors: []nodecontract.ErrorSpec{{Code: PathInvalidCode, Category: "navigation"}}, ImplementationABI: []nodecontract.ABIRequirement{{Kind: nodecontract.ABIBuiltin, Version: "v1"}}, Authoring: nodecontract.Authoring{TitleKey: key + ".title", DescriptionKey: key + ".description", Category: "data", Icon: "route", Tags: []string{"path", "navigation"}, Ports: dataPortHints(key, inputs, []nodecontract.DataOutputPort{output}, nil)}})
		if err != nil {
			return nil, err
		}
		definition, err := defineBuiltin(contract, "navigation."+op, "v1", "coordinate-reference-ordered-path/v1", pathEvaluator(op))
		if err != nil {
			return nil, err
		}
		result = append(result, definition)
	}
	return result, nil
}

func pathEvaluator(op string) InlineEvaluator {
	return func(_ context.Context, inputs map[string]json.RawMessage, _ map[string]any) (map[string]json.RawMessage, error) {
		fail := func(err error) (map[string]json.RawMessage, error) {
			return nil, &InlineFailure{Code: PathInvalidCode, Cause: err}
		}
		var p navigationpath.Path
		var err error
		if op != "make-path" && op != "join-path" {
			p, err = navigationpath.Decode(inputs["path"])
			if err != nil {
				return fail(err)
			}
		}
		var value any
		port := "result"
		switch op {
		case "make-path":
			p.Version = navigationpath.Version
			if err = json.Unmarshal(inputs["reference"], &p.Reference); err != nil {
				return fail(err)
			}
			if err = json.Unmarshal(inputs["points"], &p.Points); err != nil {
				return fail(err)
			}
			if err = p.Validate(); err != nil {
				return fail(err)
			}
			value = p
		case "reverse-path":
			value = p.Reverse()
		case "align-path":
			var reference navigationpath.Reference
			var origin navigationpath.Point
			var angle float64
			if err = json.Unmarshal(inputs["reference"], &reference); err != nil {
				return fail(err)
			}
			if err = json.Unmarshal(inputs["origin"], &origin); err != nil {
				return fail(err)
			}
			if err = json.Unmarshal(inputs["angle"], &angle); err != nil {
				return fail(err)
			}
			value, err = p.Align(reference, origin, angle)
			if err != nil {
				return fail(err)
			}
		case "slice-path", "path-point":
			var start, end int
			key := "start"
			if op == "path-point" {
				key = "index"
			}
			if err = json.Unmarshal(inputs[key], &start); err != nil {
				return fail(err)
			}
			end = start
			if op == "slice-path" {
				if err = json.Unmarshal(inputs["end"], &end); err != nil {
					return fail(err)
				}
			}
			p, err = p.Slice(start, end)
			if err != nil {
				return fail(err)
			}
			value = p
			if op == "path-point" {
				value = p.Points[0]
				port = "point"
			}
		case "join-path":
			var paths []navigationpath.Path
			if err = json.Unmarshal(inputs["paths"], &paths); err != nil {
				return fail(err)
			}
			value, err = navigationpath.Join(paths...)
			if err != nil {
				return fail(err)
			}
		default:
			return fail(fmt.Errorf("unknown path operation"))
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return fail(err)
		}
		return map[string]json.RawMessage{port: raw}, nil
	}
}
