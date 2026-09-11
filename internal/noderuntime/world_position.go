package noderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/yottaapp/yotta/internal/automation/navigation"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
)

func makeWorldPosition(b nodes.Builtins) nodeadapter.Adapter {
	return func(ctx context.Context, i nodeadapter.Invocation) (_ nodeadapter.AdapterResult, runErr error) {
		defer func() {
			if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
				runErr = automationFailure(nodes.NavigationFailedCode, runErr)
			}
			runErr = errors.Join(runErr, recordAdapterOutcome(ctx, i, nodeadapter.AdapterAction{EffectID: nodes.PositionObservedEffectID, Action: "navigation.observe-position", SummaryCode: "navigation.observe-position"}, nodes.NavigationFailedCode, runErr))
		}()
		x, e1 := numberInput(i, "x")
		y, e2 := numberInput(i, "y")
		heading, e3 := numberInput(i, "heading")
		sample, e4 := integerInput(i, "sample-at")
		sequence, e5 := integerInput(i, "sequence")
		var valid bool
		e6 := json.Unmarshal(i.Inputs["valid"].InlineJSON(), &valid)
		if err := errors.Join(e1, e2, e3, e4, e5, e6); err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		position := navigation.WorldPosition{X: x, Y: y, Heading: heading, Frame: navString(i, "frame", "world"), Unit: navString(i, "unit", "world"), AxisHeading: navNumber(i, "axisHeading", 0), AxisSign: int(navNumber(i, "axisSign", 1)), Valid: valid, ReceivedAt: i.ObservedAt.UnixMilli(), SampleAt: sample, Sequence: sequence, Epoch: navString(i, "epoch", "")}
		if position.SampleAt > 0 {
			position.ReceivedAt = min(position.ReceivedAt, position.SampleAt)
		}
		result, err := sealVisionOutputs(b, i, map[string]any{"position": position})
		result.ExecOutputs = []string{"done"}
		return result, err
	}
}

func parseWorldPosition(b nodes.Builtins) nodeadapter.Adapter {
	return func(ctx context.Context, i nodeadapter.Invocation) (_ nodeadapter.AdapterResult, runErr error) {
		defer func() {
			if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
				runErr = automationFailure(nodes.NavigationFailedCode, runErr)
			}
			runErr = errors.Join(runErr, recordAdapterOutcome(ctx, i, nodeadapter.AdapterAction{EffectID: nodes.PositionObservedEffectID, Action: "navigation.observe-position", SummaryCode: "navigation.observe-position"}, nodes.NavigationFailedCode, runErr))
		}()
		source, err := stringInput(i, "source")
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal([]byte(source), &document); err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		now := time.Now()
		pose, readErr := decodeNavigationPose([]byte(source), i, now)
		position := navigation.WorldPosition{X: pose.X, Y: pose.Y, Heading: pose.Heading, Frame: navString(i, "frame", "world"), Unit: navString(i, "unit", "world"), AxisHeading: navNumber(i, "axisHeading", 0), AxisSign: int(navNumber(i, "axisSign", 1)), Valid: readErr == nil, ReceivedAt: now.UnixMilli(), Epoch: navString(i, "epoch", "")}
		if readErr == nil {
			position.SampleAt = pose.Time.UnixMilli()
			position.Sequence = position.SampleAt
			// This parser's timestamp field is a checked Unix-millisecond sample
			// time. Parsing a delayed response must not make an old pose new.
			position.ReceivedAt = min(position.ReceivedAt, position.SampleAt)
		}
		if position.SampleAt > 0 {
			position.ReceivedAt = min(position.ReceivedAt, position.SampleAt)
		}
		result, err := sealVisionOutputs(b, i, map[string]any{"position": position})
		result.ExecOutputs = []string{"done"}
		return result, err
	}
}
