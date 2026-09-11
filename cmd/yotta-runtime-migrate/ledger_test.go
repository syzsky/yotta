package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/capability"
	"github.com/yottaapp/yotta/internal/nodes"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/storage"
	"github.com/yottaapp/yotta/internal/storage/catalog"
)

func TestOfflineLedgerMigrationPreservesHistoryAndBacksUpWAL(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	digest, err := artifact.Sum("yotta/test/v1", []byte("identity"))
	if err != nil {
		t.Fatal(err)
	}
	builtins, err := nodes.Build()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := capability.SealPlan(nil)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := capability.SealRunGrant(capability.GrantRequest{ProgramHash: digest, Plan: plan, RunID: "0190c7d4-1e40-7cc5-a783-57b16d5c8e3a", Principal: "test", PolicyGeneration: "test", IssuedAt: at}, builtins.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	current, err := run.NewQueuedRecord(run.QueueRequest{ProgramHash: digest, CatalogHash: digest, CapabilityPlanDigest: plan.Digest(), Grant: grant, QueuedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	current, err = current.Start(at)
	if err != nil {
		t.Fatal(err)
	}
	var full []json.RawMessage
	var events []catalog.RunEventRecord
	summary, _ := run.NewRedactedSummary("node.execute", nil, nil)
	for n := 1; n <= 140; n++ {
		for _, outcome := range []run.AttemptOutcome{run.AttemptStarted, run.AttemptSucceeded} {
			fact, err := run.NewNodeAttemptFact(run.NodeAttemptInput{GraphPath: []string{"main"}, NodeID: "tick", Attempt: n, Outcome: outcome, OccurredAt: at, Summary: summary})
			if err != nil {
				t.Fatal(err)
			}
			current, err = current.AppendJournal(fact)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Journal []json.RawMessage `json:"journal"`
			}
			if err := json.Unmarshal(current.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			raw := doc.Journal[len(doc.Journal)-1]
			full = append(full, raw)
			events = append(events, catalog.RunEventRecord{Sequence: uint64(len(full)), Kind: string(run.JournalNodeAttempt), OccurredAt: at, Artifact: raw})
		}
	}
	current, err = current.Succeed(at, builtins.Catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	old, err := decodeDocument(current.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	delete(old, "checkpoint")
	old["version"], old["journal"], old["recordDigest"] = "1", full, ""
	body, _ := artifact.Marshal(old)
	oldDigest, _ := artifact.Sum("yotta/run-record/v1", body)
	old["recordDigest"] = oldDigest.String()
	oldRaw, _ := artifact.Marshal(old)
	converted, err := migrateRunRecord(oldRaw, builtins.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	if converted.JournalCount() != 280 || len(converted.Journal()) >= 280 {
		t.Fatal("conversion lost history or failed to compact")
	}
	old["recordDigest"] = digest.String()
	corrupted, _ := artifact.Marshal(old)
	if _, err := migrateRunRecord(corrupted, builtins.Catalog); err == nil {
		t.Fatal("accepted tampered old head")
	}
	old["recordDigest"] = oldDigest.String()
	for _, key := range []string{"generation", "recordDigest", "journal", "values"} {
		delete(old, key)
	}
	old["format"], old["version"] = run.LedgerSummaryFormat, "1"
	summaryRaw, _ := artifact.Marshal(old)
	root := t.TempDir()
	profile, err := storage.Open(ctx, storage.OpenOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	roots := profile.Roots
	foundation, err := catalog.Open(ctx, roots)
	if err != nil {
		t.Fatal(err)
	}
	end := at
	stored := catalog.RunLedgerRecord{Summary: catalog.RunSummaryRecord{RunID: grant.RunID(), Generation: current.Generation(), Digest: oldDigest, Status: string(run.StatusSucceeded), QueuedAt: at, StartedAt: &end, EndedAt: &end, SummaryArtifact: summaryRaw, JournalCount: 280, UpdatedAt: at}, Events: events}
	if err := foundation.Runs().Create(ctx, stored); err != nil {
		t.Fatal(err)
	}
	if err := migrateLedger(root, true); err == nil {
		t.Fatal("migration bypassed active profile lease")
	}
	foundation.Close()
	profile.Close()
	if err := migrateLedger(root, false); err != nil {
		t.Fatal(err)
	}
	if err := migrateLedger(root, true); err != nil {
		t.Fatal(err)
	}
	if err := migrateLedger(root, true); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(roots.Backups, "before-event-runtime-*", "manifest.json"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
	if info, err := os.Stat(backups[0]); err != nil || info.Size() == 0 {
		t.Fatal("missing backup manifest")
	}
	foundation, err = catalog.Open(ctx, roots)
	if err != nil {
		t.Fatal(err)
	}
	defer foundation.Close()
	loaded, err := foundation.Runs().Get(ctx, stored.Summary.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Summary.Digest != converted.Digest() || len(loaded.Events) != 280 {
		t.Fatal("migrated head/history mismatch")
	}
	for n := range events {
		if !bytes.Equal(events[n].Artifact, loaded.Events[n].Artifact) {
			t.Fatal("event payload changed")
		}
	}
}
