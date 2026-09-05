package workflow_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/registryclient"
	"github.com/yottaapp/yotta/internal/services/workflow"
)

type screenshotRegistry struct {
	registryClientFake
	previous  registryclient.WorkflowRelease
	content   []byte
	captured  []registryclient.ScreenshotUpload
	downloads int
}

func (c *screenshotRegistry) GetWorkflowRelease(context.Context, string) (registryclient.WorkflowRelease, error) {
	return c.previous, nil
}
func (c *screenshotRegistry) DownloadArtifact(context.Context, string) ([]byte, error) {
	c.downloads++
	return c.content, nil
}
func (c *screenshotRegistry) PublishWorkflow(ctx context.Context, request registryclient.PublishRequest) (registryclient.WorkflowRelease, error) {
	c.captured = request.Screenshots
	return c.registryClientFake.PublishWorkflow(ctx, request)
}

func TestPublicationRetainsOnlyKnownPreviousScreenshots(t *testing.T) {
	runtime := workflowRuntime(t, time.Now())
	if err := runtime.Application.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	registry := &screenshotRegistry{content: encoded.Bytes()}
	service, err := workflow.NewService(runtime.Application, workflow.WithBundleManager(runtime.Bundles), workflow.WithRegistryClient(registry))
	if err != nil {
		t.Fatal(err)
	}
	source, err := service.CreateSource("Screenshot update")
	if err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	registry.previous = registryclient.WorkflowRelease{WorkflowID: source.WorkflowID, Screenshots: []registryclient.Screenshot{{URL: "/v1/artifacts/" + digest, Alt: "old"}}}
	path := filepath.Join(t.TempDir(), "new.png")
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	request := workflow.PublishRegistryRequest{WorkflowID: source.WorkflowID, ReleaseVersion: "2.0.0", Title: "Screenshots", Summary: "Update", PreviousReleaseID: "old-release", ExistingScreenshots: []workflow.RegistryScreenshotView{{URL: "https://registry.example/v1/artifacts/" + digest, Alt: "old"}}, Screenshots: []workflow.PublishRegistryScreenshot{{Path: path, Alt: "new"}}}
	if _, err := service.PublishSourceToRegistry(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(registry.captured) != 2 || registry.captured[0].Alt != "old" || registry.captured[1].Alt != "new" {
		t.Fatalf("screenshot order=%#v", registry.captured)
	}
	request.ExistingScreenshots[0].URL = "https://registry.example/v1/artifacts/sha256:" + strings.Repeat("b", 64)
	if _, err := service.PublishSourceToRegistry(context.Background(), request); apperr.From(err).ID != "workflow.registry.invalid_listing" {
		t.Fatalf("unknown reference=%v", err)
	}
	if registry.downloads != 1 {
		t.Fatalf("unapproved screenshot was downloaded: %d", registry.downloads)
	}
}
