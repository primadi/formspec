package consult

import (
	"context"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/consult/llm"
)

// mockProvider replays scripted responses — proves the tool-use loop pairs
// tool_calls with tool_results and stops on the final content message.
type mockProvider struct {
	responses []llm.GenerateResponse
	calls     int
}

func (m *mockProvider) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	if m.calls >= len(m.responses) {
		return nil, context.DeadlineExceeded
	}
	resp := m.responses[m.calls]
	m.calls++
	return &resp, nil
}

func (m *mockProvider) Name() string { return "mock" }

// mockMCP records tool calls and returns scripted results.
type mockMCP struct {
	calls []struct {
		name string
		args map[string]any
	}
	results map[string]string
}

func (m *mockMCP) CallTool(_ context.Context, name string, args map[string]any) (string, error) {
	m.calls = append(m.calls, struct {
		name string
		args map[string]any
	}{name, args})
	if r, ok := m.results[name]; ok {
		return r, nil
	}
	return "ok", nil
}

func TestLoop_ToolCallCycle(t *testing.T) {
	provider := &mockProvider{responses: []llm.GenerateResponse{
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
			{ID: "tc-1", Name: "read_workspace_manifest", Arguments: `{}`},
		}}},
		{Message: llm.Message{Role: llm.RoleAssistant, Content: "final answer"}},
	}}
	mcp := &mockMCP{results: map[string]string{
		"read_workspace_manifest": `{"apps": []}`,
	}}
	var toolResults []string
	loop := &Loop{
		Provider: provider,
		MCP:      nil, // replaced below via interface shim
		Tools:    []llm.ToolDefinition{{Name: "read_workspace_manifest"}},
		Cfg: LoopConfig{
			OnToolCall: func(name string, args map[string]any, result string, err error) {
				toolResults = append(toolResults, name)
			},
		},
	}
	// The Loop expects *MCPClient; for the unit test we inject the mock via
	// the toolExecutor seam.
	loop.executor = func(ctx context.Context, name string, args map[string]any) (string, error) {
		return mcp.CallTool(ctx, name, args)
	}

	history := []llm.Message{}
	final, err := loop.Run(context.Background(), &history, "halo")
	if err != nil {
		t.Fatal(err)
	}
	if final.Content != "final answer" {
		t.Errorf("final = %q", final.Content)
	}
	if len(toolResults) != 1 || toolResults[0] != "read_workspace_manifest" {
		t.Errorf("tool calls = %v", toolResults)
	}
	// History: user, assistant(tool_call), tool(result), assistant(final).
	if len(history) != 4 {
		t.Fatalf("history len = %d, want 4", len(history))
	}
	if history[2].Role != llm.RoleTool || history[2].ToolCallID != "tc-1" {
		t.Errorf("tool result message wrong: %+v", history[2])
	}
	if !strings.Contains(history[2].Content, "apps") {
		t.Errorf("tool result content = %q", history[2].Content)
	}
}

func TestLoop_MaxStepsGuard(t *testing.T) {
	// Provider always returns tool calls — the loop must abort, not spin.
	resp := llm.GenerateResponse{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
		{ID: "tc", Name: "ping", Arguments: `{}`},
	}}}
	provider := &mockProvider{responses: []llm.GenerateResponse{resp, resp, resp, resp, resp}}
	loop := &Loop{Provider: provider, Tools: nil, Cfg: LoopConfig{MaxSteps: 3}}
	loop.executor = func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return "ok", nil
	}
	history := []llm.Message{}
	if _, err := loop.Run(context.Background(), &history, "go"); err == nil {
		t.Fatal("expected max-steps error")
	}
}

func TestLoop_ToolErrorSurfacedToModel(t *testing.T) {
	provider := &mockProvider{responses: []llm.GenerateResponse{
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
			{ID: "tc-1", Name: "propose_spec_file", Arguments: `{"path": "vendors/x.yaml"}`},
		}}},
		{Message: llm.Message{Role: llm.RoleAssistant, Content: "understood, using extension instead"}},
	}}
	loop := &Loop{Provider: provider, Cfg: LoopConfig{}}
	loop.executor = func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return "", context.Canceled // any error
	}
	history := []llm.Message{}
	_, err := loop.Run(context.Background(), &history, "write to vendors")
	if err != nil {
		t.Fatal(err)
	}
	// The tool failure must reach the model as tool_result content, not
	// crash the turn.
	toolMsg := history[2]
	if toolMsg.Role != llm.RoleTool || !strings.HasPrefix(toolMsg.Content, "ERROR:") {
		t.Errorf("tool error not surfaced: %+v", toolMsg)
	}
}
