package noderuntime

import (
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
	"strings"
)

// Old Source nodes remain explainable, but cannot create or modify definitions.
func panelAdapter(_ nodes.Builtins, id string) nodeadapter.Adapter {
	return func(ctx context.Context, inv nodeadapter.Invocation) (nodeadapter.AdapterResult, error) {
		kind := strings.TrimPrefix(id, nodes.PanelNodePrefix)
		failure := &nodeadapter.NodeFailure{Code: "panels.workflow_creation_retired", Output: "failed"}
		err := recordAdapterOutcome(ctx, inv, nodeadapter.AdapterAction{EffectID: nodes.PanelEffect(kind), Action: "panel." + kind, SummaryCode: "panel." + kind}, failure.Code, failure)
		return nodeadapter.AdapterResult{}, errors.Join(failure, err)
	}
}
