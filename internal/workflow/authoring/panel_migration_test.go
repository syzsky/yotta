package authoring_test

import (
	"encoding/json"
	"github.com/yottaapp/yotta/internal/nodecontract"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/workflow/authoring"
	"github.com/yottaapp/yotta/internal/workflow/schema"
	"os"
	"reflect"
	"testing"
)

func TestPanelV1MigrationPreservesNodeConfiguration(t *testing.T) {
	raw, err := os.ReadFile("testdata/panel-v1-node-refs.json")
	if err != nil {
		t.Fatal(err)
	}
	var previous map[string]nodecontract.NodeRef
	if err = json.Unmarshal(raw, &previous); err != nil {
		t.Fatal(err)
	}
	builtins, projection := testContracts(t)
	if len(previous) != len(nodes.PanelKinds) {
		t.Fatal("incomplete frozen panel refs")
	}
	for id, ref := range previous {
		t.Run(id, func(t *testing.T) {
			engine, err := authoring.New(builtins.Catalog, projection, func() string { return "panel-node" })
			if err != nil {
				t.Fatal(err)
			}
			created, err := engine.Apply(emptySource(), []authoring.Command{{Kind: authoring.CommandAddNode, AddNode: &authoring.AddNodeCommand{GraphID: "main", NodeTypeID: id, Position: schema.Position{X: 17, Y: 23}}}})
			if err != nil {
				t.Fatal(err)
			}
			expected := created.Source.Graphs[0].Nodes[0]
			created.Source.Graphs[0].Nodes[0].NodeRef = ref
			upgraded, err := engine.Apply(created.Source, []authoring.Command{{Kind: authoring.CommandUpgradeNodeContract, UpgradeNodeContract: &authoring.NodeCommand{GraphID: "main", NodeID: "panel-node"}}})
			if err != nil {
				t.Fatal(err)
			}
			if got := upgraded.Source.Graphs[0].Nodes[0]; !reflect.DeepEqual(got, expected) {
				t.Fatalf("migration changed node contents: %+v", got)
			}
		})
	}
}
