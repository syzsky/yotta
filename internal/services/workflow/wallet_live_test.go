package workflow_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/nativeoidc"
	"github.com/yottaapp/yotta/internal/registryclient"
	"github.com/yottaapp/yotta/internal/securestore"
	"github.com/yottaapp/yotta/internal/services/workflow"
	"github.com/yottaapp/yotta/internal/storage"
)

// This opt-in test uses real Identity/Commerce and a controlled browser driver.
// Only browser selection differs from the desktop; tokens and wallet grants are
// acquired by production code and retained in the explicit test profile's OS store.
func TestWalletLiveNativeAuthorization(t *testing.T) {
	profile, driver := os.Getenv("YOTTA_WALLET_TEST_PROFILE"), os.Getenv("YOTTA_WALLET_TEST_BROWSER")
	if profile == "" || driver == "" {
		t.Skip("explicit isolated profile and browser driver required")
	}
	roots, err := storage.Resolve(profile)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := storage.CredentialScope(roots)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	browser := walletLiveBrowser{ctx: ctx, driver: driver}
	config := nativeoidc.Config{Credentials: securestore.New(), CredentialScope: scope,
		AccountURL: "http://127.0.0.1:13800/", AuthorizationEndpoint: "http://127.0.0.1:13800/oauth2/authorize",
		TokenEndpoint: "http://127.0.0.1:18881/oauth2/token", UserinfoEndpoint: "http://127.0.0.1:18881/oauth2/userinfo",
		ClientID: "yotta-desktop-wallet-simulation", CallbackAddress: "127.0.0.1:43825",
		Scopes: []string{"openid", "profile", "offline_access"}, Browser: browser}
	account, err := nativeoidc.New(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = account.Login(ctx); err != nil {
		t.Fatal(err)
	}
	if account.Profile().UserKey != "CiRZ3pe3" || account.Profile().SessionOnly {
		t.Fatal("expected persisted simulation buyer")
	}
	client, err := registryclient.New(registryclient.Options{BaseURL: "http://127.0.0.1:18116", Tokens: account, AllowLoopbackHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	runtime := workflowRuntime(t, time.Now())
	if err = runtime.Application.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(context.Background())
	options := []workflow.Option{workflow.WithRegistryClient(client), workflow.WithRegistryAccount(account), workflow.WithWallet(securestore.New(), scope+"\x00"+config.TokenEndpoint+"\x00http://127.0.0.1:18116", browser)}
	service, err := workflow.NewService(runtime.Application, options...)
	if err != nil {
		t.Fatal(err)
	}
	view, err := service.AuthorizeRegistryWallet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Authorized || view.BalanceCents != 10000 {
		t.Fatalf("unexpected public wallet view: %+v", view)
	}
	restored, err := nativeoidc.New(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = restored.RefreshProfile(ctx); err != nil {
		t.Fatal(err)
	}
	if restored.Profile().UserKey != "CiRZ3pe3" {
		t.Fatal("native account was not restored")
	}
	client, err = registryclient.New(registryclient.Options{BaseURL: "http://127.0.0.1:18116", Tokens: restored, AllowLoopbackHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	options[0], options[1] = workflow.WithRegistryClient(client), workflow.WithRegistryAccount(restored)
	again, err := workflow.NewService(runtime.Application, options...)
	if err != nil {
		t.Fatal(err)
	}
	view, err = again.RegistryWallet(ctx)
	if err != nil || !view.Authorized || view.BalanceCents != 10000 {
		t.Fatalf("wallet restore failed: %+v %v", view, err)
	}
}

type walletLiveBrowser struct {
	ctx    context.Context
	driver string
}

func (b walletLiveBrowser) OpenURL(raw string) error {
	// URL contains only the normal OAuth challenge/state or public consent request.
	// Credentials remain in the browser driver and OS store, never in command output.
	command := exec.CommandContext(b.ctx, "node", b.driver, raw)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	return command.Run()
}
