package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/registryclient"
	"github.com/yottaapp/yotta/internal/workflowbundle"
	"github.com/yottaapp/yotta/pkg/version"
)

type RegistryCreatorView struct {
	UserKey     string `json:"userKey"`
	DisplayName string `json:"displayName,omitempty"`
}

type RegistryWorkflowReleaseView struct {
	ReleaseID          string                   `json:"releaseId"`
	PublisherNamespace string                   `json:"publisherNamespace"`
	WorkflowID         string                   `json:"workflowId"`
	ReleaseVersion     string                   `json:"releaseVersion"`
	SourceHash         string                   `json:"sourceHash"`
	BundleDigest       string                   `json:"bundleDigest"`
	Title              string                   `json:"title"`
	Summary            string                   `json:"summary"`
	ReleaseNotes       string                   `json:"releaseNotes,omitempty"`
	Examples           []PublishRegistryExample `json:"examples"`
	Screenshots        []RegistryScreenshotView `json:"screenshots"`
	Creator            RegistryCreatorView      `json:"creator"`
	Availability       string                   `json:"availability"`
	PublishedAt        string                   `json:"publishedAt"`
}

type RegistryScreenshotView struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

type RegistrySearchPageView struct {
	Items      []RegistryWorkflowReleaseView `json:"items"`
	NextCursor string                        `json:"nextCursor,omitempty"`
}

type PublishRegistryExample struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type PublishRegistryScreenshot struct {
	Path string `json:"path"`
	Alt  string `json:"alt"`
}

type PublishRegistryRequest struct {
	WorkflowID     string                      `json:"workflowId"`
	ReleaseVersion string                      `json:"releaseVersion"`
	Title          string                      `json:"title"`
	Summary        string                      `json:"summary"`
	ReleaseNotes   string                      `json:"releaseNotes"`
	Examples       []PublishRegistryExample    `json:"examples"`
	Screenshots    []PublishRegistryScreenshot `json:"screenshots"`
}

func (s *Service) PublishSourceToRegistry(
	ctx context.Context, request PublishRegistryRequest,
) (RegistryWorkflowReleaseView, error) {
	if s.bundles == nil || s.registry == nil {
		return RegistryWorkflowReleaseView{}, unavailable("registry")
	}
	info, bundle, err := s.bundles.ExportBytes(ctx, request.WorkflowID)
	if err != nil {
		return RegistryWorkflowReleaseView{}, bundleError("publish", err)
	}
	examples := make([]registryclient.Example, 0, len(request.Examples))
	for _, example := range request.Examples {
		examples = append(examples, registryclient.Example{Title: example.Title, Description: example.Description})
	}
	screenshots := make([]registryclient.ScreenshotUpload, 0, len(request.Screenshots))
	for _, screenshot := range request.Screenshots {
		fileInfo, statErr := os.Stat(screenshot.Path)
		if statErr != nil || !fileInfo.Mode().IsRegular() || fileInfo.Size() <= 0 || fileInfo.Size() > 16<<20 {
			if statErr == nil {
				statErr = errors.New("screenshot file is not a supported regular file")
			}
			return RegistryWorkflowReleaseView{}, projectError("workflow.registry.invalid_screenshot", apperr.CategoryValidation, nil, false, statErr)
		}
		content, readErr := os.ReadFile(screenshot.Path)
		if readErr != nil {
			return RegistryWorkflowReleaseView{}, projectError("workflow.registry.invalid_screenshot", apperr.CategoryValidation, nil, false, readErr)
		}
		screenshots = append(screenshots, registryclient.ScreenshotUpload{Filename: filepath.Base(screenshot.Path), Alt: screenshot.Alt, Content: content})
	}
	release, err := s.registry.PublishWorkflow(ctx, registryclient.PublishRequest{
		Bundle: bytes.NewReader(bundle), Filename: request.WorkflowID + ".yotta-workflow",
		IdempotencyKey: publicationKey(request.WorkflowID, request.ReleaseVersion, string(info.SourceHash)),
		ReleaseVersion: request.ReleaseVersion, Title: request.Title, Summary: request.Summary,
		ReleaseNotes: request.ReleaseNotes, Examples: examples, Screenshots: screenshots,
	})
	if err != nil {
		return RegistryWorkflowReleaseView{}, registryError("publish", err)
	}
	return registryReleaseView(release), nil
}

func publicationKey(workflowID, version, sourceHash string) string {
	digest := sha256.Sum256([]byte(workflowID + "\x00" + version + "\x00" + sourceHash))
	return hex.EncodeToString(digest[:])
}

func (s *Service) SearchRegistry(ctx context.Context, search string, limit int) (RegistrySearchPageView, error) {
	if s.registry == nil {
		return RegistrySearchPageView{}, unavailable("registry")
	}
	page, err := s.registry.Search(ctx, search, limit)
	if err != nil {
		return RegistrySearchPageView{}, registryError("search", err)
	}
	view := RegistrySearchPageView{NextCursor: page.NextCursor, Items: make([]RegistryWorkflowReleaseView, 0, len(page.Items))}
	for _, item := range page.Items {
		if item.Kind == "workflow" {
			view.Items = append(view.Items, registryReleaseView(item.Workflow))
		}
	}
	return view, nil
}

func (s *Service) InstallRegistryWorkflow(ctx context.Context, releaseID string) (SourceView, error) {
	if s.bundles == nil || s.registry == nil {
		return SourceView{}, unavailable("registry")
	}
	plan, err := s.registry.CreateInstallPlan(ctx, releaseID, registryclient.Environment{
		YottaVersion: version.Version, OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH,
	})
	if err != nil {
		return SourceView{}, registryError("plan", err)
	}
	if len(plan.IncompatibleRequirements) != 0 {
		return SourceView{}, projectError("workflow.registry.incompatible", apperr.CategoryDomain, nil, false, errors.New("registry install plan is incompatible"))
	}
	bundle, err := s.registry.DownloadArtifact(ctx, plan.ResolvedWorkflowRelease.BundleDigest)
	if err != nil {
		return SourceView{}, registryError("download", err)
	}
	temporary, err := os.CreateTemp("", ".yotta-registry-*.yotta-workflow")
	if err != nil {
		return SourceView{}, projectError("workflow.registry.install_failed", apperr.CategoryInfrastructure, nil, true, err)
	}
	path := temporary.Name()
	defer os.Remove(path)
	if err := temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(bundle)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return SourceView{}, projectError("workflow.registry.install_failed", apperr.CategoryInfrastructure, nil, true, err)
	}
	result, err := s.bundles.Import(ctx, workflowbundle.ImportRequest{Path: path, Mode: workflowbundle.ImportCopy})
	if err != nil {
		return SourceView{}, bundleError("registry_import", err)
	}
	return sourceView(result.Source, false)
}

func registryReleaseView(release registryclient.WorkflowRelease) RegistryWorkflowReleaseView {
	examples := make([]PublishRegistryExample, 0, len(release.Examples))
	for _, example := range release.Examples {
		examples = append(examples, PublishRegistryExample{Title: example.Title, Description: example.Description})
	}
	screenshots := make([]RegistryScreenshotView, 0, len(release.Screenshots))
	for _, screenshot := range release.Screenshots {
		screenshots = append(screenshots, RegistryScreenshotView{URL: screenshot.URL, Alt: screenshot.Alt})
	}
	return RegistryWorkflowReleaseView{
		ReleaseID: release.ReleaseID, PublisherNamespace: release.PublisherNamespace,
		WorkflowID: release.WorkflowID, ReleaseVersion: release.ReleaseVersion,
		SourceHash: release.SourceHash, BundleDigest: release.BundleDigest,
		Title: release.Title, Summary: release.Summary, ReleaseNotes: release.ReleaseNotes,
		Examples: examples, Screenshots: screenshots,
		Creator:      RegistryCreatorView{UserKey: release.Creator.UserKey, DisplayName: release.Creator.DisplayName},
		Availability: release.Availability, PublishedAt: release.PublishedAt,
	}
}

func registryError(operation string, cause error) error {
	if errors.Is(cause, registryclient.ErrAuthenticationRequired) {
		return projectError("workflow.registry.authentication_required", apperr.CategoryPolicy, nil, false, cause)
	}
	if errors.Is(cause, context.Canceled) {
		return projectError("workflow.registry.cancelled", apperr.CategoryInfrastructure, map[string]any{"operation": operation}, false, cause)
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return projectError("workflow.registry.timeout", apperr.CategoryInfrastructure, map[string]any{"operation": operation}, true, cause)
	}
	var problem registryclient.Problem
	if errors.As(cause, &problem) {
		switch problem.Code {
		case "registry.release_version_conflict":
			return projectError("workflow.registry.release_version_conflict", apperr.CategoryDomain, nil, false, cause)
		case "registry.authentication_required":
			return projectError("workflow.registry.authentication_required", apperr.CategoryPolicy, nil, false, cause)
		case "registry.workflow_rejected", "registry.invalid_publication", "registry.bundle_required":
			return projectError("workflow.registry.workflow_rejected", apperr.CategoryDomain, nil, false, cause)
		case "registry.bundle_too_large":
			return projectError("workflow.registry.bundle_too_large", apperr.CategoryValidation, nil, false, cause)
		case "registry.invalid_search":
			return projectError("workflow.registry.invalid_search", apperr.CategoryValidation, nil, false, cause)
		}
	}
	return projectError("workflow.registry.unavailable", apperr.CategoryInfrastructure, map[string]any{"operation": operation}, true, cause)
}
