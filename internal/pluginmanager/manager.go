// Package pluginmanager owns user-installed plugins and cooperative companions.
package pluginmanager

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/application"
	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/nodepackage"
	"github.com/yottaapp/yotta/internal/services"
	"github.com/yottaapp/yotta/internal/workflow/schema"
	"github.com/yottaapp/yotta/sdk/plugin/packaging"
)

type WorkflowUse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Running bool   `json:"running"`
}
type CompanionView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Status  string `json:"status"`
}
type View struct {
	InUse           bool            `json:"inUse"`
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Version         string          `json:"version"`
	Enabled         bool            `json:"enabled"`
	Loaded          bool            `json:"loaded"`
	RestartRequired bool            `json:"restartRequired"`
	CanRollback     bool            `json:"canRollback"`
	Nodes           []string        `json:"nodes"`
	Workflows       []WorkflowUse   `json:"workflows"`
	Companions      []CompanionView `json:"companions"`
}
type record struct {
	rollbackManaged bool
	manifest        nodepackage.Manifest
	root            string
	descriptor      packaging.Descriptor
}
type Manager struct {
	mu        sync.Mutex
	root      string
	store     *nodepackage.Store
	current   atomic.Pointer[nodepackage.Store]
	app       *services.App
	workflows *application.Application
	records   map[string]record
	loaded    map[string]artifact.Digest
}

func New(root string, store *nodepackage.Store, app *services.App, workflows *application.Application) (*Manager, error) {
	m := &Manager{root: root, store: store, app: app, workflows: workflows, records: map[string]record{}, loaded: map[string]artifact.Digest{}}
	if workflows != nil {
		for _, d := range workflows.NodePackageDependencies() {
			m.loaded[d.PackageID] = d.ManifestDigest
		}
	}
	if store != nil {
		for _, p := range store.List() {
			if err := m.load(p.PackageID); err != nil {
				return nil, err
			}
		}
	}
	m.current.Store(store)
	if workflows != nil {
		workflows.SetPluginValidator(m.checkSource)
		workflows.SetRunServicePreparer(m.prepareRunServices)
	}
	return m, nil
}
func problem(id string, cause error) error {
	switch {
	case errors.Is(cause, nodepackage.ErrFilesBusy):
		return fmt.Errorf("%w: %v", apperr.NewRetryable("plugins.files_busy", nil), cause)
	case errors.Is(cause, nodepackage.ErrGenerationInvalid):
		return fmt.Errorf("%w: %v", apperr.New("plugins.installed_files_changed", nil), cause)
	}

	if cause == nil {
		return apperr.New(id, nil)
	}
	var envelope apperr.EnvelopeProvider
	if errors.As(cause, &envelope) {
		return cause
	}
	return fmt.Errorf("%w: %v", apperr.New(id, nil), cause)
}
func (m *Manager) load(id string) error {
	manifest, root, err := m.store.InstalledManifest(context.Background(), id)
	if err != nil {
		return err
	}
	descriptor, err := readDescriptor(root, manifest)
	if errors.Is(err, os.ErrNotExist) {
		descriptor = packaging.Descriptor{Name: id}
	} else if err != nil {
		return err
	}
	rollbackManaged := false
	if previous, previousRoot, e := m.store.RollbackManifest(context.Background(), id); e == nil {
		_, e = readDescriptor(previousRoot, previous)
		rollbackManaged = e == nil
	}
	m.records[id] = record{manifest: manifest, root: root, descriptor: descriptor, rollbackManaged: rollbackManaged}
	return nil
}
func (m *Manager) uses(id string) []WorkflowUse {
	result := []WorkflowUse{}
	if m.workflows == nil {
		return result
	}
	for _, s := range m.workflows.ListSources() {
		var source schema.WorkflowSource
		if json.Unmarshal(s.Artifact(), &source) != nil {
			continue
		}
		for _, d := range source.Dependencies {
			if d.PackageID == id {
				result = append(result, WorkflowUse{ID: s.WorkflowID(), Name: source.Workflow.Name, Running: len(m.workflows.ActiveSourceRuns(s.WorkflowID())) > 0})
				break
			}
		}
	}
	return result
}
func (m *Manager) idle(id string) error {
	if m.workflows != nil && m.workflows.PluginInUse(id) {
		return apperr.New("plugins.in_use", nil)
	}
	for _, u := range m.uses(id) {
		if u.Running {
			return apperr.New("plugins.in_use", nil)
		}
	}
	return nil
}
func (m *Manager) List() ([]View, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := []View{}
	if m.store == nil {
		return result, nil
	}
	for _, p := range m.store.List() {
		r, ok := m.records[p.PackageID]
		if !ok {
			return nil, problem("plugins.load_failed", errors.New("installed descriptor unavailable"))
		}
		v := View{ID: p.PackageID, Name: r.descriptor.Name, Description: r.descriptor.Description, Version: r.manifest.PackageVersion(), Enabled: p.Enabled, Loaded: p.Enabled && m.loaded[p.PackageID] == p.Current, CanRollback: p.Rollback.Valid() && r.rollbackManaged, Nodes: []string{}, Workflows: m.uses(p.PackageID), Companions: []CompanionView{}}
		if m.workflows != nil {
			v.InUse = m.workflows.PluginInUse(p.PackageID)
		}
		v.RestartRequired = p.Enabled && !v.Loaded
		for _, n := range r.manifest.Nodes() {
			v.Nodes = append(v.Nodes, n.Contract.Authoring().TitleKey)
		}
		for _, c := range r.descriptor.Companions {
			running, status := health(c)
			v.Companions = append(v.Companions, CompanionView{ID: c.ID, Name: c.Name, Running: running, Status: status})
		}
		result = append(result, v)
	}
	return result, nil
}
func (m *Manager) Messages() map[string]map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]map[string]string{"zh": {}, "en": {}}
	ids := make([]string, 0, len(m.records))
	for id := range m.records {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		for locale, messages := range m.records[id].descriptor.Messages {
			for key, value := range messages {
				out[locale][key] = value
			}
		}
	}
	return out
}

// Import is the installation intent. Signature checks and publisher registration
// stay within this single action; callers do not orchestrate trust internals.
func (m *Manager) importPackage(archive string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx := context.Background()
	if err := os.MkdirAll(filepath.Dir(m.root), 0700); err != nil {
		return problem("plugins.install_failed", err)
	}
	temp, err := os.MkdirTemp(filepath.Dir(m.root), ".plugin-import-")
	if err != nil {
		return problem("plugins.install_failed", err)
	}
	defer os.RemoveAll(temp)
	stableArchive := filepath.Join(temp, "input.ynp")
	if err = copyArchive(archive, stableArchive); err != nil {
		return problem("plugins.invalid_package", err)
	}
	archive = stableArchive
	extracted := filepath.Join(temp, "package")
	manifest, err := nodepackage.ExtractArchive(ctx, archive, extracted)
	if err != nil {
		return problem("plugins.invalid_package", err)
	}
	descriptor, err := readDescriptor(extracted, manifest)
	if err != nil {
		return problem("plugins.invalid_package", err)
	}
	if err = m.idle(manifest.PackageID()); err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(descriptor.PublicKey)
	if err != nil {
		return problem("plugins.invalid_package", err)
	}
	var policy nodepackage.TrustPolicy
	if m.store != nil {
		policy = m.store.TrustPolicy()
	}
	next, err := policy.WithInstalledPublisher(manifest.PublisherNamespace(), ed25519.PublicKey(key))
	if err != nil {
		return problem("plugins.publisher_changed", err)
	}
	raw, err := archiveSignature(archive)
	if err != nil {
		return problem("plugins.invalid_package", err)
	}
	signature, err := nodepackage.OpenSignatureEnvelope(raw)
	if err != nil {
		return problem("plugins.invalid_package", err)
	}
	if _, err = nodepackage.VerifySignature(manifest, signature, next); err != nil {
		return problem("plugins.invalid_package", err)
	}
	// Validate target ownership before changing the installed generation.
	old, hadOld := m.records[manifest.PackageID()]
	if err = m.configure(descriptor, extracted, old, true); err != nil {
		return err
	}
	if m.store == nil {
		m.store, err = nodepackage.CreateStore(ctx, m.root, next)
	} else if next.Digest() != policy.Digest() {
		err = m.store.ApplyTrustPolicy(ctx, next)
	}
	if err != nil {
		return problem("plugins.install_failed", err)
	}
	m.current.Store(m.store)
	trust, err := m.store.InspectArchiveTrust(ctx, archive)
	if err != nil {
		return problem("plugins.invalid_package", err)
	}
	if err = m.store.GrantPackageTrust(ctx, trust.PublisherKeyID, trust.PackageID); err != nil {
		return problem("plugins.install_failed", err)
	}
	var wasEnabled bool
	for _, p := range m.store.List() {
		if p.PackageID == manifest.PackageID() {
			wasEnabled = p.Enabled
		}
	}
	if hadOld {
		for _, c := range old.descriptor.Companions {
			if err = stop(c); err != nil {
				return problem("plugins.stop_failed", err)
			}
		}
	}
	if _, err = m.store.InstallArchive(ctx, archive); err != nil {
		return problem("plugins.install_failed", err)
	}
	_, root, err := m.store.InstalledManifest(ctx, manifest.PackageID())
	if err == nil {
		err = m.configure(descriptor, root, old, false)
	}
	if err != nil {
		if hadOld {
			_, _ = m.store.Rollback(ctx, manifest.PackageID())
			if wasEnabled {
				_, _ = m.store.Enable(manifest.PackageID())
			}
		} else {
			_ = m.store.Uninstall(manifest.PackageID())
		}
		return problem("plugins.install_failed", err)
	}
	if err = m.load(manifest.PackageID()); err != nil {
		return problem("plugins.load_failed", err)
	}
	return nil
}

func (m *Manager) setenabled(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.records[id]
	if !ok {
		return apperr.New("plugins.not_found", nil)
	}
	if err := m.idle(id); err != nil {
		return err
	}
	if !enabled {
		for _, c := range r.descriptor.Companions {
			if err := stop(c); err != nil {
				return problem("plugins.stop_failed", err)
			}
		}
	}
	var err error
	if enabled {
		_, err = m.store.Enable(id)
	} else {
		_, err = m.store.Disable(id)
	}
	return wrapIf("plugins.change_failed", err)
}
func (m *Manager) uninstall(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.records[id]
	if !ok {
		return apperr.New("plugins.not_found", nil)
	}
	if err := m.idle(id); err != nil {
		return err
	}
	for _, c := range r.descriptor.Companions {
		if err := stop(c); err != nil {
			return problem("plugins.stop_failed", err)
		}
	}
	if err := m.removeTargets(r); err != nil {
		return problem("plugins.change_failed", err)
	}
	if err := m.store.Uninstall(id); err != nil {
		_ = m.configure(r.descriptor, r.root, record{}, false)
		return problem("plugins.change_failed", err)
	}
	delete(m.records, id)
	return nil
}
func (m *Manager) rollback(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.records[id]
	if !ok {
		return apperr.New("plugins.not_found", nil)
	}
	if !old.rollbackManaged {
		return problem("plugins.change_failed", errors.New("previous release does not support desktop restoration"))
	}
	wasEnabled := false
	for _, p := range m.store.List() {
		if p.PackageID == id {
			wasEnabled = p.Enabled
		}
	}
	if err := m.idle(id); err != nil {
		return err
	}
	for _, c := range old.descriptor.Companions {
		if err := stop(c); err != nil {
			return problem("plugins.stop_failed", err)
		}
	}
	if _, err := m.store.Rollback(context.Background(), id); err != nil {
		return problem("plugins.change_failed", err)
	}
	restore := func(cause error) error {
		_, err := m.store.Rollback(context.Background(), id)
		if err == nil && wasEnabled {
			_, err = m.store.Enable(id)
		}
		loadErr := m.load(id)
		return errors.Join(cause, wrapIf("plugins.change_failed", err), wrapIf("plugins.load_failed", loadErr))
	}
	if err := m.load(id); err != nil {
		return restore(problem("plugins.load_failed", err))
	}
	r := m.records[id]
	if err := m.configure(r.descriptor, r.root, old, false); err != nil {
		return restore(err)
	}
	_, err := m.store.Enable(id)
	if err != nil {
		_ = m.configure(old.descriptor, old.root, r, false)
		return restore(problem("plugins.change_failed", err))
	}
	return wrapIf("plugins.change_failed", err)
}
func wrapIf(id string, err error) error {
	if err == nil {
		return nil
	}
	return problem(id, err)
}

func (m *Manager) change(action func() error) error {
	if m.workflows != nil {
		return m.workflows.WithPluginChange(action)
	}
	return action()
}
func (m *Manager) Import(path string) error {
	return m.change(func() error { return m.importPackage(path) })
}
func (m *Manager) SetEnabled(id string, enabled bool) error {
	return m.change(func() error { return m.setenabled(id, enabled) })
}
func (m *Manager) Uninstall(id string) error {
	return m.change(func() error { return m.uninstall(id) })
}
func (m *Manager) Rollback(id string) error { return m.change(func() error { return m.rollback(id) }) }
func (m *Manager) checkSource(raw []byte) ([]string, error) {
	var source schema.WorkflowSource
	if json.Unmarshal(raw, &source) != nil || len(source.Dependencies) == 0 {
		return nil, nil
	}
	store := m.current.Load()
	if store == nil {
		return nil, apperr.New("plugins.not_loaded", nil)
	}
	installed := store.List()
	ids := make([]string, 0, len(source.Dependencies))
	for _, d := range source.Dependencies {
		found := false
		for _, p := range installed {
			if p.PackageID != d.PackageID {
				continue
			}
			found = true
			if !p.Enabled {
				return nil, apperr.New("plugins.disabled", nil)
			}
			if m.loaded[p.PackageID] != p.Current {
				return nil, apperr.New("plugins.not_loaded", nil)
			}
		}
		if !found {
			return nil, apperr.New("plugins.not_found", nil)
		}
		ids = append(ids, d.PackageID)
	}
	return ids, nil
}

// NeedsRestart reports only enabled generations absent from the process catalog.
// Disabled and removed packages are blocked immediately; cached contracts remain
// available to render existing workflows without keeping the plugin active.
func (m *Manager) NeedsRestart() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.store == nil {
		return false
	}
	for _, p := range m.store.List() {
		if p.Enabled && m.loaded[p.PackageID] != p.Current {
			return true
		}
	}
	return false
}

func (m *Manager) InstallationDirectory(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[id]
	if !ok {
		return "", apperr.New("plugins.not_found", nil)
	}
	return record.root, nil
}
