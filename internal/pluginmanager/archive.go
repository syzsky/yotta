package pluginmanager

import (
	"archive/zip"
	"errors"
	"github.com/yottaapp/yotta/internal/nodepackage"
	"io"
	"os"
)

func copyArchive(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, err := io.Copy(output, io.LimitReader(input, (16<<30)+1))
	closeErr := output.Close()
	if err != nil {
		return err
	}
	if n > 16<<30 {
		return errors.New("plugin archive exceeds budget")
	}
	return closeErr
}
func archiveSignature(path string) ([]byte, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	for _, f := range archive.File {
		if f.Name == nodepackage.ArchiveSignaturePath {
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			return io.ReadAll(io.LimitReader(r, (16<<10)+1))
		}
	}
	return nil, errors.New("package signature missing")
}
