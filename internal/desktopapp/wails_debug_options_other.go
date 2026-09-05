//go:build !windows

package desktopapp

import "github.com/wailsapp/wails/v3/pkg/application"

func wailsWindowsOptions() application.WindowsOptions {
	return application.WindowsOptions{}
}
