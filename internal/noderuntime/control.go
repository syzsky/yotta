package noderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
)

func branch() nodeadapter.Adapter {
	return func(_ context.Context, invocation nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
		input, ok := invocation.Inputs["condition"]
		if !ok {
			return nodeadapter.AdapterResult{}, errors.New("branch condition is missing")
		}
		var condition bool
		if err := json.Unmarshal(input.InlineJSON(), &condition); err != nil {
			return nodeadapter.AdapterResult{}, fmt.Errorf("decode branch condition: %w", err)
		}
		selected := "false"
		if condition {
			selected = "true"
		}
		return nodeadapter.AdapterResult{ExecOutputs: []string{selected}}, nil
	}
}

func delay() nodeadapter.Adapter {
	return func(ctx context.Context, invocation nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
		counters := map[string]int64{}
		finish := func(ctx context.Context, cause error) ([]string, error) {
			err := cause
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				err = delayFailure(err)
			}
			err = errors.Join(err, recordAdapterOutcome(ctx, invocation, nodeadapter.AdapterAction{
				EffectID: nodes.DelayWaitEffectID, Action: "control.delay-completed", SummaryCode: "control.delay", Counters: counters,
			}, nodes.DelayFailedCode, err))
			if err != nil {
				return nil, err
			}
			return []string{"done"}, nil
		}
		duration, err := integerInput(invocation, "duration-milliseconds")
		if err != nil || duration < 0 || duration > nodes.MaxDelayMilliseconds {
			_, err = finish(ctx, errors.Join(errors.New("delay duration is outside its supported range"), err))
			return nodeadapter.AdapterResult{}, err
		}
		if invocation.EmitStatus == nil {
			_, err := finish(ctx, errors.New("delay status host function is missing"))
			return nodeadapter.AdapterResult{}, err
		}
		counters["duration"] = duration
		if err := invocation.EmitStatus(ctx, nodes.DelayWaitingStatus, counters); err != nil {
			_, err = finish(ctx, err)
			return nodeadapter.AdapterResult{}, err
		}
		return nodeadapter.AdapterResult{Wait: &nodeadapter.WaitRequest{
			Duration: time.Duration(duration) * time.Millisecond, Complete: finish,
		}}, nil
	}
}

func delayFailure(err error) error {
	return &nodeadapter.NodeFailure{Code: nodes.DelayFailedCode, Output: "failed", Cause: err}
}

func endBranch() nodeadapter.Adapter {
	return func(context.Context, nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
		return nodeadapter.AdapterResult{}, nil
	}
}
