package workflow_test

import (
	"context"
	"github.com/yottaapp/yotta/internal/registryclient"
	"github.com/yottaapp/yotta/internal/services/workflow"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type commerceBuyerToken string

func (t commerceBuyerToken) Token(context.Context) (string, error) { return string(t), nil }

func TestCommerceExportLocalFixture(t *testing.T) {
	dir := os.Getenv("YOTTA_COMMERCE_TEST_FIXTURE_DIR")
	if dir == "" {
		t.Skip("explicit fixture output directory required")
	}
	runtime := workflowRuntime(t, time.Now())
	if err := runtime.Application.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })
	service, err := workflow.NewService(runtime.Application, workflow.WithBundleManager(runtime.Bundles))
	if err != nil {
		t.Fatal(err)
	}
	source, err := service.CreateSource("Commerce 本地安装验收")
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"v1", "v2"} {
		if version == "v2" {
			if _, err = service.UpdateSourceMetadata(source.WorkflowID, source.Revision, workflow.UpdateSourceMetadataRequest{Name: "Commerce 本地更新验收"}); err != nil {
				t.Fatal(err)
			}
		}
		_, bundle, err := runtime.Bundles.ExportBytes(context.Background(), source.WorkflowID)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "commerce-current-"+version+".yotta-workflow"), bundle, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// Opt-in live consumer acceptance: this uses real Registry downloads and the
// production ImportRegistry path, with a disposable local workflow database.
func TestCommerceLivePurchasedInstall(t *testing.T) {
	base := os.Getenv("YOTTA_COMMERCE_TEST_REGISTRY_URL")
	if base == "" {
		t.Skip("explicit local Commerce acceptance environment required")
	}
	token, release := os.Getenv("YOTTA_COMMERCE_TEST_TOKEN"), os.Getenv("YOTTA_COMMERCE_TEST_RELEASE")
	if token == "" || release == "" {
		t.Fatal("buyer token and paid release required")
	}
	client, err := registryclient.New(registryclient.Options{BaseURL: base, Tokens: commerceBuyerToken(token), AllowLoopbackHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	consumer := workflowRuntime(t, time.Now())
	if err = consumer.Application.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = consumer.Close(context.Background()) })
	state := filepath.Join(t.TempDir(), "installations.json")
	service, err := workflow.NewService(consumer.Application, workflow.WithBundleManager(consumer.Bundles), workflow.WithRegistryClient(client), workflow.WithRegistryState(state))
	if err != nil {
		t.Fatal(err)
	}
	installed, err := service.InstallRegistryWorkflow(context.Background(), release)
	if err != nil {
		t.Fatal(err)
	}
	if installed.WorkflowID == "" {
		t.Fatal("no installed workflow")
	}
	if update := os.Getenv("YOTTA_COMMERCE_TEST_UPDATE"); update != "" {
		updated, err := service.InstallRegistryWorkflow(context.Background(), update)
		if err != nil {
			t.Fatal(err)
		}
		if updated.WorkflowID != installed.WorkflowID {
			t.Fatal("ordinary update changed work identity")
		}
	}
	// Reopen the application service without any network client: local content
	// and the installation record must remain usable after installation.
	offline, err := workflow.NewService(consumer.Application, workflow.WithBundleManager(consumer.Bundles), workflow.WithRegistryState(state))
	if err != nil {
		t.Fatal(err)
	}
	records, err := offline.RegistryInstallations()
	if err != nil || len(records) != 1 {
		t.Fatalf("offline installations=%v error=%v", records, err)
	}
	if _, err = consumer.Application.GetSource(installed.WorkflowID); err != nil {
		t.Fatal(err)
	}
}
