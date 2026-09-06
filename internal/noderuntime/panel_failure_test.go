package noderuntime

import (
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
	"testing"
)

func TestRetiredPanelNodeCannotCreateDefinitions(t *testing.T) {
	inv := nodeadapter.Invocation{RecordAction: func(context.Context, nodeadapter.AdapterAction) error { return nil }}
	_, err := panelAdapter(nodes.Builtins{}, nodes.PanelNodePrefix+"create")(context.Background(), inv)
	var f *nodeadapter.NodeFailure
	if !errors.As(err, &f) || f.Code != "panels.workflow_creation_retired" {
		t.Fatal(err)
	}
}
