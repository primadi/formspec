package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// OpenAIConfig configures the OpenAI-compatible adapter.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string // empty = api.openai.com; override for DeepSeek/GLM/gateways
	Model   string // model id, e.g. "glm-5.3-flash", "deepseek-chat"
}

// OpenAIProvider implements Provider over the OpenAI-compatible chat
// completions wire format (openai-go SDK).
type OpenAIProvider struct {
	client     openai.Client
	model      string
	providerNm string // human label for logs/transcript
}

// NewOpenAI creates a provider. providerName labels the backend in the
// transcript (e.g. "deepseek", "glm-opencode", "openai").
func NewOpenAI(cfg OpenAIConfig, providerName string) *OpenAIProvider {
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	if providerName == "" {
		providerName = "openai"
	}
	return &OpenAIProvider{
		client:     openai.NewClient(opts...),
		model:      cfg.Model,
		providerNm: providerName,
	}
}

// Name identifies the provider in logs and the transcript header.
func (p *OpenAIProvider) Name() string { return p.providerNm }

// Generate performs one chat completion (todo 10.2.3).
func (p *OpenAIProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(p.model),
		Messages: []openai.ChatCompletionMessageParamUnion{},
	}
	if req.System != "" {
		params.Messages = append(params.Messages, openai.SystemMessage(req.System))
	}
	for _, m := range req.Messages {
		u, err := convertMessage(m)
		if err != nil {
			return nil, err
		}
		params.Messages = append(params.Messages, u)
	}
	for _, t := range req.Tools {
		var schema shared.FunctionParameters
		if len(t.Parameters) > 0 {
			if err := json.Unmarshal(t.Parameters, &schema); err != nil {
				return nil, fmt.Errorf("tool %s parameters: %w", t.Name, err)
			}
		}
		params.Tools = append(params.Tools, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  schema,
			},
		})
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("llm generate: empty choices")
	}
	choice := resp.Choices[0]

	out := &GenerateResponse{
		Message: Message{Role: RoleAssistant, Content: choice.Message.Content},
	}
	if resp.Usage.TotalTokens > 0 {
		out.Usage = Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}
	for _, tc := range choice.Message.ToolCalls {
		out.Message.ToolCalls = append(out.Message.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return out, nil
}

// convertMessage maps our neutral Message to the SDK union type.
func convertMessage(m Message) (openai.ChatCompletionMessageParamUnion, error) {
	switch m.Role {
	case RoleUser:
		return openai.UserMessage(m.Content), nil
	case RoleSystem:
		return openai.SystemMessage(m.Content), nil
	case RoleTool:
		return openai.ToolMessage(m.Content, m.ToolCallID), nil
	case RoleAssistant:
		ap := openai.ChatCompletionAssistantMessageParam{
			Content: openai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: openai.String(m.Content),
			},
		}
		for _, tc := range m.ToolCalls {
			ap.ToolCalls = append(ap.ToolCalls, openai.ChatCompletionMessageToolCallParam{
				ID: tc.ID,
				Function: openai.ChatCompletionMessageToolCallFunctionParam{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		return openai.ChatCompletionMessageParamUnion{OfAssistant: &ap}, nil
	}
	return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("unknown message role %q", m.Role)
}
