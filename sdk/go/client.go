// Package formspec is the thin Go SDK for formspec sidecar communication.
//
// Usage:
//
//	client, err := formspec.Connect("acme-corp")
//	if err != nil { ... }
//
//	// Invoke a business logic handler
//	result, err := client.InvokeAction("billing", "order", "checkout", params)
//
//	// Entity operations (workspace-scoped by the sidecar — no tenantId params)
//	err := client.Entity().Update("ord-001", map[string]any{"status": "paid"})
package formspec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// DefaultSidecarPath is the default Unix socket path for the formspec sidecar.
const DefaultSidecarPath = "/tmp/formspec/sidecar.sock"

// Client is a thin client for the formspec sidecar.
// It is scoped to a single workspace — the workspace ID is bound at
// connection time and injected as a header on every request.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	workspaceID string
}

// Connect establishes a connection to the formspec sidecar.
// workspaceID is bound for the lifetime of this client.
// If endpoint is empty, defaults to DefaultSidecarPath (Unix socket).
//
// The client communicates with the sidecar via HTTP over a Unix socket.
// The workspace ID is automatically injected as X-FormSpec-Workspace header
// on every request — the sidecar derives the internal tenant ID from it.
func Connect(workspaceID string, endpoint string) (*Client, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("formspec: workspaceID is required")
	}
	if endpoint == "" {
		endpoint = "unix://" + DefaultSidecarPath
	}

	return connect(workspaceID, endpoint)
}

func connect(workspaceID, endpoint string) (*Client, error) {
	u, pathPrefix, transport, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("formspec: %w", err)
	}

	return &Client{
		httpClient:  &http.Client{Transport: transport, Timeout: 30 * time.Second},
		baseURL:     u + pathPrefix,
		workspaceID: workspaceID,
	}, nil
}

func parseEndpoint(endpoint string) (string, string, *http.Transport, error) {
	if len(endpoint) < 3 {
		return "", "", nil, fmt.Errorf("invalid endpoint %q", endpoint)
	}

	if endpoint[:3] == "unix" {
		// unix:///path/to/sock or unix://relative/path.sock
		socketPath := endpoint[7:] // strip "unix://"
		if socketPath == "" {
			return "", "", nil, fmt.Errorf("invalid unix endpoint %q: missing socket path", endpoint)
		}
		transport := &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(context.Background(), "unix", socketPath)
			},
		}
		return "http://formspec-sidecar", "", transport, nil
	}

	if endpoint[:4] == "http" {
		// http://localhost:9090
		transport := &http.Transport{}
		return endpoint, "", transport, nil
	}

	return "", "", nil, fmt.Errorf("unsupported endpoint scheme %q (want unix:// or http://)", endpoint)
}

// do sends an HTTP request to the sidecar, auto-injecting workspace header.
func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("formspec: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("formspec: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-FormSpec-Workspace", c.workspaceID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("formspec: request failed: %w", err)
	}
	return resp, nil
}

// doJSON sends a request and decodes the JSON response.
func (c *Client) doJSON(method, path string, body, target any) error {
	resp, err := c.do(method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.NewDecoder(resp.Body).Decode(&errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("formspec: %s", errResp.Error)
		}
		return fmt.Errorf("formspec: sidecar returned status %d", resp.StatusCode)
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("formspec: decode response: %w", err)
		}
	}
	return nil
}

// ─── Action Invocation ───

// ActionResult is the result of an action invocation.
type ActionResult struct {
	Data     any    `json:"data"`
	NewState string `json:"new_state,omitempty"`
}

// InvokeAction calls a business logic handler registered via App.Handle().
func (c *Client) InvokeAction(module, entity, action string, params map[string]any) (*ActionResult, error) {
	body := map[string]any{
		"params": params,
	}

	var result ActionResult
	err := c.doJSON("POST", fmt.Sprintf("/invoke/%s/%s/%s", module, entity, action), body, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ─── Primitive Handles ───

// Primitive is a handle to a ctx.* primitive (db, cache, lock, etc.).
type Primitive struct {
	client *Client
	typ    string
	name   string // empty = default
}

// DB returns a handle for ctx.db operations.
func (c *Client) DB() *Primitive { return &Primitive{client: c, typ: "db"} }

// Cache returns a handle for ctx.cache operations.
func (c *Client) Cache() *Primitive { return &Primitive{client: c, typ: "cache"} }

// Lock returns a handle for ctx.lock operations.
func (c *Client) Lock() *Primitive { return &Primitive{client: c, typ: "lock"} }

// Named binds this primitive to a named datastore instead of the default one.
func (p *Primitive) Named(name string) *Primitive {
	return &Primitive{client: p.client, typ: p.typ, name: name}
}

// Query executes a SQL query on the datastore.
func (p *Primitive) Query(sql string, args ...any) ([]map[string]any, error) {
	body := map[string]any{"sql": sql}
	if len(args) > 0 {
		body["args"] = args
	}
	if p.name != "" {
		body["named"] = p.name
	}

	var result struct {
		Data []map[string]any `json:"data"`
	}
	err := p.client.doJSON("POST", fmt.Sprintf("/ctx/%s/query", p.typ), body, &result)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// Get retrieves a value by key from cache/kvstore.
func (p *Primitive) Get(key string) (any, error) {
	body := map[string]any{"key": key}
	if p.name != "" {
		body["named"] = p.name
	}

	var result struct {
		Data any `json:"data"`
	}
	err := p.client.doJSON("POST", fmt.Sprintf("/ctx/%s/get", p.typ), body, &result)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// Set stores a value by key with an optional TTL.
func (p *Primitive) Set(key string, value any, ttlSeconds int) error {
	body := map[string]any{"key": key, "value": value}
	if ttlSeconds > 0 {
		body["ttl_seconds"] = ttlSeconds
	}
	if p.name != "" {
		body["named"] = p.name
	}
	return p.client.doJSON("POST", fmt.Sprintf("/ctx/%s/set", p.typ), body, nil)
}

// Delete removes a key from cache/kvstore.
func (p *Primitive) Delete(key string) error {
	body := map[string]any{"key": key}
	if p.name != "" {
		body["named"] = p.name
	}
	return p.client.doJSON("POST", fmt.Sprintf("/ctx/%s/delete", p.typ), body, nil)
}

// Acquire acquires a distributed lock.
func (p *Primitive) Acquire(key string, ttlSeconds int) (bool, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 30
	}
	body := map[string]any{"key": key, "ttl_seconds": ttlSeconds}
	if p.name != "" {
		body["named"] = p.name
	}

	var result struct {
		OK *bool `json:"ok"`
	}
	err := p.client.doJSON("POST", fmt.Sprintf("/ctx/%s/acquire", p.typ), body, &result)
	if err != nil {
		return false, err
	}
	return result.OK != nil && *result.OK, nil
}

// Release releases a distributed lock.
func (p *Primitive) Release(key string) error {
	body := map[string]any{"key": key}
	if p.name != "" {
		body["named"] = p.name
	}
	return p.client.doJSON("POST", fmt.Sprintf("/ctx/%s/release", p.typ), body, nil)
}

// ─── Entity Primitive ───

// EntityPrimitive provides entity CRUD operations proxied to the sidecar.
// Workspace isolation is enforced by the sidecar — no tenantId parameters.
type EntityPrimitive struct {
	client *Client
	name   string // module/entity, empty for runtime resolution
}

// Entity returns a handle for entity CRUD operations.
func (c *Client) Entity() *EntityPrimitive {
	return &EntityPrimitive{client: c}
}

// Named binds this entity handle to a specific module/entity.
func (e *EntityPrimitive) Named(ref string) *EntityPrimitive {
	return &EntityPrimitive{client: e.client, name: ref}
}

// Update atomically merges fields into an entity record.
// Uses jsonb_merge / json_patch — single SQL statement, no race condition.
func (e *EntityPrimitive) Update(id string, fields map[string]any) error {
	body := map[string]any{"key": id, "fields": fields}
	if e.name != "" {
		body["named"] = e.name
	}
	return e.client.doJSON("POST", "/ctx/entity/update", body, nil)
}

// Increment atomically increments a numeric field on an entity record.
func (e *EntityPrimitive) Increment(id, field string, amount float64) error {
	body := map[string]any{"key": id, "field": field, "amount": amount}
	if e.name != "" {
		body["named"] = e.name
	}
	return e.client.doJSON("POST", "/ctx/entity/increment", body, nil)
}

// Decrement atomically decrements a numeric field on an entity record.
// Includes a guard against negative values. Returns the new field value.
func (e *EntityPrimitive) Decrement(id, field string, amount float64) (float64, error) {
	body := map[string]any{"key": id, "field": field, "amount": amount}
	if e.name != "" {
		body["named"] = e.name
	}

	var result struct {
		Data float64 `json:"data"`
	}
	err := e.client.doJSON("POST", "/ctx/entity/decrement", body, &result)
	if err != nil {
		return 0, err
	}
	return result.Data, nil
}
