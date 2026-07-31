//go:build windows

package main

import "os/exec"

// setProcessGroup is a no-op on Windows: the syscall.SysProcAttr type has no
// Setpgid field (there is no POSIX process-group concept). killDescendants is
// pgrep-based and Unix-only anyway, so nothing relies on the process group
// here; the direct child is terminated via SIGTERM (synthetic on Windows).
func setProcessGroup(cmd *exec.Cmd) {
	// no-op
}
