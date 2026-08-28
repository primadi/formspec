// Package consult — `formspec consult` client (todo 10.2, docs/ai/02).
//
// REPL client that runs the tool-use loop against an LLM provider and
// executes tools through the local MCP server (`formspec mcp-serve`, stdio).
// The MCP boundary is kept deliberately: the client takes the same path as
// external MCP clients, so the server-side validation gate and vendors/
// guard apply identically (docs/ai/01 §4, 03 §2).
package consult

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPClient wraps one session against a spawned `formspec mcp-serve`.
type MCPClient struct {
	session *mcp.ClientSession
	cmd     *exec.Cmd
}

// StartMCPServer spawns `formspec mcp-serve` as a child process and connects.
// exe is the formspec binary; args are forwarded (e.g. --spec, --schema).
func StartMCPServer(ctx context.Context, exe string, args []string) (*MCPClient, error) {
	cmd := exec.Command(exe, append([]string{"mcp-serve"}, args...)...)
	// The server logs to stderr; route it to our stderr for visibility.
	cmd.Stderr = nil

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "formspec-consult",
		Version: "0.1.0",
	}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect mcp-serve: %w", err)
	}
	return &MCPClient{session: session, cmd: cmd}, nil
}

// Close terminates the server session and child process.
func (c *MCPClient) Close() error {
	if c.session != nil {
		c.session.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// MCPTool is one tool exposed by the server.
type MCPTool struct {
	Name        string
	Description string
	// Schema is the JSON Schema of the tool input (raw JSON).
	Schema []byte
}

// ListTools returns the server's tool catalog.
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPTool, error) {
	res, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}
	var out []MCPTool
	for _, t := range res.Tools {
		tool := MCPTool{Name: t.Name, Description: t.Description}
		if t.InputSchema != nil {
			tool.Schema, err = marshalJSONSchema(t.InputSchema)
			if err != nil {
				return nil, err
			}
		}
		out = append(out, tool)
	}
	return out, nil
}

// CallTool executes one tool call and returns the text result.
// Tool-level errors (IsError) are returned as errors — the loop surfaces
// them to the model as tool_result content instead of crashing.
func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	res, err := c.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", err
	}
	var text string
	for _, content := range res.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if res.IsError {
		return text, fmt.Errorf("tool %s failed: %s", name, text)
	}
	return text, nil
}
