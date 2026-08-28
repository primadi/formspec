package consult

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// REPLConfig configures the interactive loop.
type REPLConfig struct {
	In  io.Reader // defaults to stdin
	Out io.Writer // defaults to stdout
}

// REPL is the interactive consult client (todo 10.2.5).
type REPL struct {
	Session *Session
	Loop    *Loop
	Cfg     REPLConfig

	// WorkspaceContext is injected into the system prompt (auto-invoke
	// results, todo 10.2.6).
	WorkspaceContext string
}

// Run drives the read-eval loop until EOF, "/quit", or context cancellation.
func (r *REPL) Run(ctx context.Context) error {
	in := r.Cfg.In
	if in == nil {
		in = bufio.NewReader(osStdin())
	}
	out := r.Cfg.Out
	if out == nil {
		out = osStdout()
	}

	fmt.Fprintln(out, "FormSpec Consult — Discovery → Proposal → Draft")
	fmt.Fprintln(out, "Perintah: /diff /apply [path] /reject <path> /summary /status /quit")
	fmt.Fprintln(out)

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for {
		fmt.Fprint(out, "\nyou> ")
		if !scanner.Scan() {
			return nil // EOF
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		switch {
		case input == "/quit" || input == "/exit":
			return nil
		case input == "/status":
			fmt.Fprintf(out, "session: %s\ntranscript: %s\nturns: %d\n",
				r.Session.ID, r.Session.TranscriptPath(), len(r.Session.History))
			continue
		case input == "/diff":
			r.showDiff(out)
			continue
		case input == "/apply" || strings.HasPrefix(input, "/apply "):
			// 10.4.3: accept — tanpa argumen = semua draft; dengan path =
			// satu file (accept/reject per file, docs/ai/02 §4).
			r.applyDrafts(ctx, out, strings.TrimSpace(strings.TrimPrefix(input, "/apply")))
			continue
		case strings.HasPrefix(input, "/reject "):
			r.rejectDraft(out, strings.TrimSpace(strings.TrimPrefix(input, "/reject ")))
			continue
		case input == "/summary":
			// 10.4.1: discovery-summary.md — model merangkum hasil
			// discovery dalam bahasa awam untuk konfirmasi owner (02 §1).
			r.writeDiscoverySummary(ctx, out)
			continue
		}

		r.Session.RecordUser(input)
		final, err := r.Loop.Run(ctx, &r.Session.History, input)
		if err != nil {
			fmt.Fprintf(out, "[error] %v\n", err)
			continue
		}
		r.Session.RecordAssistant(final)

		// Option picker (keputusan desain): detect A/B/C blocks and offer
		// single-letter selection; free text always allowed.
		if opts := DetectOptions(final.Content); opts != nil {
			fmt.Fprintf(out, "\n[pilihan terdeteksi: %s — ketik huruf untuk memilih, atau teks bebas]\n",
				strings.Join(opts.Labels, "/"))
			fmt.Fprint(out, "pilih> ")
			if !scanner.Scan() {
				return nil
			}
			answer := strings.TrimSpace(scanner.Text())
			if answer != "" {
				label, text, picked := opts.Pick(answer)
				if picked {
					sel := SelectionMessage(label, text)
					r.Session.RecordUser(sel)
					fmt.Fprintf(out, "\n[memilih %s]\n", label)
					final, err = r.Loop.Run(ctx, &r.Session.History, sel)
					if err != nil {
						fmt.Fprintf(out, "[error] %v\n", err)
						continue
					}
					r.Session.RecordAssistant(final)
				} else {
					// Free text — treat as a normal follow-up turn.
					r.Session.RecordUser(answer)
					final, err = r.Loop.Run(ctx, &r.Session.History, answer)
					if err != nil {
						fmt.Fprintf(out, "[error] %v\n", err)
						continue
					}
					r.Session.RecordAssistant(final)
				}
			}
		}
	}
}

// showDiff prints a unified diff of draft/ vs the real spec tree (10.4.2 —
// text unified diff is enough for v1, docs/ai/02 §4).
func (r *REPL) showDiff(out io.Writer) {
	diffs, err := DiffDrafts(r.Session.Dir)
	if err != nil {
		fmt.Fprintf(out, "[error] diff: %v\n", err)
		return
	}
	if len(diffs) == 0 {
		fmt.Fprintln(out, "tidak ada draft.")
		return
	}
	for _, d := range diffs {
		fmt.Fprintf(out, "--- %s\n%s\n", d.Path, d.Unified)
	}
}

// applyDrafts applies drafts via the MCP apply_draft tool (guard + undo
// live server-side). With pathFilter empty, every draft is applied
// (accept-all); with a path, only that file (10.4.3 accept per file).
func (r *REPL) applyDrafts(ctx context.Context, out io.Writer, pathFilter string) {
	diffs, err := DiffDrafts(r.Session.Dir)
	if err != nil {
		fmt.Fprintf(out, "[error] diff: %v\n", err)
		return
	}
	applied := 0
	for _, d := range diffs {
		if pathFilter != "" && d.Path != pathFilter && d.Path != strings.TrimSuffix(pathFilter, "/") {
			continue
		}
		result, err := r.Loop.MCP.CallTool(ctx, "apply_draft", map[string]any{
			"session": r.Session.ID,
			"file":    d.Path,
		})
		if err != nil {
			fmt.Fprintf(out, "[error] apply %s: %v\n%s\n", d.Path, err, result)
			continue
		}
		fmt.Fprintf(out, "applied: %s\n", d.Path)
		applied++
	}
	if pathFilter != "" && applied == 0 {
		fmt.Fprintf(out, "draft tidak ditemukan: %s (lihat /diff)\n", pathFilter)
	}
}

// rejectDraft removes one draft file (10.4.3 reject per file) — the real
// spec tree is untouched; only the session draft is discarded.
func (r *REPL) rejectDraft(out io.Writer, path string) {
	if path == "" {
		fmt.Fprintln(out, "usage: /reject <path> (lihat /diff)")
		return
	}
	draftPath := filepath.Join(r.Session.Dir, "draft", filepath.FromSlash(path))
	if err := os.Remove(draftPath); err != nil {
		fmt.Fprintf(out, "[error] reject %s: %v\n", path, err)
		return
	}
	fmt.Fprintf(out, "rejected: %s\n", path)
}

// writeDiscoverySummary asks the model to summarize the discovery phase in
// plain language and writes the result to discovery-summary.md (10.4.1,
// docs/ai/02 §1) — untuk dikonfirmasi business owner.
func (r *REPL) writeDiscoverySummary(ctx context.Context, out io.Writer) {
	prompt := "[/summary] Rangkum hasil discovery kita sejauh ini dalam bahasa awam " +
		"(tujuan aplikasi, alur kerja utama, aturan bisnis penting, entity yang " +
		"teridentifikasi, dan pertanyaan yang belum terjawab). Tulis ringkas dan " +
		"terstruktur — output ini akan dikonfirmasi ke business owner."
	final, err := r.Loop.Run(ctx, &r.Session.History, prompt)
	if err != nil {
		fmt.Fprintf(out, "[error] summary: %v\n", err)
		return
	}
	r.Session.RecordAssistant(final)
	path, err := r.Session.WriteDiscoverySummary(final.Content)
	if err != nil {
		fmt.Fprintf(out, "[error] write summary: %v\n", err)
		return
	}
	fmt.Fprintf(out, "%s\n\ndiscovery summary: %s\n", final.Content, path)
}
