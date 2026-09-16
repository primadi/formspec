package spec

import (
	"strings"
	"testing"
)

// TestParseIndexWhere_AcceptedForms pins the closed grammar (S8): the forms kafe
// needs must be expressible, and nothing more.
func TestParseIndexWhere_AcceptedForms(t *testing.T) {
	fields := map[string]bool{"status": true, "branch_id": true, "is_active": true}

	cases := []struct {
		name  string
		where string
		want  []IndexWhereTerm
	}{
		{
			name:  "string equality",
			where: "status = 'open'",
			want:  []IndexWhereTerm{{Field: "status", Op: "=", Value: "open", Quoted: true}},
		},
		{
			name:  "bare word literal is a string",
			where: "status = open",
			want:  []IndexWhereTerm{{Field: "status", Op: "=", Value: "open"}},
		},
		{
			name:  "numeric-looking string stays a string",
			where: "status = '007'",
			want:  []IndexWhereTerm{{Field: "status", Op: "=", Value: "007", Quoted: true}},
		},
		{
			name:  "numeric comparison",
			where: "branch_id >= 7",
			want:  []IndexWhereTerm{{Field: "branch_id", Op: ">=", Value: "7"}},
		},
		{
			name:  "not equal",
			where: "status <> 'void'",
			want:  []IndexWhereTerm{{Field: "status", Op: "<>", Value: "void", Quoted: true}},
		},
		{
			name:  "is null",
			where: "status IS NULL",
			want:  []IndexWhereTerm{{Field: "status", Null: true}},
		},
		{
			name:  "is not null",
			where: "status IS NOT NULL",
			want:  []IndexWhereTerm{{Field: "status", NotNull: true}},
		},
		{
			name:  "conjunction",
			where: "status = 'open' AND is_active = true",
			want: []IndexWhereTerm{
				{Field: "status", Op: "=", Value: "open", Quoted: true},
				{Field: "is_active", Op: "=", Value: "true"},
			},
		},
		{
			name:  "system column is allowed",
			where: "deleted_at IS NULL",
			want:  []IndexWhereTerm{{Field: "deleted_at", Null: true}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseIndexWhere(c.where, fields)
			if err != nil {
				t.Fatalf("ParseIndexWhere(%q): %v", c.where, err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("terms: got %d (%+v), want %d", len(got), got, len(c.want))
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("term %d: got %+v, want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

// TestParseIndexWhere_Rejects pins the other half of a closed grammar: anything
// outside it must fail loudly rather than reach the database as DDL.
func TestParseIndexWhere_Rejects(t *testing.T) {
	fields := map[string]bool{"status": true}

	cases := []struct {
		name    string
		where   string
		wantMsg string
	}{
		{"typo in field name", "staus = 'open'", "unknown field"},
		{"or is not in the grammar", "status = 'open' OR status = 'paid'", "unexpected text after the string literal"},
		{"function call", "lower(status) = 'open'", "not a valid field name"},
		{"unterminated literal", "status = 'open", "unterminated string literal"},
		{"no comparison at all", "status", "unsupported"},
		{"numeric field name", "1status = 'x'", "not a valid field name"},
		{"injection attempt via bare literal", "status = 1=1", "unsupported literal"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseIndexWhere(c.where, fields)
			if err == nil {
				t.Fatalf("ParseIndexWhere(%q): expected rejection", c.where)
			}
			if !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("error %q should contain %q", err, c.wantMsg)
			}
		})
	}
}

// TestValidateEntitySpec_IndexWhere wires the parser into manifest validation at
// both declaration sites — the error has to name the site so the author knows
// which `indexes:` block to fix.
func TestValidateEntitySpec_IndexWhere(t *testing.T) {
	valid := &EntitySpec{
		Version: "v1",
		Fields: []Field{
			{Name: "branch_id", Type: FieldRelation},
			{Name: "cashier_id", Type: FieldRelation},
			{Name: "status", Type: FieldEnum, EnumValues: []string{"open", "closed"}},
		},
		Indexes: []IndexDecl{{
			Fields: []string{"branch_id", "cashier_id"},
			Unique: true,
			Where:  "status = 'open'",
		}},
	}
	if err := ValidateEntitySpec(valid); err != nil {
		t.Fatalf("expected partial unique index to validate, got %v", err)
	}

	badWhere := *valid
	badWhere.Indexes = []IndexDecl{{Fields: []string{"status"}, Where: "status = 'open' OR 1=1"}}
	if err := ValidateEntitySpec(&badWhere); err == nil {
		t.Error("expected rejection for a predicate outside the grammar")
	} else if !strings.Contains(err.Error(), "indexes[0]") {
		t.Errorf("error %q should name the declaration site", err)
	}

	unknownField := *valid
	unknownField.Indexes = []IndexDecl{{Fields: []string{"status"}, Where: "nope = 1"}}
	if err := ValidateEntitySpec(&unknownField); err == nil {
		t.Error("expected rejection for a predicate on an unknown field")
	}

	whereWithoutFields := *valid
	whereWithoutFields.Indexes = []IndexDecl{{Where: "status = 'open'"}}
	if err := ValidateEntitySpec(&whereWithoutFields); err == nil {
		t.Error("expected rejection for where without fields")
	}

	// persist.indexes is the other declaration site and must be validated too.
	persistSite := *valid
	persistSite.Indexes = nil
	persistSite.Persist = &PersistSpec{Indexes: []IndexDecl{{Fields: []string{"status"}, Where: "nope = 1"}}}
	if err := ValidateEntitySpec(&persistSite); err == nil {
		t.Error("expected rejection at persist.indexes")
	} else if !strings.Contains(err.Error(), "persist.indexes[0]") {
		t.Errorf("error %q should name persist.indexes", err)
	}
}
