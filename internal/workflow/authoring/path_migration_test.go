package authoring_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/blob"
	"github.com/yottaapp/yotta/internal/navigationpath"
	"github.com/yottaapp/yotta/internal/nodecontract"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/workflow/authoring"
	"github.com/yottaapp/yotta/internal/workflow/schema"
)

func TestPathFollowerUpgradePreservesRecordedPathBindingsAndSettings(t *testing.T) {
	builtins, projection := testContracts(t)
	for id, digest := range map[string]string{
		nodes.FollowPathNodeID:      "sha256:d1417bed7085cd3b433602442dd5149e4c1a195a02c0c90907f3333189f6b568",
		nodes.FollowSavedPathNodeID: "sha256:a78eef488c0a48bb74ce24423d366acb6449933329052d46dfc75676f43cf85b",
	} {
		t.Run(id, func(t *testing.T) {
			engine, err := authoring.New(builtins.Catalog, projection, func() string { return "path-node" })
			if err != nil {
				t.Fatal(err)
			}
			created, err := engine.Apply(emptySource(), []authoring.Command{{Kind: authoring.CommandAddNode, AddNode: &authoring.AddNodeCommand{GraphID: "main", NodeTypeID: id, Position: schema.Position{X: 17, Y: 23}}}})
			if err != nil {
				t.Fatal(err)
			}
			node := &created.Source.Graphs[0].Nodes[0]
			node.Config = map[string]any{"slot": "game", "position-variable": "position-source", "forwardKey": "W", "turnSign": float64(-1)}
			node.Bindings["start"] = schema.InputBinding{Kind: schema.BindingValue, Value: json.RawMessage(`7`)}
			if id == nodes.FollowSavedPathNodeID {
				node.Bindings["asset"] = schema.InputBinding{Kind: schema.BindingBlob, Blob: &blob.BlobRef{Digest: artifact.Digest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), MediaType: navigationpath.MediaType, Size: 7600}}
			} else {
				node.Bindings["path"] = schema.InputBinding{Kind: schema.BindingValue, Value: json.RawMessage(`{"version":1,"reference":{"kind":"world","frame":"test","unit":"m","axisHeading":0,"axisSign":1},"points":[{"id":"a","x":1,"y":2}]}`)}
			}
			for key, binding := range node.Bindings {
				if binding.Kind == schema.BindingValue {
					binding.Value, err = artifact.Canonicalize(binding.Value)
					if err != nil {
						t.Fatal(err)
					}
					node.Bindings[key] = binding
				}
			}
			expected := *node
			node.NodeRef = nodecontract.NodeRef{NodeTypeID: id, Version: "1.0.0", SemanticDigest: artifact.Digest(digest)}
			upgraded, err := engine.Apply(created.Source, []authoring.Command{{Kind: authoring.CommandUpgradeNodeContract, UpgradeNodeContract: &authoring.NodeCommand{GraphID: "main", NodeID: "path-node"}}})
			if err != nil {
				t.Fatal(err)
			}
			got := upgraded.Source.Graphs[0].Nodes[0]
			for key, value := range map[string]any{"recovery-attempts": float64(2), "action-interval": float64(500), "marker-mode": "continue"} {
				if !reflect.DeepEqual(got.Config[key], value) {
					t.Fatalf("missing new default %s: %#v", key, got.Config[key])
				}
				delete(got.Config, key)
			}
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("path upgrade changed settings: got=%+v expected=%+v", got, expected)
			}
		})
	}
}
