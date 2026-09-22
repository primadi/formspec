package devserver

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
)

// TestEnsurePort_ForeignOwnerMessageIsActionable pins the guidance added for
// gap #24 ("pesan error port `formspec dev` lebih menuntun"): when the port is
// held by a process that is NOT a previous instance of this binary, the error
// must name the port and tell the caller how to move on (--addr), instead of
// only saying "stop it manually first".
func TestEnsurePort_ForeignOwnerMessageIsActionable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	// The owner name cannot match this test binary, so EnsurePort must treat
	// the holder as foreign and refuse instead of killing it.
	err = EnsurePort(fmt.Sprintf(":%d", port), "formspec-not-this-test-binary")
	if err == nil {
		t.Fatalf("EnsurePort(:%d) = nil, want an error while a foreign process holds the port", port)
	}

	msg := err.Error()
	if !strings.Contains(msg, strconv.Itoa(port)) {
		t.Errorf("error %q does not name the busy port %d", msg, port)
	}
	if !strings.Contains(msg, "--addr") {
		t.Errorf("error %q does not tell the caller how to recover (expected a --addr hint)", msg)
	}
}

// TestEnsurePort_FreePortReturnsNil guards the happy path: a free port is not
// an error, and a unix socket address is skipped entirely.
func TestEnsurePort_FreePortReturnsNil(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close() // free again

	if err := EnsurePort(fmt.Sprintf(":%d", port), "formspec"); err != nil {
		t.Errorf("EnsurePort(:%d) on a free port = %v, want nil", port, err)
	}
	if err := EnsurePort("unix:///tmp/formspec/sidecar.sock", "formspec"); err != nil {
		t.Errorf("EnsurePort(unix socket) = %v, want nil (port check skipped)", err)
	}
}
