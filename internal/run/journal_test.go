package run_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/nodecontract"
	run "github.com/yottaapp/yotta/internal/run"
)

func TestRunJournalPersistsAppendOnlyAttemptAndAdapterFacts(t *testing.T) {
	catalog, _ := stringValueCatalog(t)
	root := t.TempDir()
	store, err := openRunStore(t, root, catalog, run.StoreOptions{MaxRecords: 8})
	if err != nil {
		t.Fatal(err)
	}
	queuedAt := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
	queued := queuedRecord(t, queuedAt)
	if _, err := store.Create(context.Background(), queued); err != nil {
		t.Fatal(err)
	}
	running, err := queued.Start(queuedAt.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), queued.Digest(), running); err != nil {
		t.Fatal(err)
	}

	summary, err := run.NewRedactedSummary("blob.stream", map[string]int64{"bytes": 4}, map[string]string{"request_id": "req_123"})
	if err != nil {
		t.Fatal(err)
	}
	started := nodeAttemptFact(t, queuedAt.Add(2*time.Second), run.AttemptStarted, "", summary)
	withStart, err := running.AppendJournal(started)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), running.Digest(), withStart); err != nil {
		t.Fatal(err)
	}
	action, err := run.NewAdapterActionFact(run.AdapterActionInput{
		GraphPath: []string{"main"}, NodeID: "convert", EffectID: "https://schemas.yotta.dev/effects/blob/read/v1",
		Attempt: 1, Action: "blob.open-reader", Outcome: run.ActionSucceeded,
		OccurredAt: queuedAt.Add(3 * time.Second), Summary: summary,
	})
	if err != nil {
		t.Fatal(err)
	}
	withAction, err := withStart.AppendJournal(action)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), withStart.Digest(), withAction); err != nil {
		t.Fatal(err)
	}
	finished := nodeAttemptFact(t, queuedAt.Add(4*time.Second), run.AttemptSucceeded, "", summary)
	withFinish, err := withAction.AppendJournal(finished)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), withAction.Digest(), withFinish); err != nil {
		t.Fatal(err)
	}
	writer, err := store.OpenJournal(testRunID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Succeed(context.Background(), queuedAt.Add(5*time.Second), catalog, nil); err != nil {
		t.Fatal(err)
	}

	reopened, err := openRunStore(t, root, catalog, run.StoreOptions{MaxRecords: 8})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Load(testRunID)
	if err != nil {
		t.Fatal(err)
	}
	journal := loaded.Journal()
	if loaded.Status() != run.StatusSucceeded || len(journal) != 3 || journal[0].Sequence != 1 || journal[1].Kind != run.JournalAdapterAction ||
		journal[2].AttemptOutcome != run.AttemptSucceeded || journal[1].Summary.Counters["bytes"] != 4 ||
		journal[1].Summary.Facts["request_id"] != "req_123" {
		t.Fatalf("journal = %#v", journal)
	}
}

func TestRunJournalRejectsInvalidOrderingAndMutableHistory(t *testing.T) {
	queuedAt := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
	running, err := queuedRecord(t, queuedAt).Start(queuedAt.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	summary, err := run.NewRedactedSummary("node.execute", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	action, err := run.NewAdapterActionFact(run.AdapterActionInput{
		GraphPath: []string{"main"}, NodeID: "convert", EffectID: "https://schemas.yotta.dev/effects/blob/read/v1",
		Attempt: 1, Action: "blob.open-reader", Outcome: run.ActionSucceeded,
		OccurredAt: queuedAt.Add(2 * time.Second), Summary: summary,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := running.AppendJournal(action); !errors.Is(err, run.ErrJournalOrder) {
		t.Fatalf("adapter action before attempt = %v", err)
	}
	started := nodeAttemptFact(t, queuedAt.Add(2*time.Second), run.AttemptStarted, "", summary)
	withStart, err := running.AppendJournal(started)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := withStart.AppendJournal(started); !errors.Is(err, run.ErrJournalOrder) {
		t.Fatalf("duplicate attempt start = %v", err)
	}
	failed := nodeAttemptFact(t, queuedAt.Add(3*time.Second), run.AttemptFailed, "adapter.open_failed", summary)
	withFailure, err := withStart.AppendJournal(failed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := withFailure.AppendJournal(action); !errors.Is(err, run.ErrJournalOrder) {
		t.Fatalf("adapter action after terminal attempt = %v", err)
	}
}

func TestRunJournalPersistsStatusDuringAttemptAndAllowsRoutedFailure(t *testing.T) {
	catalog, _ := stringValueCatalog(t)
	queuedAt := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
	running, err := queuedRecord(t, queuedAt).Start(queuedAt.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	summary, err := run.NewRedactedSummary("node.progress", map[string]int64{"percent": 50}, nil)
	if err != nil {
		t.Fatal(err)
	}
	status, err := run.NewNodeStatusFact(run.NodeStatusInput{
		GraphPath: []string{"main"}, NodeID: "convert", Attempt: 1, Code: "conversion.progress",
		Category: nodecontract.StatusProgress, OccurredAt: queuedAt.Add(3 * time.Second), Summary: summary,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := running.AppendJournal(status); !errors.Is(err, run.ErrJournalOrder) {
		t.Fatalf("status outside active attempt = %v", err)
	}
	startedSummary, err := run.NewRedactedSummary("node.execute", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	current, err := running.AppendJournal(nodeAttemptFact(t, queuedAt.Add(2*time.Second), run.AttemptStarted, "", startedSummary))
	if err != nil {
		t.Fatal(err)
	}
	current, err = current.AppendJournal(status)
	if err != nil {
		t.Fatal(err)
	}
	current, err = current.AppendJournal(nodeAttemptFact(t, queuedAt.Add(4*time.Second), run.AttemptRouted, "conversion.failed", startedSummary))
	if err != nil {
		t.Fatal(err)
	}
	succeeded, err := current.Succeed(queuedAt.Add(5*time.Second), catalog, nil)
	if err != nil {
		t.Fatalf("handled failure could not complete Run: %v", err)
	}
	journal := succeeded.Journal()
	if len(journal) != 3 || journal[1].Kind != run.JournalNodeStatus || journal[1].StatusCode != "conversion.progress" ||
		journal[1].StatusCategory != nodecontract.StatusProgress || journal[2].AttemptOutcome != run.AttemptRouted {
		t.Fatalf("journal = %#v", journal)
	}
}

func TestRunJournalRejectsAttributionThatCanCarryPathsOrPrompts(t *testing.T) {
	summary, err := run.NewRedactedSummary("node.execute", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, nodeID := range []string{`C:\secrets\api-key`, "ignore previous instructions"} {
		if _, err := run.NewNodeAttemptFact(run.NodeAttemptInput{
			GraphPath: []string{"main"}, NodeID: nodeID, Attempt: 1, Outcome: run.AttemptStarted,
			OccurredAt: time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC), Summary: summary,
		}); err == nil {
			t.Fatalf("accepted unsafe node attribution %q", nodeID)
		}
	}
}

func TestRunJournalKeepsChildCancellationDistinctFromRunSuccess(t *testing.T) {
	catalog, _ := stringValueCatalog(t)
	queuedAt := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
	summary, err := run.NewRedactedSummary("node.execute", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name           string
		actionOutcome  run.ActionOutcome
		actionError    string
		attemptOutcome run.AttemptOutcome
		attemptError   string
	}{
		{name: "failed action", actionOutcome: run.ActionFailed, actionError: "adapter.open_failed", attemptOutcome: run.AttemptFailed, attemptError: "adapter.open_failed"},
		{name: "cancelled action", actionOutcome: run.ActionCancelled, attemptOutcome: run.AttemptCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			running, err := queuedRecord(t, queuedAt).Start(queuedAt.Add(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			withStart, err := running.AppendJournal(nodeAttemptFact(t, queuedAt.Add(2*time.Second), run.AttemptStarted, "", summary))
			if err != nil {
				t.Fatal(err)
			}
			action, err := run.NewAdapterActionFact(run.AdapterActionInput{
				GraphPath: []string{"main"}, NodeID: "convert", EffectID: "https://schemas.yotta.dev/effects/blob/read/v1",
				Attempt: 1, Action: "blob.open-reader", Outcome: test.actionOutcome, ErrorCode: test.actionError,
				OccurredAt: queuedAt.Add(3 * time.Second), Summary: summary,
			})
			if err != nil {
				t.Fatal(err)
			}
			withAction, err := withStart.AppendJournal(action)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := withAction.AppendJournal(nodeAttemptFact(t, queuedAt.Add(4*time.Second), run.AttemptSucceeded, "", summary)); !errors.Is(err, run.ErrJournalOrder) {
				t.Fatalf("successful attempt after %s action = %v", test.actionOutcome, err)
			}
			withTerminal, err := withAction.AppendJournal(nodeAttemptFact(t, queuedAt.Add(4*time.Second), test.attemptOutcome, test.attemptError, summary))
			if err != nil {
				t.Fatal(err)
			}
			_, err = withTerminal.Succeed(queuedAt.Add(5*time.Second), catalog, nil)
			if test.attemptOutcome == run.AttemptCancelled && err != nil {
				t.Fatalf("completed child cancellation prevents enclosing Run success: %v", err)
			}
			if test.attemptOutcome == run.AttemptFailed && err == nil {
				t.Fatal("successful Run accepted failed child")
			}
		})
	}
}

func TestRunJournalUsesFailureWhenAnAttemptHasFailedAndCancelledActions(t *testing.T) {
	queuedAt := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
	summary, err := run.NewRedactedSummary("node.execute", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, outcomes := range [][]run.ActionOutcome{
		{run.ActionFailed, run.ActionCancelled},
		{run.ActionCancelled, run.ActionFailed},
	} {
		running, err := queuedRecord(t, queuedAt).Start(queuedAt.Add(time.Second))
		if err != nil {
			t.Fatal(err)
		}
		current, err := running.AppendJournal(nodeAttemptFact(t, queuedAt.Add(2*time.Second), run.AttemptStarted, "", summary))
		if err != nil {
			t.Fatal(err)
		}
		for index, outcome := range outcomes {
			code := ""
			if outcome == run.ActionFailed {
				code = "adapter.open_failed"
			}
			action, err := run.NewAdapterActionFact(run.AdapterActionInput{
				GraphPath: []string{"main"}, NodeID: "convert", EffectID: fmt.Sprintf("https://schemas.yotta.dev/effects/test/%d/v1", index+1),
				Attempt: 1, Action: fmt.Sprintf("test.action-%d", index+1), Outcome: outcome, ErrorCode: code,
				OccurredAt: queuedAt.Add(time.Duration(3+index) * time.Second), Summary: summary,
			})
			if err != nil {
				t.Fatal(err)
			}
			current, err = current.AppendJournal(action)
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err := current.AppendJournal(nodeAttemptFact(t, queuedAt.Add(5*time.Second), run.AttemptFailed, "adapter.open_failed", summary)); err != nil {
			t.Fatalf("failed terminal after mixed action outcomes = %v", err)
		}
	}
}

func TestJournalWriterRejectsASecondRecordOwner(t *testing.T) {
	catalog, _ := stringValueCatalog(t)
	store, err := openRunStore(t, t.TempDir(), catalog, run.StoreOptions{MaxRecords: 8})
	if err != nil {
		t.Fatal(err)
	}
	queuedAt := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
	queued := queuedRecord(t, queuedAt)
	if _, err := store.Create(context.Background(), queued); err != nil {
		t.Fatal(err)
	}
	running, err := queued.Start(queuedAt.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), queued.Digest(), running); err != nil {
		t.Fatal(err)
	}
	first, err := store.OpenJournal(testRunID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.OpenJournal(testRunID)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := run.NewRedactedSummary("node.execute", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Append(context.Background(), nodeAttemptFact(t, queuedAt.Add(2*time.Second), run.AttemptStarted, "", summary)); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Append(context.Background(), nodeAttemptFact(t, queuedAt.Add(2*time.Second), run.AttemptStarted, "", summary)); !errors.Is(err, run.ErrRunConflict) {
		t.Fatalf("second journal owner = %v", err)
	}
}

func nodeAttemptFact(t *testing.T, at time.Time, outcome run.AttemptOutcome, code string, summary run.RedactedSummary) run.JournalFact {
	t.Helper()
	fact, err := run.NewNodeAttemptFact(run.NodeAttemptInput{
		GraphPath: []string{"main"}, NodeID: "convert", Attempt: 1, Outcome: outcome,
		OccurredAt: at, ErrorCode: code, Summary: summary,
	})
	if err != nil {
		t.Fatal(err)
	}
	return fact
}

// A long-lived main attempt spans several archived segments while periodic
// child attempts finish. Reopening must preserve both the active main and all
// historical events without loading the entire history into the Run record.
func TestRunJournalSegmentsPreserveLongLivedAttemptsAndFullTimeline(t *testing.T) {
	types, _ := stringValueCatalog(t)
	store, err := openRunStore(t, t.TempDir(), types, run.StoreOptions{MaxRecords: 8})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	at := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	queued := queuedRecord(t, at)
	if _, err := store.Create(ctx, queued); err != nil {
		t.Fatal(err)
	}
	current, err := queued.Start(at)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(ctx, queued.Digest(), current); err != nil {
		t.Fatal(err)
	}
	writer, err := store.OpenJournal(testRunID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := run.NewRedactedSummary("node.execute", nil, nil)
	appendAttempt := func(node string, attempt int, outcome run.AttemptOutcome, stamp time.Time) {
		t.Helper()
		fact, err := run.NewNodeAttemptFact(run.NodeAttemptInput{GraphPath: []string{"main"}, NodeID: node, Attempt: attempt, Outcome: outcome, OccurredAt: stamp, Summary: summary})
		if err != nil {
			t.Fatal(err)
		}
		current, err = writer.Append(ctx, fact)
		if err != nil {
			t.Fatalf("append %s/%d/%s: %v", node, attempt, outcome, err)
		}
		if len(current.Journal()) > run.JournalSegmentEntries {
			t.Fatal("unbounded retained history")
		}
	}
	appendAttempt("main", 1, run.AttemptStarted, at)
	const ticks = 300
	for attempt := 1; attempt <= ticks; attempt++ {
		appendAttempt("tick", attempt, run.AttemptStarted, at.Add(time.Duration(attempt)*time.Millisecond))
		appendAttempt("tick", attempt, run.AttemptSucceeded, at.Add(time.Duration(attempt)*time.Millisecond))
		if attempt == 128 || attempt == 256 {
			loaded, err := store.Load(testRunID)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Digest() != current.Digest() || loaded.JournalCount() != current.JournalCount() {
				t.Fatal("checkpoint head changed on reopen")
			}
			writer, err = store.OpenJournal(testRunID)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	// An asynchronous fact may be delivered after newer facts from another
	// scope; append sequence, rather than wall time, establishes event order.
	appendAttempt("main", 1, run.AttemptSucceeded, at.Add(time.Millisecond))
	if _, err := writer.Succeed(ctx, at.Add(time.Second), types, nil); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(testRunID)
	if err != nil {
		t.Fatal(err)
	}
	const count = 2*ticks + 2
	if loaded.JournalCount() != count || len(loaded.Journal()) >= count {
		t.Fatal("history not segmented")
	}
	records, err := store.List()
	if err != nil || len(records) != 1 || records[0].Digest() != loaded.Digest() {
		t.Fatalf("list: %v", err)
	}
	seen := map[uint64]bool{}
	for page := 1; ; page++ {
		result, err := store.TimelinePage(ctx, testRunID, page, 100)
		if err != nil {
			t.Fatal(err)
		}
		if result.Total != count {
			t.Fatalf("timeline total %d", result.Total)
		}
		for _, entry := range result.Entries {
			if seen[entry.Sequence] {
				t.Fatal("duplicate event")
			}
			seen[entry.Sequence] = true
		}
		if page == result.Pages {
			break
		}
	}
	if len(seen) != count || !seen[1] || !seen[count] {
		t.Fatal("archived events lost")
	}
	exported, err := store.TimelineSnapshot(ctx, testRunID)
	if err != nil || len(exported.Entries) != count || exported.Summary.Digest != loaded.Digest() {
		t.Fatalf("export lost archived events: %v", err)
	}

}
