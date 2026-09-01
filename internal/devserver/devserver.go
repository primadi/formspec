// Package devserver holds the dev-mode process helpers shared by the
// `formspec dev` command and the native binaries (formspec-registry):
//
//   - PID-file based auto-kill of a previous instance (AutoKillPrevious /
//     WritePIDFile / CleanupPIDFile)
//   - Port conflict resolution — kill a previous instance of the same
//     binary holding the port, error out for foreign processes (EnsurePort)
//   - Spec hot-reload watcher — fsnotify on the spec directory, debounced
//     App.ReloadSpec() (WatchSpec)
//
// Extracted from cmd/formspec/dev.go so native binaries that boot via
// App.ListenAndServe() get the same DX without duplicating logic.
package devserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	formspec "github.com/primadi/formspec/resource"
)

// ─── PID File ───

// AutoKillPrevious reads the PID file, kills the process if it still exists,
// then removes the stale file. No-op when the file does not exist.
func AutoKillPrevious(pidFile string) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("[formspec] warning: cannot read PID file %s: %v\n", pidFile, err)
		}
		return
	}

	oldPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Printf("[formspec] warning: invalid PID in %s, removing...\n", pidFile)
		os.Remove(pidFile)
		return
	}

	proc, err := os.FindProcess(oldPID)
	if err != nil {
		// Process not found — clean up stale PID file.
		os.Remove(pidFile)
		return
	}

	// Send SIGTERM; if it fails, the process is already dead.
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		os.Remove(pidFile)
		return
	}

	fmt.Printf("[formspec] killing previous instance (PID %d)...\n", oldPID)

	// Give it a real chance to run its own graceful shutdown (which stops
	// its children cleanly) instead of racing it with a fixed short sleep.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && ProcessAlive(oldPID) {
		time.Sleep(150 * time.Millisecond)
	}

	if ProcessAlive(oldPID) {
		fmt.Printf("[formspec] previous instance (PID %d) did not exit gracefully — forcing\n", oldPID)
		KillDescendants(oldPID)
		proc.Signal(syscall.SIGKILL)
		time.Sleep(200 * time.Millisecond)
	}

	os.Remove(pidFile)
}

// WritePIDFile writes the current PID to the given PID file (creating the
// parent directory if needed).
func WritePIDFile(pidFile string) {
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		fmt.Printf("[formspec] warning: cannot create dir for PID file %s: %v\n", pidFile, err)
		return
	}
	if err := os.WriteFile(pidFile, fmt.Appendf(nil, "%d\n", os.Getpid()), 0644); err != nil {
		fmt.Printf("[formspec] warning: cannot write PID file: %v\n", err)
	}
}

// CleanupPIDFile removes the PID file.
func CleanupPIDFile(pidFile string) {
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		fmt.Printf("[formspec] warning: cannot remove PID file: %v\n", err)
	}
}

// ProcessAlive reports whether pid still exists, via a zero-signal probe.
func ProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// ─── Port Conflict Resolution ───

// EnsurePort checks whether the given address is free. If it is in use by a
// previous instance of the same program (ownProcessName, matched against
// /proc/<pid>/comm), it kills that instance and waits for the port to free
// up. A foreign process holding the port yields a descriptive error.
func EnsurePort(addr, ownProcessName string) error {
	port, err := ExtractPort(addr)
	if err != nil || port == 0 {
		return nil // Unix sockets / unparseable — skip check
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err == nil {
		ln.Close()
		return nil
	}

	pid, procName, err := FindProcessOnPort(port)
	if err != nil {
		return fmt.Errorf("port %d is in use but cannot identify the owner: %w", port, err)
	}

	if procName == ownProcessName || procName == "exe" || strings.Contains(procName, ownProcessName) {
		fmt.Fprintf(os.Stderr, "port %d is held by a previous %s (PID %d) — killing it...\n", port, ownProcessName, pid)
		proc, err := os.FindProcess(pid)
		if err != nil {
			return nil
		}
		proc.Signal(syscall.SIGTERM)

		// Give the old instance a real chance to run its own graceful
		// shutdown instead of racing it with a fixed short sleep — that
		// previously SIGKILLed it before it signaled its children, leaving
		// them orphaned in their own process group.
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
				ln.Close()
				return nil
			}
			time.Sleep(150 * time.Millisecond)
		}

		// Still holding the port after a generous wait — force it, and
		// sweep any children it leaked so they don't accumulate.
		fmt.Fprintf(os.Stderr, "port %d: previous instance (PID %d) did not exit gracefully — forcing\n", port, pid)
		KillDescendants(pid)
		proc.Signal(syscall.SIGKILL)
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	return fmt.Errorf("port %d is already in use by %q (PID %d). Stop it manually first", port, procName, pid)
}

// KillDescendants force-kills every descendant of pid (depth-first) so that
// children left behind in their own process group don't survive a forced
// kill of that parent.
func KillDescendants(pid int) {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		return
	}
	for _, field := range strings.Fields(string(out)) {
		childPid, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		KillDescendants(childPid)
		if proc, err := os.FindProcess(childPid); err == nil {
			proc.Signal(syscall.SIGKILL)
		}
	}
}

// ExtractPort extracts the TCP port from an address string. Supports :8080,
// http://127.0.0.1:9090, etc. Unix sockets return 0.
func ExtractPort(addr string) (int, error) {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, "unix://") {
		return 0, nil
	}
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimPrefix(addr, "tcp://")

	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			portStr = addr[1:]
		} else {
			return 0, fmt.Errorf("cannot parse address %q: %w", addr, err)
		}
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q in address %q", portStr, addr)
	}
	return port, nil
}

// FindProcessOnPort returns the PID and process name for the process
// listening on the given port (lsof first, fuser fallback).
func FindProcessOnPort(port int) (int, string, error) {
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	out, err := cmd.Output()
	if err == nil {
		pidStr := strings.TrimSpace(string(out))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			// Read /proc/PID/comm on Linux for the process name.
			comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
			if err == nil {
				return pid, strings.TrimSpace(string(comm)), nil
			}
			return pid, fmt.Sprintf("PID %d", pid), nil
		}
	}

	cmd = exec.Command("fuser", fmt.Sprintf("%d/tcp", port))
	out, err = cmd.Output()
	if err == nil {
		pidStr := strings.TrimSpace(string(out))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			return pid, fmt.Sprintf("PID %d", pid), nil
		}
	}

	return 0, "", fmt.Errorf("cannot determine owner of port %d", port)
}

// ─── Spec Hot-Reload Watcher ───

// WatchSpec watches specPath for YAML/STAR file changes and triggers a full
// reload via App.ReloadSpec() (debounced 300ms). onReload, when non-nil, is
// called after each successful reload (e.g. to notify a Vite HMR endpoint).
//
// Runs until ctx is cancelled (SIGINT/SIGTERM).
func WatchSpec(ctx context.Context, app *formspec.App, specPath string, onReload func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("[formspec] spec watcher: %v (hot-reload disabled)\n", err)
		return
	}
	defer watcher.Close()

	// Add the spec directory and all subdirectories recursively.
	filepath.Walk(specPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == "impl" {
				return filepath.SkipDir
			}
			if watchErr := watcher.Add(path); watchErr != nil {
				fmt.Printf("[formspec] spec watcher: cannot watch %s: %v\n", path, watchErr)
			}
		}
		return nil
	})

	fmt.Printf("[formspec] watching %s for spec changes (hot-reload)\n", specPath)

	// Debounce: coalesce rapid file events (editor save sequences) into a
	// single reload call after 300ms of inactivity.
	const debounceInterval = 300 * time.Millisecond
	var timer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Automatically watch newly created subdirectories.
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					base := filepath.Base(event.Name)
					if !strings.HasPrefix(base, ".") && base != "node_modules" && base != "impl" {
						watcher.Add(event.Name)
					}
				}
			}

			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			ext := strings.ToLower(filepath.Ext(event.Name))
			if ext != ".yaml" && ext != ".yml" && ext != ".star" {
				continue
			}

			// Reset debounce timer on each event.
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(debounceInterval)
			go func(name string) {
				<-timer.C
				fmt.Printf("[formspec] spec change detected: %s — reloading...\n", filepath.Base(name))
				if err := app.ReloadSpec(); err != nil {
					fmt.Printf("[formspec] spec reload error: %v\n", err)
				} else {
					fmt.Printf("[formspec] spec reload complete\n")
					if onReload != nil {
						onReload()
					}
				}
			}(event.Name)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("[formspec] spec watcher error: %v\n", err)
		}
	}
}

// ServeAppUntilSignal runs app.ListenAndServe() (which also starts the
// background workers) and blocks until ctx is cancelled (SIGINT/SIGTERM) or
// the server errors. It then gracefully closes the App — stopping the outbox,
// escalation, and streaming workers plus the HTTP server (10s deadline).
func ServeAppUntilSignal(ctx context.Context, app *formspec.App) {
	errCh := make(chan error, 1)
	go func() { errCh <- app.ListenAndServe() }()

	select {
	case <-ctx.Done():
		fmt.Println("[formspec] shutting down...")
	case err := <-errCh:
		fmt.Printf("[formspec] server error: %v\n", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = app.Close(shutdownCtx)
}
