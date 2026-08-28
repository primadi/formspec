// Package llm — LLM Provider Layer for `formspec consult` (todo 10.2.3).
//
// Thin internal interface over provider SDKs (docs/ai/05-llm-provider-layer.md).
// The SDK types never leak outside this package: consult logic depends only
// on the Provider interface, so swapping the underlying SDK is a local change.
//
// Wire format: OpenAI-compatible chat completions (openai-go SDK with base
// URL override) — covers OpenAI, DeepSeek, GLM/Zhipu, and any
// OpenAI-compatible gateway in one adapter.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// Role of a conversation message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is one function call requested by the model.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // raw JSON string (OpenAI wire format)
}

// ArgumentsMap parses the raw JSON arguments into a map.
func (tc ToolCall) ArgumentsMap() (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(tc.Arguments), &m); err != nil {
		return nil, fmt.Errorf("tool %s arguments: %w", tc.Name, err)
	}
	return m, nil
}

// ToolDefinition describes a tool the model may call (JSON Schema parameters).
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema object
}

// Message is one conversation turn.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant only
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool role only
}

// Usage is token accounting for one generation.
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// GenerateRequest is one completion request.
type GenerateRequest struct {
	System   string           // system prompt (prepended as its own message)
	Messages []Message        // conversation history (without the system prompt)
	Tools    []ToolDefinition // available tools; empty = plain completion
}

// GenerateResponse is one completion result.
type GenerateResponse struct {
	Message Message // assistant message (content and/or tool calls)
	Usage   Usage
}

// Provider is the LLM backend behind `formspec consult`.
type Provider interface {
	// Generate performs one chat completion. Tool execution and the
	// tool-use loop live in internal/consult — the provider only talks
	// to the API.
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)

	// Name identifies the provider in logs and the transcript header.
	Name() string
}
