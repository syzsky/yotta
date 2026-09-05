// Yotta desktop process entrypoint.
package main

import (
	"embed"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yottaapp/yotta/internal/desktopapp"
	"github.com/yottaapp/yotta/internal/serviceconfig"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	desktopMainWithReporter(desktopapp.Run, os.Stderr, showStartupError, os.Exit)
}

func desktopMain(start func(desktopapp.Config) error, stderr io.Writer, exit func(int)) {
	desktopMainWithReporter(start, stderr, func(string) {}, exit)
}

func desktopMainWithReporter(start func(desktopapp.Config) error, stderr io.Writer, report func(string), exit func(int)) {
	endpoints := serviceconfig.Values()
	if err := start(desktopapp.Config{
		Assets: assets, TrayIcon: trayIcon,
		HubURL: endpoints["YOTTA_HUB_URL"], HubAllowLoopbackHTTP: strings.EqualFold(endpoints["YOTTA_HUB_ALLOW_INSECURE_HTTP"], "true"),
		RegistryURL:               endpoints["YOTTA_REGISTRY_URL"],
		RegistryAllowLoopbackHTTP: strings.EqualFold(endpoints["YOTTA_REGISTRY_ALLOW_INSECURE_HTTP"], "true"),
		OIDCAuthorizationEndpoint: endpoints["YOTTA_OIDC_AUTHORIZATION_ENDPOINT"],
		OIDCTokenEndpoint:         endpoints["YOTTA_OIDC_TOKEN_ENDPOINT"],
		OIDCUserinfoEndpoint:      endpoints["YOTTA_OIDC_USERINFO_ENDPOINT"],
		OIDCClientID:              endpoints["YOTTA_OIDC_CLIENT_ID"],
		OIDCCallbackAddress:       endpoints["YOTTA_OIDC_CALLBACK_ADDRESS"],
		OIDCAudience:              endpoints["YOTTA_REGISTRY_AUDIENCE"],
	}); err != nil {
		message := fmt.Sprintf("Yotta startup failed: %v", err)
		_, _ = fmt.Fprintln(stderr, message)
		report(message)
		exit(1)
	}
}
