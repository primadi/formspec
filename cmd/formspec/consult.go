// Command `formspec consult` — AI business consultant client (todo 10.2).
//
//	formspec consult [--session <id>] [--resume <id>] [--provider <name>]
//	                 [--model <id>] [--base-url <url>] [--api-key-env <var>]
//	                 [--spec <path>] [--schema <dir>]
//	formspec consult diff [--session <id>]
//
// REPL client that spawns `formspec mcp-serve` (stdio) and runs the
// tool-use loop against an OpenAI-compatible LLM (BYOK — keyring or env).
// Every turn is appended to `.formspec/consult/{session}/transcript.md`
// for review (keputusan desain 2026-08-27).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/consult"
	"github.com/primadi/formspec/internal/consult/llm"
)

// validatedProviders is the capability-bar list (docs/ai/05 §2): providers
// whose tool-calling behavior is known to work with the consult flow.
var validatedProviders = map[string]llm.OpenAIConfig{
	"deepseek": {
		BaseURL: "https://api.deepseek.com/v1",
		Model:   "deepseek-chat",
	},
	"glm": {
		// OpenCode gateway — OpenAI-compatible endpoint for GLM models.
		BaseURL: "https://api.opencode.uz/v1",
		Model:   "glm-5.3-flash",
	},
	"openai": {
		Model: "gpt-4o",
	},
}

func runConsult(args []string) {
	if len(args) > 0 && args[0] == "diff" {
		runConsultDiff(args[1:])
		return
	}

	fs := flag.NewFlagSet("consult", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	sessionID := fs.String("session", "", "session id (default: consult-<timestamp>)")
	resumeID := fs.String("resume", "", "resume an existing session (rebuilds history from transcript.md)")
	providerName := fs.String("provider", "deepseek", "provider: deepseek|glm|openai (validated list) or 'custom' with --base-url")
	model := fs.String("model", "", "model id override")
	baseURL := fs.String("base-url", "", "OpenAI-compatible base URL override (BYOK)")
	apiKeyEnv := fs.String("api-key-env", "FORMSPEC_LLM_API_KEY", "env var holding the API key")
	specPath := fs.String("spec", "spec", "spec directory (forwarded to mcp-serve)")
	schemaDir := fs.String("schema", "", "local schemas/ dir (forwarded to mcp-serve)")
	allowUnvalidated := fs.Bool("allow-unvalidated", false, "allow providers outside the validated list (warning only)")
	fs.Parse(args)

	// ── Provider setup (todo 10.2.3/10.2.4) ──
	cfg, ok := validatedProviders[*providerName]
	if !ok && !*allowUnvalidated {
		fmt.Fprintf(os.Stderr, "formspec consult: provider %q is not in the validated list (%s) — pass --allow-unvalidated to proceed\n",
			*providerName, strings.Join(validatedProviderNames(), ", "))
		os.Exit(1)
	}
	if *baseURL != "" {
		cfg.BaseURL = *baseURL
	}
	if *model != "" {
		cfg.Model = *model
	}
	if cfg.Model == "" {
		fmt.Fprintln(os.Stderr, "formspec consult: --model is required for custom providers")
		os.Exit(1)
	}

	creds := llm.CredentialStore{
		KeyringService:  "formspec-consult",
		KeyringUser:     *providerName,
		EnvVar:          *apiKeyEnv,
		FallbackEnvVars: []string{"OPENAI_API_KEY"},
	}
	apiKey, err := creds.GetAPIKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec consult: %v\n", err)
		os.Exit(1)
	}
	cfg.APIKey = apiKey
	provider := llm.NewOpenAI(cfg, *providerName)

	// ── MCP server (boundary identik dengan client eksternal) ──
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec consult: %v\n", err)
		os.Exit(1)
	}
	mcpArgs := []string{"--spec", *specPath}
	if *schemaDir != "" {
		mcpArgs = append(mcpArgs, "--schema", *schemaDir)
	}
	ctx := context.Background()
	mcpClient, err := consult.StartMCPServer(ctx, exe, mcpArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec consult: %v\n", err)
		os.Exit(1)
	}
	defer mcpClient.Close()

	// ── Session (todo 10.2.8 — transcript ke file untuk review) ──
	consultDir := filepath.Join(".formspec", "consult")
	var session *consult.Session
	if *resumeID != "" {
		session, err = consult.Resume(consultDir, *resumeID)
	} else {
		id := *sessionID
		if id == "" {
			id = "consult-" + time.Now().Format("20060102-150405")
		}
		session, err = consult.NewSession(consultDir, id, provider.Name(), cfg.Model)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec consult: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	// ── Tools dari MCP → LLM tool definitions ──
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec consult: %v\n", err)
		os.Exit(1)
	}
	var defs []llm.ToolDefinition
	for _, t := range tools {
		defs = append(defs, llm.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Schema,
		})
	}

	// ── Auto-invoke deterministik (todo 10.2.6, docs/ai/01 §5) ──
	workspaceCtx := autoInvokeContext(ctx, mcpClient)

	loop := &consult.Loop{
		Provider: provider,
		MCP:      mcpClient,
		Tools:    defs,
		Cfg: consult.LoopConfig{
			OnAssistant: func(msg llm.Message) {
				fmt.Fprintf(os.Stdout, "\nconsultant>\n%s\n", msg.Content)
			},
			OnToolCall: func(name string, args map[string]any, result string, err error) {
				status := "ok"
				if err != nil {
					status = "ERROR: " + err.Error()
				}
				fmt.Fprintf(os.Stdout, "  [tool] %s (%s)\n", name, status)
				session.RecordTool(name, args, result, err)
			},
		},
	}

	repl := &consult.REPL{
		Session:          session,
		Loop:             loop,
		WorkspaceContext: workspaceCtx,
	}
	if err := repl.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "formspec consult: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "\ntranscript: %s\n", session.TranscriptPath())
}

// autoInvokeContext calls the grounding tools deterministically at session
// start (docs/ai/01 §5) and renders the results into the system prompt
// context. Failures are noted, never fatal (a project may have no App yet).
func autoInvokeContext(ctx context.Context, c *consult.MCPClient) string {
	var b strings.Builder
	for _, name := range []string{"read_workspace_manifest", "list_installed_modules", "list_skills"} {
		result, err := c.CallTool(ctx, name, map[string]any{})
		if err != nil {
			fmt.Fprintf(&b, "### %s\n(unavailable: %v)\n\n", name, err)
			continue
		}
		fmt.Fprintf(&b, "### %s\n%s\n\n", name, truncateStr(result, 3000))
	}
	return b.String()
}

// runConsultDiff implements `formspec consult diff` (todo 10.4.2).
func runConsultDiff(args []string) {
	fs := flag.NewFlagSet("consult diff", flag.ExitOnError)
	sessionID := fs.String("session", "", "session id (required)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *sessionID == "" {
		fmt.Fprintln(os.Stderr, "formspec consult diff: --session is required")
		os.Exit(2)
	}
	diffs, err := consult.DiffDrafts(filepath.Join(".formspec", "consult", *sessionID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec consult diff: %v\n", err)
		os.Exit(1)
	}
	if len(diffs) == 0 {
		fmt.Println("tidak ada draft.")
		return
	}
	for _, d := range diffs {
		marker := "modified"
		if d.IsNew {
			marker = "new file"
		}
		fmt.Printf("--- %s (%s)\n%s\n\n", d.Path, marker, d.Unified)
	}
}

func validatedProviderNames() []string {
	names := make([]string, 0, len(validatedProviders))
	for k := range validatedProviders {
		names = append(names, k)
	}
	return names
}

// truncateStr shortens tool results injected into the system prompt context.
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n… (truncated)"
}
