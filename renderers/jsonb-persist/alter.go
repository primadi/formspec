package db

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// IndexDef is a generated index in structured form — extracted from the
// CREATE INDEX statement the DDL generator already produced, so the preflight
// checks exactly what the index will enforce instead of re-deriving it (and
// drifting from it).
type IndexDef struct {
	Name    string
	Columns []string // derived column names, as written in the statement
	Where   string   // rendered predicate, "" when the index is not partial
	Unique  bool
}

// parseIndexSQL extracts the structured form of a generated CREATE INDEX
// statement. It returns ok=false for anything that is not one, because a
// statement this parser cannot read must not be silently treated as harmless.
func parseIndexSQL(stmt string) (IndexDef, bool) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(stmt), ";"))
	upper := strings.ToUpper(s)

	def := IndexDef{}
	switch {
	case strings.HasPrefix(upper, "CREATE UNIQUE INDEX "):
		def.Unique = true
		s = s[len("CREATE UNIQUE INDEX "):]
	case strings.HasPrefix(upper, "CREATE INDEX "):
		s = s[len("CREATE INDEX "):]
	default:
		return IndexDef{}, false
	}
	// Optional IF NOT EXISTS.
	if strings.HasPrefix(strings.ToUpper(s), "IF NOT EXISTS ") {
		s = s[len("IF NOT EXISTS "):]
	}

	onIdx := strings.Index(strings.ToUpper(s), " ON ")
	if onIdx < 0 {
		return IndexDef{}, false
	}
	def.Name = strings.TrimSpace(s[:onIdx])

	rest := s[onIdx+len(" ON "):]
	open := strings.Index(rest, "(")
	close := strings.LastIndex(rest, ")")
	if open < 0 || close < open {
		return IndexDef{}, false
	}
	for _, col := range strings.Split(rest[open+1:close], ",") {
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}
		def.Columns = append(def.Columns, col)
	}
	if wh := strings.TrimSpace(rest[close+1:]); wh != "" {
		w := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(wh), "WHERE"))
		def.Where = strings.TrimSpace(w)
	}
	return def, len(def.Columns) > 0
}

// indexDefsByName returns every generated index of an entity keyed by name.
func indexDefsByName(ti *TableInfo) map[string]IndexDef {
	out := make(map[string]IndexDef, len(ti.CreateIndexSQL))
	for _, stmt := range ti.CreateIndexSQL {
		if def, ok := parseIndexSQL(stmt); ok {
			out[def.Name] = def
		}
	}
	return out
}

// indexTouchesColumns reports whether a generated CREATE INDEX statement
// references any of the given derived columns — in its column list or in its
// partial predicate. These are the indexes that must be dropped before a stale
// derived column can be rebuilt: SQLite refuses to drop a column an index still
// references (kafe TODO 3.11).
func indexTouchesColumns(stmt string, cols map[string]bool) bool {
	def, ok := parseIndexSQL(stmt)
	if !ok {
		// A statement this parser cannot read may reference the column. Dropping
		// and re-creating it is harmless; leaving it in place makes the column
		// rebuild fail, so assume the worst.
		return true
	}
	for _, col := range def.Columns {
		if cols[col] {
			return true
		}
	}
	for _, tok := range derivedColRe.FindAllString(def.Where, -1) {
		if cols[tok] {
			return true
		}
	}
	return false
}

// addDerivedColumnSQL builds the statement that materializes one field's derived
// column on a table that already exists.
//
// SQLite refuses `ALTER TABLE ADD COLUMN` for a STORED generated column
// ("cannot add a STORED column"), but it accepts a VIRTUAL one — which is
// computed on read, so it is correct for the rows already stored *and* for
// every row inserted later. That matters: the insert path writes only
// `(id, tenant_id, version, data)` and relies on the column computing itself,
// so a plain column here would stay NULL forever and any unique index built over
// it would enforce nothing (kafe TODO 3.11).
func addDerivedColumnSQL(ti *TableInfo, f spec.Field, driver DriverType) string {
	sqlType := fieldTypeToSQLFor(f.Type, f.EnumValues, driver)
	if f.Type == spec.FieldRelation {
		// Relations live in the JSONB payload; the derived column holds the
		// reference id as text.
		sqlType = "text"
	}

	var colDef string
	if driver == DriverPostgres {
		colDef = generateGeneratedColumn(f.Name, f.Type, sqlType, driver)
	} else {
		colDef = fmt.Sprintf("%s %s GENERATED ALWAYS AS (%s) VIRTUAL",
			generatedColumnName(f.Name), sqlType, generatedColumnExpr(f.Name, f.Type, driver))
	}
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;",
		qualifiedName(ti.Schema, ti.TableName, driver), colDef)
}

// dropIndexSQL drops one index if it exists. PostgreSQL indexes live in the
// category schema, so they must be qualified there; SQLite has no schemas.
func dropIndexSQL(ti *TableInfo, name string, driver DriverType) string {
	if driver == DriverPostgres && ti.Schema != "" {
		return fmt.Sprintf("DROP INDEX IF EXISTS %s.%s;", ti.Schema, name)
	}
	return fmt.Sprintf("DROP INDEX IF EXISTS %s;", name)
}

// dropColumnSQL drops one column. SQLite has no DROP COLUMN IF EXISTS, so callers
// check existence first (the modernc driver supports DROP COLUMN since
// SQLite 3.35).
func dropColumnSQL(ti *TableInfo, col string, driver DriverType) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;",
		qualifiedName(ti.Schema, ti.TableName, driver), col)
}

// stripKeySQL removes one field's values from the JSONB payload. This is the
// statement that makes a declared field removal actually free the values — and
// it is the reason removal needs consent at all: nothing can bring them back.
func (r *MigrationRunner) stripKeySQL(ti *TableInfo, field string) string {
	tbl := qualifiedName(ti.Schema, ti.TableName, r.driver)
	if r.driver == DriverPostgres {
		// jsonb_exists is the functional form of the `?` operator — same
		// semantics, and no `?` character that the placeholder rewriter
		// (postgres_db.go) would mistake for a bind parameter.
		return fmt.Sprintf("UPDATE %s SET data = data - '%s' WHERE jsonb_exists(data, '%s');", tbl, field, field)
	}
	return fmt.Sprintf(
		"UPDATE %s SET data = json_remove(data, '$.%s') WHERE json_extract(data, '$.%s') IS NOT NULL;",
		tbl, field, field)
}

// countRowsWithKey counts the rows still holding a value for a field — the
// number the report must show, because "removed a field" says nothing while
// "1,240 rows lost their value" says everything.
func (r *MigrationRunner) countRowsWithKey(ctx context.Context, ti *TableInfo, field string) (int, error) {
	tbl := qualifiedName(ti.Schema, ti.TableName, r.driver)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE json_extract(data, '$.%s') IS NOT NULL", tbl, field)
	if r.driver == DriverPostgres {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE jsonb_exists(data, '%s')", tbl, field)
	}
	var n int
	if err := r.db.QueryRowContext(ctx, query).Scan(&n); err != nil {
		return 0, fmt.Errorf("count rows with %s.%s: %w", ti.TableName, field, err)
	}
	return n, nil
}

// derivedColRe matches the derived column names the DDL generator writes
// (`_branch_id`), which map back to a manifest field by stripping the
// underscore. Generated by us, so the rewrite below never has to guess.
var derivedColRe = regexp.MustCompile(`\b_[a-z][a-z0-9_]*\b`)

// payloadExpr reads one field out of the JSONB payload for the current driver.
func payloadExpr(driver DriverType, field string) string {
	if driver == DriverPostgres {
		return fmt.Sprintf("(data->>'%s')", field)
	}
	return fmt.Sprintf("json_extract(data, '$.%s')", field)
}

// payloadColumn maps a column name from a generated index back to something a
// query can read. Two shapes exist: a derived column over a payload field
// (`_branch_id`, may not exist yet) and a materialized column that is real from
// table creation (`_tpath_branch`).
func payloadColumn(driver DriverType, col string) string {
	if strings.HasPrefix(col, "_tpath_") {
		return col
	}
	if strings.HasPrefix(col, "_") {
		return payloadExpr(driver, strings.TrimPrefix(col, "_"))
	}
	return col
}

// payloadPredicate rewrites a rendered partial-index predicate so it reads the
// payload instead of the derived columns. Evaluating the predicate against
// columns that the same migration is about to create would fail — and skipping
// the predicate instead would count rows the index never covers, refusing
// migrations that are perfectly safe.
func payloadPredicate(driver DriverType, where string) string {
	return derivedColRe.ReplaceAllStringFunc(where, func(tok string) string {
		return payloadColumn(driver, tok)
	})
}

// countDuplicateGroups counts the groups that would violate a unique index —
// the preflight answer for the case that motivated declared data repairs in the
// first place: a constraint can only be added once the data satisfies it.
//
// Rows where any indexed column is NULL are excluded: both SQLite and PostgreSQL
// treat NULLs as distinct in a unique index, so counting them would refuse a
// perfectly addable index.
func (r *MigrationRunner) countDuplicateGroups(ctx context.Context, ti *TableInfo, def IndexDef) (int, error) {
	tbl := qualifiedName(ti.Schema, ti.TableName, r.driver)

	cols := make([]string, 0, len(def.Columns))
	for _, col := range def.Columns {
		cols = append(cols, payloadColumn(r.driver, col))
	}

	conds := make([]string, 0, len(cols)+1)
	if def.Where != "" {
		conds = append(conds, "("+payloadPredicate(r.driver, def.Where)+")")
	}
	for _, col := range cols {
		conds = append(conds, col+" IS NOT NULL")
	}

	query := fmt.Sprintf(
		"SELECT COUNT(*) FROM (SELECT 1 FROM %s WHERE %s GROUP BY %s HAVING COUNT(*) > 1) dup",
		tbl, strings.Join(conds, " AND "), strings.Join(cols, ", "))

	var n int
	if err := r.db.QueryRowContext(ctx, query).Scan(&n); err != nil {
		return 0, fmt.Errorf("count duplicate groups for index %s: %w", def.Name, err)
	}
	return n, nil
}

// castGuardSQL describes how a target field type is checked against the stored
// values. The check is deliberately conservative: when a type cannot be verified,
// the caller refuses the change until the manifest declares `accept_data_loss`,
// rather than assuming the values are fine.
//
// `col` is the derived column name; the payload key is recovered from it, because
// the guard has to be evaluable before the column exists (the same migration
// creates it).
func castGuardSQL(driver DriverType, ft spec.FieldType, col string) (cond string, assessed bool) {
	sqliteTypeof := fmt.Sprintf("typeof(json_extract(data, '$.%s'))", strings.TrimPrefix(col, "_"))

	if spec.IsNumericField(ft) {
		if driver == DriverPostgres {
			return fmt.Sprintf("(data->>'%s') !~ '^-?[0-9]+(\\.[0-9]+)?$'", strings.TrimPrefix(col, "_")), true
		}
		return fmt.Sprintf("%s NOT IN ('integer', 'real')", sqliteTypeof), true
	}

	switch ft {
	case spec.FieldBoolean:
		// SQLite stores booleans as 0/1; PostgreSQL as 'true'/'false' text.
		if driver == DriverPostgres {
			return "", false
		}
		return fmt.Sprintf("%s NOT IN ('integer', 'real')", sqliteTypeof), true
	case spec.FieldString, spec.FieldText, spec.FieldRichText, spec.FieldEnum, spec.FieldDate,
		spec.FieldDateTime, spec.FieldTime, spec.FieldUUID, spec.FieldJSON:
		if driver == DriverPostgres {
			// Every stored value can be read as text.
			return "", false
		}
		return fmt.Sprintf("%s != 'text'", sqliteTypeof), true
	default:
		return "", false
	}
}

// countCastFailures counts the rows whose stored value would not survive a type
// change on a projected column. `assessed=false` means the shape of the check is
// unknown for this type on this driver — the caller must then require consent
// instead of assuming safety.
func (r *MigrationRunner) countCastFailures(ctx context.Context, ti *TableInfo, field string, ft spec.FieldType) (int, bool, error) {
	tbl := qualifiedName(ti.Schema, ti.TableName, r.driver)
	cond, assessed := castGuardSQL(r.driver, ft, generatedColumnName(field))
	if !assessed {
		return 0, false, nil
	}

	exists := fmt.Sprintf("json_extract(data, '$.%s') IS NOT NULL", field)
	if r.driver == DriverPostgres {
		exists = fmt.Sprintf("jsonb_exists(data, '%s')", field)
	}
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s AND (%s)", tbl, exists, cond)

	var n int
	if err := r.db.QueryRowContext(ctx, query).Scan(&n); err != nil {
		return 0, true, fmt.Errorf("count cast failures for %s.%s: %w", ti.TableName, field, err)
	}
	return n, true, nil
}

// measureChanges runs the preflight and records what each measurable change
// actually costs. Nothing here decides policy — it only turns claims into
// numbers, which is what the gate needs to be trustworthy.
func (r *MigrationRunner) measureChanges(ctx context.Context, ti *TableInfo, entity *spec.EntitySpec, changes []Change) ([]Change, error) {
	defs := indexDefsByName(ti)

	for i := range changes {
		c := &changes[i]
		switch {
		case c.Kind == ChangeFieldRemoved:
			n, err := r.countRowsWithKey(ctx, ti, c.Name)
			if err != nil {
				return nil, err
			}
			c.Measure(n, "")

		case c.Class == ClassAdditive && c.Unique:
			def, ok := defs[c.Name]
			if !ok {
				// The index could not be read back in structured form, so its
				// duplicate situation is unknown. Refusing is the honest move:
				// creating it may fail mid-migration, which is worse.
				c.Class = ClassLossy
				c.Remedy = "could not verify duplicates for this index — repair any duplicates, then apply again"
				continue
			}
			n, err := r.countDuplicateGroups(ctx, ti, def)
			if err != nil {
				return nil, err
			}
			c.Measure(n, "repair the duplicates first — run the repair once via `formspec repl`, then apply again")

		case c.Kind == ChangeTypeChanged && c.Projected:
			f := fieldByName(entity, c.Name)
			if f == nil {
				continue
			}
			n, assessed, err := r.countCastFailures(ctx, ti, c.Name, f.Type)
			if err != nil {
				return nil, err
			}
			if !assessed {
				c.Class = ClassLossy
				c.Remedy = "this type change cannot be verified automatically — declare `accept_data_loss: true` + `reason`"
				continue
			}
			c.Measure(n, c.Remedy)
		}
	}
	return changes, nil
}

// buildChangeDDL turns classified changes into executable statements. Ordering is
// the substance here, not an implementation detail:
//
//  1. indexes that go away, change, or sit on a column being rebuilt are dropped
//     first — SQLite refuses to drop a column an index still references, and an
//     index whose predicate names a column that does not exist yet cannot be
//     created at all;
//  2. removed fields lose their payload values, then their derived column;
//  3. retyped fields get their column rebuilt;
//  4. the additive diff runs (missing derived columns, missing indexes) so it sees
//     the end state and stays idempotent;
//  5. dropped indexes are re-created only now, when every column they reference
//     exists;
//  6. declared raw_ddl that is new or changed runs last.
func (r *MigrationRunner) buildChangeDDL(ctx context.Context, ti *TableInfo, entity *spec.EntitySpec, old, desired *EntitySnapshot, changes []Change) (string, error) {
	existingCols, err := r.existingColumns(ctx, ti.Schema, ti.TableName)
	if err != nil {
		return "", err
	}
	desiredDefs := indexDefsByName(ti)

	// Columns this change set rebuilds or removes: any index over them has to be
	// dropped before the ALTER, or the ALTER fails on SQLite.
	rebuilt := map[string]bool{}
	for _, c := range changes {
		switch c.Kind {
		case ChangeFieldRemoved, ChangeTypeChanged:
			rebuilt[generatedColumnName(c.Name)] = true
		case ChangeFieldProjection:
			if !desiredFieldIsDerived(desired, c.Name) {
				rebuilt[generatedColumnName(c.Name)] = true
			}
		}
	}

	dropSet := map[string]bool{}
	var dropOrder []string
	markDrop := func(name string) {
		if name == "" || dropSet[name] {
			return
		}
		dropSet[name] = true
		dropOrder = append(dropOrder, name)
	}
	for _, c := range changes {
		if c.Kind == ChangeIndexRemoved || c.Kind == ChangeIndexChanged {
			markDrop(c.Name)
		}
	}
	for _, def := range desiredDefs {
		for _, col := range def.Columns {
			if rebuilt[col] {
				markDrop(def.Name)
				break
			}
		}
	}
	sort.Strings(dropOrder)

	var stmts []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			stmts = append(stmts, s)
		}
	}

	// 1. Index drops.
	for _, name := range dropOrder {
		add(dropIndexSQL(ti, name, r.driver))
	}

	// 2. Removed fields: strip the stored values, then drop the derived column.
	for _, c := range changes {
		if c.Kind != ChangeFieldRemoved {
			continue
		}
		add(r.stripKeySQL(ti, c.Name))
		col := generatedColumnName(c.Name)
		if existingCols[col] {
			add(dropColumnSQL(ti, col, r.driver))
		}
	}

	// 3. Retype: drop the old column and materialize the new type. A field that
	// merely *became* projected needs nothing here — step 4 adds it.
	for _, c := range changes {
		if c.Kind != ChangeTypeChanged {
			continue
		}
		col := generatedColumnName(c.Name)
		if existingCols[col] {
			add(dropColumnSQL(ti, col, r.driver))
		}
		if f := fieldByName(entity, c.Name); f != nil && desiredFieldIsDerived(desired, c.Name) {
			add(addDerivedColumnSQL(ti, *f, r.driver))
		}
	}

	// 4. Additive reconciliation: missing derived columns + missing indexes.
	alterDDL, _, err := r.diffExistingTable(ctx, ti, *entity, rebuilt)
	if err != nil {
		return "", err
	}
	add(alterDDL)

	// 5. Re-create the indexes dropped in step 1, now that their columns exist.
	// New indexes are not here: step 4 already created them.
	for _, c := range changes {
		if c.Kind != ChangeIndexRemoved && c.Kind != ChangeIndexChanged {
			continue
		}
		if _, stillWanted := desiredDefs[c.Name]; !stillWanted {
			continue
		}
		if stmt, ok := findIndexSQL(ti, c.Name); ok {
			add(withIfNotExists(stmt))
		}
	}

	// 6. raw_ddl that is new or whose statement changed.
	for _, c := range changes {
		if c.Kind != ChangeRawDDLAdded && c.Kind != ChangeRawDDLChanged {
			continue
		}
		stmt, ok := rawDDLStatement(entity, c.Name, r.driver)
		if !ok {
			continue
		}
		add(stmt)
	}

	return strings.Join(stmts, "\n"), nil
}

// findIndexSQL returns the generated statement for one index name.
func findIndexSQL(ti *TableInfo, name string) (string, bool) {
	for _, stmt := range ti.CreateIndexSQL {
		if indexNameOf(stmt) == name {
			return stmt, true
		}
	}
	return "", false
}

// desiredFieldIsDerived reports whether the desired shape projects the field.
func desiredFieldIsDerived(desired *EntitySnapshot, field string) bool {
	f, ok := desired.Field(field)
	return ok && f.Derived
}

// fieldByName returns a field of the entity, nil when absent.
func fieldByName(entity *spec.EntitySpec, name string) *spec.Field {
	if entity == nil {
		return nil
	}
	for i := range entity.Fields {
		if entity.Fields[i].Name == name {
			return &entity.Fields[i]
		}
	}
	return nil
}

// rawDDLStatement resolves one declared raw_ddl statement for this driver.
func rawDDLStatement(entity *spec.EntitySpec, name string, driver DriverType) (string, bool) {
	if entity == nil || entity.Persist == nil {
		return "", false
	}
	for i := range entity.Persist.RawDDL {
		decl := &entity.Persist.RawDDL[i]
		if decl.Name != name {
			continue
		}
		return decl.DDLForDialect(string(driver))
	}
	return "", false
}
