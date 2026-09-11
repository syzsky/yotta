package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/nodecontract"
	"github.com/yottaapp/yotta/internal/nodes"
)

func TestOfflineContractMigrationBacksUpAndPreservesNodeIdentity(t *testing.T) {
	b, err := nodes.Build()
	if err != nil {
		t.Fatal(err)
	}
	current := b.ConcatContract
	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(current.Bytes()))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	document["version"] = "2"
	old, err := artifact.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nodecontract.Open(old); err == nil {
		t.Fatal("desktop reader unexpectedly accepts retired format")
	}
	path := filepath.Join(t.TempDir(), "node.json")
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	if err := execute(path, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".before-runtime-v3"); !os.IsNotExist(err) {
		t.Fatal("preview mutated files")
	}
	if err := execute(path, true); err != nil {
		t.Fatal(err)
	}
	backup, _ := os.ReadFile(path + ".before-runtime-v3")
	if !bytes.Equal(backup, old) {
		t.Fatal("backup differs from original")
	}
	raw, _ := os.ReadFile(path)
	converted, err := nodecontract.Open(raw)
	if err != nil || converted.NodeRef() != current.NodeRef() {
		t.Fatalf("identity changed: %v", err)
	}
	if err := execute(path, true); err != nil {
		t.Fatal("migration is not idempotent", err)
	}
}
