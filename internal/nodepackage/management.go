package nodepackage

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"github.com/yottaapp/yotta/internal/artifact"
)

// WithInstalledPublisher adds a previously unknown publisher for an explicit
// local install. Existing namespaces cannot silently change signing keys.
func (p TrustPolicy) WithInstalledPublisher(namespace string, key ed25519.PublicKey) (TrustPolicy, error) {
	if !p.Valid() {
		return SealTrustPolicy(TrustPolicyDraft{Revision: 1, Publishers: []PublisherAuthorityDraft{{Namespace: namespace, Keys: []ed25519.PublicKey{key}}}})
	}
	id, err := PublisherKeyID(key)
	if err != nil {
		return TrustPolicy{}, err
	}
	draft := TrustPolicyDraft{Revision: p.Revision() + 1, PreviousDigest: p.Digest()}
	for _, publisher := range p.state.document.Publishers {
		authority := PublisherAuthorityDraft{Namespace: publisher.Namespace}
		for _, k := range publisher.Keys {
			raw, e := base64.RawStdEncoding.DecodeString(k.PublicKey)
			if e != nil {
				return TrustPolicy{}, e
			}
			authority.Keys = append(authority.Keys, ed25519.PublicKey(raw))
			if publisher.Namespace == namespace && k.KeyID == id {
				return p, nil
			}
		}
		if publisher.Namespace == namespace {
			return TrustPolicy{}, errors.New("installed publisher key differs")
		}
		draft.Publishers = append(draft.Publishers, authority)
	}
	draft.Publishers = append(draft.Publishers, PublisherAuthorityDraft{Namespace: namespace, Keys: []ed25519.PublicKey{key}})
	for _, s := range p.state.document.RevokedKeys {
		draft.RevokedKeys = append(draft.RevokedKeys, TrustStatusDraft(s))
	}
	for _, s := range p.state.document.RevokedManifests {
		draft.RevokedManifests = append(draft.RevokedManifests, TrustStatusDraft(s))
	}
	for _, s := range p.state.document.QuarantinedManifests {
		draft.QuarantinedManifests = append(draft.QuarantinedManifests, TrustStatusDraft(s))
	}
	return SealTrustPolicy(draft)
}

// InstalledManifest verifies a current generation once at a management boundary.
func (s *Store) InstalledManifest(ctx context.Context, id string) (Manifest, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	installed, ok := s.entries[id]
	if !ok {
		return Manifest{}, "", errors.New("package not installed")
	}
	return s.managementManifest(ctx, installed, installed.Current)
}

func (s *Store) RollbackManifest(ctx context.Context, id string) (Manifest, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	installed, ok := s.entries[id]
	if !ok || !installed.Rollback.Valid() || installed.Rollback == installed.Current {
		return Manifest{}, "", errors.New("rollback release unavailable")
	}
	return s.managementManifest(ctx, installed, installed.Rollback)
}
func (s *Store) managementManifest(ctx context.Context, installed PackageInstallation, digest artifact.Digest) (Manifest, string, error) {
	index := releaseIndex(installed.Releases, digest)
	if index < 0 {
		return Manifest{}, "", errors.New("release not installed")
	}
	root := s.generationPath(digest)
	manifest, err := OpenExtracted(ctx, root)
	if err != nil {
		return Manifest{}, "", err
	}
	if err = s.policy.verifyStored(manifest, installed.Releases[index].Trust); err != nil {
		return Manifest{}, "", err
	}
	return manifest, root, nil
}
