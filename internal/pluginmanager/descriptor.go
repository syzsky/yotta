package pluginmanager

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/yottaapp/yotta/internal/nodepackage"
	"github.com/yottaapp/yotta/sdk/plugin/packaging"
	"github.com/yottaapp/yotta/sdk/plugin/panel"
)

func readDescriptor(root string, manifest nodepackage.Manifest) (packaging.Descriptor, error) {
	var d packaging.Descriptor
	payloads := map[string]bool{}
	for _, p := range manifest.Documentation() {
		payloads[p.Path] = true
	}
	for _, p := range manifest.Resources() {
		payloads[p.Path] = true
	}
	if !payloads[packaging.DescriptorPath] {
		return d, os.ErrNotExist
	}
	raw, err := os.ReadFile(filepath.Join(root, packaging.DescriptorPath))
	if err != nil {
		return d, err
	}
	if len(raw) > 1<<20 {
		return d, errors.New("descriptor exceeds budget")
	}
	if err = json.Unmarshal(raw, &d); err != nil {
		return d, err
	}
	if (d.Format != "yotta.plugin/v1" && d.Format != "yotta.plugin/v2") || len(d.Name) == 0 || len(d.Name) > 256 || len(d.Description) > 2048 || len(d.Companions) > 16 || len(d.Panels) > 32 || (d.Format == "yotta.plugin/v1" && len(d.Panels) > 0) {
		return d, errors.New("invalid plugin descriptor")
	}
	key, err := base64.StdEncoding.DecodeString(d.PublicKey)
	if err != nil || len(key) != 32 {
		return d, errors.New("invalid publisher key")
	}
	seen := map[string]bool{}
	for _, c := range d.Companions {
		if c.ID == "" || seen[c.ID] || c.ApplicationSlot == "" || c.NetworkSlot == "" || c.Protocol == "" || !payloads[c.Executable] || len(c.Arguments) > 64 {
			return d, errors.New("invalid companion")
		}
		seen[c.ID] = true
		u, e := url.Parse(c.Origin)
		if e != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() {
			return d, errors.New("companion must use a loopback origin")
		}
		for _, endpoint := range []string{c.HealthPath, c.StopPath} {
			if !strings.HasPrefix(endpoint, "/") || strings.HasPrefix(endpoint, "//") || strings.ContainsAny(endpoint, "?#\\") || path.Clean(endpoint) != endpoint {
				return d, errors.New("invalid companion endpoint")
			}
		}
		for _, a := range c.Arguments {
			if strings.HasPrefix(a, "${package}/") && !payloads[strings.TrimPrefix(a, "${package}/")] {
				return d, errors.New("companion argument references missing payload")
			}
		}
	}
	// Plugins may translate only keys belonging to their declared contracts.
	allowed := map[string]bool{}
	panelIDs := map[string]bool{}
	for _, p := range d.Panels {
		if p.Definition.Validate() != nil || panelIDs[p.Definition.ID] || !seen[p.CompanionID] || !panel.Endpoint(p.SnapshotPath) || !panel.Endpoint(p.EventPath) {
			return d, errors.New("invalid panel contribution")
		}
		panelIDs[p.Definition.ID] = true
		for _, key := range p.Definition.MessageKeys() {
			if key != "" {
				allowed[key] = true
			}
		}
	}
	for _, n := range manifest.Nodes() {
		a := n.Contract.Authoring()
		allowed[a.TitleKey] = true
		allowed[a.DescriptionKey] = true
		for _, p := range a.Ports {
			allowed[p.TitleKey] = true
		}
		machine := n.Contract.Machine()
		for _, e := range machine.Errors {
			allowed["error."+e.Code] = true
		}
		for _, resource := range machine.ConfigSchemaBundle {
			var schema any
			if json.Unmarshal(resource.Schema, &schema) == nil {
				collectTitleKeys(schema, allowed)
			}
		}
	}
	for locale, messages := range d.Messages {
		if locale != "zh" && locale != "en" {
			return d, errors.New("unsupported plugin locale")
		}
		for key, value := range messages {
			if key == "" || !allowed[key] || len(value) > 4096 || strings.Contains(key, "__proto__") || strings.Contains(key, "constructor") {
				return d, errors.New("message key is not owned by plugin contract")
			}
		}
	}
	return d, nil
}
func collectTitleKeys(value any, keys map[string]bool) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if key == "x-yotta-title-key" {
				if s, ok := item.(string); ok {
					keys[s] = true
				}
			}
			collectTitleKeys(item, keys)
		}
	case []any:
		for _, item := range v {
			collectTitleKeys(item, keys)
		}
	}
}

func companionPaths(c packaging.Companion, root string) (string, []string) {
	args := append([]string(nil), c.Arguments...)
	for i, a := range args {
		if strings.HasPrefix(a, "${package}/") {
			args[i] = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(a, "${package}/")))
		}
	}
	return filepath.Join(root, filepath.FromSlash(c.Executable)), args
}
