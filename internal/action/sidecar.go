package action

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/primadi/forma/renderers/jsonbpersist"
	"github.com/primadi/forma/pkg/spec"
)

// DefaultSidecarInvokeTimeout is how long the executor waits for the app
// process to finish a handler before giving up (docs/runtimes/04-forma-sidecar.md §4.2).
const DefaultSidecarInvokeTimeout = 30 * time.Second

// SidecarExecutor executes impl: { type: sidecar } actions by invoking the
// application process (PHP/Python/Node/...) over a local HTTP endpoint —
// a unix domain socket (default) or localhost TCP — per the protocol in
// docs/runtimes/04-forma-sidecar.md §4.2:
//
//	POST {endpoint}/invoke/{module}/{entity}/{action}
//	body:     serialized ExecuteParams (resource_id, resource, params, ...)
//	response: serialized ExecuteResult (data, new_state, events)
//
// A zero-value / NewSidecarExecutor() executor is unconfigured and fails
// every Execute — that is the behavior embedded Go apps get, where no app
// process exists. cmd/forma-sidecar wires a configured executor via
// NewSidecarExecutorWithEndpoint.
type SidecarExecutor struct {
	client  *http.Client
	baseURL string
	timeout time.Duration
}

// NewSidecarExecutor creates an unconfigured sidecar executor. Execute
// always fails until an app endpoint is configured — use
// NewSidecarExecutorWithEndpoint in a forma-sidecar process.
func NewSidecarExecutor() *SidecarExecutor {
	return &SidecarExecutor{}
}

// NewSidecarExecutorWithEndpoint creates a sidecar executor that invokes
// app handlers at endpoint. Supported endpoint forms:
//
//	unix:///tmp/forma/app.sock   — HTTP/1.1 over a unix domain socket
//	http://localhost:9000            — HTTP over localhost TCP
//
// timeout bounds each handler invocation; zero means DefaultSidecarInvokeTimeout.
func NewSidecarExecutorWithEndpoint(endpoint string, timeout time.Duration) (*SidecarExecutor, error) {
	if timeout <= 0 {
		timeout = DefaultSidecarInvokeTimeout
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("sidecar endpoint %q: %w", endpoint, err)
	}

	switch u.Scheme {
	case "unix":
		socketPath := u.Path
		if u.Host != "" { // tolerate unix://relative/path.sock
			socketPath = u.Host + u.Path
		}
		if socketPath == "" {
			return nil, fmt.Errorf("sidecar endpoint %q: missing socket path", endpoint)
		}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		}
		return &SidecarExecutor{
			client:  &http.Client{Transport: transport},
			baseURL: "http://forma-app", // host is ignored; the dialer targets the socket
			timeout: timeout,
		}, nil
	case "http":
		return &SidecarExecutor{
			client:  &http.Client{},
			baseURL: strings.TrimRight(endpoint, "/"),
			timeout: timeout,
		}, nil
	default:
		return nil, fmt.Errorf("sidecar endpoint %q: unsupported scheme %q (want unix:// or http://)", endpoint, u.Scheme)
	}
}

// sidecarInvokeRequest is the wire form of ExecuteParams (§4.2).
type sidecarInvokeRequest struct {
	ResourceID string         `json:"resource_id,omitempty"`
	Resource   map[string]any `json:"resource,omitempty"`
	Params     map[string]any `json:"params,omitempty"`
	UserID     string         `json:"user_id,omitempty"`
}

// sidecarEventEmission is the wire form of EventEmission.
type sidecarEventEmission struct {
	Name    string         `json:"name"`
	Durable bool           `json:"durable,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

// sidecarInvokeResponse is the wire form of ExecuteResult (§4.2).
type sidecarInvokeResponse struct {
	Data     any                    `json:"data"`
	NewState string                 `json:"new_state,omitempty"`
	Events   []sidecarEventEmission `json:"events,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

// Execute invokes the app-process handler for the action and maps its
// response back to an ExecuteResult.
func (e *SidecarExecutor) Execute(ctx context.Context, action spec.Action, params ExecuteParams) (*ExecuteResult, error) {
	if e.client == nil {
		return nil, fmt.Errorf(
			"sidecar execution not configured (action %s.%s, ref=%s): no app endpoint — run under forma-sidecar or wire NewSidecarExecutorWithEndpoint",
			params.Module, params.ActionName, action.Impl.Ref,
		)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	body, err := json.Marshal(sidecarInvokeRequest{
		ResourceID: params.ResourceID,
		Resource:   params.Resource,
		Params:     params.Params,
		UserID:     params.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("sidecar invoke %s.%s: marshal request: %w", params.Module, params.ActionName, err)
	}

	invokeURL := fmt.Sprintf("%s/invoke/%s/%s/%s",
		e.baseURL, url.PathEscape(params.Module), url.PathEscape(params.Entity), url.PathEscape(params.ActionName))

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, invokeURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sidecar invoke %s.%s: %w", params.Module, params.ActionName, err)
	}
	req.Header.Set("Content-Type", "application/json")
	// If a request-scoped TxScope is active (HandleCustomAction opened one
	// for this action execution), forward its registry id so the app
	// process can echo it back on its /ctx/entity/{op} callbacks — the Go
	// host reconstructs the same scope server-side (internal/sidecar/ctx.go)
	// even though it's a separate HTTP round-trip. See
	// renderers/jsonbpersist/txscope.go's scopeRegistry doc comment.
	if scopeID := db.ScopeIDFromContext(ctx); scopeID != "" {
		req.Header.Set("X-Forma-Scope-Id", scopeID)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || (ctx.Err() == context.DeadlineExceeded) {
			return nil, fmt.Errorf("sidecar invoke %s.%s: app did not respond within %s (gateway timeout)",
				params.Module, params.ActionName, e.timeout)
		}
		return nil, fmt.Errorf("sidecar invoke %s.%s: %w", params.Module, params.ActionName, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("sidecar invoke %s.%s: read response: %w", params.Module, params.ActionName, err)
	}

	var wire sidecarInvokeResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &wire); err != nil && resp.StatusCode == http.StatusOK {
			return nil, fmt.Errorf("sidecar invoke %s.%s: invalid response JSON: %w", params.Module, params.ActionName, err)
		}
	}

	if resp.StatusCode != http.StatusOK {
		msg := wire.Error
		if msg == "" {
			msg = strings.TrimSpace(string(respBody))
		}
		return nil, fmt.Errorf("sidecar invoke %s.%s: app returned %d: %s",
			params.Module, params.ActionName, resp.StatusCode, msg)
	}

	result := &ExecuteResult{Data: wire.Data, NewState: wire.NewState}
	for _, ev := range wire.Events {
		result.Events = append(result.Events, EventEmission{
			Name:    ev.Name,
			Durable: ev.Durable,
			Payload: ev.Payload,
		})
	}
	return result, nil
}
