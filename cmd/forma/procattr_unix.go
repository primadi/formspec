//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts cmd in its own process group so that Shutdown can
// sweep the whole descendant tree via killDescendants. Unix-only: relies
// on Setpgid which does not exist on Windows.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
