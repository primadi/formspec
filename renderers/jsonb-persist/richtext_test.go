package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestSanitizeHTML(t *testing.T) {
	cases := []struct{ in, want string }{
		{`<p>Hello</p>`, `<p>Hello</p>`},
		{`<p>Hello<script>alert(1)</script></p>`, `<p>Hello</p>`},
		{`<p onclick="x()">Hi</p>`, `<p>Hi</p>`},
		{`<a href="javascript:alert(1)">x</a>`, `<a>x</a>`},
		{`<iframe src="x"></iframe>`, ``},
		{`<style>body{}</style><b>ok</b>`, `<b>ok</b>`},
	}
	for _, c := range cases {
		if got := sanitizeHTML(c.in); got != c.want {
			t.Errorf("sanitizeHTML(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestEntityStore_SanitizesRichTextOnInsert(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "richtext.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	entity := &spec.EntitySpec{
		Version: "v1", Plural: "posts", Characteristic: spec.CharMaster,
		Fields: []spec.Field{
			{Name: "title", Type: spec.FieldString, Required: true},
			{Name: "body", Type: spec.FieldRichText},
		},
	}
	meta := spec.Metadata{Name: "post", Module: "blog"}
	r := NewMigrationRunner(d, DriverSQLite)
	if _, err := r.ApplyMigrations(context.Background(), []EntityMigration{{Metadata: meta, EntitySpec: *entity}}); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	store := NewEntityStore(d, DriverSQLite, meta, entity)

	_, err = store.Insert(context.Background(), InsertParams{
		WorkspaceID: "demo", CreatedBy: "test",
		Data: map[string]any{
			"title": "Hello",
			"body":  `<p>Hello</p><script>alert(1)</script><p onclick="x()">Hi</p>`,
		},
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Fetch and verify the script/onclick were stripped.
	res, err := store.List(context.Background(), ListParams{WorkspaceID: "demo"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Data) != 1 {
		t.Fatalf("expected 1 record, got %d", len(res.Data))
	}
	body, _ := res.Data[0].Data["body"].(string)
	if contains(body, "<script>") || contains(body, "onclick") {
		t.Errorf("richtext not sanitized on insert: %q", body)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
