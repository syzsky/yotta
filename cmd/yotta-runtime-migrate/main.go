// Command yotta-runtime-migrate performs explicit, offline development data
// conversions. It is not linked into the desktop application's readers.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/nodecontract"
)

func migrateNodeContract(raw []byte) ([]byte, error) {
	if len(raw) > nodecontract.MaxContractBytes {
		return nil, errors.New("node contract exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, errors.New("expected one contract document")
	}
	if document["format"] != nodecontract.Format {
		return nil, errors.New("not a standalone node contract; signed packages must be rebuilt by their owner")
	}
	if document["version"] == nodecontract.Version {
		_, err := nodecontract.Open(raw)
		return raw, err
	}
	if document["version"] != "2" {
		return nil, errors.New("only development Node Contract v2 is supported")
	}
	document["version"] = nodecontract.Version
	converted, err := artifact.Marshal(document)
	if err != nil {
		return nil, err
	}
	validated, err := nodecontract.Open(converted)
	if err != nil {
		return nil, err
	}
	return validated.Bytes(), nil
}

func main() {
	profile := flag.String("profile", "", "existing offline profile whose Run Ledger should be converted")
	navigationProfile := flag.String("navigation-profile", "", "offline profile with terminal Move v1 sources to expand into explicit position tasks")
	path := flag.String("node-contract", "", "standalone Node Contract JSON file")
	write := flag.Bool("write", false, "write the conversion after creating an exclusive .before-runtime-v3 backup")
	flag.Parse()
	var err error
	if *navigationProfile != "" && (*profile != "" || *path != "") {
		err = errors.New("choose one migration target")
	} else if *navigationProfile != "" {
		err = migrateNavigationProfile(*navigationProfile, *write)
	} else if *profile != "" && *path != "" {
		err = errors.New("choose --profile or --node-contract")
	} else if *profile != "" {
		err = migrateLedger(*profile, *write)
	} else {
		err = execute(*path, *write)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute(path string, write bool) error {
	if path == "" {
		return errors.New("--node-contract is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, nodecontract.MaxContractBytes+1))
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	converted, err := migrateNodeContract(raw)
	if err != nil {
		return err
	}
	if bytes.Equal(raw, converted) {
		fmt.Println("already current")
		return nil
	}
	if !write {
		fmt.Println("conversion validated; use --write to back up and convert")
		return nil
	}
	backup, err := os.OpenFile(path+".before-runtime-v3", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := backup.Write(raw)
	syncErr := backup.Sync()
	closeErr = backup.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	staged, err := os.CreateTemp(filepath.Dir(path), ".runtime-migrate-*")
	if err != nil {
		return err
	}
	defer os.Remove(staged.Name())
	_, writeErr = staged.Write(converted)
	syncErr = staged.Sync()
	closeErr = staged.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err := os.Rename(staged.Name(), path); err != nil {
		return err
	}
	fmt.Println("converted; original retained in .before-runtime-v3")
	return nil
}
