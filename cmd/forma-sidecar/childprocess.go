package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// runtimeCommand maps a --runtime value to the interpreter invocation and
// the conventional entrypoint filename inside --app-dir
// (docs/runtimes/04-forma-sidecar.md §5, child-process mode).
type runtimeCommand struct {
	command           string
	defaultEntrypoint string
}

var knownRuntimes = map[string]runtimeCommand{
	"php":    {command: "php", defaultEntrypoint: "app.php"},
	"python": {command: "python3", defaultEntrypoint: "app.py"},
	"node":   {command: "node", defaultEntrypoint: "app.js"},
}

// isLocalRuntime reports whether runtime selects the default,
// no-child-process behavior: the app process is started externally
// (its own K8s container, or manually in dev) and only needs to reach
// --app-endpoint on its own. This is the right choice for any language —
// with or without a lib-forma-* SDK — that isn't meant to be exec'd by
// the sidecar itself.
func isLocalRuntime(runtime string) bool {
	return runtime == "" || runtime == "local"
}

// appProcess supervises a child app process exec'd directly by the sidecar
// (docs/runtimes/04-forma-sidecar.md §5 "child process" mode) — the
// alternative to the separate-container mode, appropriate for lightweight
// runtimes where a second container's overhead isn't worth it.
type appProcess struct {
	cmd  *exec.Cmd
	done chan struct{} // closed once cmd.Wait() returns
}

// startAppProcess execs the app for the given --runtime. runtime may carry
// a version suffix (e.g. "php:8.3") which is informational only — the
// interpreter binary invoked is always the bare command ("php"), resolved
// via PATH. entrypoint overrides the runtime's conventional filename; empty
// uses the default (app.php / app.py / app.js).
//
// The child's env carries FORMA_APP_SOCKET and FORMA_SIDECAR_SOCKET so a
// lib-forma-* SDK boots with zero configuration — the same env vars its
// App()/App defaults read (sdk/README.md).
func startAppProcess(ctx context.Context, runtime, appDir, entrypoint, appEndpoint, listenEndpoint string) (*appProcess, error) {
	runtimeName := strings.SplitN(runtime, ":", 2)[0]
	rt, ok := knownRuntimes[runtimeName]
	if !ok {
		known := make([]string, 0, len(knownRuntimes))
		for name := range knownRuntimes {
			known = append(known, name)
		}
		return nil, fmt.Errorf("--runtime %q: unknown (want one of %s, or \"local\" to skip exec'ing a child)", runtime, strings.Join(known, ", "))
	}

	appSocket, err := socketPathOf("--app-endpoint", appEndpoint)
	if err != nil {
		return nil, err
	}
	sidecarSocket, err := socketPathOf("--listen", listenEndpoint)
	if err != nil {
		return nil, err
	}

	if entrypoint == "" {
		entrypoint = rt.defaultEntrypoint
	}
	entrypointPath := filepath.Join(appDir, entrypoint)
	if _, err := os.Stat(entrypointPath); err != nil {
		return nil, fmt.Errorf("app entrypoint %s: %w", entrypointPath, err)
	}

	cmd := exec.CommandContext(ctx, rt.command, entrypointPath)
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(),
		"FORMA_APP_SOCKET="+appSocket,
		"FORMA_SIDECAR_SOCKET="+sidecarSocket,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Run in its own process group so Shutdown can signal it directly
	// without the terminal's signal also hitting it twice.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("exec %s %s: %w", rt.command, entrypointPath, err)
	}
	log.Printf("[forma-sidecar] app process started: %s %s (pid %d)", rt.command, entrypointPath, cmd.Process.Pid)

	proc := &appProcess{cmd: cmd, done: make(chan struct{})}
	go proc.wait()
	return proc, nil
}

// wait calls cmd.Wait() exactly once — the sole owner of that call — and
// closes done when it returns, so Shutdown can wait for exit without racing
// a second Wait() call (which os/exec forbids).
func (p *appProcess) wait() {
	err := p.cmd.Wait()
	close(p.done)
	if err != nil && err.Error() != "signal: terminated" && err.Error() != "signal: killed" {
		log.Printf("[forma-sidecar] app process exited: %v", err)
	} else {
		log.Printf("[forma-sidecar] app process stopped")
	}
}

// Shutdown asks the app process to exit, escalating to SIGKILL if it does
// not within timeout.
func (p *appProcess) Shutdown(timeout time.Duration) {
	if p == nil || p.cmd.Process == nil {
		return
	}
	p.cmd.Process.Signal(syscall.SIGTERM)

	select {
	case <-p.done:
	case <-time.After(timeout):
		p.cmd.Process.Kill()
		<-p.done
	}
}

// socketPathOf extracts the filesystem path from a unix:// endpoint —
// child-process mode requires unix sockets on both directions since that
// is what the lib-forma-* SDKs' env-var defaults assume (sdk/README.md).
func socketPathOf(flagName, endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("%s %q: %w", flagName, endpoint, err)
	}
	if u.Scheme != "unix" {
		return "", fmt.Errorf("%s %q: child-process mode (--runtime other than local) requires a unix:// socket, got scheme %q", flagName, endpoint, u.Scheme)
	}
	path := u.Path
	if u.Host != "" {
		path = u.Host + u.Path
	}
	return path, nil
}
