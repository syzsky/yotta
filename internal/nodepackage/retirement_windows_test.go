package nodepackage

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallWithLockedPayloadNeverLeavesPartialGeneration(t *testing.T) {
	ctx := context.Background()
	policy, key := lifecyclePolicy(t)
	store, err := CreateStore(ctx, filepath.Join(t.TempDir(), "packages"), policy)
	if err != nil {
		t.Fatal(err)
	}
	manifest, archive := lifecycleArchive(t, key, "1.0.0", "locked process")
	grantArchive(t, ctx, store, archive)
	installed, err := store.InstallArchive(ctx, archive)
	if err != nil {
		t.Fatal(err)
	}
	root := store.generationPath(manifest.Digest())
	name, err := windows.UTF16PtrFromString(filepath.Join(root, filepath.FromSlash(manifest.Nodes()[0].Implementation.Payload.Path)))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if handle != windows.InvalidHandle {
			windows.CloseHandle(handle)
		}
	}()
	if err = store.Uninstall(installed.PackageID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(root); err == nil {
		if _, err = OpenExtracted(ctx, root); err != nil {
			t.Fatalf("locked uninstall left a partial generation: %v", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if reopened, err := OpenStore(ctx, store.root); err != nil || len(reopened.List()) != 0 {
		t.Fatalf("locked remnants blocked store reopening: %v", err)
	}
	windows.CloseHandle(handle)
	handle = windows.InvalidHandle
	if _, err = store.InstallArchive(ctx, archive); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenExtracted(ctx, root); err != nil {
		t.Fatal(err)
	}
}
