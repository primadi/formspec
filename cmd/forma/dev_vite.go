// ─── Vite & App Process Management ───
//
// Adapted from cmd/forma-sidecar/childprocess.go.
// Handles spawning Vite dev server (--dev-ui) and child app processes
// (--runtime php/python/node).

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

// ─── App Process (child-process mode) ───

// appProcess supervises a child process exec'd by forma.
type appProcess struct {
	cmd  *exec.Cmd
	done chan struct{} // closed once cmd.Wait() returns
}

// wait calls cmd.Wait() exactly once and closes done.
func (p *appProcess) wait() {
	err := p.cmd.Wait()
	close(p.done)
	if err != nil && err.Error() != "signal: terminated" && err.Error() != "signal: killed" {
		log.Printf("[forma] app process exited: %v", err)
	} else {
		log.Printf("[forma] app process stopped")
	}
}

// Shutdown asks the process to exit, escalating to SIGKILL if needed.
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

// ─── Runtime Commands ───

type runtimeCommand struct {
	command           string
	defaultEntrypoint string
}

var knownRuntimes = map[string]runtimeCommand{
	"php":    {command: "php", defaultEntrypoint: "app.php"},
	"python": {command: "python3", defaultEntrypoint: "app.py"},
	"node":   {command: "node", defaultEntrypoint: "app.js"},
}

// startAppProcess execs the app for the given --runtime.
func startAppProcess(ctx context.Context, runtime, appDir, entrypoint, appEndpoint, listenEndpoint string) (*appProcess, error) {
	runtimeName := strings.SplitN(runtime, ":", 2)[0]
	rt, ok := knownRuntimes[runtimeName]
	if !ok {
		known := make([]string, 0, len(knownRuntimes))
		for name := range knownRuntimes {
			known = append(known, name)
		}
		return nil, fmt.Errorf("--runtime %q: unknown (want one of %s, or \"local\" to skip)", runtime, strings.Join(known, ", "))
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("exec %s %s: %w", rt.command, entrypointPath, err)
	}
	log.Printf("[forma] app process started: %s %s (pid %d)", rt.command, entrypointPath, cmd.Process.Pid)

	proc := &appProcess{cmd: cmd, done: make(chan struct{})}
	go proc.wait()
	return proc, nil
}

// socketPathOf extracts the filesystem path from a unix:// endpoint.
func socketPathOf(flagName, endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("%s %q: %w", flagName, endpoint, err)
	}
	if u.Scheme != "unix" {
		return "", fmt.Errorf("%s %q: child-process mode requires a unix:// socket, got scheme %q", flagName, endpoint, u.Scheme)
	}
	path := u.Path
	if u.Host != "" {
		path = u.Host + u.Path
	}
	return path, nil
}

// ─── Vite Dev Server ───

// findWebDir locates the web/ directory (for Vite HMR via --dev-ui).
// Uses runtime.Caller to find the source file location, which works even
// when forma is run via `go run` from a different working directory.
func findWebDir() (string, error) {
	// 1. Try CWD
	candidates := []string{"web", "../web", "./web"}
	for _, c := range candidates {
		info, err := os.Stat(filepath.Join(c, "package.json"))
		if err == nil && !info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}

	// 2. Try module cache (source file location)
	// Use runtime.Caller to find the main.go location
	// This only works for `go run` / built binary not moved
	execPath, err := os.Executable()
	if err == nil {
		// Walk up from executable to find web/
		dir := filepath.Dir(execPath)
		for i := 0; i < 5; i++ {
			candidate := filepath.Join(dir, "web")
			if info, err := os.Stat(filepath.Join(candidate, "package.json")); err == nil && !info.IsDir() {
				return candidate, nil
			}
			// Also try ../web from current dir
			dir = filepath.Dir(dir)
		}
	}

	return "", fmt.Errorf("cannot find web/ directory with package.json (checked %v and executable path)", candidates)
}

// startVite spawns npm run dev in the given web directory.
func startVite(ctx context.Context, webDir string) (*appProcess, error) {
	// Check if node_modules exists; if not, run npm install
	if _, err := os.Stat(filepath.Join(webDir, "node_modules")); os.IsNotExist(err) {
		log.Printf("[forma] running npm install in %s...", webDir)
		installCmd := exec.CommandContext(ctx, "npm", "install")
		installCmd.Dir = webDir
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			return nil, fmt.Errorf("npm install failed: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, "npm", "run", "dev")
	cmd.Dir = webDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start npm run dev: %w", err)
	}
	return &appProcess{cmd: cmd, done: make(chan struct{})}, nil
}
