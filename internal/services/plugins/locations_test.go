package plugins

import (
	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/storage"
	"testing"
)

func TestLocationsUseActiveProfileAndOnlyOpenKnownDirectories(t *testing.T) {
	roots, err := storage.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	opened := ""
	service := &Service{roots: roots, openDirectory: func(path string) error { opened = path; return nil }}
	locations, err := service.Locations("")
	if err != nil || len(locations) != 7 || locations[0].Path != roots.Root {
		t.Fatalf("locations=%+v %v", locations, err)
	}
	if err = service.OpenLocation("root", ""); err != nil || opened != roots.Root {
		t.Fatal("wrong directory opened", err)
	}
	if err = service.OpenLocation("../../other", ""); err == nil || apperr.Project(err).ID != "plugins.location_unavailable" {
		t.Fatal("arbitrary path accepted", err)
	}
}
