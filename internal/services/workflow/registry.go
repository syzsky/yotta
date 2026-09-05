package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/yottaapp/yotta/internal/workflowstore"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/nativeoidc"
	"github.com/yottaapp/yotta/internal/registryclient"
	"github.com/yottaapp/yotta/internal/workflowbundle"
	"github.com/yottaapp/yotta/pkg/version"
)

type RegistryCreatorView struct {
	Picture     string `json:"picture,omitempty"`
	UserKey     string `json:"userKey"`
	DisplayName string `json:"displayName,omitempty"`
}

type RegistryWorkflowReleaseView struct {
	DownloadCount      int64                              `json:"downloadCount"`
	Dependencies       []registryclient.DependencySummary `json:"dependencies"`
	Listing            registryclient.Listing             `json:"listing"`
	Facts              registryclient.BundleFacts         `json:"facts"`
	ReleaseID          string                             `json:"releaseId"`
	PublisherNamespace string                             `json:"publisherNamespace"`
	WorkflowID         string                             `json:"workflowId"`
	ReleaseVersion     string                             `json:"releaseVersion"`
	SourceHash         string                             `json:"sourceHash"`
	BundleDigest       string                             `json:"bundleDigest"`
	Title              string                             `json:"title"`
	Summary            string                             `json:"summary"`
	ReleaseNotes       string                             `json:"releaseNotes,omitempty"`
	Examples           []PublishRegistryExample           `json:"examples"`
	Screenshots        []RegistryScreenshotView           `json:"screenshots"`
	Creator            RegistryCreatorView                `json:"creator"`
	Availability       string                             `json:"availability"`
	PublishedAt        string                             `json:"publishedAt"`
}

type RegistryScreenshotView struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

type RegistrySearchPageView struct {
	Facets     registryclient.Facets         `json:"facets"`
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
	PreviousReleaseID   string                      `json:"previousReleaseId,omitempty"`
	ExistingScreenshots []RegistryScreenshotView    `json:"existingScreenshots,omitempty"`
	Listing             registryclient.Listing      `json:"listing"`
	WorkflowID          string                      `json:"workflowId"`
	ReleaseVersion      string                      `json:"releaseVersion"`
	Title               string                      `json:"title"`
	Summary             string                      `json:"summary"`
	ReleaseNotes        string                      `json:"releaseNotes"`
	Examples            []PublishRegistryExample    `json:"examples"`
	Screenshots         []PublishRegistryScreenshot `json:"screenshots"`
}

func (s *Service) PublishSourceToRegistry(
	ctx context.Context, request PublishRegistryRequest,
) (RegistryWorkflowReleaseView, error) {
	if n := utf8.RuneCountInString(strings.TrimSpace(request.Title)); n < 1 || n > 160 {
		return RegistryWorkflowReleaseView{}, projectError("workflow.registry.invalid_title", apperr.CategoryValidation, nil, false, errors.New("title length outside 1..160"))
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(request.Summary)); n < 1 || n > 1000 {
		return RegistryWorkflowReleaseView{}, projectError("workflow.registry.invalid_summary", apperr.CategoryValidation, nil, false, errors.New("summary length outside 1..1000"))
	}
	if !registryclient.ValidWorkflowReleaseVersion(request.ReleaseVersion) {
		return RegistryWorkflowReleaseView{}, projectError("workflow.registry.invalid_version", apperr.CategoryValidation, nil, false, errors.New("workflow version requires three non-negative integers"))
	}
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
	if len(request.ExistingScreenshots)+len(request.Screenshots) > 6 {
		return RegistryWorkflowReleaseView{}, projectError("workflow.registry.invalid_listing", apperr.CategoryValidation, nil, false, errors.New("too many screenshots"))
	}
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
	if len(request.ExistingScreenshots) > 0 {
		newScreenshots := screenshots
		screenshots = make([]registryclient.ScreenshotUpload, 0, len(newScreenshots)+len(request.ExistingScreenshots))
		previous, err := s.registry.GetWorkflowRelease(ctx, request.PreviousReleaseID)
		if err != nil {
			return RegistryWorkflowReleaseView{}, registryError("previous_release", err)
		}
		if previous.WorkflowID != request.WorkflowID {
			return RegistryWorkflowReleaseView{}, projectError("workflow.registry.invalid_listing", apperr.CategoryValidation, nil, false, errors.New("screenshot source workflow differs"))
		}
		for _, reference := range request.ExistingScreenshots {
			parsed, parseErr := url.Parse(reference.URL)
			allowed := false
			if parseErr == nil {
				for _, image := range previous.Screenshots {
					candidate, err := url.Parse(image.URL)
					if err == nil && candidate.Path == parsed.Path {
						allowed = true
						break
					}
				}
			}
			if !allowed || !strings.HasPrefix(parsed.Path, "/v1/artifacts/sha256:") {
				return RegistryWorkflowReleaseView{}, projectError("workflow.registry.invalid_listing", apperr.CategoryValidation, nil, false, errors.New("unknown previous screenshot"))
			}
			content, err := s.registry.DownloadArtifact(ctx, strings.TrimPrefix(parsed.Path, "/v1/artifacts/"))
			if err != nil {
				return RegistryWorkflowReleaseView{}, registryError("screenshot", err)
			}
			if len(content) > 16<<20 {
				return RegistryWorkflowReleaseView{}, projectError("workflow.registry.invalid_screenshot", apperr.CategoryValidation, nil, false, errors.New("previous screenshot exceeds limit"))
			}
			screenshots = append(screenshots, registryclient.ScreenshotUpload{Filename: "previous-image.png", Alt: reference.Alt, Content: content})
		}
		screenshots = append(screenshots, newScreenshots...)
	}
	release, err := s.registry.PublishWorkflow(ctx, registryclient.PublishRequest{
		Listing: request.Listing,
		Bundle:  bytes.NewReader(bundle), Filename: request.WorkflowID + ".yotta-workflow",
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
	s.registryMu.Lock()
	defer s.registryMu.Unlock()
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
	records, err := s.readRegistryInstallations()
	if err != nil {
		return SourceView{}, registryError("installed", err)
	}
	release := plan.ResolvedWorkflowRelease
	request := workflowbundle.ImportRequest{Mode: workflowbundle.ImportRegistry}
	current, currentErr := s.application.GetSource(release.WorkflowID)
	if currentErr == nil {
		if record, tracked := records[release.WorkflowID]; tracked && !registryclient.IsNewerWorkflowVersion(release.ReleaseVersion, record.ReleaseVersion) {
			return sourceView(current, false)
		}
		if string(current.Hash()) == release.SourceHash {
			records[release.WorkflowID] = RegistryInstallation{WorkflowID: release.WorkflowID, ReleaseID: releaseID, ReleaseVersion: release.ReleaseVersion, SourceHash: string(current.Hash())}
			if err := s.saveRegistryInstallations(records); err != nil {
				return SourceView{}, registryError("record_install", err)
			}
			return sourceView(current, false)
		}
		record, tracked := records[release.WorkflowID]
		if tracked && record.ReleaseID == releaseID {
			return sourceView(current, false)
		}
		if !tracked || record.SourceHash != string(current.Hash()) {
			return SourceView{}, localRegistryChanges()
		}
		request.Mode, request.TargetWorkflowID = workflowbundle.ImportReplace, release.WorkflowID
		request.ExpectedRevision, request.ExpectedSourceHash = current.Revision(), current.Hash()
	} else if !errors.Is(currentErr, workflowstore.ErrSourceNotFound) {
		return SourceView{}, sourceError("install", currentErr)
	}
	var bundle []byte
	if downloader, ok := s.registry.(interface {
		DownloadWorkflow(context.Context, string, string) ([]byte, error)
	}); ok {
		bundle, err = downloader.DownloadWorkflow(ctx, releaseID, release.BundleDigest)
	} else {
		bundle, err = s.registry.DownloadArtifact(ctx, release.BundleDigest)
	}
	if err != nil {
		return SourceView{}, registryError("download", err)
	}
	temporary, err := os.CreateTemp("", ".yotta-registry-*.yotta-workflow")
	if err != nil {
		return SourceView{}, projectError("workflow.registry.install_failed", apperr.CategoryInfrastructure, nil, true, err)
	}
	path := temporary.Name()
	defer os.Remove(path)
	if err = temporary.Chmod(0o600); err == nil {
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
	info, err := s.bundles.Inspect(ctx, path)
	if err != nil {
		return SourceView{}, bundleError("registry_inspect", err)
	}
	if info.WorkflowID != release.WorkflowID || string(info.SourceHash) != release.SourceHash {
		return SourceView{}, registryError("identity", errors.New("release and bundle identity differ"))
	}
	request.Path = path
	result, err := s.bundles.Import(ctx, request)
	if err != nil {
		return SourceView{}, bundleError("registry_import", err)
	}
	records[release.WorkflowID] = RegistryInstallation{WorkflowID: release.WorkflowID, ReleaseID: releaseID, ReleaseVersion: release.ReleaseVersion, SourceHash: string(result.Source.Hash())}
	if err := s.saveRegistryInstallations(records); err != nil {
		return SourceView{}, registryError("record_install", err)
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
		DownloadCount: release.DownloadCount,
		Dependencies:  release.Dependencies,
		Listing:       release.Listing, Facts: release.Facts,
		ReleaseID: release.ReleaseID, PublisherNamespace: release.PublisherNamespace,
		WorkflowID: release.WorkflowID, ReleaseVersion: release.ReleaseVersion,
		SourceHash: release.SourceHash, BundleDigest: release.BundleDigest,
		Title: release.Title, Summary: release.Summary, ReleaseNotes: release.ReleaseNotes,
		Examples: examples, Screenshots: screenshots,
		Creator:      RegistryCreatorView{UserKey: release.Creator.UserKey, DisplayName: release.Creator.DisplayName, Picture: release.Creator.Picture},
		Availability: release.Availability, PublishedAt: release.PublishedAt,
	}
}

func registryError(operation string, cause error) error {
	if errors.Is(cause, registryclient.ErrAuthenticationRequired) || errors.Is(cause, nativeoidc.ErrAuthenticationRequired) {
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
		case "registry.version_not_increasing":
			return projectError("workflow.registry.version_not_increasing", apperr.CategoryValidation, nil, false, cause)
		case "registry.invalid_title":
			return projectError("workflow.registry.invalid_title", apperr.CategoryValidation, nil, false, cause)
		case "registry.invalid_summary":
			return projectError("workflow.registry.invalid_summary", apperr.CategoryValidation, nil, false, cause)
		case "registry.invalid_screenshot", "registry.screenshot_too_large":
			return projectError("workflow.registry.invalid_screenshot", apperr.CategoryValidation, nil, false, cause)
		case "registry.invalid_presentation":
			return projectError("workflow.registry.invalid_presentation", apperr.CategoryValidation, nil, false, cause)
		case "registry.bundle_invalid":
			return projectError("workflow.registry.bundle_invalid", apperr.CategoryDomain, nil, false, cause)
		case "registry.invalid_listing":
			return projectError("workflow.registry.invalid_listing", apperr.CategoryValidation, nil, false, cause)
		case "registry.invalid_release_version":
			return projectError("workflow.registry.invalid_version", apperr.CategoryValidation, nil, false, cause)
		case "registry.workflow_not_owner":
			return projectError("workflow.registry.not_owner", apperr.CategoryPolicy, nil, false, cause)
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
