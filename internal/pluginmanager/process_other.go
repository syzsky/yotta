//go:build !windows

package pluginmanager

import "os/exec"

func hideProcess(cmd *exec.Cmd) {}
