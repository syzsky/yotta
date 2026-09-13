package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodes"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/storage"
	"github.com/yottaapp/yotta/internal/storage/catalog"
)

func decodeDocument(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	canonical, err := artifact.Marshal(document)
	if err != nil || !bytes.Equal(canonical, raw) {
		return nil, errors.New("expected canonical artifact")
	}
	return document, nil
}

func migrateRunRecord(raw []byte, types datatype.ValueTypeCatalog) (run.Record, error) {
	doc, err := decodeDocument(raw)
	if err != nil {
		return run.Record{}, err
	}
	if doc["format"] != run.RecordFormat || doc["version"] != "1" {
		return run.Record{}, errors.New("expected development RunRecord v1")
	}
	original := doc["recordDigest"]
	doc["recordDigest"] = ""
	body, err := artifact.Marshal(doc)
	if err != nil {
		return run.Record{}, err
	}
	digest, err := artifact.Sum("yotta/run-record/v1", body)
	if err != nil || original != digest.String() {
		return run.Record{}, errors.New("old RunRecord digest mismatch")
	}
	doc["version"] = run.RecordVersion
	body, err = artifact.Marshal(doc)
	if err != nil {
		return run.Record{}, err
	}
	digest, err = artifact.Sum("yotta/run-record/v1", body)
	if err != nil {
		return run.Record{}, err
	}
	doc["recordDigest"] = digest.String()
	converted, err := artifact.Marshal(doc)
	if err != nil {
		return run.Record{}, err
	}
	return run.CompactRecord(converted, types)
}

type ledgerConversion struct {
	id             string
	previous, next artifact.Digest
	summary        []byte
}

func convertLedger(stored catalog.RunLedgerRecord, types datatype.ValueTypeCatalog) (ledgerConversion, error) {
	summary, err := decodeDocument(stored.Summary.SummaryArtifact)
	if err != nil {
		return ledgerConversion{}, err
	}
	if summary["format"] != run.LedgerSummaryFormat || summary["version"] != "1" {
		return ledgerConversion{}, errors.New("expected development Run summary v1")
	}
	summary["format"], summary["version"] = run.RecordFormat, "1"
	summary["generation"] = stored.Summary.Generation
	summary["recordDigest"] = stored.Summary.Digest.String()
	events := make([]json.RawMessage, 0, len(stored.Events))
	for _, e := range stored.Events {
		events = append(events, e.Artifact)
	}
	values := make([]json.RawMessage, 0, len(stored.Values))
	for _, v := range stored.Values {
		values = append(values, v.Artifact)
	}
	summary["journal"], summary["values"] = events, values
	raw, err := artifact.Marshal(summary)
	if err != nil {
		return ledgerConversion{}, err
	}
	converted, err := migrateRunRecord(raw, types)
	if err != nil {
		return ledgerConversion{}, err
	}
	current, err := decodeDocument(converted.Bytes())
	if err != nil {
		return ledgerConversion{}, err
	}
	for _, key := range []string{"generation", "recordDigest", "journal", "values"} {
		delete(current, key)
	}
	current["format"], current["version"] = run.LedgerSummaryFormat, run.LedgerSummaryVersion
	summaryRaw, err := artifact.Marshal(current)
	return ledgerConversion{id: stored.Summary.RunID, previous: stored.Summary.Digest, next: converted.Digest(), summary: summaryRaw}, err
}

// Holds the same profile writer lease as the desktop application throughout
// validation, WAL-aware backup and one atomic transaction. Event/value rows
// are never rewritten: only the head projection changes to the new format.
func migrateLedger(root string, write bool) error {
	ctx := context.Background()
	roots, err := storage.Resolve(root)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(roots.Root, "root.json")); err != nil {
		return err
	}
	profile, err := storage.Open(ctx, storage.OpenOptions{Root: roots.Root})
	if err != nil {
		return err
	}
	defer profile.Close()
	foundation, err := catalog.Open(ctx, profile.Roots)
	if err != nil {
		return err
	}
	defer foundation.Close()
	records, err := foundation.Runs().List(ctx)
	if err != nil {
		return err
	}
	builtins, err := nodes.Build()
	if err != nil {
		return err
	}
	var changes []ledgerConversion
	for _, record := range records {
		doc, err := decodeDocument(record.Summary.SummaryArtifact)
		if err != nil {
			return err
		}
		if doc["format"] == run.LedgerSummaryFormat && doc["version"] == run.LedgerSummaryVersion {
			continue
		}
		change, err := convertLedger(record, builtins.Catalog)
		if err != nil {
			return fmt.Errorf("run %s: %w", record.Summary.RunID, err)
		}
		changes = append(changes, change)
	}
	if len(changes) == 0 {
		fmt.Println("Run Ledger already current")
		return nil
	}
	if !write {
		fmt.Printf("validated %d Run heads; use --write to back up and convert\n", len(changes))
		return nil
	}
	backup := filepath.Join(roots.Backups, "before-event-runtime-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	if _, err := foundation.Backup(ctx, backup); err != nil {
		return err
	}
	path := filepath.ToSlash(filepath.Join(roots.State, catalog.RunFilename))
	if len(path) > 1 && path[1] == ':' {
		path = "/" + path
	}
	uri := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite", uri.String()+"?mode=rw")
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, change := range changes {
		result, err := tx.ExecContext(ctx, "UPDATE runs SET record_digest = ?, summary_artifact = ? WHERE run_id = ? AND record_digest = ?", change.next.String(), change.summary, change.id, change.previous.String())
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			return errors.New("run head changed during migration")
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Printf("converted %d Run heads; backup: %s\n", len(changes), backup)
	return nil
}
