package catalog

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestPathMigrationPreservesExistingAssetsAndChildren(t *testing.T) {
	roots := testRoots(t)
	stop := errors.New("leave version eight")
	_, err := open(context.Background(), roots, openOptions{faults: faultHooks{beforeMigrationCommit: func(kind databaseKind, version int) error {
		if kind == contentKind && version == 9 {
			return stop
		}
		return nil
	}}})
	if !errors.Is(err, stop) {
		t.Fatal(err)
	}
	db := openRaw(t, filepath.Join(roots.Catalog, ContentFilename), false)
	for _, sql := range []string{
		`INSERT INTO assets VALUES ('old','template','Old','description','category','user','','2026-09-12T00:00:00Z',1,NULL,NULL,NULL)`,
		`INSERT INTO asset_tags VALUES ('old',0,'Tag','tag')`,
		`INSERT INTO asset_variants VALUES ('old',0,1920,1080,1,2,3,4,'[]','image/png','digest',10)`,
	} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := Open(context.Background(), roots)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	db = openRaw(t, filepath.Join(roots.Catalog, ContentFilename), true)
	defer db.Close()
	for _, table := range []string{"assets", "asset_tags", "asset_variants"} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s: %d %v", table, count, err)
		}
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM assets WHERE guid='old'`).Scan(&name); err != nil || name != "Old" {
		t.Fatalf("asset changed: %s %v", name, err)
	}
}
