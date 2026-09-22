package db

import (
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// ChangeClass rates how much a schema change can cost. The classification is
// storage-agnostic on purpose (docs/spec/backend/04-persist-backend.md §2): it
// describes the *change*, not the SQL one backend happens to emit for it, so a
// second PersistBackend inherits the same guardrail instead of reinventing it.
//
// The rule the classes encode: a change that only adds, or that only rebuilds
// what the spec derives from the payload, may happen automatically — a change
// that destroys something the spec cannot recompute must be stated in the
// manifest first (01-core-basic.md §4.2).
type ChangeClass string

const (
	// ClassAdditive adds storage that did not exist (a table, a column, an
	// index). Nothing is discarded and no existing row is rewritten.
	ClassAdditive ChangeClass = "additive"
	// ClassDerived rebuilds projections derived from the JSONB payload —
	// generated columns, indexes, renames. The raw value in `data` is not
	// touched, so restoring the declaration restores the projection.
	ClassDerived ChangeClass = "derived"
	// ClassLossy destroys something that cannot be recomputed: keys stripped
	// from `data`, a derived column whose values do not cast to the new type,
	// a unique index blocked by existing duplicates. Refused unless the
	// manifest declares the loss (`removed` / `accept_data_loss` + `reason`).
	ClassLossy ChangeClass = "lossy"
	// ClassNever is refused no matter what the manifest says. Dropping a table
	// is the only member: its blast radius is every row of the entity, and a
	// diff cannot show what is being discarded. The path is `formspec backup`
	// → drop by hand → remove the manifest.
	ClassNever ChangeClass = "never"
)

// ChangeKind names the concrete difference, so a report can say what happened
// rather than only how dangerous it is.
type ChangeKind string

const (
	ChangeTableAdded        ChangeKind = "table_added"
	ChangeTableRemoved      ChangeKind = "table_removed"
	ChangeFieldAdded        ChangeKind = "field_added"
	ChangeChildTableAdded   ChangeKind = "child_table_added"
	ChangeChildTableRemoved ChangeKind = "child_table_removed"
	ChangeFieldRemoved      ChangeKind = "field_removed"
	// ChangeFieldProjection marks a field that started or stopped being
	// projected into a derived column.
	ChangeFieldProjection ChangeKind = "field_projection_changed"
	ChangeTypeChanged     ChangeKind = "type_changed"
	ChangeIndexAdded      ChangeKind = "index_added"
	ChangeIndexRemoved    ChangeKind = "index_removed"
	ChangeIndexChanged    ChangeKind = "index_changed"
	ChangeRawDDLAdded     ChangeKind = "raw_ddl_added"
	ChangeRawDDLChanged   ChangeKind = "raw_ddl_changed"
	ChangeRawDDLRemoved   ChangeKind = "raw_ddl_removed"
	// ChangeStorageDrift marks a database whose *storage* no longer matches the
	// manifest while the manifest itself is unchanged — an index whose
	// definition was never rebuilt after it changed (kafe TODO 3.9), or a
	// derived column that exists as a plain column instead of a generated one
	// (kafe TODO 3.11). It is not a difference in the snapshot diff at all; it
	// is discovered by looking at the database.
	ChangeStorageDrift ChangeKind = "storage_drift"
)

// Change is one difference between the applied schema and the manifest, rated
// by ChangeClass.
type Change struct {
	Class ChangeClass
	Kind  ChangeKind
	// Entity is "module/entity" — the scope the change belongs to.
	Entity string
	// Name is the field, index, or raw_ddl name the change is about.
	Name string
	// Detail is a human explanation (e.g. "string → integer").
	Detail string
	// Reason is the manifest's stated justification, empty when undeclared.
	Reason string
	// Declared reports whether the manifest consented to a lossy change.
	Declared bool
	// Remedy is what the operator should do when the change is refused.
	Remedy string
	// Unique marks index changes that add or remove a uniqueness guarantee, so
	// a report can say enforcement was gained or lost rather than only that an
	// index moved.
	Unique bool
	// Projected marks changes that reach a derived column, the only kind of
	// field a type change can actually damage.
	Projected bool
	// Rows is the preflight count of rows the change touches (values that would
	// be stripped, duplicates blocking an index, values that cannot be cast).
	// Zero means "not counted", never "none affected" — the two are different
	// claims and only the counter knows which one it is.
	Rows int
	// Counted reports whether Rows is a real measurement.
	Counted bool
}

// Required reports whether the change must be refused: a never-class change
// never passes, and a lossy one passes only when the manifest declared it.
func (c Change) Required() bool {
	switch c.Class {
	case ClassNever:
		return true
	case ClassLossy:
		return !c.Declared
	default:
		return false
	}
}

// Measure records what the preflight found and, when rows are actually
// affected, raises the change to lossy. This is the step that turns a
// general-sounding warning ("a unique index fails on duplicates") into the
// specific one ("43 duplicate groups block this index").
func (c *Change) Measure(rows int, remedy string) {
	c.Rows = rows
	c.Counted = true
	if rows > 0 {
		c.Class = ClassLossy
		if c.Remedy == "" {
			c.Remedy = remedy
		}
	}
}

// String renders one line for `migrate plan` / `migrate apply` output.
func (c Change) String() string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "[%s] %s", c.Class, c.Kind)
	if c.Name != "" {
		_, _ = fmt.Fprintf(&b, " %s", c.Name)
	}
	if c.Detail != "" {
		_, _ = fmt.Fprintf(&b, ": %s", c.Detail)
	}
	if c.Rows > 0 && c.Counted {
		_, _ = fmt.Fprintf(&b, " (%d row(s) affected)", c.Rows)
	}
	if c.Declared {
		_, _ = fmt.Fprintf(&b, " — declared: %s", c.Reason)
	}
	return b.String()
}

// Declarations carries what the manifest consented to: which fields it states
// are gone, and which type changes it accepts losing values for. Both map a
// field name to the `reason` that makes the statement auditable.
type Declarations struct {
	Removed        map[string]string
	AcceptDataLoss map[string]string
}

// FieldShape is one field as last applied: enough to notice a removal or a type
// change without re-reading the old manifest (which no longer exists).
type FieldShape struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Derived marks fields whose value is projected into a real column or index
	// (index/unique/natural_key, or named by a declared index). A non-derived
	// field lives only inside the JSONB payload, so removing or retyping it
	// costs no DDL — only the payload key changes meaning.
	Derived bool `json:"derived,omitempty"`
	// Child marks a child field stored in its own table (`storage: table`),
	// whose columns are real rather than projected.
	Child bool `json:"child,omitempty"`
}

// IndexShape is one index as last applied. Sig is a checksum of the generated
// CREATE INDEX statement, so a changed predicate or column list counts as a
// change while a mere reordering of declarations does not.
type IndexShape struct {
	Name   string `json:"name"`
	Sig    string `json:"sig"`
	Unique bool   `json:"unique,omitempty"`
}

// EntitySnapshot is the storage-relevant shape of one entity at the moment its
// last migration was applied. It exists because `formspec_schema_migrations`
// only records a checksum per entity: a checksum can say *that* something
// changed, never *what* — and without knowing what was there before, a removed
// field is indistinguishable from a field that never existed.
type EntitySnapshot struct {
	Table   string            `json:"table"`
	Schema  string            `json:"schema,omitempty"`
	Fields  []FieldShape      `json:"fields"`
	Indexes []IndexShape      `json:"indexes"`
	RawDDL  map[string]string `json:"raw_ddl,omitempty"`
}

// Field returns the shape of a field, if the snapshot has it.
func (s *EntitySnapshot) Field(name string) (FieldShape, bool) {
	if s == nil {
		return FieldShape{}, false
	}
	for _, f := range s.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return FieldShape{}, false
}

// Index returns the shape of an index, if the snapshot has it.
func (s *EntitySnapshot) Index(name string) (IndexShape, bool) {
	if s == nil {
		return IndexShape{}, false
	}
	for _, i := range s.Indexes {
		if i.Name == name {
			return i, true
		}
	}
	return IndexShape{}, false
}

// derivedColumnFields returns the entity fields whose value is projected into a
// real column: index/unique/natural-key fields, plus every field a declared
// index depends on — including the fields named only by a partial predicate
// (S8), which get a derived column too or CREATE INDEX fails on a missing
// column. Field order is preserved so generated DDL (and its checksum) stays
// deterministic.
func derivedColumnFields(entity *spec.EntitySpec) []string {
	if entity == nil {
		return nil
	}
	needed := make(map[string]bool)
	var ordered []string
	mark := func(f spec.Field) {
		if f.Type == spec.FieldChild || needed[f.Name] {
			return
		}
		needed[f.Name] = true
		ordered = append(ordered, f.Name)
	}

	indexDecls := append([]spec.IndexDecl{}, entity.Indexes...)
	if entity.Persist != nil {
		indexDecls = append(indexDecls, entity.Persist.Indexes...)
	}

	byName := make(map[string]spec.Field, len(entity.Fields))
	for _, f := range entity.Fields {
		byName[f.Name] = f
	}

	for _, f := range entity.Fields {
		// A tombstoned field is being removed on purpose: keeping it in the
		// derived-column set makes the additive step re-add its column right
		// after the removal step dropped it, and the plan never converges
		// (observed on the PG round-trip: apply → drop, plan → re-add).
		if f.Removed {
			continue
		}
		if f.Index || f.Unique || f.NaturalKey {
			mark(f)
		}
		if f.Relation != nil && f.Relation.ForeignKey != "" && (f.Index || f.Unique) {
			mark(f)
		}
		// A scoped natural key restarts per scope, so its uniqueness index
		// covers the scope column too: `(tenant_id, _branch_id, _number)` (3.6).
		// The scope field therefore needs a real column of its own — without
		// this, a table created before the scope was declared fails the index
		// rebuild with `no such column: _branch_id` (kafe TODO 3.9).
		if f.NaturalKey && f.NaturalKeyRule != nil && f.NaturalKeyRule.ScopeField != "" {
			if scope, ok := byName[f.NaturalKeyRule.ScopeField]; ok {
				mark(scope)
			}
		}
	}

	for _, idx := range indexDecls {
		for _, fn := range indexDeclFields(idx, byName) {
			if f, ok := byName[fn]; ok {
				mark(f)
			}
		}
	}
	return ordered
}

// DesiredSnapshot builds the shape the manifest asks for, together with the
// declarations that state a lossy intent. Removed fields are deliberately
// absent from the snapshot: a tombstone states intent, it is not part of the
// schema — while a field carrying `accept_data_loss` stays in the shape with its
// *new* type, which is what makes the change detectable at all.
func DesiredSnapshot(ti *TableInfo, entity *spec.EntitySpec, driver DriverType) (*EntitySnapshot, Declarations) {
	snap := &EntitySnapshot{Table: ti.TableName, Schema: ti.Schema}
	decls := Declarations{Removed: map[string]string{}, AcceptDataLoss: map[string]string{}}

	derived := make(map[string]bool)
	for _, name := range derivedColumnFields(entity) {
		derived[name] = true
	}

	for _, f := range entity.Fields {
		if f.Removed {
			decls.Removed[f.Name] = f.Reason
			continue
		}
		if f.AcceptDataLoss {
			decls.AcceptDataLoss[f.Name] = f.Reason
		}
		child := f.Type == spec.FieldChild && f.Child != nil && f.Child.Storage == "table"
		snap.Fields = append(snap.Fields, FieldShape{
			Name:    f.Name,
			Type:    string(f.Type),
			Derived: derived[f.Name],
			Child:   child,
		})
	}
	sort.Slice(snap.Fields, func(i, j int) bool { return snap.Fields[i].Name < snap.Fields[j].Name })

	for _, stmt := range ti.CreateIndexSQL {
		name := indexNameOf(stmt)
		if name == "" {
			continue
		}
		snap.Indexes = append(snap.Indexes, IndexShape{
			Name:   name,
			Sig:    checksumDDL(normalizeIndexSQL(stmt)),
			Unique: strings.Contains(strings.ToUpper(stmt), "UNIQUE INDEX"),
		})
	}
	sort.Slice(snap.Indexes, func(i, j int) bool { return snap.Indexes[i].Name < snap.Indexes[j].Name })

	if entity.Persist != nil && len(entity.Persist.RawDDL) > 0 {
		snap.RawDDL = make(map[string]string, len(entity.Persist.RawDDL))
		for i := range entity.Persist.RawDDL {
			decl := &entity.Persist.RawDDL[i]
			stmt, ok := decl.DDLForDialect(string(driver))
			if !ok {
				// Declared per-dialect but silent for this driver: the caller
				// reports the skip. Recording it would make the snapshot claim
				// a statement this database never ran.
				continue
			}
			snap.RawDDL[decl.Name] = checksumDDL(stmt)
		}
		if len(snap.RawDDL) == 0 {
			snap.RawDDL = nil
		}
	}

	return snap, decls
}

// normalizeIndexSQL makes two statements comparable that differ only in
// idempotence wording or whitespace, so re-planning a converged schema stays
// quiet.
func normalizeIndexSQL(stmt string) string {
	s := strings.Join(strings.Fields(stmt), " ")
	s = strings.ReplaceAll(s, "IF NOT EXISTS ", "")
	return strings.TrimSpace(s)
}

// DiffShapes compares the applied shape with the desired one and rates every
// difference. It is pure: no database is read, so the classification can be
// unit-tested and reused by any PersistBackend.
//
// `decls` carries the manifest's declarations and is what turns a field removal
// from "refused" into "declared" — and a type change from "refused if any row
// fails to cast" into "accepted".
func DiffShapes(entity string, old, desired *EntitySnapshot, decls Declarations) []Change {
	if old == nil {
		// Nothing applied yet — the table itself is the change.
		return []Change{{
			Class:  ClassAdditive,
			Kind:   ChangeTableAdded,
			Entity: entity,
			Name:   desired.Table,
			Detail: "table does not exist yet",
		}}
	}

	changes := []Change{}
	for _, of := range old.Fields {
		nf, ok := desired.Field(of.Name)
		if !ok {
			if of.Child {
				changes = append(changes, Change{
					Class:  ClassNever,
					Kind:   ChangeChildTableRemoved,
					Entity: entity,
					Name:   of.Name,
					Detail: "child table dropped",
					Remedy: "back up, drop the child table by hand, then remove the field from the manifest",
				})
				continue
			}
			reason, declared := decls.Removed[of.Name]
			changes = append(changes, Change{
				Class:    ClassLossy,
				Kind:     ChangeFieldRemoved,
				Entity:   entity,
				Name:     of.Name,
				Detail:   "stored values are discarded",
				Reason:   reason,
				Declared: declared,
				Remedy:   "declare the removal: `removed: true` + `reason: \"...\"` on the field",
			})
			continue
		}
		if nf.Type != of.Type {
			reason, declared := decls.AcceptDataLoss[of.Name]
			changes = append(changes, Change{
				Class:     ClassDerived,
				Kind:      ChangeTypeChanged,
				Entity:    entity,
				Name:      of.Name,
				Detail:    fmt.Sprintf("%s → %s", of.Type, nf.Type),
				Reason:    reason,
				Declared:  declared,
				Projected: nf.Derived,
				Remedy:    "declare the loss: `accept_data_loss: true` + `reason: \"...\"` on the field",
			})
		}
		if nf.Derived != of.Derived {
			detail := "no longer projected into a column"
			if nf.Derived {
				detail = "now projected into a column"
			}
			changes = append(changes, Change{
				Class:  ClassDerived,
				Kind:   ChangeFieldProjection,
				Entity: entity,
				Name:   of.Name,
				Detail: detail,
			})
		}
	}

	for _, nf := range desired.Fields {
		if _, ok := old.Field(nf.Name); ok {
			continue
		}
		kind := ChangeFieldAdded
		detail := "new field"
		if nf.Child {
			kind = ChangeChildTableAdded
			detail = "new child table"
		}
		changes = append(changes, Change{
			Class:  ClassAdditive,
			Kind:   kind,
			Entity: entity,
			Name:   nf.Name,
			Detail: detail,
		})
	}

	for _, oi := range old.Indexes {
		ni, ok := desired.Index(oi.Name)
		if !ok {
			detail := "index dropped"
			if oi.Unique {
				detail = "unique index dropped — uniqueness is no longer enforced by the database"
			}
			changes = append(changes, Change{
				Class:  ClassDerived,
				Kind:   ChangeIndexRemoved,
				Entity: entity,
				Name:   oi.Name,
				Detail: detail,
				Unique: oi.Unique,
			})
			continue
		}
		if ni.Sig != oi.Sig {
			changes = append(changes, Change{
				Class:  ClassDerived,
				Kind:   ChangeIndexChanged,
				Entity: entity,
				Name:   oi.Name,
				Detail: "definition changed",
				Unique: ni.Unique,
			})
		}
	}

	for _, ni := range desired.Indexes {
		if _, ok := old.Index(ni.Name); ok {
			continue
		}
		detail := "new index"
		remedy := ""
		if ni.Unique {
			detail = "new unique index"
			remedy = "repair the duplicates first — run the repair once via `formspec repl`, then apply again"
		}
		changes = append(changes, Change{
			Class:  ClassAdditive,
			Kind:   ChangeIndexAdded,
			Entity: entity,
			Name:   ni.Name,
			Detail: detail,
			Unique: ni.Unique,
			Remedy: remedy,
		})
	}

	for name, sig := range desired.RawDDL {
		oldSig, ok := old.RawDDL[name]
		switch {
		case !ok:
			changes = append(changes, Change{
				Class:  ClassAdditive,
				Kind:   ChangeRawDDLAdded,
				Entity: entity,
				Name:   name,
			})
		case oldSig != sig:
			changes = append(changes, Change{
				Class:  ClassDerived,
				Kind:   ChangeRawDDLChanged,
				Entity: entity,
				Name:   name,
				Detail: "statement changed",
			})
		}
	}
	for name := range old.RawDDL {
		if _, ok := desired.RawDDL[name]; ok {
			continue
		}
		// Forward-only: the DDL stays applied. Reporting it keeps the removal
		// visible instead of letting the artifact become untraceable.
		changes = append(changes, Change{
			Class:  ClassDerived,
			Kind:   ChangeRawDDLRemoved,
			Entity: entity,
			Name:   name,
			Detail: "declaration removed — the DDL stays applied (forward-only)",
		})
	}

	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].Class != changes[j].Class {
			return changes[i].Class < changes[j].Class
		}
		if changes[i].Kind != changes[j].Kind {
			return changes[i].Kind < changes[j].Kind
		}
		return changes[i].Name < changes[j].Name
	})
	return changes
}

// RefuseUndeclared returns an error when any change must be refused: a lossy
// change the manifest did not declare, or a never-class change (dropping a
// table) that no declaration can make acceptable.
//
// The message names every offending change rather than the first, because a
// refusal that shows one problem at a time turns one review into several deploy
// attempts.
func RefuseUndeclared(changes []Change) error {
	refused := Refused(changes)
	if len(refused) == 0 {
		return nil
	}

	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%d destructive change(s) refused", len(refused))
	for _, c := range refused {
		b.WriteString("\n  - ")
		b.WriteString(c.String())
		if c.Remedy != "" {
			b.WriteString("\n    ")
			b.WriteString(c.Remedy)
		}
	}
	return fmt.Errorf("%s", b.String())
}

// Refused returns the changes the gate rejects, in diff order.
func Refused(changes []Change) []Change {
	var out []Change
	for _, c := range changes {
		if c.Required() {
			out = append(out, c)
		}
	}
	return out
}

// DeclaredChanges returns the lossy changes the manifest did consent to — the
// ones the engine executes, having said out loud what they cost.
func DeclaredChanges(changes []Change) []Change {
	var out []Change
	for _, c := range changes {
		if c.Class == ClassLossy && c.Declared {
			out = append(out, c)
		}
	}
	return out
}

// Measurable returns the changes the preflight must count before the gate can
// decide: a unique index that may already have duplicates, or a type change on a
// projected column whose values may not cast.
func Measurable(changes []Change) []Change {
	var out []Change
	for _, c := range changes {
		if c.Class == ClassAdditive && c.Unique {
			out = append(out, c)
		}
		if c.Class == ClassDerived && c.Kind == ChangeTypeChanged && c.Projected {
			out = append(out, c)
		}
	}
	return out
}
