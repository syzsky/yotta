package asset

import (
	"context"
	"testing"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/navigationpath"
)

func TestPathLibraryPreservesBoundContent(t *testing.T) {
	store, _ := newTestStore(t)
	s := NewService(store, nil, nil)
	p := navigationpath.Path{Version: 1, Reference: navigationpath.Reference{Kind: "world", Frame: "test", Unit: "m", AxisSign: 1}, Points: []navigationpath.Point{{ID: "a", X: 1}, {ID: "b", X: 2}}}
	first, err := s.SavePath("", "A to B", p)
	if err != nil {
		t.Fatal(err)
	}
	p.Points[1].X = 8
	second, err := s.SavePath(first.GUID, "Edited", p)
	if err != nil {
		t.Fatal(err)
	}
	if first.Blob.Digest == second.Blob.Digest {
		t.Fatal("editing did not produce new content")
	}
	old, err := store.ReadBlob(context.Background(), first.Blob)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := navigationpath.Decode(old)
	if err != nil || prior.Points[1].X != 2 {
		t.Fatalf("old binding changed: %+v %v", prior, err)
	}
	reopened, err := NewService(store, nil, nil).GetPath(first.GUID)
	if err != nil || reopened.Path.Points[1].X != 8 || reopened.Name != "Edited" {
		t.Fatalf("reopen: %+v %v", reopened, err)
	}
	page, err := s.QueryAssets(AssetQuery{Kind: KindPath})
	if err != nil || page.Total != 1 {
		t.Fatalf("query: %+v %v", page, err)
	}
	results := s.BatchDelete([]string{first.GUID})
	if len(results) != 1 || !results[0].Deleted {
		t.Fatalf("delete: %+v", results)
	}
	if _, err := s.GetPath(first.GUID); apperr.From(err).ID != "asset.not_found" {
		t.Fatalf("missing: %v", err)
	}
}

func TestSavePathRejectsInvalidAndForeignIdentity(t *testing.T) {
	store, _ := newTestStore(t)
	s := NewService(store, nil, nil)
	p := navigationpath.Path{Version: 1, Reference: navigationpath.Reference{Kind: "world", Frame: "test", Unit: "m", AxisSign: 1}, Points: []navigationpath.Point{}}
	if _, err := s.SavePath("", " ", p); apperr.From(err).ID != "path.name_invalid" {
		t.Fatal(err)
	}
	if _, err := s.SavePath("missing", "Path", p); apperr.From(err).ID != "asset.not_found" {
		t.Fatal(err)
	}
	if err := store.PutRecord(makeRecord("other", "Other", KindClip)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SavePath("other", "Path", p); apperr.From(err).ID != "path.identity_conflict" {
		t.Fatal(err)
	}
	p.Points = nil
	if _, err := s.SavePath("", "Path", p); apperr.From(err).ID != "path.invalid" {
		t.Fatal(err)
	}
}
