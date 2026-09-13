// Package nodeadapter defines the host ABI implemented by built-in and plugin
// node adapters. It contains no compiler, scheduler, or Program ownership.
package nodeadapter

import (
	"context"
	"time"

	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodecatalog"
	"github.com/yottaapp/yotta/internal/problem"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/signals"
	"github.com/yottaapp/yotta/internal/targetruntime"
	"github.com/yottaapp/yotta/internal/workflow/schema"
)

type Adapter func(context.Context, Invocation) (AdapterResult, error)

// BranchRequest starts a declared child output of the current invocation.
// Outputs is a complete, immutable snapshot of the node's durable data outputs.
// Values are visible only to that child, never published as completed outputs.
type BranchRequest struct {
	Output  string
	Outputs map[string]datatype.ValueEnvelope
	// CooperativeInput shares physical keyboard coordination with the caller,
	// while preserving independent pause/cancellation ownership. Use false for
	// recovery and stopped actions. Non-keyboard mutations are rejected by the
	// installed input adapter; this is input composition, not permission.
	CooperativeInput bool
}

// BranchHandle belongs to one invocation. Await Done using Invocation.Await
// when the caller must join the branch; otherwise the caller can keep working.
// Err is read after Done closes. Invocation completion cancels unfinished
// branches, so callers must join branches whose work must finish first.
// Coalescing retains the original request's snapshot, context and input mode.
type BranchHandle interface {
	Done() <-chan struct{}
	Err() error
}

type AdapterResult struct {
	Outputs      map[string]datatype.ValueEnvelope
	ExecOutputs  []string
	Wait         *WaitRequest
	Subscription *SubscriptionRequest
}

// SubscriptionRequest supplies an event source. Only the executor routes events,
// starts branches, and owns the subscription's structured lifetime.
type SubscriptionRequest struct {
	Initial map[string]datatype.ValueEnvelope
	Ready   <-chan struct{}
	Poll    func() (map[string]datatype.ValueEnvelope, bool, error)
	Close   func(context.Context, error) error
}

// WaitRequest suspends an execution-only node until its duration elapses.
// The executor owns the timer and calls Complete exactly once on its own
// scheduling thread, including cancellation. No output or action is complete
// merely because the adapter returned this request.
// Data-producing asynchronous operations need their own activation values;
// this timer continuation intentionally exposes execution signals only.
type WaitRequest struct {
	Duration time.Duration
	Complete func(context.Context, error) ([]string, error)
}

type NodeFailure struct {
	Code   string
	Output string
	Params problem.Params
	Cause  error
}

func (failure *NodeFailure) Error() string {
	if failure == nil {
		return "node failure"
	}
	if failure.Cause != nil {
		return failure.Cause.Error()
	}
	return failure.Code
}

func (failure *NodeFailure) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.Cause
}

type RoutedFailure struct {
	Code         string
	Category     string
	RetryHint    bool
	SourceNodeID string
	SourcePortID string
	Attempt      int
	Params       problem.Params
}

type SignalTrigger struct {
	Channel   schema.EdgeChannel
	InputPort string
	From      schema.Endpoint
	Failure   *RoutedFailure
}

type InstalledAdapter struct {
	Implementation nodecatalog.ImplementationLock
	Run            Adapter
	// Blocking adapters execute on the Run's bounded worker pool. Their frozen
	// invocation may produce data, but only the scheduler publishes that data.
	Blocking bool
	// PauseAtWait requires boundaries without held physical input.
	PauseAtWait bool
}

type StateSnapshot struct {
	Value     datatype.ValueEnvelope
	Revision  int64
	ChangedAt time.Time
}

// StateBinding is invocation-scoped attenuated authority. Implementations are
// created by the Executor from the Program's immutable state access contract.
type StateBinding interface {
	Read() (StateSnapshot, error)
	Write(datatype.ValueEnvelope) (StateSnapshot, error)
	Update(func(datatype.ValueEnvelope) (datatype.ValueEnvelope, error)) (StateSnapshot, error)
}

type Invocation struct {
	// Branch is available only to blocking adapters with Invoke.Branches. It
	// waits for scheduler acknowledgement, not child completion. Calls from
	// the invocation worker are serial; coalescing is declared by the contract.
	Branch func(context.Context, BranchRequest) (BranchHandle, error)
	// HasBranch reads the frozen route lookup; undeclared or unwired outputs
	// return false. Branch's context owns the child lifetime after admission.
	// Cancel that context and await its handle to join an activity before
	// recovery. Expected activity cancellation does not fail the parent.
	HasBranch func(string) bool
	// WaitWithPause releases operation-owned input before acknowledging a
	// requested pause. The callback runs on the operation's worker.
	WaitWithPause func(context.Context, time.Duration, func(context.Context) error) error
	Await         func(context.Context, <-chan struct{}, time.Duration) error
	Signals       *signals.Hub
	RunID         string
	WorkflowID    string
	RunStartedAt  time.Time
	InvocationID  string
	Attempt       int
	GraphID       string
	NodeID        string
	Config        map[string]any
	Inputs        map[string]datatype.ValueEnvelope
	InputTypes    map[string]datatype.ResolvedType
	OutputTypes   map[string]datatype.ResolvedType
	Sessions      map[string]*run.Session
	Targets       *targetruntime.Run
	State         map[string]StateBinding
	Trigger       *SignalTrigger
	ObservedAt    time.Time
	MonotonicNow  func() time.Time
	ReadEntropy   func([]byte) error
	Wait          func(context.Context, time.Duration) error
	Spawn         func(func(context.Context) error) error
	RecordAction  func(context.Context, AdapterAction) error
	EmitStatus    func(context.Context, string, map[string]int64) error
}

type AdapterAction struct {
	EffectID    string
	Action      string
	Outcome     run.ActionOutcome
	ErrorCode   string
	SummaryCode string
	Counters    map[string]int64
	Facts       map[string]string
}
