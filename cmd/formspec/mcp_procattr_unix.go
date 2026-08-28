//go:build unix

package main

import (
	"os"
	"syscall"
)

// detachedSysProcAttr puts the spawned `formspec dev` in its own session so
// it survives the MCP server process exiting (restart_server, 03 §5).
func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// syscallSIGTERM returns the termination signal (POSIX).
func syscallSIGTERM() os.Signal {
	return syscall.SIGTERM
}
