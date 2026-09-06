package storage

import (
	"encoding/json"
	"errors"
	"github.com/yottaapp/yotta/internal/durablefs"
	"os"
	"path/filepath"
)

const identityFilename = "profile-identity.json"

type profileIdentity struct {
	Format          string `json:"format"`
	CredentialScope string `json:"credentialScope"`
}

// CredentialScope remains stable across physical profile relocation. It is an
// identity used to locate OS-managed credentials, not a path that gets opened.
func CredentialScope(roots Roots) (string, error) {
	raw, err := os.ReadFile(filepath.Join(roots.Config, identityFilename))
	if errors.Is(err, os.ErrNotExist) {
		return filepath.Clean(roots.Data), nil
	}
	if err != nil {
		return "", err
	}
	var identity profileIdentity
	if len(raw) > 16384 || json.Unmarshal(raw, &identity) != nil || identity.Format != "yotta.profile-identity/v1" || !filepath.IsAbs(identity.CredentialScope) {
		return "", errors.New("invalid profile identity")
	}
	return filepath.Clean(identity.CredentialScope), nil
}

func PreserveCredentialScope(roots Roots, scope string) error {
	if !filepath.IsAbs(scope) {
		return errors.New("invalid profile credential scope")
	}
	raw, err := json.Marshal(profileIdentity{Format: "yotta.profile-identity/v1", CredentialScope: filepath.Clean(scope)})
	if err != nil {
		return err
	}
	return durablefs.WriteFile(filepath.Join(roots.Config, identityFilename), raw, 0600)
}
