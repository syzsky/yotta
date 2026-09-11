package nodes

import (
	"encoding/json"
	"fmt"

	"github.com/yottaapp/yotta/internal/capability"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodecontract"
)

const (
	DurationMillisecondsTypeID = "https://schemas.yotta.dev/types/time/duration-milliseconds/v1"

	RunStartedNodeID = "https://schemas.yotta.dev/nodes/event/run-started"
	BranchNodeID     = "https://schemas.yotta.dev/nodes/control/branch"
	DelayNodeID      = "https://schemas.yotta.dev/nodes/control/delay"
	EndBranchNodeID  = "https://schemas.yotta.dev/nodes/control/end-branch"
	RepeatNodeID     = "https://schemas.yotta.dev/nodes/control/repeat"
	ForEachNodeID    = "https://schemas.yotta.dev/nodes/control/for-each"
	PeriodicNodeID   = "https://schemas.yotta.dev/nodes/control/periodic"
	MonitorNodeID    = "https://schemas.yotta.dev/nodes/control/monitor"
	RetryNodeID      = "https://schemas.yotta.dev/nodes/control/retry"

	DelayWaitEffectID          = "https://schemas.yotta.dev/effects/time/delay/v1"
	DelayWaitingStatus         = "control.delay.waiting"
	DelayFailedCode            = "control.delay_failed"
	MaxDelayMilliseconds int64 = 86_400_000
	MaxRegionIterations        = 10_000
	MaxRetryAttempts           = 100
)

func sealDurationMillisecondsType() (datatype.Definition, error) {
	return sealStructuredType(
		DurationMillisecondsTypeID,
		json.RawMessage(fmt.Sprintf(`{
			"$id":%q,"$schema":"https://json-schema.org/draft/2020-12/schema",
			"type":"integer","minimum":0,"maximum":%d
		}`, DurationMillisecondsTypeID+"/schema", MaxDelayMilliseconds)),
		datatype.Authoring{
			TitleKey: "type.time.durationMilliseconds.title", DescriptionKey: "type.time.durationMilliseconds.description",
			Color: "#a78bfa", Icon: "clock-hour-4", EditorAdapter: "duration", Unit: "ms", Importance: "common", InlinePriority: 30,
			Examples: []json.RawMessage{json.RawMessage("1000")},
		},
	)
}

func defineControlNodes(types primitiveTypes, durationRef datatype.TypeRef) ([]BuiltinDefinition, error) {
	booleanType := datatype.RefExpression(types.booleanRef)
	durationType := datatype.RefExpression(durationRef)
	defaultCount := json.RawMessage("0")
	defaultTrue := json.RawMessage("true")
	defaultDelay := json.RawMessage("1000")
	defaultRepeat := json.RawMessage("10")
	defaultAttempts := json.RawMessage("3")
	integerType := datatype.RefExpression(types.integerRef)
	itemVariable := datatype.VariableExpression("T")
	type controlNode struct {
		id, entrypoint, conformance, key, category, icon string
		ports                                            nodecontract.PortSet
		execution                                        nodecontract.ExecutionSpec
		instruction                                      nodecontract.InstructionSpec
		statuses                                         []nodecontract.StatusEventSpec
		errors                                           []nodecontract.ErrorSpec
	}
	nodes := []controlNode{
		{
			id: RunStartedNodeID, entrypoint: "event.run-started", conformance: "exactly-once-run-root/v1",
			key: "node.event.runStarted", category: "event", icon: "player-play",
			ports: signalPorts(nil, []string{"started"}), execution: eventExecution(), instruction: nodecontract.RunRoot("started"),
		},
		{
			id: BranchNodeID, entrypoint: "control.branch", conformance: "strict-boolean-exclusive-route/v1",
			key: "node.control.branch", category: "control", icon: "git-branch",
			ports: nodecontract.PortSet{
				DataInputs:  []nodecontract.DataInputPort{{ID: "condition", Type: booleanType, Required: true, Default: &defaultTrue}},
				DataOutputs: []nodecontract.DataOutputPort{}, ExecInputs: signalList("in"),
				ExecOutputs: signalList("true", "false"), ErrorOutputs: []nodecontract.SignalPort{},
			},
			execution: controlExecution(), instruction: nodecontract.Invoke(),
		},
		{
			id: DelayNodeID, entrypoint: "control.delay", conformance: "scheduler-owned-timer-continuation/v2",
			key: "node.control.delay", category: "control", icon: "clock-pause",
			ports: nodecontract.PortSet{
				DataInputs:  []nodecontract.DataInputPort{{ID: "duration-milliseconds", Type: durationType, Required: true, Default: &defaultDelay}},
				DataOutputs: []nodecontract.DataOutputPort{}, ExecInputs: signalList("in"),
				ExecOutputs: signalList("done"), ErrorOutputs: signalList("failed"),
			},
			execution: effectExecution(DelayWaitEffectID), instruction: nodecontract.Invoke(),
			statuses: []nodecontract.StatusEventSpec{{Code: DelayWaitingStatus, Category: nodecontract.StatusWaiting}},
			errors:   []nodecontract.ErrorSpec{{Code: DelayFailedCode, Category: "control", RetryHint: false}},
		},
		{
			id: EndBranchNodeID, entrypoint: "control.end-branch", conformance: "explicit-branch-termination/v1",
			key: "node.control.endBranch", category: "control", icon: "player-stop",
			ports: signalPorts([]string{"in"}, nil), execution: controlExecution(), instruction: nodecontract.Invoke(),
		},
		{
			id: RepeatNodeID, entrypoint: "control.repeat", conformance: "activation-scoped-counted-loop/v1",
			key: "node.control.repeat", category: "control", icon: "repeat",
			ports: nodecontract.PortSet{
				DataInputs:  []nodecontract.DataInputPort{{ID: "count", Type: integerType, Required: true, Default: &defaultRepeat}},
				DataOutputs: []nodecontract.DataOutputPort{{ID: "index", Type: integerType}},
				ExecInputs:  signalList("in", "break", "continue"), ExecOutputs: signalList("body", "completed"), ErrorOutputs: []nodecontract.SignalPort{},
			},
			execution: regionExecution(), instruction: nodecontract.InstructionSpec{
				Kind: nodecontract.InstructionCountedLoop,
				CountedLoop: &nodecontract.CountedLoopInstruction{
					EntryInput: "in", BreakInput: "break", ContinueInput: "continue", BodyOutput: "body", CompletedOutput: "completed",
					CountInput: "count", IndexOutput: "index", OrdinalType: types.integerRef, MaxIterations: MaxRegionIterations,
				},
			},
		},
		{
			id: ForEachNodeID, entrypoint: "control.for-each", conformance: "activation-scoped-list-iteration/v1",
			key: "node.control.forEach", category: "control", icon: "list-numbers",
			ports: nodecontract.PortSet{
				DataInputs:  []nodecontract.DataInputPort{{ID: "items", Type: datatype.ListExpression(itemVariable), Required: true}},
				DataOutputs: []nodecontract.DataOutputPort{{ID: "index", Type: integerType}, {ID: "item", Type: itemVariable}},
				ExecInputs:  signalList("in", "break", "continue"), ExecOutputs: signalList("body", "completed"), ErrorOutputs: []nodecontract.SignalPort{},
			},
			execution: regionExecution(), instruction: nodecontract.InstructionSpec{
				Kind: nodecontract.InstructionForEach,
				ForEach: &nodecontract.ForEachInstruction{
					EntryInput: "in", BreakInput: "break", ContinueInput: "continue", BodyOutput: "body", CompletedOutput: "completed",
					ItemsInput: "items", IndexOutput: "index", ItemOutput: "item", OrdinalType: types.integerRef, MaxItems: MaxRegionIterations,
				},
			},
		},
		{
			id: RetryNodeID, entrypoint: "control.retry", conformance: "activation-scoped-explicit-error-retry/v1",
			key: "node.control.retry", category: "control", icon: "refresh-dot",
			ports: nodecontract.PortSet{
				DataInputs:  []nodecontract.DataInputPort{{ID: "attempts", Type: integerType, Required: true, Default: &defaultAttempts}},
				DataOutputs: []nodecontract.DataOutputPort{{ID: "attempt", Type: integerType}},
				ExecInputs:  signalList("in", "retry"), ExecOutputs: signalList("body", "completed", "exhausted"), ErrorOutputs: []nodecontract.SignalPort{},
			},
			execution: regionExecution(), instruction: nodecontract.InstructionSpec{
				Kind: nodecontract.InstructionRetry,
				Retry: &nodecontract.RetryInstruction{
					EntryInput: "in", RetryInput: "retry", BodyOutput: "body", CompletedOutput: "completed", ExhaustedOutput: "exhausted",
					AttemptsInput: "attempts", AttemptOutput: "attempt", OrdinalType: types.integerRef, MaxAttempts: MaxRetryAttempts,
				},
			},
			statuses: []nodecontract.StatusEventSpec{
				{Code: nodecontract.RetryAttemptStatusID, Category: nodecontract.StatusProgress},
				{Code: nodecontract.RetryExhaustedStatusID, Category: nodecontract.StatusProgress},
			},
		},
	}

	for _, monitored := range []bool{false, true} {
		id, key, entrypoint := PeriodicNodeID, "node.control.periodic", "control.periodic"
		v := &nodecontract.TaskInstruction{EntryInput: "in", StopInput: "stop", BodyOutput: "tick", CompletedOutput: "completed", IntervalInput: "interval-milliseconds", CountInput: "count", IndexOutput: "index", OrdinalType: types.integerRef, DurationType: durationRef}
		ports := nodecontract.PortSet{
			DataInputs:  []nodecontract.DataInputPort{{ID: "interval-milliseconds", Type: durationType, Required: true, Default: &defaultDelay}, {ID: "count", Type: integerType, Required: true, Default: &defaultCount}},
			DataOutputs: []nodecontract.DataOutputPort{{ID: "index", Type: integerType}},
			ExecInputs:  signalList("in", "stop"), ExecOutputs: signalList("tick", "completed"), ErrorOutputs: []nodecontract.SignalPort{},
		}
		if monitored {
			id, key, entrypoint = MonitorNodeID, "node.control.monitor", "control.monitor"
			v.MainOutput, v.InterruptInput, v.HandlerOutput = "main", "interrupt", "handler"
			ports.ExecInputs = signalList("in", "stop", "interrupt")
			ports.ExecOutputs = signalList("main", "tick", "handler", "completed")
		}
		statuses := []nodecontract.StatusEventSpec{}
		for _, code := range []string{nodecontract.TaskStartedStatusID, nodecontract.TaskPausingStatusID, nodecontract.TaskPausedStatusID, nodecontract.TaskResumedStatusID, nodecontract.TaskOverrunStatusID} {
			statuses = append(statuses, nodecontract.StatusEventSpec{Code: code, Category: nodecontract.StatusProgress})
		}
		nodes = append(nodes, controlNode{id: id, key: key, entrypoint: entrypoint, category: "control", icon: "clock-play", conformance: "isolated-periodic-task/v1", ports: ports, execution: regionExecution(), statuses: statuses, instruction: nodecontract.InstructionSpec{Kind: nodecontract.InstructionTask, Task: v}})
	}
	definitions := make([]BuiltinDefinition, 0, len(nodes))
	for _, item := range nodes {
		configID := item.id + "/config"
		version := BuiltinNodeVersion
		if item.id == RetryNodeID {
			version = "1.1.0"
		}
		contract, err := nodecontract.Seal(nodecontract.Draft{Version: version,
			NodeTypeID: item.id, ConfigSchemaRoot: configID, ConfigSchemaBundle: emptyConfigSchema(configID),
			Ports: item.ports, Execution: item.execution, Instruction: item.instruction, CapabilityRequirements: []capability.Requirement{},
			Errors: item.errors, StatusEvents: item.statuses,
			ImplementationABI: []nodecontract.ABIRequirement{{Kind: instructionABI(item.instruction), Version: "v1"}},
			Authoring: nodecontract.Authoring{
				TitleKey: item.key + ".title", DescriptionKey: item.key + ".description", Category: item.category,
				Tags: controlAuthoringTags(item.id, item.category), Icon: item.icon,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("seal built-in %s: %w", item.id, err)
		}
		implementationVersion := "v1"
		if item.id == DelayNodeID {
			implementationVersion = "v2"
		}
		definition, err := defineBuiltin(contract, item.entrypoint, implementationVersion, item.conformance, nil)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

func controlAuthoringTags(nodeID, category string) []string {
	tags := []string{category, "execution"}
	if nodeID == DelayNodeID || nodeID == RepeatNodeID || nodeID == PeriodicNodeID || nodeID == MonitorNodeID {
		// Make timer and monitoring intent discoverable alongside loop controls.
		tags = append(tags, "eventtick", "tick", "timer", "polling")
	}
	return tags
}

func instructionABI(instruction nodecontract.InstructionSpec) nodecontract.ABIKind {
	if instruction.Kind == nodecontract.InstructionInvoke {
		return nodecontract.ABIBuiltin
	}
	return nodecontract.ABIHostInstruction
}

func signalPorts(inputs, outputs []string) nodecontract.PortSet {
	return nodecontract.PortSet{
		DataInputs: []nodecontract.DataInputPort{}, DataOutputs: []nodecontract.DataOutputPort{},
		ExecInputs: signalList(inputs...), ExecOutputs: signalList(outputs...), ErrorOutputs: []nodecontract.SignalPort{},
	}
}

func signalList(ids ...string) []nodecontract.SignalPort {
	result := make([]nodecontract.SignalPort, len(ids))
	for index, id := range ids {
		result[index] = nodecontract.SignalPort{ID: id}
	}
	return result
}

func eventExecution() nodecontract.ExecutionSpec {
	return nodecontract.ExecutionSpec{
		Class: nodecontract.ExecutionEvent, Effects: []nodecontract.EffectID{}, Determinism: nodecontract.Deterministic,
		Evaluation: nodecontract.EvaluationPush, Cache: nodecontract.CacheNone, Retry: nodecontract.RetryNever,
		Cancellation: nodecontract.CancellationCooperative, Timeout: nodecontract.TimeoutNone,
	}
}

func controlExecution() nodecontract.ExecutionSpec {
	return nodecontract.ExecutionSpec{
		Class: nodecontract.ExecutionControl, Effects: []nodecontract.EffectID{}, Determinism: nodecontract.Deterministic,
		Evaluation: nodecontract.EvaluationPush, Cache: nodecontract.CacheNone, Retry: nodecontract.RetryNever,
		Cancellation: nodecontract.CancellationCooperative, Timeout: nodecontract.TimeoutNone,
	}
}

func effectExecution(effect nodecontract.EffectID) nodecontract.ExecutionSpec {
	return nodecontract.ExecutionSpec{
		Class: nodecontract.ExecutionEffect, Effects: []nodecontract.EffectID{effect}, Determinism: nodecontract.Recorded,
		Evaluation: nodecontract.EvaluationPush, Cache: nodecontract.CacheNone, Retry: nodecontract.RetryNever,
		Cancellation: nodecontract.CancellationCooperative, Timeout: nodecontract.TimeoutNone,
	}
}

func regionExecution() nodecontract.ExecutionSpec {
	return nodecontract.ExecutionSpec{
		Class: nodecontract.ExecutionRegion, Effects: []nodecontract.EffectID{}, Determinism: nodecontract.Deterministic,
		Evaluation: nodecontract.EvaluationPush, Cache: nodecontract.CacheNone, Retry: nodecontract.RetryNever,
		Cancellation: nodecontract.CancellationCooperative, Timeout: nodecontract.TimeoutNone,
	}
}
