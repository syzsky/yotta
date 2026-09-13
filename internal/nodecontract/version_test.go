package nodecontract

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/yottaapp/yotta/internal/artifact"
)

func TestOpenV3PreservesNodeRefAndSemanticArtifact(t *testing.T) {
	draft := concatContractDraftForTest()
	draft.Authoring.Tags = []string{"a", "z"}
	c, err := Seal(draft)
	if err != nil {
		t.Fatal(err)
	}
	var doc document
	if err := json.Unmarshal(c.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	doc.Version = "3"
	raw, err := artifact.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := Open(raw)
	if err != nil {
		t.Fatal(err)
	}
	if opened.NodeRef() != c.NodeRef() || !bytes.Equal(opened.SemanticBytes(), c.SemanticBytes()) || !bytes.Equal(opened.Bytes(), c.Bytes()) {
		t.Fatal("v3 upgrade changed stable node identity or semantic bytes")
	}
	// Existing golden identity predates branch support; omitting branches must
	// not invalidate every installed node reference when the envelope changes.
	if c.NodeRef().SemanticDigest != "sha256:e88214ec5b6d09f16d3cfec785ce809e6b62274d3f2f136502456cdbe8972f9c" {
		t.Fatal("existing node semantic digest changed")
	}
	doc.Authoring.Tags = []string{"z", "a"}
	raw, err = artifact.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(raw); err == nil {
		t.Fatal("v3 reader accepted non-normalized authoring")
	}
}

func TestBranchContractCannotUseLegacyEnvelope(t *testing.T) {
	c, err := Seal(branchDraftForTest())
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1", "2", "3", "5"} {
		t.Run(version, func(t *testing.T) {
			var doc document
			if err := json.Unmarshal(c.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			doc.Version = version
			raw, err := artifact.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Open(raw); err == nil {
				t.Fatal("new branch semantics were accepted under unsupported envelope")
			}
		})
	}
}
