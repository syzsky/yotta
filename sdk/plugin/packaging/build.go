package packaging

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/hostapi"
	"github.com/yottaapp/yotta/internal/nodecontract"
	"github.com/yottaapp/yotta/internal/nodepackage"
	"github.com/yottaapp/yotta/sdk/plugin/authoring"
)

type Node struct {
	Contract         authoring.Contract
	PayloadPath      string
	Entrypoint       string
	OperatingSystems []string
	Architectures    []string
}
type Build struct {
	Namespace  string
	PackageID  string
	Version    string
	Descriptor Descriptor
	Nodes      []Node
	Files      map[string][]byte
}

// Write signs one exact manifest and all declared payloads. The private key
// belongs to the publisher and is never placed in the archive.
func Write(output string, build Build, key ed25519.PrivateKey) (string, error) {
	if len(key) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid publisher key")
	}
	files := make(map[string][]byte, len(build.Files)+1)
	for name, data := range build.Files {
		files[name] = data
	}
	build.Descriptor.Format = "yotta.plugin/v1"
	if len(build.Descriptor.Panels) > 0 {
		build.Descriptor.Format = "yotta.plugin/v2"
		for _, p := range build.Descriptor.Panels {
			if err := p.Definition.Validate(); err != nil {
				return "", err
			}
		}
	}
	build.Descriptor.PublicKey = base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
	raw, err := artifact.Marshal(build.Descriptor)
	if err != nil {
		return "", err
	}
	files[DescriptorPath] = raw
	payload := func(name string) nodepackage.Payload {
		data := files[name]
		hash := sha256.Sum256(data)
		media := "application/octet-stream"
		switch filepath.Ext(name) {
		case ".json":
			media = "application/json"
		case ".md", ".txt":
			media = "text/plain"
		case ".exe":
			media = "application/vnd.microsoft.portable-executable"
		}
		return nodepackage.Payload{Path: name, Digest: artifact.Digest("sha256:" + hex.EncodeToString(hash[:])), Size: int64(len(data)), MediaType: media}
	}
	draft := nodepackage.Draft{PublisherNamespace: build.Namespace, PackageID: build.PackageID, PackageVersion: build.Version, HostAPI: nodepackage.HostAPIRange{Min: hostapi.Current, MaxExclusive: hostapi.NextMajor}}
	used := map[string]bool{}
	for _, n := range build.Nodes {
		if _, ok := files[n.PayloadPath]; !ok {
			return "", fmt.Errorf("missing node payload")
		}
		draft.Nodes = append(draft.Nodes, nodepackage.NodeDraft{Contract: n.Contract, Implementation: nodepackage.Implementation{ABI: nodecontract.ABIRequirement{Kind: nodecontract.ABIProcess, Version: "v1"}, Entrypoint: n.Entrypoint, Payload: payload(n.PayloadPath), Platforms: nodepackage.PlatformSupport{OperatingSystems: n.OperatingSystems, Architectures: n.Architectures}}})
		used[n.PayloadPath] = true
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !used[name] {
			p := payload(name)
			if p.MediaType == "application/json" || p.MediaType == "text/plain" {
				draft.Documentation = append(draft.Documentation, p)
			} else {
				draft.Resources = append(draft.Resources, p)
			}
		}
	}
	manifest, err := nodepackage.Seal(draft)
	if err != nil {
		return "", err
	}
	signature, err := nodepackage.SignManifest(manifest, key)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(output), 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(output), ".plugin-*.ynp")
	if err != nil {
		return "", err
	}
	temp := f.Name()
	defer os.Remove(temp)
	z := zip.NewWriter(f)
	files[nodepackage.ArchiveManifestPath] = manifest.Bytes()
	files[nodepackage.ArchiveSignaturePath] = signature.Bytes()
	names = append(names, nodepackage.ArchiveManifestPath, nodepackage.ArchiveSignaturePath)
	for _, name := range names {
		w, e := z.Create(name)
		if e == nil {
			_, e = w.Write(files[name])
		}
		if e != nil {
			z.Close()
			f.Close()
			return "", e
		}
	}
	if err = z.Close(); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(temp, output); err != nil {
		return "", err
	}
	return manifest.Digest().String(), nil
}
