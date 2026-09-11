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
