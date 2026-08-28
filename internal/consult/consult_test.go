package consult

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/consult/llm"
)

// ─── Option picker (todo 10.2.5 — keputusan desain) ───

func TestDetectOptions(t *testing.T) {
	reply := "Berikut pilihan alur bisnis:\n\nA) Arisan sederhana — satu grup, satu pemenang\nB) Arisan bertingkat — multi grup\nC) Arisan dengan iuran fleksibel\n\nMana yang cocok?"
	opts := DetectOptions(reply)
	if opts == nil {
		t.Fatal("options not detected")
	}
	if len(opts.Labels) != 3 || opts.Labels[0] != "A" || opts.Labels[2] != "C" {
		t.Errorf("labels = %v", opts.Labels)
	}
	if !strings.HasPrefix(opts.Texts[1], "Arisan bertingkat") {
		t.Errorf("text[1] = %q", opts.Texts[1])
	}
}

func TestDetectOptions_None(t *testing.T) {
	if opts := DetectOptions("Halo, apa yang ingin dibangun?"); opts != nil {
		t.Errorf("plain prose detected as options: %+v", opts)
	}
	if opts := DetectOptions("A) satu-satunya opsi"); opts != nil {
		t.Errorf("single option detected as block: %+v", opts)
	}
}

func TestOptionsPick(t *testing.T) {
	opts := &Options{
		Labels: []string{"A", "B", "C"},
		Texts:  []string{"satu", "dua", "tiga"},
	}
	// Bare letter.
	label, text, ok := opts.Pick("b")
	if !ok || label != "B" || text != "dua" {
		t.Errorf("Pick(b) = %q %q %v", label, text, ok)
	}
	// Letter with paren.
	if _, _, ok := opts.Pick("C)"); !ok {
		t.Error("Pick(C) rejected")
	}
	// Free text passes through.
	label, text, ok = opts.Pick("sebenarnya saya mau kombinasi A dan B")
	if ok || text != "sebenarnya saya mau kombinasi A dan B" {
		t.Errorf("free text mangled: %q %q %v", label, text, ok)
	}
	// Unknown letter → free text.
	if _, _, ok := opts.Pick("Z"); ok {
		t.Error("unknown label accepted")
	}
}

func TestSelectionMessage(t *testing.T) {
	msg := SelectionMessage("B", "Arisan bertingkat — multi grup")
	if msg != "[memilih: B) Arisan bertingkat — multi grup]" {
		t.Errorf("selection message = %q", msg)
	}
}

// ─── Session transcript (todo 10.2.8 — history ke file untuk review) ───

func TestSessionTranscriptAndResume(t *testing.T) {
	base := t.TempDir()
	s, err := NewSession(base, "s-test", "glm", "glm-5.3-flash")
	if err != nil {
		t.Fatal(err)
	}

	s.RecordUser("saya mau aplikasi arisan")
	s.RecordAssistant(llm.Message{Role: llm.RoleAssistant, Content: "Baik. Opsi:\n\nA) Sederhana\nB) Bertingkat"})
	s.RecordTool("list_installed_modules", map[string]any{"x": 1}, "[]", nil)

	// Transcript exists on disk with all turns.
	data, err := osReadFile(s.TranscriptPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Consult Session s-test", "## Turn 1 — user", "saya mau aplikasi arisan", "## Turn 2 — assistant", "## Turn 3 — tool", "list_installed_modules"} {
		if !strings.Contains(data, want) {
			t.Errorf("transcript missing %q", want)
		}
	}
	s.Close()

	// Resume rebuilds history from user/assistant turns (tool summaries
	// are not replayed into the model).
	r, err := Resume(base, "s-test")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if len(r.History) != 2 {
		t.Fatalf("resumed history len = %d, want 2 (user+assistant)", len(r.History))
	}
	if r.History[0].Content != "saya mau aplikasi arisan" {
		t.Errorf("resumed user turn = %q", r.History[0].Content)
	}
}

// ─── Discovery summary (todo 10.4.1) & per-file reject (todo 10.4.3) ───

func TestWriteDiscoverySummary(t *testing.T) {
	base := t.TempDir()
	s, err := NewSession(base, "s-sum", "glm", "glm-5.3-flash")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	path, err := s.WriteDiscoverySummary("Tujuan: aplikasi arisan.\nAlur: setor iuran → undian.")
	if err != nil {
		t.Fatal(err)
	}
	data, err := osReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Discovery Summary — s-sum", "Tujuan: aplikasi arisan", "business owner"} {
		if !strings.Contains(data, want) {
			t.Errorf("summary missing %q", want)
		}
	}
}

func TestRejectDraft(t *testing.T) {
	base := t.TempDir()
	s, err := NewSession(base, "s-rej", "glm", "glm-5.3-flash")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	draftPath := filepathJoin(s.Dir, "draft", "modules", "shop", "entity.yaml")
	if err := osMkdirAll(filepath.Dir(draftPath)); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(draftPath, "a: 1\n"); err != nil {
		t.Fatal(err)
	}

	repl := &REPL{Session: s}
	out := &strings.Builder{}
	repl.rejectDraft(out, "modules/shop/entity.yaml")
	if !strings.Contains(out.String(), "rejected") {
		t.Errorf("reject output = %q", out.String())
	}
	if _, err := os.Stat(draftPath); !os.IsNotExist(err) {
		t.Error("draft file not removed")
	}

	// Rejecting again → clear error, not a crash.
	out.Reset()
	repl.rejectDraft(out, "modules/shop/entity.yaml")
	if !strings.Contains(out.String(), "[error]") {
		t.Errorf("second reject output = %q", out.String())
	}
}

// ─── Diff (todo 10.4.2) ───

func TestUnifiedDiff(t *testing.T) {
	oldText := "a: 1\nb: 2\nc: 3\n"
	newText := "a: 1\nb: 22\nc: 3\nd: 4\n"
	got := unifiedDiff(oldText, newText)
	for _, want := range []string{"-b: 2", "+b: 22", "+d: 4", " a: 1"} {
		if !strings.Contains(got, want) {
			t.Errorf("diff missing %q:\n%s", want, got)
		}
	}
	if got := unifiedDiff(newText, newText); got != "" {
		t.Errorf("identical texts produced diff: %q", got)
	}
}

func TestDiffDrafts(t *testing.T) {
	// Layout: project/{spec, .formspec/consult/s1/draft}
	project := t.TempDir()
	specDir := filepathJoin(project, "spec", "modules", "shop")
	if err := osMkdirAll(specDir); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepathJoin(project, "spec", "modules", "shop", "entity.yaml"), "a: 1\n"); err != nil {
		t.Fatal(err)
	}
	sessionDir := filepathJoin(project, ".formspec", "consult", "s1")
	if err := osMkdirAll(filepathJoin(sessionDir, "draft", "modules", "shop")); err != nil {
		t.Fatal(err)
	}
	// Modified file + new file.
	if err := osWriteFile(filepathJoin(sessionDir, "draft", "modules", "shop", "entity.yaml"), "a: 2\n"); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepathJoin(sessionDir, "draft", "modules", "shop", "new.yaml"), "b: 1\n"); err != nil {
		t.Fatal(err)
	}

	diffs, err := DiffDrafts(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 2 {
		t.Fatalf("diffs = %d, want 2", len(diffs))
	}
	byPath := map[string]DraftDiff{}
	for _, d := range diffs {
		byPath[d.Path] = d
	}
	mod := byPath["modules/shop/entity.yaml"]
	if mod.IsNew || !strings.Contains(mod.Unified, "-a: 1") || !strings.Contains(mod.Unified, "+a: 2") {
		t.Errorf("modified diff wrong: %+v", mod)
	}
	newFile := byPath["modules/shop/new.yaml"]
	if !newFile.IsNew || !strings.Contains(newFile.Unified, "+b: 1") {
		t.Errorf("new file diff wrong: %+v", newFile)
	}
}
