package consult

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/consult/llm"
)

// Session is one consult session (todo 10.2.8, 10.4.1): in-memory history
// for the model + an incremental transcript.md on disk for human review.
//
// The transcript is appended per turn — never buffered in memory — so a
// crash never loses the conversation record.
type Session struct {
	ID        string
	Dir       string // .formspec/consult/{id}
	History   []llm.Message
	Provider  string // provider label for the header
	Model     string
	StartedAt time.Time

	transcript *os.File
	turn       int
}

// NewSession creates the session directory and the transcript header.
func NewSession(baseDir, id, provider, model string) (*Session, error) {
	s := &Session{
		ID:        id,
		Dir:       filepath.Join(baseDir, id),
		Provider:  provider,
		Model:     model,
		StartedAt: time.Now(),
	}
	for _, sub := range []string{"", "draft", "undo"} {
		if err := os.MkdirAll(filepath.Join(s.Dir, sub), 0755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(s.TranscriptPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	s.transcript = f
	fmt.Fprintf(f, "# Consult Session %s\n\n- **Started**: %s\n- **Provider**: %s / %s\n\n---\n\n",
		s.ID, s.StartedAt.Format(time.RFC3339), s.Provider, s.Model)
	return s, nil
}

// TranscriptPath is the on-disk conversation record.
func (s *Session) TranscriptPath() string {
	return filepath.Join(s.Dir, "transcript.md")
}

// WriteDiscoverySummary persists the plain-language discovery summary for
// business-owner confirmation (todo 10.4.1, docs/ai/02 §1).
func (s *Session) WriteDiscoverySummary(content string) (string, error) {
	path := filepath.Join(s.Dir, "discovery-summary.md")
	body := fmt.Sprintf("# Discovery Summary — %s\n\n> Dibuat oleh formspec consult pada %s.\n> Konfirmasi isi ini dengan business owner sebelum lanjut ke Draft.\n\n%s\n",
		s.ID, time.Now().Format(time.RFC3339), strings.TrimRight(content, "\n"))
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// Close flushes and closes the transcript.
func (s *Session) Close() error {
	if s.transcript != nil {
		return s.transcript.Close()
	}
	return nil
}

// RecordUser appends the user turn to history and transcript.
func (s *Session) RecordUser(content string) {
	s.History = append(s.History, llm.Message{Role: llm.RoleUser, Content: content})
	s.writeTurn("user", content)
}

// RecordAssistant appends the assistant turn (final answer) to history and
// transcript.
func (s *Session) RecordAssistant(msg llm.Message) {
	s.History = append(s.History, msg)
	s.writeTurn("assistant", msg.Content)
}

// RecordTool writes a tool call summary to the transcript (tool traffic is
// summarized, not dumped — the transcript is for human review).
func (s *Session) RecordTool(name string, args map[string]any, result string, err error) {
	status := "ok"
	if err != nil {
		status = "ERROR: " + err.Error()
	}
	argsJSON, _ := jsonMarshal(args)
	s.writeTurn("tool", fmt.Sprintf("**%s** — %s\n\n```json\n%s\n```\n\nResult (truncated):\n\n```\n%s\n```",
		name, status, truncateStr(argsJSON, 500), truncateStr(result, 1000)))
}

// writeTurn appends one turn block to transcript.md.
func (s *Session) writeTurn(role, content string) {
	if s.transcript == nil {
		return
	}
	s.turn++
	fmt.Fprintf(s.transcript, "## Turn %d — %s\n\n%s\n\n", s.turn, role, strings.TrimRight(content, "\n"))
}

// Resume rebuilds a session from its transcript (best-effort: the in-memory
// history is reconstructed from the recorded turns so the model keeps
// context; the transcript file itself is appended to, not rewritten).
func Resume(baseDir, id string) (*Session, error) {
	dir := filepath.Join(baseDir, id)
	data, err := os.ReadFile(filepath.Join(dir, "transcript.md"))
	if err != nil {
		return nil, fmt.Errorf("read transcript: %w", err)
	}
	s := &Session{ID: id, Dir: dir, StartedAt: time.Now()}
	f, err := os.OpenFile(s.TranscriptPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	s.transcript = f
	fmt.Fprintf(f, "\n---\n\n# Resumed %s\n\n", s.StartedAt.Format(time.RFC3339))

	// Rebuild history from recorded turns (assistant/user only — tool
	// summaries are context for the human, not replayed into the model).
	for _, block := range strings.Split(string(data), "\n## Turn ") {
		if !strings.Contains(block, " — user\n") && !strings.Contains(block, " — assistant\n") {
			continue
		}
		role := llm.RoleUser
		if strings.Contains(block, " — assistant\n") {
			role = llm.RoleAssistant
		}
		body := block
		if idx := strings.Index(block, "\n"); idx >= 0 {
			body = block[idx+1:]
		}
		body = strings.TrimSpace(strings.TrimPrefix(body, "\n"))
		if body == "" {
			continue
		}
		s.History = append(s.History, llm.Message{Role: role, Content: body})
	}
	return s, nil
}

// jsonMarshal marshals v or returns "{}" on failure (never panics).
func jsonMarshal(v any) (string, error) {
	b, err := jsonMarshalRaw(v)
	return string(b), err
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n… (truncated)"
}
