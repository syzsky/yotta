package nodes

import (
	"encoding/json"

	"github.com/yottaapp/yotta/internal/automation/installed"
	"github.com/yottaapp/yotta/internal/capability"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodecontract"
)

const FollowPathNodeID = PathNodePrefix + "follow-path"
const FollowSavedPathNodeID = PathNodePrefix + "follow-saved-path"
const FollowSavedPathEffectID = "https://schemas.yotta.dev/effects/navigation/follow-saved-path/v1"
const FollowPathEffectID = "https://schemas.yotta.dev/effects/navigation/follow-path/v1"
const PathProgressStatus = "navigation.path.progress"
const PathRecoveringStatus = "navigation.path.recovering"
const PathStuckStatus = "navigation.path.stuck"
const PathTurningStuckStatus = "navigation.path.turning-stuck"
const PathUnavailableStatus = "navigation.path.unavailable"
const PathReferenceStatus = "navigation.path.reference-mismatch"
const PathHeightStatus = "navigation.path.height-mismatch"
const PathInputConflictStatus = "navigation.path.input-conflict"

func defineFollowPath(t primitiveTypes, duration, path datatype.TypeRef) (BuiltinDefinition, error) {
	return definePathFollower(t, duration, path, false, capability.Definition{})
}
func definePathFollower(t primitiveTypes, duration, path datatype.TypeRef, saved bool, blobRead capability.Definition) (BuiltinDefinition, error) {
	num, integer := datatype.RefExpression(t.numberRef), datatype.RefExpression(t.integerRef)
	input := func(id string, typ datatype.TypeExpression, value string) nodecontract.DataInputPort {
		return nodecontract.DataInputPort{ID: id, Type: typ, Required: true, Default: rawDefault(value)}
	}
	inputs := []nodecontract.DataInputPort{{ID: "path", Type: datatype.RefExpression(path), Required: true}, input("start", integer, "0"), input("end", integer, "-1"), input("tolerance", num, "20"), input("height-tolerance", num, "20"), input("timeout", datatype.RefExpression(duration), "30000"), input("interval", datatype.RefExpression(duration), "100"), input("slow-distance", num, "100")}
	outputs := []nodecontract.DataOutputPort{{ID: "last-index", Type: integer}, {ID: "current-index", Type: integer}, {ID: "point-id", Type: datatype.RefExpression(t.stringRef)}, {ID: "x", Type: num}, {ID: "y", Type: num}, {ID: "distance", Type: num}}
	outputs = append(outputs, nodecontract.DataOutputPort{ID: "point-name", Type: datatype.RefExpression(t.stringRef)}, nodecontract.DataOutputPort{ID: "progress", Type: num}, nodecontract.DataOutputPort{ID: "recovery-attempt", Type: integer}, nodecontract.DataOutputPort{ID: "reason", Type: datatype.RefExpression(t.stringRef)})
	id, effectID, entrypoint := FollowPathNodeID, FollowPathEffectID, "navigation.follow-path"
	titleKey, descriptionKey := "node.navigation.followPath.title", "node.navigation.followPath.description"
	var requirements []capability.Requirement
	if saved {
		id, effectID, entrypoint = FollowSavedPathNodeID, FollowSavedPathEffectID, "navigation.follow-saved-path"
		inputs[0].ID = "asset"
		titleKey, descriptionKey = "node.navigation.followSavedPath.title", "node.navigation.followSavedPath.description"
		requirements = []capability.Requirement{requirement(blobRead, "blob-read", []string{"read-range"}, "blob-store")}
	}
	props := map[string]any{
		"slot":              map[string]any{"type": "string", "minLength": 1, "maxLength": 128, "pattern": "^[a-z][a-z0-9]*(?:[-_][a-z0-9]+)*$", "x-yotta-title-key": "node.automation.config.slot.title"},
		"position-variable": map[string]any{"type": "string", "minLength": 1, "maxLength": 128, "x-yotta-control": "state-variable", "x-yotta-title-key": "node.navigation.followPath.sourceVariable"},
		"forwardKey":        map[string]any{"type": "string", "default": "W", "x-yotta-title-key": "node.navigation.config.forwardKey"},
		"turnSign":          map[string]any{"type": "integer", "enum": []int{-1, 1}, "default": 1, "x-yotta-title-key": "node.navigation.config.turnSign"},
		"recovery-attempts": map[string]any{"type": "integer", "minimum": 0, "maximum": 20, "default": 2, "x-yotta-title-key": "node.navigation.followPath.recoveryAttempts"},
		"marker-mode":       map[string]any{"type": "string", "enum": []string{"continue", "pause"}, "default": "continue", "x-yotta-title-key": "node.navigation.followPath.markerMode"},
		"action-interval":   map[string]any{"type": "integer", "minimum": 100, "maximum": 60000, "default": 500, "x-yotta-title-key": "node.navigation.followPath.actionInterval"},
	}
	schema, err := json.Marshal(map[string]any{"$id": id + "/config", "$schema": datatype.JSONSchemaDialect, "type": "object", "properties": props, "required": []string{"slot", "position-variable"}, "additionalProperties": false})
	if err != nil {
		return BuiltinDefinition{}, err
	}
	errors := automationTemplateErrors(true)
	errors = append(errors, nodecontract.ErrorSpec{Code: PathInvalidCode, Category: "navigation"}, nodecontract.ErrorSpec{Code: NavigationFailedCode, Category: "navigation"})
	hints := dataPortHints("node.navigation.followPath", inputs, outputs, nil)
	for _, port := range []string{"moving", "marker", "recover"} {
		hints = append(hints, nodecontract.PortAuthoring{ID: port, TitleKey: "node.navigation.followPath.branch." + port + ".title", DescriptionKey: "node.navigation.followPath.branch." + port + ".description"})
	}
	instruction := nodecontract.Invoke()
	instruction.Invoke.Branches = []nodecontract.BranchInstruction{{Output: "moving", Coalesce: true}, {Output: "marker"}, {Output: "recover"}}
	c, err := nodecontract.Seal(nodecontract.Draft{NodeTypeID: id, Version: "1.1.0", ConfigSchemaRoot: id + "/config", ConfigSchemaBundle: []datatype.SchemaResource{{ID: id + "/config", Schema: schema}},
		Ports:     nodecontract.PortSet{DataInputs: inputs, DataOutputs: outputs, ExecInputs: signalList("in"), ExecOutputs: signalList("arrived", "timeout", "stuck", "unavailable", "reference-mismatch", "height-mismatch", "moving", "marker", "recover"), ErrorOutputs: signalList("failed")},
		Execution: automationEffectExecution(effectID), Instruction: instruction, ConfiguredTargets: automationTargetSpec("input-target", installed.TargetKindDesktopWindow),
		CapabilityRequirements: requirements,
		StateAccesses:          []nodecontract.StateAccessSpec{{ID: "position", SlotConfigKey: "position-variable", Type: datatype.RefExpression(t.stringRef), Mode: nodecontract.StateRead}}, Errors: errors,
		StatusEvents:      []nodecontract.StatusEventSpec{{Code: PathProgressStatus, Category: nodecontract.StatusProgress}, {Code: PathRecoveringStatus, Category: nodecontract.StatusProgress}, {Code: PathStuckStatus, Category: nodecontract.StatusProgress}, {Code: PathTurningStuckStatus, Category: nodecontract.StatusProgress}, {Code: PathUnavailableStatus, Category: nodecontract.StatusProgress}, {Code: PathReferenceStatus, Category: nodecontract.StatusProgress}, {Code: PathHeightStatus, Category: nodecontract.StatusProgress}, {Code: PathInputConflictStatus, Category: nodecontract.StatusProgress}, {Code: NavigationWaitingStatus, Category: nodecontract.StatusWaiting}, {Code: NavigationTimeoutStatus, Category: nodecontract.StatusProgress}, {Code: NavigationFinishedStatus, Category: nodecontract.StatusProgress}},
		ImplementationABI: []nodecontract.ABIRequirement{{Kind: nodecontract.ABIBuiltin, Version: "v1"}},
		Authoring:         nodecontract.Authoring{TitleKey: titleKey, DescriptionKey: descriptionKey, Category: "automation", Icon: "route", Tags: []string{"path", "navigation"}, Ports: hints},
	})
	if err != nil {
		return BuiltinDefinition{}, err
	}
	return defineBuiltin(c, entrypoint, "v2", "continuous-position-source-navigation/v2", nil)
}
