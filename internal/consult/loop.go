package consult

import (
	"context"
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/consult/llm"
)

// LoopConfig configures the tool-use loop.
type LoopConfig struct {
	// MaxSteps bounds one user turn's tool-call cycles (safety guard).
	MaxSteps int
	// OnAssistant is called with each assistant message as it arrives
	// (REPL rendering + transcript). May be nil.
	OnAssistant func(msg llm.Message)
	// OnToolCall is called after each tool execution (name, args, result,
	// err). May be nil.
	OnToolCall func(name string, args map[string]any, result string, err error)
}

// Loop runs the tool-use cycle (docs/ai/01 §3): send history → model returns
// tool calls → execute via MCP → append tool_result → repeat until the model
// answers with plain content or MaxSteps is exhausted.
type Loop struct {
	Provider llm.Provider
	MCP      *MCPClient
	Tools    []llm.ToolDefinition // resolved from MCP at session start
	Cfg      LoopConfig

	// executor overrides tool execution (test seam). When nil, tools run
	// through MCP.CallTool.
	executor func(ctx context.Context, name string, args map[string]any) (string, error)
}

// executeTool runs one tool call via the executor seam or the MCP client.
func (l *Loop) executeTool(ctx context.Context, name string, args map[string]any) (string, error) {
	if l.executor != nil {
		return l.executor(ctx, name, args)
	}
	if l.MCP == nil {
		return "", fmt.Errorf("no MCP server connected")
	}
	return l.MCP.CallTool(ctx, name, args)
}

// Run performs one full turn: it keeps calling the provider until the model
// produces a final assistant message without tool calls. The returned
// message is the final answer; intermediate messages are appended to history.
func (l *Loop) Run(ctx context.Context, history *[]llm.Message, userMsg string) (llm.Message, error) {
	maxSteps := l.Cfg.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 15
	}

	*history = append(*history, llm.Message{Role: llm.RoleUser, Content: userMsg})

	for step := 0; step < maxSteps; step++ {
		resp, err := l.Provider.Generate(ctx, llm.GenerateRequest{
			Messages: *history,
			Tools:    l.Tools,
		})
		if err != nil {
			return llm.Message{}, err
		}

		if l.Cfg.OnAssistant != nil {
			l.Cfg.OnAssistant(resp.Message)
		}
		*history = append(*history, resp.Message)

		if len(resp.Message.ToolCalls) == 0 {
			return resp.Message, nil // final answer
		}

		// Execute each tool call and append the paired tool_result — the
		// pair must stay adjacent for history compression (01 §6).
		for _, tc := range resp.Message.ToolCalls {
			args, argErr := tc.ArgumentsMap()
			var result string
			var execErr error
			if argErr != nil {
				execErr = argErr
			} else {
				result, execErr = l.executeTool(ctx, tc.Name, args)
			}

			if l.Cfg.OnToolCall != nil {
				l.Cfg.OnToolCall(tc.Name, args, result, execErr)
			}

			content := result
			if execErr != nil {
				// Surface the failure to the model as tool_result content —
				// it can correct course (e.g. fix a rejected draft).
				content = "ERROR: " + execErr.Error()
			}
			*history = append(*history, llm.Message{
				Role:       llm.RoleTool,
				Content:    truncate(content, 8000),
				ToolCallID: tc.ID,
			})
		}
	}
	return llm.Message{}, fmt.Errorf("tool-use loop exceeded %d steps — aborting turn", maxSteps)
}

// truncate shortens s for the model context (full content stays in the
// transcript via OnToolCall).
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n… (truncated — full content in transcript.md)"
}

// SystemPrompt builds the consultant system prompt (docs/ai/02 §1):
// Discovery → Proposal → Draft, plus the option-format convention the REPL
// picker relies on.
func SystemPrompt(workspaceContext string) string {
	var b strings.Builder
	b.WriteString(`You are a FormSpec business consultant. You help the user discover their
business needs and author valid FormSpec YAML specs.

Follow three phases strictly — never jump to YAML before the business needs
are clear:
1. DISCOVERY — ask active questions about goals, workflows, and business
   rules. Summarize your understanding in plain language and confirm.
2. PROPOSAL — propose the system flow and module composition: which entities,
   which lifecycles, which existing modules to reuse.
3. DRAFT — write spec YAML ONLY via the propose_spec_file tool (never output
   full YAML files in chat). Every draft is validated automatically; fix any
   reported problems before moving on.

When presenting choices, format them one per line as:
A) first option
B) second option
The user will answer with a letter or free text.

Use the available tools to ground yourself in the workspace before proposing
anything. Read skills with read_skill when the topic matches their description.
`)
	if workspaceContext != "" {
		b.WriteString("\n--- Workspace context (auto-collected at session start) ---\n")
		b.WriteString(workspaceContext)
		b.WriteString("\n--- End workspace context ---\n")
	}
	return b.String()
}
