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

// runtimeCommand describes how to build, prepare, and run a sidecar app
// for a specific language/framework.
type runtimeCommand struct {
	command           string   // base command to run the app
	defaultEntrypoint string   // default entrypoint filename (relative to appDir)
	depDir            string   // subdirectory created by dependency install (relative to appDir)
	depInstallCmd     []string // command + args for installing dependencies
	buildCmd          []string // command + args for build step (empty = no build needed)
	buildCheck        string   // file to check after build (relative to appDir; empty = no build)
	devCmd            []string // command + args for dev mode with file watching (empty = use base command)
	useRunDir         bool     // if true, app process runs in the appDir (needed for go/cargo/dotnet)
}

var knownRuntimes = map[string]*runtimeCommand{
	"php": {
		command:           "php",
		defaultEntrypoint: "app.php",
		depDir:            "vendor",
		depInstallCmd:     []string{"composer", "install"},
		devCmd:            []string{"npx", "nodemon", "--watch", ".", "-e", "php", "--exec", "php app.php"},
	},
	"python": {
		command:           "python3",
		defaultEntrypoint: "app.py",
		depDir:            ".venv",
		depInstallCmd:     []string{"pip", "install", "-e", "."},
		devCmd:            []string{"python3", "-m", "watchfiles", "python app.py", "."},
	},
	"ruby": {
		command:           "ruby",
		defaultEntrypoint: "app.rb",
		depDir:            "vendor/bundle",
		depInstallCmd:     []string{"bundle", "install"},
		devCmd:            []string{"rerun", "--", "ruby", "app.rb"},
	},
	"java": {
		command:           "java",
		defaultEntrypoint: "App.java",
		depDir:            "target",
		depInstallCmd:     []string{"mvn", "compile", "-q"},
		buildCmd:          []string{"mvn", "compile", "-q"},
		buildCheck:        "target/classes",
		devCmd:            []string{"mvn", "compile", "exec:java", "-Dexec.mainClass=App", "-q"},
		useRunDir:         true,
	},
	"dotnet": {
		command:           "dotnet",
		defaultEntrypoint: "Program.cs",
		depDir:            "obj",
		depInstallCmd:     []string{"dotnet", "restore"},
		buildCmd:          []string{"dotnet", "build", "-q"},
		buildCheck:        "bin",
		devCmd:            []string{"dotnet", "watch", "run"},
		useRunDir:         true,
	},
	"go": {
		command:           "go",
		defaultEntrypoint: ".",
		depDir:            "vendor",
		depInstallCmd:     []string{"go", "mod", "tidy"},
		devCmd:            []string{"go", "run", "."},
		useRunDir:         true,
	},
	"rust": {
		command:           "cargo",
		defaultEntrypoint: ".",
		depDir:            "target",
		depInstallCmd:     []string{"cargo", "fetch"},
		buildCmd:          []string{"cargo", "build", "-q"},
		buildCheck:        "target/debug",
		devCmd:            []string{"cargo", "watch", "-x", "run"},
		useRunDir:         true,
	},
	"node": {
		command:           "node",
		defaultEntrypoint: "app.js",
		depDir:            "node_modules",
		depInstallCmd:     []string{"npm", "install"},
	},
}

// startAppProcess execs the app for the given --runtime.
// It handles dependency installation, optional build step, and dev-mode
// watch/reload based on per-runtime metadata in knownRuntimes.
func startAppProcess(ctx context.Context, runtime, appDir, entrypoint, appEndpoint, listenEndpoint string, devMode bool) (*appProcess, error) {
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

	// ── Dependency install ──
	if rt.depInstallCmd != nil {
		depDir := filepath.Join(appDir, rt.depDir)
		if _, err := os.Stat(depDir); os.IsNotExist(err) {
			log.Printf("[forma] running %s in %s...", rt.depInstallCmd[0], appDir)
			install := exec.CommandContext(ctx, rt.depInstallCmd[0], rt.depInstallCmd[1:]...)
			install.Dir = appDir
			install.Stdout = os.Stdout
			install.Stderr = os.Stderr
			if err := install.Run(); err != nil {
				return nil, fmt.Errorf("%s in %s: %w", rt.depInstallCmd[0], appDir, err)
			}
		}
	}

	// ── Build step (if needed and output missing) ──
	if rt.buildCmd != nil && rt.buildCheck != "" {
		buildCheckPath := filepath.Join(appDir, rt.buildCheck)
		if _, err := os.Stat(buildCheckPath); os.IsNotExist(err) {
			log.Printf("[forma] running %s...", rt.buildCmd[0])
			build := exec.CommandContext(ctx, rt.buildCmd[0], rt.buildCmd[1:]...)
			build.Dir = appDir
			build.Stdout = os.Stdout
			build.Stderr = os.Stderr
			if err := build.Run(); err != nil {
				return nil, fmt.Errorf("%s in %s: %w", rt.buildCmd[0], appDir, err)
			}
			log.Printf("[forma] app build complete")
		}
	}

	// ── Node.js special handling: TypeScript entrypoint via tsx ──
	isTS := strings.HasSuffix(entrypoint, ".ts") && runtimeName == "node"

	// ── Determine command: dev mode vs normal ──
	var command string
	var args []string

	if devMode && rt.devCmd != nil {
		command = rt.devCmd[0]
		args = append(args, rt.devCmd[1:]...)
		log.Printf("[forma] app: %s (dev mode)", strings.Join(rt.devCmd, " "))
	} else if isTS {
		// Node.js TypeScript (non-dev): tsx without --watch
		absDir, err := filepath.Abs(appDir)
		if err != nil {
			return nil, fmt.Errorf("resolve app dir: %w", err)
		}
		tsxCli := filepath.Join(absDir, "node_modules", "tsx", "dist", "cli.mjs")
		if _, err := os.Stat(tsxCli); err == nil {
			command = "node"
			args = append(args, tsxCli)
		} else {
			command = "npx"
			args = append(args, "tsx")
		}
		args = append(args, entrypoint)
		log.Printf("[forma] app: tsx %s", entrypoint)
	} else if rt.useRunDir {
		// Project-mode runtimes: go/cargo/dotnet run from their project dir
		command = rt.command
		switch runtimeName {
		case "go":
			args = []string{"run", "."}
		case "rust":
			args = []string{"run"}
		case "dotnet":
			args = []string{"run"}
		case "java":
			class := strings.TrimSuffix(entrypoint, ".java")
			args = []string{"-cp", "target/classes", class}
		default:
			args = []string{"run", "."}
		}
	} else {
		// Simple runtimes: php/python/ruby/node — run the entrypoint file
		entrypointPath := filepath.Join(appDir, entrypoint)
		if _, err := os.Stat(entrypointPath); err != nil {
			return nil, fmt.Errorf("app entrypoint %s: %w", entrypointPath, err)
		}
		command = rt.command
		args = append(args, entrypoint)
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(),
		"FORMA_APP_SOCKET="+appSocket,
		"FORMA_SIDECAR_SOCKET="+sidecarSocket,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	label := command
	if len(args) > 0 {
		label = command + " " + args[0]
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("exec %s: %w", label, err)
	}
	log.Printf("[forma] app process started: %s (pid %d)", label, cmd.Process.Pid)

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
