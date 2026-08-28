package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOpenAIProvider_Generate proves the adapter speaks the OpenAI-compatible
// wire format: request carries model/messages/tools, response tool_calls are
// parsed into neutral ToolCall values.
func TestOpenAIProvider_Generate(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" && r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing bearer token")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "cmpl-1", "object": "chat.completion", "model": "glm-5.3-flash",
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]any{{
						"id":   "call-1",
						"type": "function",
						"function": map[string]any{
							"name":      "propose_spec_file",
							"arguments": `{"path":"modules/shop/entity/customer.yaml"}`,
						},
					}},
				},
				"finish_reason": "tool_calls",
			}},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	}))
	defer srv.Close()

	p := NewOpenAI(OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL + "/v1",
		Model:   "glm-5.3-flash",
	}, "glm")

	resp, err := p.Generate(context.Background(), GenerateRequest{
		System: "you are a consultant",
		Messages: []Message{
			{Role: RoleUser, Content: "buat entity customer"},
			{Role: RoleAssistant, Content: "ok"},
			{Role: RoleTool, Content: "written", ToolCallID: "call-0"},
		},
		Tools: []ToolDefinition{{
			Name:        "propose_spec_file",
			Description: "write a draft",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Request shape.
	if gotBody["model"] != "glm-5.3-flash" {
		t.Errorf("model = %v", gotBody["model"])
	}
	msgs := gotBody["messages"].([]any)
	if len(msgs) != 4 { // system + 3
		t.Fatalf("messages = %d, want 4", len(msgs))
	}
	if msgs[0].(map[string]any)["role"] != "system" {
		t.Errorf("first message not system: %v", msgs[0])
	}
	if msgs[3].(map[string]any)["tool_call_id"] != "call-0" {
		t.Errorf("tool message missing tool_call_id: %v", msgs[3])
	}
	tools := gotBody["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["type"] != "function" {
		t.Errorf("tools = %v", gotBody["tools"])
	}

	// Response shape.
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(resp.Message.ToolCalls))
	}
	tc := resp.Message.ToolCalls[0]
	if tc.ID != "call-1" || tc.Name != "propose_spec_file" {
		t.Errorf("tool call = %+v", tc)
	}
	args, err := tc.ArgumentsMap()
	if err != nil {
		t.Fatal(err)
	}
	if args["path"] != "modules/shop/entity/customer.yaml" {
		t.Errorf("args = %v", args)
	}
	if resp.Usage.OutputTokens != 5 {
		t.Errorf("usage = %+v", resp.Usage)
	}
}

// TestCredentialStore proves the tiered resolution: env var → fallback →
// clear error (keyring is skipped in tests — no secret service in CI).
func TestCredentialStore(t *testing.T) {
	t.Setenv("FORMSPEC_TEST_KEY", "abc")
	c := CredentialStore{EnvVar: "FORMSPEC_TEST_KEY"}
	key, err := c.GetAPIKey()
	if err != nil || key != "abc" {
		t.Fatalf("env var tier failed: %q %v", key, err)
	}

	c2 := CredentialStore{EnvVar: "FORMSPEC_TEST_MISSING", FallbackEnvVars: []string{"FORMSPEC_TEST_KEY"}}
	if key, err := c2.GetAPIKey(); err != nil || key != "abc" {
		t.Fatalf("fallback tier failed: %q %v", key, err)
	}

	c3 := CredentialStore{EnvVar: "FORMSPEC_TEST_MISSING"}
	if _, err := c3.GetAPIKey(); err == nil || !contains(err.Error(), "FORMSPEC_TEST_MISSING") {
		t.Fatalf("expected guiding error, got %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
