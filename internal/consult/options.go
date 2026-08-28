package consult

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Option picker (todo 10.2.5 — keputusan desain: ditangani langsung di REPL).
//
// Models frequently present choices as "A) … B) … C) …". The REPL detects
// such blocks, lets the user answer with a single letter, and injects the
// explicit selection (with the full option text) back into the history so
// context survives history compression. Free-text input is always allowed —
// detection is a convenience, never a required path (weak models must not
// be relied upon, same principle as the validation gate).

// optionLine matches a leading option marker like "A)" / "b." / "3)".
var optionLine = regexp.MustCompile(`(?m)^([A-Za-z0-9])[).]\s+(.+)$`)

// Options is a detected choice block.
type Options struct {
	// Labels in order, e.g. ["A", "B", "C"].
	Labels []string
	// Texts are the full option texts, parallel to Labels.
	Texts []string
}

// DetectOptions scans an assistant reply for a choice block. It returns nil
// when fewer than two consecutive option lines are found.
func DetectOptions(reply string) *Options {
	matches := optionLine.FindAllStringSubmatch(reply, -1)
	if len(matches) < 2 {
		return nil
	}
	// Require the options to be consecutive lines in one block: walk the
	// reply line by line and find the longest run of option lines.
	lines := strings.Split(reply, "\n")
	var best, cur Options
	flush := func() {
		if len(cur.Labels) > len(best.Labels) {
			best = cur
		}
		cur = Options{}
	}
	for _, line := range lines {
		m := optionLine.FindStringSubmatch(strings.TrimRight(line, " \t"))
		if m == nil {
			flush()
			continue
		}
		cur.Labels = append(cur.Labels, strings.ToUpper(m[1]))
		cur.Texts = append(cur.Texts, strings.TrimSpace(m[2]))
	}
	flush()
	if len(best.Labels) < 2 {
		return nil
	}
	return &best
}

// Pick resolves the user's answer: a bare label ("b") selects the option;
// anything else is returned as free text (ok=false).
func (o *Options) Pick(input string) (label, text string, ok bool) {
	input = strings.TrimSpace(input)
	if len(input) == 1 || (len(input) == 2 && strings.HasSuffix(strings.ToLower(input), ")")) {
		l := strings.ToUpper(strings.TrimSuffix(input, ")"))
		for i, have := range o.Labels {
			if have == l {
				return o.Labels[i], o.Texts[i], true
			}
		}
	}
	return "", input, false
}

// SelectionMessage builds the explicit user message injected after a pick —
// the full option text is included so the choice survives history
// compression (10.2.7).
func SelectionMessage(label, text string) string {
	return fmt.Sprintf("[memilih: %s) %s]", label, text)
}

// jsonMarshalRaw is a tiny indirection so session.go can marshal without
// importing encoding/json twice under different names.
func jsonMarshalRaw(v any) ([]byte, error) {
	return json.Marshal(v)
}
