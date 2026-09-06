package nodepackage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrGenerationInvalid = errors.New("installed node package generation is invalid")
var ErrFilesBusy = errors.New("node package files could not be retired")

// Retire an entire unreferenced directory before deleting its contents. Windows
// may keep executable images mapped after a service has closed its listener.
// A failed rename leaves the original directory intact; a failed cleanup leaves
// only a retired directory, never a half-deleted generation at its normal path.
func retireGeneration(root, generation string) error {
	if filepath.Dir(generation) != filepath.Join(root, generationsDir) || !validGenerationName(filepath.Base(generation)) {
		return errors.New("invalid retirement path")
	}
	if _, err := os.Lstat(generation); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	retired, err := os.MkdirTemp(root, ".retired-")
	if err != nil {
		return err
	}
	destination := filepath.Join(retired, "generation")
	if err = os.Rename(generation, destination); err != nil {
		_ = os.Remove(retired)
		return fmt.Errorf("%w: %v", ErrFilesBusy, err)
	}
	if err = os.RemoveAll(retired); err != nil {
		go func() {
			for _, delay := range []time.Duration{time.Second, 4 * time.Second, 10 * time.Second} {
				time.Sleep(delay)
				if os.RemoveAll(retired) == nil {
					return
				}
			}
		}()
	}
	return nil
}

func isRetiredEntry(entry os.DirEntry) bool {
	return strings.HasPrefix(entry.Name(), ".retired-") && entry.IsDir() && entry.Type()&os.ModeSymlink == 0
}
