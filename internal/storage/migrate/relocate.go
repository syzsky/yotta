package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/yottaapp/yotta/internal/durablefs"
	"github.com/yottaapp/yotta/internal/services"
	"github.com/yottaapp/yotta/internal/storage"
	"github.com/yottaapp/yotta/internal/storage/catalog"
)

type Relocation struct {
	Format        string         `json:"format"`
	Source        string         `json:"source"`
	Destination   string         `json:"destination"`
	Backup        string         `json:"backup"`
	Files         []SnapshotFile `json:"files"`
	Bytes         uint64         `json:"bytes"`
	SettingsPaths int            `json:"settingsPaths"`
}

func relocateDefault(ctx context.Context, options Options) error {
	if strings.TrimSpace(options.Root) != "" || strings.TrimSpace(os.Getenv(storage.EnvironmentRoot)) != "" {
		return nil
	}
	target, err := storage.Resolve("")
	if err != nil {
		return err
	}
	source, err := storage.LegacyDefaultRoot()
	if err != nil {
		return err
	}
	return relocateDefaultRoots(ctx, options, source, target.Root)
}

func relocateDefaultRoots(ctx context.Context, options Options, source, target string) error {
	if _, err := os.Lstat(target); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var err error
	if _, err = os.Lstat(filepath.Join(source, "root.json")); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	// Upgrade a released older layout at its existing location before relocation.
	// Explicit Root prevents recursive default discovery.
	if _, err = Ensure(ctx, Options{Root: source, MaxRuns: options.MaxRuns}); err != nil {
		return err
	}
	_, err = RelocateProfile(ctx, source, target)
	return err
}

// RelocateProfile copies a quiescent profile under its writer lease, verifies
// every copied file, updates only mutable settings paths, and publishes once.
// Existing destinations are never merged or overwritten. Old data remains a
// complete backup; failures before publication leave the source usable.
func RelocateProfile(ctx context.Context, source, destination string) (Relocation, error) {
	var report Relocation
	if ctx == nil {
		return report, errors.New("profile relocation requires context")
	}
	from, err := storage.Resolve(source)
	if err != nil {
		return report, err
	}
	to, err := storage.Resolve(destination)
	if err != nil {
		return report, err
	}
	if _, err = os.Lstat(from.ManifestFile()); err != nil {
		return report, err
	}
	relative, err := filepath.Rel(from.Root, to.Root)
	inverse, backErr := filepath.Rel(to.Root, from.Root)
	if err != nil || backErr != nil || !outside(relative) || !outside(inverse) {
		return report, errors.New("profile relocation roots must be disjoint")
	}
	if _, err = os.Lstat(to.Root); err == nil {
		return report, errors.New("profile relocation destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return report, err
	}
	held, err := storage.Open(ctx, storage.OpenOptions{Root: from.Root})
	if err != nil {
		return report, err
	}
	defer held.Close()
	if err = os.MkdirAll(filepath.Dir(to.Root), 0700); err != nil {
		return report, err
	}
	staging, err := os.MkdirTemp(filepath.Dir(to.Root), ".yotta-relocation-")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(staging)
	report = Relocation{Format: "yotta.profile-relocation/v1", Source: from.Root, Destination: to.Root, Files: []SnapshotFile{}}
	err = filepath.WalkDir(from.Root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(from.Root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if filepath.ToSlash(rel) == "runtime/writer.lock" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("profile relocation does not follow links: %s", rel)
		}
		target := filepath.Join(staging, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported profile entry: %s", rel)
		}
		item, err := copyRelocationFile(ctx, path, target, info.Mode().Perm())
		if err != nil {
			return err
		}
		item.Path = filepath.ToSlash(rel)
		report.Files = append(report.Files, item)
		report.Bytes += uint64(item.Bytes)
		return nil
	})
	if err != nil {
		return report, err
	}
	stagedRoots, err := storage.Resolve(staging)
	if err != nil {
		return report, err
	}
	report.SettingsPaths, err = relocateSettings(stagedRoots.SettingsFile(), from.Root, to.Root)
	if err != nil {
		return report, err
	}
	scope, err := storage.CredentialScope(from)
	if err != nil {
		return report, err
	}
	if err = storage.PreserveCredentialScope(stagedRoots, scope); err != nil {
		return report, err
	}
	// SQLite checks read the full DB + WAL set after the cold copy.
	health, err := catalog.Inspect(ctx, stagedRoots)
	if err != nil {
		return report, err
	}
	if (health.Content.Present && !health.Content.Healthy) || (health.Runs.Present && !health.Runs.Healthy) {
		return report, errors.New("relocated databases failed integrity checks")
	}
	if err = ctx.Err(); err != nil {
		return report, err
	}
	report.Backup = filepath.Join(to.Backups, "vendor-yueli", time.Now().UTC().Format("20060102T150405.000000000"), "previous-profile")
	receipt := filepath.Join(stagedRoots.Config, "profile-relocation.json")
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return report, err
	}
	if err = durablefs.WriteFile(receipt, raw, 0600); err != nil {
		return report, err
	}
	if err = os.Rename(staging, to.Root); err != nil {
		return report, err
	}
	// Windows also holds the lease file's containing directory. Release it only
	// after the verified destination has been published, before archiving source.
	if err = held.Close(); err != nil {
		return report, err
	}
	// Publication is complete. If another component still holds an old cache
	// file, retain the intact source rather than jeopardizing the verified target.
	if err = os.MkdirAll(filepath.Dir(report.Backup), 0700); err == nil {
		err = os.Rename(from.Root, report.Backup)
	}
	if err != nil {
		report.Backup = from.Root
		raw, _ = json.MarshalIndent(report, "", "  ")
		_ = durablefs.WriteFile(filepath.Join(to.Config, "profile-relocation.json"), raw, 0600)
	}
	return report, nil
}

func outside(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

type relocationReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r relocationReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
func copyRelocationFile(ctx context.Context, source, target string, mode fs.FileMode) (SnapshotFile, error) {
	var result SnapshotFile
	input, err := os.Open(source)
	if err != nil {
		return result, err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return result, err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(output, hash), relocationReader{ctx: ctx, reader: input})
	syncErr := output.Sync()
	closeErr := output.Close()
	if err = errors.Join(copyErr, syncErr, closeErr); err != nil {
		return result, err
	}
	verify, err := os.Open(target)
	if err != nil {
		return result, err
	}
	defer verify.Close()
	got := sha256.New()
	n, err := io.Copy(got, relocationReader{ctx: ctx, reader: verify})
	if err != nil {
		return result, err
	}
	expected := hex.EncodeToString(hash.Sum(nil))
	if n != size || hex.EncodeToString(got.Sum(nil)) != expected {
		return result, errors.New("relocated file verification failed")
	}
	return SnapshotFile{Present: true, Bytes: size, SHA256: expected}, nil
}

func relocateSettings(path, source, destination string) (int, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	store, settings, err := services.OpenSettingsStore(path)
	if err != nil {
		return 0, err
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return 0, err
	}
	var value any
	if err = json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	count := rebaseSettingsValue(value, source, destination)
	if count == 0 {
		return 0, nil
	}
	raw, err = json.Marshal(value)
	if err != nil {
		return 0, err
	}
	var next services.Settings
	if err = json.Unmarshal(raw, &next); err != nil {
		return 0, err
	}
	if err = store.Save(&next); err != nil {
		return 0, err
	}
	// Both primary and recovery backup must point at the new physical root.
	if err = store.Save(&next); err != nil {
		return 0, err
	}
	return count, nil
}
func rebaseSettingsValue(value any, source, destination string) int {
	count := 0
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if text, ok := item.(string); ok {
				if next, changed := rebasePath(text, source, destination); changed {
					v[key] = next
					count++
				}
			} else {
				count += rebaseSettingsValue(item, source, destination)
			}
		}
	case []any:
		for i, item := range v {
			if text, ok := item.(string); ok {
				if next, changed := rebasePath(text, source, destination); changed {
					v[i] = next
					count++
				}
			} else {
				count += rebaseSettingsValue(item, source, destination)
			}
		}
	}
	return count
}
func rebasePath(value, source, destination string) (string, bool) {
	original := value
	prefix := ""
	if strings.HasPrefix(value, "-") {
		if index := strings.IndexByte(value, '='); index >= 0 {
			prefix = value[:index+1]
			value = value[index+1:]
		}
	}
	normalized := filepath.ToSlash(value)
	base := filepath.ToSlash(source)
	compare, expected := normalized, base
	if runtime.GOOS == "windows" {
		compare = strings.ToLower(compare)
		expected = strings.ToLower(expected)
	}
	if compare != expected && !strings.HasPrefix(compare, expected+"/") {
		return original, false
	}
	suffix := strings.TrimPrefix(normalized[len(base):], "/")
	return prefix + filepath.Join(destination, filepath.FromSlash(suffix)), true
}
