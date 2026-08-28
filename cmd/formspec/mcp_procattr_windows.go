//go:build windows

package main

import (
	"os"
	"syscall"
)

// detachedSysProcAttr is a no-op on Windows: CREATE_NEW_PROCESS_GROUP needs
// explicit platform plumbing; the spawned dev process is tracked via the PID
// file either way (restart_server, 03 §5).
func detachedSysProcAttr() *syscall.SysProcAttr {
	return nil
}

// syscallSIGTERM returns the closest Windows equivalent (os.Interrupt).
func syscallSIGTERM() os.Signal {
	return os.Interrupt
}
