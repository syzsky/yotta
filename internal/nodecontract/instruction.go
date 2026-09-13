package nodecontract

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/yottaapp/yotta/internal/datatype"
)

type InstructionKind string

const (
	InstructionInvoke      InstructionKind = "invoke"
	InstructionRunRoot     InstructionKind = "run-root"
	InstructionCountedLoop InstructionKind = "counted-loop"
	InstructionForEach     InstructionKind = "for-each"
	InstructionRetry       InstructionKind = "retry"
	InstructionTask        InstructionKind = "task"
)

const (
	TaskStartedStatusID = "control.task.started"
	TaskPausingStatusID = "control.task.pausing"
	TaskPausedStatusID  = "control.task.paused"
	TaskResumedStatusID = "control.task.resumed"
	TaskOverrunStatusID = "control.task.overrun"

	RetryAttemptStatusID   = "control.retry.attempt"
	RetryExhaustedStatusID = "control.retry.exhausted"
)

// InstructionSpec is the compiler-facing tagged instruction union. It makes
// host-lowered control semantics part of the semantic digest instead of
// relying on node type IDs or adapter-side conventions.
type InstructionSpec struct {
	Kind        InstructionKind         `json:"kind" jsonschema:"required,enum=invoke,enum=run-root,enum=counted-loop,enum=for-each,enum=retry,enum=task"`
	Invoke      *InvokeInstruction      `json:"invoke,omitempty"`
	RunRoot     *RunRootInstruction     `json:"runRoot,omitempty"`
	CountedLoop *CountedLoopInstruction `json:"countedLoop,omitempty"`
	ForEach     *ForEachInstruction     `json:"forEach,omitempty"`
	Retry       *RetryInstruction       `json:"retry,omitempty"`
	Task        *TaskInstruction        `json:"task,omitempty"`
}

// TaskInstruction owns an isolated periodic body. A monitored main branch and
// its interrupt handler, when declared, share the task's structured lifetime.
type TaskInstruction struct {
	EntryInput      string           `json:"entryInput"`
	StopInput       string           `json:"stopInput"`
	BodyOutput      string           `json:"bodyOutput"`
	CompletedOutput string           `json:"completedOutput"`
	IntervalInput   string           `json:"intervalInput"`
	CountInput      string           `json:"countInput"`
	IndexOutput     string           `json:"indexOutput"`
	OrdinalType     datatype.TypeRef `json:"ordinalType"`
	DurationType    datatype.TypeRef `json:"durationType"`
	MainOutput      string           `json:"mainOutput,omitempty"`
	InterruptInput  string           `json:"interruptInput,omitempty"`
	HandlerOutput   string           `json:"handlerOutput,omitempty"`
}

type InvokeInstruction struct {
	Subscription *SubscriptionInstruction `json:"subscription,omitempty"`
	Branches     []BranchInstruction      `json:"branches,omitempty"`
}

// BranchInstruction declares scheduler-owned serial child executions of an
// effect. Coalescing returns the active handle without replacing its snapshot;
// otherwise requests queue in order within the executor's fixed budget.
type BranchInstruction struct {
	Output   string `json:"output"`
	Coalesce bool   `json:"coalesce,omitempty"`
}

// SubscriptionInstruction declares scheduler-owned serial event handling.
type SubscriptionInstruction struct {
	StopInput       string `json:"stopInput"`
	EventOutput     string `json:"eventOutput"`
	MainOutput      string `json:"mainOutput"`
	CompletedOutput string `json:"completedOutput"`
}

type RunRootInstruction struct {
	Output string `json:"output" jsonschema:"required,pattern=^[a-z][a-z0-9]*(?:[-_][a-z0-9]+)*$"`
}

type CountedLoopInstruction struct {
	EntryInput      string           `json:"entryInput"`
	BreakInput      string           `json:"breakInput"`
	ContinueInput   string           `json:"continueInput"`
	BodyOutput      string           `json:"bodyOutput"`
	CompletedOutput string           `json:"completedOutput"`
	CountInput      string           `json:"countInput"`
	IndexOutput     string           `json:"indexOutput"`
	OrdinalType     datatype.TypeRef `json:"ordinalType"`
	MaxIterations   int              `json:"maxIterations" jsonschema:"required,minimum=1,maximum=16384"`
}

type ForEachInstruction struct {
	EntryInput      string           `json:"entryInput"`
	BreakInput      string           `json:"breakInput"`
	ContinueInput   string           `json:"continueInput"`
	BodyOutput      string           `json:"bodyOutput"`
	CompletedOutput string           `json:"completedOutput"`
	ItemsInput      string           `json:"itemsInput"`
	IndexOutput     string           `json:"indexOutput"`
	ItemOutput      string           `json:"itemOutput"`
	OrdinalType     datatype.TypeRef `json:"ordinalType"`
	MaxItems        int              `json:"maxItems" jsonschema:"required,minimum=1,maximum=16384"`
}

type RetryInstruction struct {
	EntryInput      string           `json:"entryInput"`
	RetryInput      string           `json:"retryInput"`
	BodyOutput      string           `json:"bodyOutput"`
	CompletedOutput string           `json:"completedOutput"`
	ExhaustedOutput string           `json:"exhaustedOutput"`
	AttemptsInput   string           `json:"attemptsInput"`
	AttemptOutput   string           `json:"attemptOutput"`
	OrdinalType     datatype.TypeRef `json:"ordinalType"`
	MaxAttempts     int              `json:"maxAttempts" jsonschema:"required,minimum=1,maximum=256"`
}

func Invoke() InstructionSpec {
	return InstructionSpec{Kind: InstructionInvoke, Invoke: &InvokeInstruction{}}
}

func RunRoot(output string) InstructionSpec {
	return InstructionSpec{Kind: InstructionRunRoot, RunRoot: &RunRootInstruction{Output: output}}
}

// AcceptsSignalInput reports whether the compiler instruction accepts a routed
// signal on the named input. Signal input channels are instruction semantics:
// the physical port set alone cannot distinguish Retry.retry from a normal
// execution input.
func (source InstructionSpec) AcceptsSignalInput(channel, input string) bool {
	switch source.Kind {
	case InstructionInvoke:
		return channel == "exec" || channel == "error"
	case InstructionRunRoot:
		return false
	case InstructionCountedLoop:
		value := source.CountedLoop
		return value != nil && channel == "exec" &&
			(input == value.EntryInput || input == value.BreakInput || input == value.ContinueInput)
	case InstructionForEach:
		value := source.ForEach
		return value != nil && channel == "exec" &&
			(input == value.EntryInput || input == value.BreakInput || input == value.ContinueInput)
	case InstructionRetry:
		value := source.Retry
		return value != nil &&
			(channel == "exec" && input == value.EntryInput || channel == "error" && input == value.RetryInput)
	case InstructionTask:
		v := source.Task
		return v != nil && channel == "exec" && (input == v.EntryInput || input == v.StopInput || v.InterruptInput != "" && input == v.InterruptInput)
	default:
		return false
	}
}

func normalizeInstruction(source InstructionSpec, execution ExecutionSpec, ports PortSet) (InstructionSpec, error) {
	payloads := []bool{source.Invoke != nil, source.RunRoot != nil, source.CountedLoop != nil, source.ForEach != nil, source.Retry != nil, source.Task != nil}
	count := 0
	for _, present := range payloads {
		if present {
			count++
		}
	}
	if count != 1 {
		return InstructionSpec{}, errors.New("node instruction must contain exactly one payload")
	}
	matches := map[InstructionKind]bool{
		InstructionInvoke: source.Invoke != nil, InstructionRunRoot: source.RunRoot != nil,
		InstructionCountedLoop: source.CountedLoop != nil, InstructionForEach: source.ForEach != nil,
		InstructionRetry: source.Retry != nil,
		InstructionTask:  source.Task != nil,
	}
	if !matches[source.Kind] {
		return InstructionSpec{}, errors.New("node instruction kind does not match its payload")
	}
	if source.Kind == InstructionInvoke {
		if len(source.Invoke.Branches) > 0 {
			if source.Invoke.Subscription != nil || execution.Class != ExecutionEffect || execution.Evaluation != EvaluationPush {
				return InstructionSpec{}, errors.New("invocation branches require a push effect without a subscription")
			}
			seen := map[string]bool{}
			for _, branch := range source.Invoke.Branches {
				if !hasExecOutput(ports, branch.Output) || seen[branch.Output] {
					return InstructionSpec{}, errors.New("invocation branch references an invalid or duplicate exec output")
				}
				seen[branch.Output] = true
			}
		}
		if v := source.Invoke.Subscription; v != nil {
			if execution.Class != ExecutionEffect || !hasExecInput(ports, v.StopInput) || !hasExecOutput(ports, v.EventOutput) || !hasExecOutput(ports, v.MainOutput) || !hasExecOutput(ports, v.CompletedOutput) {
				return InstructionSpec{}, errors.New("subscription instruction references invalid ports")
			}
		}
		if execution.Class == ExecutionRegion {
			return InstructionSpec{}, errors.New("region execution requires a host-lowered instruction")
		}
		return source, nil
	}
	if source.Kind == InstructionRunRoot {
		if execution.Class != ExecutionEvent || execution.Determinism != Deterministic || len(execution.Effects) != 0 ||
			execution.Retry != RetryNever || !hasExecOutput(ports, source.RunRoot.Output) {
			return InstructionSpec{}, errors.New("run-root instruction requires an event output")
		}
		return source, nil
	}
	if execution.Class != ExecutionRegion || execution.Evaluation != EvaluationPush || len(execution.Effects) != 0 {
		return InstructionSpec{}, errors.New("region instruction requires deterministic effect-free region execution")
	}
	switch source.Kind {
	case InstructionTask:
		v := source.Task
		inputs, outputs := []string{v.EntryInput, v.StopInput}, []string{v.BodyOutput, v.CompletedOutput}
		if v.MainOutput != "" || v.InterruptInput != "" || v.HandlerOutput != "" {
			inputs = append(inputs, v.InterruptInput)
			outputs = append(outputs, v.MainOutput, v.HandlerOutput)
		}
		if !regionPortsExist(ports, inputs, outputs) ||
			!inputHasType(ports, v.IntervalInput, datatype.RefExpression(v.DurationType)) ||
			!inputHasType(ports, v.CountInput, datatype.RefExpression(v.OrdinalType)) ||
			!outputHasType(ports, v.IndexOutput, datatype.RefExpression(v.OrdinalType)) {
			return InstructionSpec{}, errors.New("task instruction references invalid ports")
		}
	case InstructionCountedLoop:
		value := source.CountedLoop
		if value.MaxIterations < 1 || value.MaxIterations > 16_384 ||
			!regionPortsExist(ports, []string{value.EntryInput, value.BreakInput, value.ContinueInput}, []string{value.BodyOutput, value.CompletedOutput}) ||
			!inputHasType(ports, value.CountInput, datatype.RefExpression(value.OrdinalType)) ||
			!outputHasType(ports, value.IndexOutput, datatype.RefExpression(value.OrdinalType)) {
			return InstructionSpec{}, errors.New("counted-loop instruction references invalid ports or budget")
		}
	case InstructionForEach:
		value := source.ForEach
		itemsType, hasItems := inputType(ports, value.ItemsInput)
		itemType, hasItem := outputType(ports, value.ItemOutput)
		if value.MaxItems < 1 || value.MaxItems > 16_384 ||
			!regionPortsExist(ports, []string{value.EntryInput, value.BreakInput, value.ContinueInput}, []string{value.BodyOutput, value.CompletedOutput}) ||
			!hasItems || itemsType.Kind != datatype.TypeExpressionList || itemsType.Element == nil || !hasItem || !reflect.DeepEqual(*itemsType.Element, itemType) ||
			!outputHasType(ports, value.IndexOutput, datatype.RefExpression(value.OrdinalType)) {
			return InstructionSpec{}, errors.New("for-each instruction references invalid ports or budget")
		}
	case InstructionRetry:
		value := source.Retry
		if value.MaxAttempts < 1 || value.MaxAttempts > 256 ||
			!regionPortsExist(ports, []string{value.EntryInput, value.RetryInput}, []string{value.BodyOutput, value.CompletedOutput, value.ExhaustedOutput}) ||
			!inputHasType(ports, value.AttemptsInput, datatype.RefExpression(value.OrdinalType)) ||
			!outputHasType(ports, value.AttemptOutput, datatype.RefExpression(value.OrdinalType)) {
			return InstructionSpec{}, errors.New("retry instruction references invalid ports or budget")
		}
	default:
		return InstructionSpec{}, fmt.Errorf("unsupported node instruction %q", source.Kind)
	}
	return source, nil
}

func regionPortsExist(ports PortSet, inputs, outputs []string) bool {
	seen := map[string]bool{}
	for _, id := range inputs {
		if id == "" || seen[id] || !hasExecInput(ports, id) {
			return false
		}
		seen[id] = true
	}
	for _, id := range outputs {
		if id == "" || seen[id] || !hasExecOutput(ports, id) {
			return false
		}
		seen[id] = true
	}
	return true
}

func hasExecInput(ports PortSet, id string) bool {
	for _, port := range ports.ExecInputs {
		if port.ID == id {
			return true
		}
	}
	return false
}

func hasExecOutput(ports PortSet, id string) bool {
	for _, port := range ports.ExecOutputs {
		if port.ID == id {
			return true
		}
	}
	return false
}

func inputType(ports PortSet, id string) (datatype.TypeExpression, bool) {
	for _, port := range ports.DataInputs {
		if port.ID == id {
			return port.Type, true
		}
	}
	return datatype.TypeExpression{}, false
}

func outputType(ports PortSet, id string) (datatype.TypeExpression, bool) {
	for _, port := range ports.DataOutputs {
		if port.ID == id {
			return port.Type, true
		}
	}
	return datatype.TypeExpression{}, false
}

func inputHasType(ports PortSet, id string, want datatype.TypeExpression) bool {
	got, ok := inputType(ports, id)
	return ok && reflect.DeepEqual(got, want)
}

func outputHasType(ports PortSet, id string, want datatype.TypeExpression) bool {
	got, ok := outputType(ports, id)
	return ok && reflect.DeepEqual(got, want)
}
