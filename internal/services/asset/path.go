package asset

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/blob"
	"github.com/yottaapp/yotta/internal/navigationpath"
)

// SavedPath keeps mutable library identity separate from immutable workflow content.
type SavedPath struct {
	GUID string              `json:"guid"`
	Name string              `json:"name"`
	Path navigationpath.Path `json:"path"`
	Blob blob.BlobRef        `json:"blob"`
}

func (s *Service) SavePath(guid, name string, path navigationpath.Path) (SavedPath, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) == 0 || len([]rune(name)) > 80 {
		return SavedPath{}, apperr.New("path.name_invalid", nil)
	}
	if err := path.Validate(); err != nil {
		return SavedPath{}, fmt.Errorf("%w: %v", apperr.New("path.invalid", nil), err)
	}
	data, err := json.Marshal(path)
	if err != nil || len(data) > navigationpath.MaxEncodedBytes {
		return SavedPath{}, apperr.New("path.invalid", nil)
	}
	record := AssetRecord{GUID: guid, Kind: KindPath, Name: name, Origin: Origin{Kind: "user"}, CreatedAt: time.Now().UTC()}
	if guid == "" {
		record.GUID = "path-" + uuid.NewString()
	} else {
		existing, ok, err := s.store.Record(guid)
		if err != nil {
			return SavedPath{}, fmt.Errorf("%w: %v", apperr.NewRetryable("asset.load_failed", nil), err)
		}
		if !ok {
			return SavedPath{}, apperr.New("asset.not_found", nil)
		}
		if existing.Kind != KindPath {
			return SavedPath{}, apperr.New("path.identity_conflict", nil)
		}
		record = existing
		record.Name = name
	}
	before := s.store.Revision()
	ref, err := s.store.CommitRecordBlob(context.Background(), navigationpath.MediaType, bytes.NewReader(data), func(ref blob.BlobRef) AssetRecord {
		record.Blob = &ref
		return record
	})
	if err != nil {
		return SavedPath{}, fmt.Errorf("%w: %v", apperr.NewRetryable("path.save_failed", nil), err)
	}
	s.emitChangedSince(before, record.GUID)
	return SavedPath{GUID: record.GUID, Name: name, Path: path.Clone(), Blob: ref}, nil
}

func (s *Service) GetPath(guid string) (SavedPath, error) {
	record, err := s.Get(guid)
	if err != nil {
		return SavedPath{}, err
	}
	if record.Kind != KindPath || record.Blob == nil {
		return SavedPath{}, apperr.New("path.identity_conflict", nil)
	}
	if record.Blob.MediaType != navigationpath.MediaType || record.Blob.Size > navigationpath.MaxEncodedBytes {
		return SavedPath{}, apperr.New("path.invalid", nil)
	}
	data, err := s.store.ReadBlob(context.Background(), *record.Blob)
	if err != nil {
		return SavedPath{}, fmt.Errorf("%w: %v", apperr.NewRetryable("asset.load_failed", nil), err)
	}
	path, err := navigationpath.Decode(data)
	if err != nil {
		return SavedPath{}, fmt.Errorf("%w: %v", apperr.New("path.invalid", nil), err)
	}
	return SavedPath{GUID: guid, Name: record.Name, Path: path, Blob: *record.Blob}, nil
}
