package compiler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodecontract"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/workflow/schema"
)

type regionSignal struct {
	nodeID  string
	input   string
	failure *nodeadapter.RoutedFailure
}

func (s *regionSignal) Error() string {
	return fmt.Sprintf("region signal %s.%s escaped its activation", s.nodeID, s.input)
}

func (s *scheduler) dispatch(ctx context.Context, nodeID string, trigger *nodeadapter.SignalTrigger, evaluation map[string]bool) error {
	node, ok := s.nodes[nodeID]
	if !ok {
		return fmt.Errorf("scheduled node %q is missing from Program", nodeID)
	}
	switch node.Instruction.Kind {
	case nodecontract.InstructionInvoke:
		if handled, err := s.listenerSignal(ctx, nodeID, trigger); handled {
			return err
		}
		return s.invoke(ctx, nodeID, trigger, evaluation)
	case nodecontract.InstructionRunRoot:
		return s.executeRunRoot(ctx, node, trigger)
	case nodecontract.InstructionTask:
		return s.enterTask(ctx, node, trigger)
	case nodecontract.InstructionCountedLoop, nodecontract.InstructionForEach, nodecontract.InstructionRetry:
		return s.enterRegion(ctx, node, trigger)
	default:
		return fmt.Errorf("Program node %q has an unsupported instruction", nodeID)
	}
}

func (s *scheduler) executeRunRoot(ctx context.Context, node programNode, trigger *nodeadapter.SignalTrigger) error {
	if trigger != nil || node.Instruction.RunRoot == nil {
		return errors.New("run-root instruction received a signal or invalid payload")
	}
	if err := s.debugCheckpoint(ctx, node, s.attempts[node.ID]+1, nil); err != nil {
		return err
	}
	attempt, summary, err := s.beginInstruction(ctx, node)
	if err != nil {
		return err
	}
	if err := s.finishInstruction(ctx, node, attempt, summary); err != nil {
		return err
	}
	s.enqueueInstructionOutput(node.ID, node.Instruction.RunRoot.Output, nil)
	return nil
}

// controlFrame is the continuation of one active region. Parent queues and
// iteration state live in the executor, never in a recursive Go invocation.
type controlFrame struct {
	node        programNode
	attempt     int
	summary     run.RedactedSummary
	parent      []scheduledInvocation
	parentWaits waitQueue
	index       int64
	limit       int64
	items       []json.RawMessage
	lastFailure *nodeadapter.RoutedFailure
}

func (s *scheduler) enterRegion(ctx context.Context, node programNode, trigger *nodeadapter.SignalTrigger) error {
	if trigger == nil {
		return errors.New("region instruction requires a signal")
	}
	var entry, stop, next string
	switch node.Instruction.Kind {
	case nodecontract.InstructionCountedLoop:
		spec := node.Instruction.CountedLoop
		if spec == nil {
			return errors.New("counted-loop instruction has no payload")
		}
		entry, stop, next = spec.EntryInput, spec.BreakInput, spec.ContinueInput
	case nodecontract.InstructionForEach:
		spec := node.Instruction.ForEach
		if spec == nil {
			return errors.New("for-each instruction has no payload")
		}
		entry, stop, next = spec.EntryInput, spec.BreakInput, spec.ContinueInput
	case nodecontract.InstructionRetry:
		spec := node.Instruction.Retry
		if spec == nil {
			return errors.New("retry instruction has no payload")
		}
		entry, next = spec.EntryInput, spec.RetryInput
		if trigger.InputPort == next && trigger.Failure == nil {
			return errors.New("retry input requires a routed failure")
		}
	}
	if trigger.InputPort != "" && (trigger.InputPort == stop || trigger.InputPort == next) {
		return &regionSignal{nodeID: node.ID, input: trigger.InputPort, failure: cloneRoutedFailure(trigger.Failure)}
	}
	if trigger.InputPort != entry {
		return fmt.Errorf("region instruction received unknown input %q", trigger.InputPort)
	}
	if len(s.frames) >= maxControlFrames {
		return errors.New("control frame budget exceeded")
	}
	inputs, err := s.resolveInputs(ctx, node, nil, map[string]bool{})
	if err != nil {
		return err
	}
	if err := s.debugCheckpoint(ctx, node, s.attempts[node.ID]+1, inputs); err != nil {
		return err
	}
	attempt, summary, err := s.beginInstruction(ctx, node)
	if err != nil {
		return err
	}
	frame := &controlFrame{node: node, attempt: attempt, summary: summary, parent: s.queue, parentWaits: s.waits}
	s.frames = append(s.frames, frame)
	s.queue = nil
	s.waits = nil
	switch node.Instruction.Kind {
	case nodecontract.InstructionCountedLoop:
		spec := node.Instruction.CountedLoop
		frame.limit, err = decodeInstructionInteger(inputs[spec.CountInput])
		if err != nil || frame.limit < 0 || frame.limit > int64(spec.MaxIterations) {
			return errors.Join(errors.New("counted-loop count exceeds its frozen budget"), err)
		}
	case nodecontract.InstructionForEach:
		spec := node.Instruction.ForEach
		if err := json.Unmarshal(inputs[spec.ItemsInput].InlineJSON(), &frame.items); err != nil || len(frame.items) > spec.MaxItems {
			return errors.Join(errors.New("for-each items exceed the frozen budget"), err)
		}
		frame.limit = int64(len(frame.items))
	case nodecontract.InstructionRetry:
		spec := node.Instruction.Retry
		frame.limit, err = decodeInstructionInteger(inputs[spec.AttemptsInput])
		if err != nil || frame.limit < 1 || frame.limit > int64(spec.MaxAttempts) {
			return errors.Join(errors.New("retry attempts exceed the frozen budget"), err)
		}
	}
	return s.startRegionIteration(ctx, frame)
}

func (s *scheduler) startRegionIteration(ctx context.Context, frame *controlFrame) error {
	node := frame.node
	switch node.Instruction.Kind {
	case nodecontract.InstructionCountedLoop:
		spec := node.Instruction.CountedLoop
		if frame.index >= frame.limit {
			return s.completeRegion(ctx, frame, spec.CompletedOutput, nil)
		}
		if err := s.setInstructionIntegerOutput(node, spec.IndexOutput, frame.index, frame.attempt); err != nil {
			return err
		}
		s.queue = s.instructionRoutes(node.ID, spec.BodyOutput, nil)
	case nodecontract.InstructionForEach:
		spec := node.Instruction.ForEach
		if frame.index >= frame.limit {
			return s.completeRegion(ctx, frame, spec.CompletedOutput, nil)
		}
		if err := s.setInstructionIntegerOutput(node, spec.IndexOutput, frame.index, frame.attempt); err != nil {
			return err
		}
		value, err := datatype.SealInlineJSON(s.executor.catalog, node.OutputTypes[spec.ItemOutput], frame.items[frame.index])
		if err != nil {
			return fmt.Errorf("seal for-each item: %w", err)
		}
		if err := s.setInstructionOutput(node, spec.ItemOutput, value, frame.attempt); err != nil {
			return err
		}
		s.queue = s.instructionRoutes(node.ID, spec.BodyOutput, nil)
	case nodecontract.InstructionRetry:
		spec := node.Instruction.Retry
		if frame.index >= frame.limit {
			facts := map[string]string{}
			if frame.lastFailure != nil {
				facts["last_problem_id"] = frame.lastFailure.Code
			}
			if err := s.recordInstructionStatus(ctx, node, frame.attempt, nodecontract.RetryExhaustedStatusID,
				map[string]int64{"attempts": frame.limit, "max_attempts": frame.limit}, facts); err != nil {
				return err
			}
			return s.completeRegion(ctx, frame, spec.ExhaustedOutput, frame.lastFailure)
		}
		if err := s.recordInstructionStatus(ctx, node, frame.attempt, nodecontract.RetryAttemptStatusID,
			map[string]int64{"attempt": frame.index + 1, "max_attempts": frame.limit}, nil); err != nil {
			return err
		}
		if err := s.setInstructionIntegerOutput(node, spec.AttemptOutput, frame.index+1, frame.attempt); err != nil {
			return err
		}
		s.queue = s.instructionRoutes(node.ID, spec.BodyOutput, nil)
	}
	return nil
}

// resumeRegion is called after the body drains or routes a structured control
// signal. It advances exactly one frame; empty bodies also yield to the driver.
func (s *scheduler) resumeRegion(ctx context.Context, signal *regionSignal) error {
	frame := s.frames[len(s.frames)-1]
	if signal != nil {
		if err := s.closeWaits(ctx); err != nil {
			return err
		}
	}
	node := frame.node
	switch node.Instruction.Kind {
	case nodecontract.InstructionCountedLoop:
		if signal != nil && signal.input == node.Instruction.CountedLoop.BreakInput {
			return s.completeRegion(ctx, frame, node.Instruction.CountedLoop.CompletedOutput, nil)
		}
	case nodecontract.InstructionForEach:
		if signal != nil && signal.input == node.Instruction.ForEach.BreakInput {
			return s.completeRegion(ctx, frame, node.Instruction.ForEach.CompletedOutput, nil)
		}
	case nodecontract.InstructionRetry:
		if signal == nil {
			return s.completeRegion(ctx, frame, node.Instruction.Retry.CompletedOutput, nil)
		}
		frame.lastFailure = cloneRoutedFailure(signal.failure)
	}
	frame.index++
	return s.startRegionIteration(ctx, frame)
}

func (s *scheduler) completeRegion(ctx context.Context, frame *controlFrame, output string, failure *nodeadapter.RoutedFailure) error {
	if err := s.finishInstruction(ctx, frame.node, frame.attempt, frame.summary); err != nil {
		return err
	}
	s.popRegion()
	s.enqueueInstructionOutput(frame.node.ID, output, failure)
	return nil
}

func (s *scheduler) popRegion() {
	last := len(s.frames) - 1
	s.queue = s.frames[last].parent
	s.waits = s.frames[last].parentWaits
	s.frames[last] = nil
	s.frames = s.frames[:last]
}

func (s *scheduler) handleRegionSignal(ctx context.Context, signal *regionSignal) error {
	for len(s.frames) > 0 {
		frame := s.frames[len(s.frames)-1]
		if frame.node.ID == signal.nodeID {
			return s.resumeRegion(ctx, signal)
		}
		if err := s.closeWaits(ctx); err != nil {
			return err
		}
		if err := s.closeInterruptedInstruction(ctx, frame.node, frame.attempt, frame.summary, signal); err != nil {
			return err
		}
		s.popRegion()
	}
	return signal
}

func (s *scheduler) closeRegions(ctx context.Context, cause error) error {
	var result error
	for len(s.frames) > 0 {
		frame := s.frames[len(s.frames)-1]
		result = errors.Join(result, s.closeWaits(ctx), s.closeInterruptedInstruction(ctx, frame.node, frame.attempt, frame.summary, cause))
		s.popRegion()
	}
	return errors.Join(result, s.closeWaits(ctx))
}

func (s *scheduler) recordInstructionStatus(ctx context.Context, node programNode, attempt int, code string, counters map[string]int64, facts map[string]string) error {
	summary, err := run.NewRedactedSummary(code, counters, facts)
	if err != nil {
		return err
	}
	fact, err := run.NewNodeStatusFact(run.NodeStatusInput{
		GraphPath: append([]string(nil), node.GraphPath...), NodeID: node.SourceNodeID, Attempt: attempt,
		Code: code, Category: nodecontract.StatusProgress, OccurredAt: s.executor.now().UTC(), Summary: summary,
	})
	if err != nil {
		return err
	}
	_, err = s.journal.Append(context.WithoutCancel(ctx), fact)
	return err
}

func (s *scheduler) instructionRoutes(nodeID, output string, failure *nodeadapter.RoutedFailure) []scheduledInvocation {
	routes := s.routes[routeKey{channel: schema.EdgeExec, nodeID: nodeID, portID: output}]
	result := make([]scheduledInvocation, 0, len(routes))
	for _, route := range routes {
		result = append(result, scheduledInvocation{nodeID: route.To.NodeID, trigger: &nodeadapter.SignalTrigger{
			Channel: schema.EdgeExec, InputPort: route.To.PortID, From: route.From, Failure: cloneRoutedFailure(failure),
		}})
	}
	return result
}

func (s *scheduler) enqueueInstructionOutput(nodeID, output string, failure *nodeadapter.RoutedFailure) {
	s.queue = append(s.queue, s.instructionRoutes(nodeID, output, failure)...)
}

func (s *scheduler) beginInstruction(ctx context.Context, node programNode) (int, run.RedactedSummary, error) {
	s.attempts[node.ID]++
	attempt := s.attempts[node.ID]
	summary, err := run.NewRedactedSummary("node.execute", nil, nil)
	if err != nil {
		return 0, run.RedactedSummary{}, err
	}
	fact, err := run.NewNodeAttemptFact(run.NodeAttemptInput{
		GraphPath: append([]string(nil), node.GraphPath...), NodeID: node.SourceNodeID, Attempt: attempt, Outcome: run.AttemptStarted,
		OccurredAt: s.executor.now().UTC(), Summary: summary,
	})
	if err != nil {
		return 0, run.RedactedSummary{}, err
	}
	if _, err := s.journal.Append(ctx, fact); err != nil {
		return 0, run.RedactedSummary{}, err
	}
	s.clearNodeResult(node.ID)
	return attempt, summary, nil
}

func (s *scheduler) finishInstruction(ctx context.Context, node programNode, attempt int, summary run.RedactedSummary) error {
	fact, err := run.NewNodeAttemptFact(run.NodeAttemptInput{
		GraphPath: append([]string(nil), node.GraphPath...), NodeID: node.SourceNodeID, Attempt: attempt, Outcome: run.AttemptSucceeded,
		OccurredAt: s.executor.now().UTC(), Summary: summary,
	})
	if err != nil {
		return err
	}
	_, err = s.journal.Append(context.WithoutCancel(ctx), fact)
	return err
}

func (s *scheduler) closeInterruptedInstruction(ctx context.Context, node programNode, attempt int, summary run.RedactedSummary, cause error) error {
	var signal *regionSignal
	if errors.As(cause, &signal) {
		return s.finishInstruction(ctx, node, attempt, summary)
	}
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) || ctx.Err() != nil {
		return s.executor.cancelAttempt(context.WithoutCancel(ctx), s.journal, node.GraphPath, node.SourceNodeID, attempt, summary)
	}
	fact, err := run.NewNodeAttemptFact(run.NodeAttemptInput{
		GraphPath: append([]string(nil), node.GraphPath...), NodeID: node.SourceNodeID, Attempt: attempt, Outcome: run.AttemptRouted,
		OccurredAt: s.executor.now().UTC(), ErrorCode: "control.region_failed", Summary: summary,
	})
	if err != nil {
		return err
	}
	_, err = s.journal.Append(context.WithoutCancel(ctx), fact)
	return err
}

func (s *scheduler) setInstructionIntegerOutput(node programNode, portID string, value int64, attempt int) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	resolved, ok := node.OutputTypes[portID]
	if !ok {
		return fmt.Errorf("instruction output %q has no frozen type", portID)
	}
	sealed, err := datatype.SealInlineJSON(s.executor.catalog, resolved, raw)
	if err != nil {
		return err
	}
	return s.setInstructionOutput(node, portID, sealed, attempt)
}

func (s *scheduler) setInstructionOutput(node programNode, portID string, value datatype.ValueEnvelope, attempt int) error {
	previousBytes := 0
	if previous, ok := s.result.NodeOutputs[node.ID][portID]; ok {
		previousBytes = len(previous.RuntimeArtifact())
	}
	nextBytes := len(value.RuntimeArtifact())
	if !s.canRetain(nextBytes - previousBytes) {
		return errors.New("run retained value budget exceeded")
	}
	s.retainedBytes = s.retainedBytes - previousBytes + nextBytes
	if s.result.NodeOutputs[node.ID] == nil {
		s.result.NodeOutputs[node.ID] = make(map[string]datatype.ValueEnvelope)
		s.result.attempts[node.ID] = make(map[string]int)
	}
	s.result.NodeOutputs[node.ID][portID] = value
	s.result.attempts[node.ID][portID] = attempt
	return nil
}

func decodeInstructionInteger(value datatype.ValueEnvelope) (int64, error) {
	var result int64
	if !value.Valid() || json.Unmarshal(value.InlineJSON(), &result) != nil {
		return 0, errors.New("instruction integer input is invalid")
	}
	return result, nil
}
