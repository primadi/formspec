package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fieldDescriptionRe captures one `description:` line that is indented at least
// six spaces — i.e. a FIELD or ACTION description, never the entity-level one
// (two spaces, which is `metadata.description` and stays a design summary).
var fieldDescriptionRe = regexp.MustCompile(`(?m)^ {6,}description:[ \t]*(.+)$`)

// developerNoteMarkers are phrases that mark a sentence as engineering note
// rather than user help. They are literal on purpose: a heuristic that guesses
// would need tuning, while these are exactly what was found in `examples/kafe`
// (business-rule cross-references, dispatch identifiers, spec section numbers,
// "computed from …" formulas, and type-shape documentation).
var developerNoteMarkers = []string{
	"aturan bisnis #", // cross-reference to a rule ledger
	"(S",              // spec section, e.g. "(S12)"
	"(D",              // decision ledger, e.g. "(D5)"
	"script membuat",  // names the implementation instead of the outcome
	"compute dari",    // formula, not help
	"compute:",
	"denormalisasi",
	"untuk telusur balik",
	"saat draft dibuat",
	"TIDAK mengikuti",
	// An example VALUE ("Mis. KFE-JKT-01") is legitimately helpful and stays —
	// it tells the user what to type. What does not belong is documenting the
	// TYPE SHAPE, which is what an array literal in a description is doing.
	"mis. [",
}

// TestKafeFieldDescriptionsAreUserFacing pins todo 5.23.2.
//
// `Field.description` is inherited as the field's `help` wherever the field
// appears (todo 5.23.1), which makes it USER-FACING text — it renders under the
// input. In `examples/kafe` many were written as engineering notes, so the help
// under an input read things like
//
//	"compute: kas awal + tunai masuk - kas keluar"
//	"Array angka 1=Senin..7=Minggu, mis. [1,2,3,4,5]"
//	"Posting — script membuat stock-movement (adjust) sebesar selisih"
//
// The fix moved each note into a YAML comment (where engineers still find it)
// and wrote a sentence a cashier can act on. This test keeps the split honest:
// a note creeping back into `description` fails here rather than shipping as
// baffling UI copy.
//
// Only `examples/kafe` is enforced. Earlier fixes recorded 19 similar
// declarations in `verticals/`, `examples/Clinic-UI-Showcase/`, and
// `cmd/formspec-registry/` that were deliberately not touched — see the item.
func TestKafeFieldDescriptionsAreUserFacing(t *testing.T) {
	const specRoot = "../../examples/kafe/spec"
	if _, err := os.Stat(specRoot); err != nil {
		t.Skipf("kafe spec not present: %v", err)
	}

	var offenders []string
	err := filepath.WalkDir(specRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// Skip the whole line if it is commented out, so a note that was moved
		// into a `# …` comment does not trip the scan.
		for _, m := range fieldDescriptionRe.FindAllStringSubmatch(string(raw), -1) {
			text := m[1]
			for _, marker := range developerNoteMarkers {
				if strings.Contains(text, marker) {
					rel, _ := filepath.Rel(specRoot, path)
					offenders = append(offenders, rel+": "+strings.TrimSpace(text))
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk spec: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("%d field description(s) still read as developer notes — "+
			"`description` is inherited as the field's `help` (todo 5.23.1), so it "+
			"renders under the input. Move the note into a YAML comment and write a "+
			"sentence the user can act on (todo 5.23.2):\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}
