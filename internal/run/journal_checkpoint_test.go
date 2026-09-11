package run

import (
	"github.com/yottaapp/yotta/internal/nodecontract"
	"testing"
	"time"
)

func TestJournalCheckpointHasNoLifetimeEventLimit(t *testing.T) {
	at := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	summary, err := NewRedactedSummary("node.execute", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	start, _ := NewNodeAttemptFact(NodeAttemptInput{GraphPath: []string{"main"}, NodeID: "watch", Attempt: 1, Outcome: AttemptStarted, OccurredAt: at, Summary: summary})
	status, _ := NewNodeStatusFact(NodeStatusInput{GraphPath: []string{"main"}, NodeID: "watch", Attempt: 1, Code: "watch.observed", Category: nodecontract.StatusProgress, OccurredAt: at, Summary: summary})
	cursor := newJournalCheckpoint()
	var entries []journalEntry
	for n := 1; n <= MaxJournalEntries+1024; n++ {
		entry := status.entry
		if n == 1 {
			entry = start.entry
		}
		entry.Sequence = uint64(n)
		entries = append(entries, entry)
		if len(entries) == JournalSegmentEntries {
			cursor, err = cursor.archive(entries, &at)
			if err != nil {
				t.Fatalf("event %d: %v", n, err)
			}
			entries = entries[:0]
		}
	}
	if cursor.Sequence <= MaxJournalEntries || len(cursor.Latest) != 1 || len(cursor.Active) != 1 {
		t.Fatal("history limit or growing attempt state")
	}
	if err := cursor.validate(&at); err != nil {
		t.Fatal(err)
	}
	cancelled, _ := NewAdapterActionFact(AdapterActionInput{GraphPath: []string{"main"}, NodeID: "watch", Attempt: 1, EffectID: "https://schemas.yotta.dev/effects/test/watch/v1", Action: "watch.observe", Outcome: ActionCancelled, OccurredAt: at, Summary: summary})
	entry := cancelled.entry
	entry.Sequence = cursor.Sequence + 1
	if err := cursor.append(entry, &at); err != nil {
		t.Fatal(err)
	}
	success, _ := NewNodeAttemptFact(NodeAttemptInput{GraphPath: []string{"main"}, NodeID: "watch", Attempt: 1, Outcome: AttemptSucceeded, OccurredAt: at, Summary: summary})
	entry = success.entry
	entry.Sequence = cursor.Sequence + 1
	if err := cursor.append(entry, &at); err == nil {
		t.Fatal("archived attempt lost its cancelled action")
	}
}
