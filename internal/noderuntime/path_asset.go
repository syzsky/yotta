package noderuntime

import (
	"context"
	"errors"

	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/navigationpath"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
)

func readPath(b nodes.Builtins) nodeadapter.Adapter {
	return func(ctx context.Context, i nodeadapter.Invocation) (_ nodeadapter.AdapterResult, runErr error) {
		defer func() {
			var failure *nodeadapter.NodeFailure
			if !errors.As(runErr, &failure) && runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
				runErr = &nodeadapter.NodeFailure{Code: nodes.PathInvalidCode, Output: "failed", Cause: runErr}
			}
			runErr = errors.Join(runErr, recordAdapterOutcome(ctx, i, nodeadapter.AdapterAction{EffectID: nodes.ReadPathEffectID, Action: "navigation.read-path", SummaryCode: "navigation.read-path"}, nodes.PathInvalidCode, runErr))
		}()
		raw, _, err := readBlobInput(ctx, i, "asset", navigationpath.MediaType, navigationpath.MaxEncodedBytes)
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		if len(raw) > datatype.MaxInlineValueBytes {
			return nodeadapter.AdapterResult{}, &nodeadapter.NodeFailure{Code: nodes.PathInlineBudgetCode, Output: "failed", Cause: errors.New("use follow-saved-path to run this route without inline transfer")}
		}
		path, err := navigationpath.Decode(raw)
		if err != nil {
			return nodeadapter.AdapterResult{}, err
		}
		result, err := sealVisionOutputs(b, i, map[string]any{"path": path})
		result.ExecOutputs = []string{"completed"}
		return result, err
	}
}
