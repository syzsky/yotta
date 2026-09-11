package run

import (
	"bytes"
	"errors"
	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/datatype"
)

// CompactRecord imports an unsegmented current-format record, verifies its
// supplied digest and archives full segments. Ordinary Open never rewrites an
// artifact. Offline import tools explicitly opt into the changed head digest.
func CompactRecord(raw []byte, catalog datatype.ValueTypeCatalog) (Record, error) {
	var document recordDocument
	if err := decodeCanonicalLedgerArtifact(raw, &document); err != nil {
		return Record{}, err
	}
	if document.Version != RecordVersion || document.Checkpoint != nil {
		return Record{}, errors.New("expected unsegmented current RunRecord")
	}
	body := document
	body.RecordDigest = ""
	canonical, err := artifact.Marshal(body)
	if err != nil {
		return Record{}, err
	}
	digest, err := artifact.Sum(recordDigestDomain, canonical)
	if err != nil || digest != document.RecordDigest {
		return Record{}, errors.New("RunRecord import digest mismatch")
	}
	original, err := artifact.Marshal(document)
	if err != nil || !bytes.Equal(original, raw) {
		return Record{}, errors.New("RunRecord import is not canonical")
	}
	checkpoint := newJournalCheckpoint()
	for len(document.Journal) > JournalSegmentEntries {
		checkpoint, err = checkpoint.archive(document.Journal[:JournalSegmentEntries], document.StartedAt)
		if err != nil {
			return Record{}, err
		}
		document.Journal = document.Journal[JournalSegmentEntries:]
		document.Checkpoint = &checkpoint
	}
	return sealRecord(document, catalog)
}
