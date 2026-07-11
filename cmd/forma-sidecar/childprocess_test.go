package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// pythonAppScript is a minimal app.py that boots the real lib-forma-python
// SDK (imported via PYTHONPATH, set from sdk/python below) — this test
// exercises the actual child-process wiring (env vars, entrypoint
// resolution, signal handling), not a fake stand-in.
const pythonAppScript = `
import lib_forma
app = lib_forma.App()

@app.action("test.doc.ping")
def ping(inv, ctx):
    return {"pong": True}

app.run()
`

func requirePython3(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not on PATH")
	}
}

func TestStartAppProcess_ChildProcessLifecycle(t *testing.T) {
	requirePython3(t)

	sdkPath, err := filepath.Abs("../../sdk/python")
	if err != nil {
		t.Fatalf("resolve sdk path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sdkPath, "lib_forma")); err != nil {
		t.Skipf("lib_forma SDK not found at %s: %v", sdkPath, err)
	}
	t.Setenv("PYTHONPATH", sdkPath)

	appDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(appDir, "app.py"), []byte(pythonAppScript), 0644); err != nil {
		t.Fatalf("write app.py: %v", err)
	}

	socketDir := t.TempDir()
	appEndpoint := "unix://" + filepath.Join(socketDir, "app.sock")
	listenEndpoint := "unix://" + filepath.Join(socketDir, "sidecar.sock")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proc, err := startAppProcess(ctx, "python", appDir, "", appEndpoint, listenEndpoint)
	if err != nil {
		t.Fatalf("startAppProcess: %v", err)
	}

	healthURL := "http://forma-app/health"
	socketPath := filepath.Join(socketDir, "app.sock")
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: 2 * time.Second,
	}

	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				lastErr = nil
				break
			}
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("app process never became healthy on the socket its FORMA_APP_SOCKET env pointed to: %v", lastErr)
	}

	proc.Shutdown(3 * time.Second)
	// A SIGTERM'd process reports ProcessState.Exited() == false (that
	// method only means "exited on its own", not "exited due to a signal")
	// — what Shutdown actually guarantees is that Wait() has returned,
	// i.e. proc.done is closed, by the time it returns.
	select {
	case <-proc.done:
	default:
		t.Error("app process did not exit after Shutdown")
	}
	if proc.cmd.ProcessState == nil {
		t.Error("cmd.Wait() never completed")
	}
}

func TestStartAppProcess_UnknownRuntime(t *testing.T) {
	_, err := startAppProcess(context.Background(), "ruby", t.TempDir(), "", "unix:///a.sock", "unix:///b.sock")
	if err == nil {
		t.Fatal("expected error for unknown runtime")
	}
}

func TestStartAppProcess_RequiresUnixSockets(t *testing.T) {
	appDir := t.TempDir()
	os.WriteFile(filepath.Join(appDir, "app.py"), []byte("pass"), 0644)

	_, err := startAppProcess(context.Background(), "python", appDir, "", "http://localhost:9000", "unix:///b.sock")
	if err == nil {
		t.Fatal("expected error requiring unix:// for --app-endpoint in child-process mode")
	}
}

func TestStartAppProcess_MissingEntrypoint(t *testing.T) {
	_, err := startAppProcess(context.Background(), "python", t.TempDir(), "", "unix:///a.sock", "unix:///b.sock")
	if err == nil {
		t.Fatal("expected error for missing entrypoint file")
	}
}

func TestIsLocalRuntime(t *testing.T) {
	cases := map[string]bool{"": true, "local": true, "php": false, "python": false, "node": false}
	for runtime, want := range cases {
		if got := isLocalRuntime(runtime); got != want {
			t.Errorf("isLocalRuntime(%q) = %v, want %v", runtime, got, want)
		}
	}
}
