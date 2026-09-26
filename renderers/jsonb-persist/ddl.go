package db

import (
	"fmt"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// TableInfo holds the result of DDL generation for one entity.
type TableInfo struct {
	// Schema is the database schema (PostgreSQL) or "" (SQLite).
	Schema string
	// TableName is the table name (module_plural).
	TableName string
	// CreateTableSQL is the full CREATE TABLE statement.
	CreateTableSQL string
	// CreateIndexSQL are additional CREATE INDEX statements.
	CreateIndexSQL []string
	// ChildTables are DDL for child entities with storage: table.
	ChildTables []ChildTableInfo
	// Entity is the source entity metadata.
	Module    string
	Entity    string
	HasUUIDPK bool
}

// ChildTableInfo holds DDL for a child table.
type ChildTableInfo struct {
	ParentTable    string
	ChildField     string
	TableName      string
	CreateTableSQL string
}

// CategorySchema maps entity categories to PostgreSQL schemas.
var CategorySchema = map[string]string{
	"operational": "operational",
	"financial":   "financial",
	"compliance":  "compliance",
	"analytics":   "analytics",
	"master":      "master",
	"archive":     "archive",
}

// DefaultSchema is used when no category is specified.
const DefaultSchema = "operational"

// CategorySchemaValues returns the schema names in a stable order — the
// connection's default search_path (config.go) must include every schema an
// entity can live in, so unqualified SQL references resolve.
func CategorySchemaValues() []string {
	return []string{"operational", "financial", "compliance", "analytics", "master", "archive"}
}

// GeneratedColumnFunction is a PostgreSQL helper function generated columns
// depend on (see GeneratedColumnFunctions).
type GeneratedColumnFunction struct {
	Name string
	DDL  string
}

// GeneratedColumnFunctions are the IMMUTABLE SQL wrappers generated columns
// need for date/time payload text. PostgreSQL marks text→timestamp/date as
// mutable (the parse consults the DateStyle GUC), so a generated column
// refuses `data->>'x'::timestamp` with "generation expression is not
// immutable". The wrappers pin the parse to UTC — the app layer writes UTC
// ISO text — and declare IMMUTABLE explicitly. Numeric, boolean, and uuid
// casts are already immutable and stay inline (verified on PG 17). Created
// idempotently by EnsureSystemTables (master todo 15.8).
var GeneratedColumnFunctions = []GeneratedColumnFunction{
	{
		Name: "formspec_to_timestamptz",
		DDL: `CREATE OR REPLACE FUNCTION formspec_to_timestamptz(v text) RETURNS timestamptz
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $fn$
	SELECT (v)::timestamp AT TIME ZONE 'UTC'
$fn$;`,
	},
	{
		Name: "formspec_to_date",
		DDL: `CREATE OR REPLACE FUNCTION formspec_to_date(v text) RETURNS date
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $fn$
	SELECT v::date
$fn$;`,
	},
}

// dialect provides SQL type names for the target database.
type dialect struct {
	uuid        string
	timestamptz string
	jsonb       string
	nowFn       string
	uuidPK      string
	bigint      string
}

func dialectFor(driver DriverType) dialect {
	if driver == DriverSQLite {
		return dialect{
			uuid:        "text",
			timestamptz: "text",
			jsonb:       "text",
			nowFn:       "(datetime('now'))",
			// PK is a UUID v7 string generated at the app layer (see
			// NewUUIDv7 in tx.go), never SQLite AUTOINCREMENT — Core Basic
			// §2 mandates UUID v7 PKs with no per-backend exception.
			uuidPK: "text PRIMARY KEY",
			bigint: "integer",
		}
	}
	return dialect{
		uuid:        "uuid",
		timestamptz: "timestamptz",
		jsonb:       "jsonb",
		nowFn:       "now()",
		// Postgres also gets its default from the app layer (gen_uuid_v7()
		// may not exist without the pgcrypto/uuid-ossp extension enabled) —
		// the app always supplies id explicitly on INSERT.
		uuidPK: "uuid PRIMARY KEY",
		bigint: "bigint",
	}
}

// GenerateEntityDDL generates CREATE TABLE DDL from an Entity manifest.
func GenerateEntityDDL(meta spec.Metadata, entity *spec.EntitySpec, driver DriverType) (*TableInfo, error) {
	dl := dialectFor(driver)
	ti := &TableInfo{
		Module: meta.Module,
		Entity: meta.Name,
		// Both drivers now use an app-generated UUID v7 string PK (see
		// dialectFor) — HasUUIDPK is kept for callers that branched on
		// per-driver PK type, but the answer is always true today.
		HasUUIDPK: true,
	}

	// Determine schema (PostgreSQL only)
	ti.Schema = DefaultSchema
	if entity.Persist != nil && entity.Persist.Category != "" {
		if s, ok := CategorySchema[entity.Persist.Category]; ok {
			ti.Schema = s
		}
	}

	// Determine table name
	plural := entity.Plural
	if plural == "" {
		plural = inflectPlural(meta.Name)
	}
	ti.TableName = sanitizeIdent(meta.Module + "_" + plural)

	// Collect columns
	var columns []string
	var constraints []string
	var indexes []string

	// 1. Normative columns (§19)
	//
	// tenant_id / created_by / updated_by are app-layer strings, NOT UUIDs:
	// tenant_id carries the workspace slug ("kafe"), and anonymous creates
	// write "anonymous" into created_by. Declaring them uuid in PostgreSQL
	// made every insert fail with "invalid input syntax for type uuid"
	// (master todo 15.8); SQLite never showed the problem because text is
	// its only type.
	columns = append(columns,
		fmt.Sprintf("id          %s", dl.uuidPK),
		"tenant_id   text   NOT NULL",
		fmt.Sprintf("version     %s   NOT NULL DEFAULT 1", dl.bigint),
		fmt.Sprintf("created_at  %s   NOT NULL DEFAULT %s", dl.timestamptz, dl.nowFn),
		fmt.Sprintf("updated_at  %s   NOT NULL DEFAULT %s", dl.timestamptz, dl.nowFn),
	)

	// Soft delete column
	softDelete := true
	if entity.Persist != nil && entity.Persist.SoftDelete != nil {
		softDelete = *entity.Persist.SoftDelete
	}
	if softDelete {
		columns = append(columns, fmt.Sprintf("deleted_at  %s", dl.timestamptz))
	}

	columns = append(columns,
		"created_by  text",
		"updated_by  text",
	)

	// 1b. Document Model reserved columns (v0.3.0)
	columns = append(columns,
		"doc_status  VARCHAR(20) DEFAULT NULL",                                 // NULL = lifecycle-free
		fmt.Sprintf("amends      %s REFERENCES %s(id)", dl.uuid, ti.TableName), // FK to original document
		fmt.Sprintf("amended_by  %s REFERENCES %s(id)", dl.uuid, ti.TableName), // FK to new version
	)

	// 2. Data JSONB column
	columns = append(columns, fmt.Sprintf("data        %s   NOT NULL DEFAULT '{}'", dl.jsonb))

	// 3. User-defined fields
	for _, f := range entity.Fields {
		// A tombstoned field generates nothing — no derived column, no index,
		// no CHECK. Keeping it here made the CREATE/ALTER projection disagree
		// with the tombstone removal (which drops the column) and the plan
		// oscillated: apply dropped the column, the additive step re-added it
		// (master todo 15.8).
		if f.Removed {
			continue
		}
		switch f.Type {
		case spec.FieldChild:
			// Child fields are stored in JSONB or separate table
			if f.Child != nil && f.Child.Storage == "table" {
				childDDL := generateChildTableDDL(ti.TableName, f, driver)
				ti.ChildTables = append(ti.ChildTables, childDDL)
			}
			// jsonb storage: stored inside parent data JSONB
			continue

		case spec.FieldRelation:
			// Relation fields: store foreign key in data JSONB
			// unless belongs_to with explicit foreign_key
			if f.Relation != nil && f.Relation.ForeignKey != "" {
				gc := generateGeneratedColumn(f.Name, f.Type, "uuid", driver)
				columns = append(columns, gc)
				if f.Index || f.Unique {
					idx := generateIndexConstraint(ti.TableName, f.Name, f.Unique)
					indexes = append(indexes, idx)
				}
			} else if f.Index || f.Unique {
				// An indexed relation stores the reference id inside `data`, so
				// it needs the same derived column every other indexed field
				// gets. Skipping it made CREATE and ALTER disagree about which
				// fields have a column: the snapshot said the field is derived,
				// the table never had the column, and every migration plan
				// stayed non-convergent (`storage_drift` on a fresh database).
				columns = append(columns, generateGeneratedColumn(f.Name, f.Type, "text", driver))
				indexes = append(indexes, generateIndexConstraint(ti.TableName, f.Name, f.Unique))
			}
			// Tree/hierarchy (4.6.1): a self-referential relation marked
			// tree: true gets a materialized-path column _tpath_{field}.
			if f.Tree {
				columns = append(columns, fmt.Sprintf("_tpath_%s  text", f.Name))
				indexes = append(indexes,
					fmt.Sprintf("CREATE INDEX idx_%s_tpath_%s ON %s (_tpath_%s);",
						ti.TableName, f.Name, ti.TableName, f.Name))
			}
			continue
		}

		// Indexed fields get generated columns
		if f.Index || f.Unique || f.NaturalKey {
			sqlType := fieldTypeToSQLFor(f.Type, f.EnumValues, driver)
			gc := generateGeneratedColumn(f.Name, f.Type, sqlType, driver)
			columns = append(columns, gc)
		}

		// Unique constraint. `f.NaturalKey` implies uniqueness at validate time
		// (spec.ValidateEntitySpec sets Unique), but the flag is still tested here
		// because DDL can be generated from a spec that skipped validation.
		if f.Unique || f.NaturalKey {
			colName := generatedColumnName(f.Name)
			idxName := fmt.Sprintf("idx_uq_%s_%s", ti.TableName, f.Name)

			// A scoped natural key (rule `scope_field`) restarts per scope, so its
			// uniqueness must be scoped too: `(tenant_id, branch_id, _number)`.
			// Without the scope column, the second branch's `ORD-…-00001` collides
			// with the first branch's — the sequence is right, the guarantee is
			// not, and the insert dies on a UNIQUE violation (gap #9).
			keyCols := "tenant_id, " + colName
			if f.NaturalKey && f.NaturalKeyRule != nil && f.NaturalKeyRule.ScopeField != "" {
				keyCols = "tenant_id, " + generatedColumnName(f.NaturalKeyRule.ScopeField) + ", " + colName
			}

			// A natural key may be OPTIONAL (the author's "diisi user" mode), so a
			// blank value means "not set" rather than "a value" — two records
			// without a code are not duplicates, and an index that thinks they are
			// rejects the second insert ("UNIQUE constraint failed"). Uniqueness is
			// therefore enforced only over rows that actually carry a key, which
			// also matches SQL NULL semantics: a unique index never treats NULLs as
			// equal. A `required` natural key keeps the stricter index (the value is
			// always supplied), and a plain `unique` field follows its own contract.
			blankPredicate := ""
			if f.NaturalKey && !f.Required {
				blankPredicate = colName + " IS NOT NULL AND " + colName + " != ''"
			}

			predicate := ""
			switch {
			case softDelete && blankPredicate != "":
				predicate = "WHERE deleted_at IS NULL AND " + blankPredicate
			case softDelete:
				predicate = "WHERE deleted_at IS NULL"
			case blankPredicate != "":
				predicate = "WHERE " + blankPredicate
			}

			if driver == DriverSQLite {
				// SQLite: a partial unique constraint MUST be written as a CREATE
				// UNIQUE INDEX (an inline table constraint cannot carry a WHERE).
				// The unconditional case is an index too — deliberately: an inline
				// `UNIQUE` becomes an internal `sqlite_autoindex_*`, which carries
				// no name the migration diff can match, so every plan would report
				// storage drift against an index it could never find.
				stmt := fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)",
					idxName, ti.TableName, keyCols)
				if predicate != "" {
					stmt += " " + predicate
				}
				indexes = append(indexes, stmt+";")
			} else if predicate != "" {
				// PostgreSQL cannot put a WHERE on an inline UNIQUE constraint —
				// the clause is a syntax error there ("syntax error at or near
				// WHERE", first real PG run, master todo 15.8) — so the partial
				// index is the construction that works.
				indexes = append(indexes,
					fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s) %s;",
						idxName, ti.TableName, keyCols, predicate))
			} else {
				// Unconditional uniqueness: PostgreSQL may declare it inline.
				constraints = append(constraints,
					fmt.Sprintf("UNIQUE (%s)", keyCols))
			}
		}

		// Standalone index (not already handled by unique)
		if f.Index && !f.Unique && !f.NaturalKey {
			colName := generatedColumnName(f.Name)
			idx := fmt.Sprintf("CREATE INDEX idx_%s_%s ON %s (%s);",
				ti.TableName, f.Name, ti.TableName, colName)
			indexes = append(indexes, idx)
		}

		// Enum CHECK constraint — evaluates the value in the JSONB payload,
		// since the derived column may not exist when the field is not
		// indexed. The payload expression is driver-aware: SQLite reads it
		// with json_extract, PostgreSQL with data->>. Using the SQLite form
		// unconditionally produced DDL PostgreSQL rejected on the first real
		// run (`function json_extract(jsonb, unknown) does not exist`,
		// master todo 15.8).
		if f.Type == spec.FieldEnum && len(f.EnumValues) > 0 {
			var vals []string
			for _, v := range f.EnumValues {
				vals = append(vals, fmt.Sprintf("'%s'", v))
			}
			constraints = append(constraints,
				fmt.Sprintf("CHECK (%s IN (%s))", payloadExpr(driver, f.Name), strings.Join(vals, ", ")))
		}
	}

	// 4. Additional indexes from EntitySpec.Indexes and PersistSpec.Indexes.
	//
	// Both locations are honoured: `indexes:` is a top-level Entity key
	// (pkg/spec/entity.go EntitySpec.Indexes), while PersistSpec.Indexes is the
	// older storage-scoped form. Reading only the latter silently dropped every
	// declared index, so composite uniqueness such as
	// `(branch_id, menu_item_id)` was never enforced (gap #22).
	indexDecls := append([]spec.IndexDecl{}, entity.Indexes...)
	if entity.Persist != nil {
		indexDecls = append(indexDecls, entity.Persist.Indexes...)
	}
	fieldByName := make(map[string]spec.Field, len(entity.Fields))
	for _, f := range entity.Fields {
		fieldByName[f.Name] = f
	}

	// A scope field named by a natural_key_rule needs a derived column of its own:
	// the scoped unique index reads it (`tenant_id, _branch_id, _number`), and
	// referencing a column that was never materialized fails at CREATE INDEX with
	// "no such column: _branch_id" — the field itself lives inside the JSONB
	// payload (gap #9).
	{
		emitted := make(map[string]bool, len(columns))
		for _, c := range columns {
			if fields := strings.Fields(strings.TrimSpace(c)); len(fields) > 0 {
				emitted[fields[0]] = true
			}
		}
		for _, f := range entity.Fields {
			if !f.NaturalKey || f.NaturalKeyRule == nil || f.NaturalKeyRule.ScopeField == "" {
				continue
			}
			scopeField, ok := fieldByName[f.NaturalKeyRule.ScopeField]
			if !ok {
				continue
			}
			col := generatedColumnName(scopeField.Name)
			if emitted[col] {
				continue
			}
			sqlType := fieldTypeToSQLFor(scopeField.Type, scopeField.EnumValues, driver)
			if scopeField.Type == spec.FieldRelation {
				// Relations live in the JSONB payload; the derived column holds the
				// reference id as text.
				sqlType = "text"
			}
			columns = append(columns, generateGeneratedColumn(scopeField.Name, scopeField.Type, sqlType, driver))
			emitted[col] = true
		}
	}

	if len(indexDecls) > 0 {
		// Columns already emitted above (indexed/unique/natural-key fields,
		// relation foreign keys, tree paths, natural-key scope fields) — never
		// emit one twice.
		emitted := make(map[string]bool, len(columns))
		for _, c := range columns {
			fields := strings.Fields(strings.TrimSpace(c))
			if len(fields) > 0 {
				emitted[fields[0]] = true
			}
		}

		for _, idx := range indexDecls {
			// A partial index depends on two sets of fields: the indexed ones
			// and the ones its predicate compares. Both need a derived column —
			// a predicate on a column that was never materialized fails at
			// CREATE INDEX time ("no such column: _status"), because the field
			// itself lives inside the JSONB payload.
			for _, fn := range indexDeclFields(idx, fieldByName) {
				col := generatedColumnName(fn)
				if emitted[col] {
					continue
				}
				f, found := fieldByName[fn]
				if !found {
					continue
				}
				sqlType := fieldTypeToSQLFor(f.Type, f.EnumValues, driver)
				if f.Type == spec.FieldRelation {
					// Relations live in the JSONB payload; the derived column
					// is the reference id as text.
					sqlType = "text"
				}
				columns = append(columns, generateGeneratedColumn(fn, f.Type, sqlType, driver))
				emitted[col] = true
			}

			colNames := make([]string, 0, len(idx.Fields))
			ok := true
			for _, fn := range idx.Fields {
				if _, found := fieldByName[fn]; !found {
					// Unknown field: the validator owns that diagnosis; DDL
					// generation must not emit an index on a missing column.
					ok = false
					break
				}
				colNames = append(colNames, generatedColumnName(fn))
			}
			if !ok {
				continue
			}
			idxName := fmt.Sprintf("idx_%s_%s", ti.TableName, strings.Join(idx.Fields, "_"))
			// Partial index (S8): the predicate is written in field names and
			// rendered against the derived columns, exactly like idx.Fields.
			where, err := renderIndexWhere(idx.Where, fieldByName)
			if err != nil {
				// Validated upstream (ValidateEntitySpec) — reaching here means
				// the caller bypassed validation, so report rather than emit a
				// silently different index.
				return nil, fmt.Errorf("index %s on %s: %w", idxName, ti.TableName, err)
			}
			unique := ""
			if idx.Unique {
				unique = "UNIQUE "
			}
			indexes = append(indexes,
				fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)%s;", unique, idxName, ti.TableName, strings.Join(colNames, ", "), where))
		}
	}

	// Build CREATE TABLE
	var b strings.Builder
	b.WriteString("CREATE TABLE ")
	b.WriteString(qualifiedName(ti.Schema, ti.TableName, driver))
	b.WriteString(" (\n")

	for i, col := range columns {
		if i > 0 {
			b.WriteString(",\n")
		}
		b.WriteString("  ")
		b.WriteString(col)
	}

	// Add constraints
	for _, c := range constraints {
		b.WriteString(",\n  ")
		b.WriteString(c)
	}

	b.WriteString("\n);")

	ti.CreateTableSQL = b.String()
	ti.CreateIndexSQL = indexes

	return ti, nil
}

// generatedColumnExpr returns the SQL expression a derived column computes from
// the JSONB payload. Shared by the CREATE TABLE path and the ALTER path
// (addDerivedColumnSQL), because a column added later must compute exactly the
// same value as one created with the table — otherwise the two disagree and
// indexes over the ALTER-ed one enforce a different rule (kafe TODO 3.11).
func generatedColumnExpr(fieldName string, ft spec.FieldType, driver DriverType) string {
	// A money value is the object {amount, currency} (05-field-types.md §2).
	// Indexing the JSON text of that object is what made `9000` sort after
	// `10000` — the derived column must reach into `.amount` and be numeric, or
	// sorting, ranges, and aggregates silently compare JSON strings (#23).
	if ft == spec.FieldMoney {
		if driver == DriverPostgres {
			return fmt.Sprintf("data->'%s'->>'amount'", fieldName)
		}
		return fmt.Sprintf("CAST(json_extract(data, '$.%s.amount') AS REAL)", fieldName)
	}
	if driver == DriverPostgres {
		return fmt.Sprintf("data->>'%s'", fieldName)
	}
	return fmt.Sprintf("json_extract(data, '$.%s')", fieldName)
}

// generateGeneratedColumn creates a generated column for indexed/unique fields.
// PostgreSQL: data->>'field'  —  SQLite: json_extract(data, '$.field')
//
// Only usable where the table is created: SQLite refuses to add a STORED
// generated column with ALTER TABLE, so late columns go through
// addDerivedColumnSQL instead.
//
// The payload expression always yields text (`data->>`), so when the column's
// SQL type is anything other than text the expression must be cast to the
// column type — PostgreSQL refuses a generated column whose expression does
// not match its type ("column _transaction_date is of type timestamptz but
// default expression is of type text"), which no one had ever seen because
// this path had never run against PostgreSQL (master todo 15.8).
func generateGeneratedColumn(fieldName string, ft spec.FieldType, sqlType string, driver DriverType) string {
	expr := generatedColumnExpr(fieldName, ft, driver)
	if driver == DriverPostgres && sqlType != "text" && sqlType != "varchar(50)" {
		// date/time payload text cannot cast inline: the parse depends on the
		// DateStyle GUC, so the cast is not immutable and generated columns
		// refuse it. The IMMUTABLE wrappers (GeneratedColumnFunctions, created
		// by EnsureSystemTables) pin the parse to UTC. Numeric, boolean, and
		// uuid casts are already immutable and stay inline.
		switch sqlType {
		case "timestamptz":
			expr = fmt.Sprintf("formspec_to_timestamptz(%s)", expr)
		case "date":
			expr = fmt.Sprintf("formspec_to_date(%s)", expr)
		default:
			expr = fmt.Sprintf("(%s)::%s", expr, sqlType)
		}
	}
	return fmt.Sprintf("%s %s GENERATED ALWAYS AS (%s) STORED",
		generatedColumnName(fieldName), sqlType, expr)
}

// generatedColumnName returns the generated column name for a field.
func generatedColumnName(fieldName string) string {
	return "_" + fieldName
}

// renderIndexWhere renders a partial-index predicate (S8) against the entity's
// physical columns, returning " WHERE <predicate>" or "" when the index is not
// partial.
//
// The predicate is authored in field names (`status = 'open'`) because that is
// what a manifest author sees. Physical storage keeps every field inside the
// JSONB payload and indexes derived columns, so the predicate must be rewritten
// the same way idx.Fields is — otherwise the index would be created with a
// predicate referring to a column that does not exist.
//
// fieldByName doubles as the "is this a field or a system column?" test: names
// present there map to `_name`; anything else (deleted_at, tenant_id, …) is a
// real column and is emitted unchanged. That mirrors how the index column list
// is built just above.
func renderIndexWhere(where string, fieldByName map[string]spec.Field) (string, error) {
	if strings.TrimSpace(where) == "" {
		return "", nil
	}
	terms, err := spec.ParseIndexWhere(where, fieldNamesOf(fieldByName))
	if err != nil {
		return "", err
	}
	columnOf := func(field string) string {
		if _, ok := fieldByName[field]; ok {
			return generatedColumnName(field)
		}
		return field
	}
	predicate := spec.RenderIndexWhereTerms(terms, columnOf)
	if predicate == "" {
		return "", nil
	}
	return " WHERE " + predicate, nil
}

// indexDeclFields returns every entity field an index depends on: the indexed
// fields plus any field its partial predicate compares.
//
// Both need a derived column. The indexed fields already did; predicate fields
// did not, which made `where: "status = 'open'"` fail with "no such column:
// _status" because `status` itself only exists inside the JSONB payload.
// Names that are not entity fields (system columns such as `deleted_at`) are
// skipped — those are real columns and must not be shadowed by a `_`-prefixed
// duplicate.
func indexDeclFields(idx spec.IndexDecl, fieldByName map[string]spec.Field) []string {
	fields := append([]string{}, idx.Fields...)
	if strings.TrimSpace(idx.Where) == "" {
		return fields
	}
	terms, err := spec.ParseIndexWhere(idx.Where, fieldNamesOf(fieldByName))
	if err != nil {
		// Validated upstream; without a parseable predicate the caller reports
		// it when rendering. Do not guess which columns are needed.
		return fields
	}
	for _, t := range terms {
		if _, ok := fieldByName[t.Field]; ok {
			fields = append(fields, t.Field)
		}
	}
	return fields
}

// fieldNamesOf projects a field map onto the name set the predicate parser
// validates against.
func fieldNamesOf(fieldByName map[string]spec.Field) map[string]bool {
	names := make(map[string]bool, len(fieldByName))
	for name := range fieldByName {
		names[name] = true
	}
	return names
}

// GenerateIndexConstraint generates a CREATE INDEX statement for a single field.
func generateIndexConstraint(table, field string, unique bool) string {
	idxName := fmt.Sprintf("idx_%s_%s", table, field)
	colName := generatedColumnName(field)
	if unique {
		return fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s);", idxName, table, colName)
	}
	return fmt.Sprintf("CREATE INDEX %s ON %s (%s);", idxName, table, colName)
}

// fieldTypeToSQL maps a FormSpec FieldType to SQL type.
// params: enumValues for FieldEnum
func fieldTypeToSQL(ft spec.FieldType, _ []string) string {
	// Every numeric-but-not-integer type shares one column type: integer is the
	// only one with a distinct SQL type, so the numeric family is handled here
	// rather than enumerated case by case.
	if spec.IsNumericField(ft) && ft != spec.FieldInteger {
		return "numeric(20,8)"
	}
	switch ft {
	case spec.FieldString:
		return "text"
	case spec.FieldInteger:
		return "bigint"
	case spec.FieldBoolean:
		return "boolean"
	case spec.FieldEnum:
		return "varchar(50)"
	case spec.FieldDate:
		return "date"
	case spec.FieldDateTime:
		return "timestamptz"
	case spec.FieldJSON:
		return "jsonb"
	case spec.FieldUUID:
		return "uuid"
	case spec.FieldChild:
		return "jsonb"
	default:
		return "text"
	}
}

// fieldTypeToSQLFor is the dialect-aware form of fieldTypeToSQL. PostgreSQL
// native types (timestamptz, jsonb, uuid, bigint) must never leak into SQLite
// DDL — SQLite has no such types, so the column ends up with the wrong
// affinity (gap #27).
func fieldTypeToSQLFor(ft spec.FieldType, enumValues []string, driver DriverType) string {
	sqlType := fieldTypeToSQL(ft, enumValues)
	if driver == DriverSQLite {
		switch sqlType {
		case "timestamptz", "jsonb", "uuid":
			return "text"
		case "bigint":
			return "integer"
		}
	}
	return sqlType
}

// generateChildTableDDL generates DDL for a child with storage: table.
func generateChildTableDDL(parentTable string, field spec.Field, driver DriverType) ChildTableInfo {
	dl := dialectFor(driver)
	tableName := parentTable + "__" + sanitizeIdent(field.Name)

	var columns []string

	// Primary key
	columns = append(columns, fmt.Sprintf("  id          %s", dl.uuidPK))

	// Foreign key to parent — parent PK is a UUID v7 string on both drivers.
	// SQLite doesn't enforce FK by default, so we still declare it.
	columns = append(columns, fmt.Sprintf("  parent_id   %s   NOT NULL REFERENCES %s(id) ON DELETE CASCADE",
		dl.uuid, parentTable))

	// Sequence field (monotonically ordered per parent)
	if field.Child != nil && field.Child.SequenceField != "" {
		seqField := field.Child.SequenceField
		columns = append(columns, fmt.Sprintf("  %s        %s NOT NULL", seqField, dl.bigint))
	}

	// doc_status — child follows parent lifecycle (2.3.9)
	columns = append(columns, "  doc_status  VARCHAR(20) DEFAULT NULL")

	// Timestamp — useful for ordering
	columns = append(columns, fmt.Sprintf("  created_at  %s   NOT NULL DEFAULT %s", dl.timestamptz, dl.nowFn))

	// Child data stored in JSONB column
	columns = append(columns, fmt.Sprintf("  data        %s   NOT NULL DEFAULT '{}'", dl.jsonb))

	var b strings.Builder
	b.WriteString("CREATE TABLE ")
	b.WriteString(qualifiedName("", tableName, driver))
	b.WriteString(" (\n")
	for i, col := range columns {
		if i > 0 {
			b.WriteString(",\n")
		}
		b.WriteString("  ")
		b.WriteString(col)
	}
	b.WriteString("\n);")

	return ChildTableInfo{
		ParentTable:    parentTable,
		ChildField:     field.Name,
		TableName:      tableName,
		CreateTableSQL: b.String(),
	}
}

// inflectPlural is a simple English pluralizer for table names.
func inflectPlural(singular string) string {
	if singular == "" {
		return ""
	}
	// Handle common cases
	if strings.HasSuffix(singular, "s") || strings.HasSuffix(singular, "x") ||
		strings.HasSuffix(singular, "ch") || strings.HasSuffix(singular, "sh") {
		return singular + "es"
	}
	if strings.HasSuffix(singular, "y") && len(singular) > 2 {
		last := singular[len(singular)-2]
		if last != 'a' && last != 'e' && last != 'i' && last != 'o' && last != 'u' {
			return singular[:len(singular)-1] + "ies"
		}
	}
	return singular + "s"
}

// qualifiedName returns the qualified table name with schema for PostgreSQL.
func qualifiedName(schema, table string, driver DriverType) string {
	if driver == DriverPostgres && schema != "" {
		return schema + "." + table
	}
	return table
}

// TableName returns the entity table name following FormSpec convention.
func TableName(module, entity, plural string) string {
	if plural == "" {
		plural = inflectPlural(entity)
	}
	return sanitizeIdent(module + "_" + plural)
}

// sanitizeIdent makes a manifest name safe as an unquoted SQL identifier:
// kebab-case resource names (e.g. "medical-record") become snake_case table
// names, and dotted module names (e.g. "formspec.core") become underscore
// names ("formspec_core"). Manifest names stay kebab-case/dotted everywhere
// else (routes, permissions).
func sanitizeIdent(s string) string {
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ReplaceAll(s, ".", "_")
}

// ExtensionDDLInfo holds the DDL for an entity extension.
type ExtensionDDLInfo struct {
	TargetTable    string   // e.g. "billing_invoices"
	ExtensionTable string   // e.g. "ext_custext"
	Namespace      string   // e.g. "custext"
	AlterTableSQL  string   // ALTER TABLE ... ADD COLUMN ext_custext jsonb
	CreateIndexSQL []string // Generated column + index for indexed fields
}

// GenerateExtensionDDL generates ALTER TABLE DDL for an entity extension.
// According to Core spec §10-ext, extension adds a separate column ext_{namespace}
// to the target table, not nested inside the base data column.
func GenerateExtensionDDL(_ spec.Metadata, entity *spec.EntitySpec, driver DriverType) (*ExtensionDDLInfo, error) {
	if entity.ExtendStorage == nil {
		return nil, fmt.Errorf("extension: extend_storage is nil")
	}

	dl := dialectFor(driver)
	ns := entity.ExtendStorage.Namespace

	// Parse target: "module/entity" → table name
	parts := strings.Split(entity.ExtendStorage.Target, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("extension: invalid target %q", entity.ExtendStorage.Target)
	}
	targetTable := TableName(parts[0], parts[1], "")

	extCol := "ext_" + ns
	info := &ExtensionDDLInfo{
		TargetTable:    targetTable,
		ExtensionTable: extCol,
		Namespace:      ns,
	}

	// Generate ALTER TABLE ADD COLUMN ext_{namespace} jsonb NOT NULL DEFAULT '{}'
	info.AlterTableSQL = fmt.Sprintf(
		"ALTER TABLE %s ADD COLUMN %s %s NOT NULL DEFAULT '{}';",
		targetTable, extCol, dl.jsonb)

	// Generate generated columns and indexes for indexed/unique fields
	for _, f := range entity.Fields {
		if f.Index || f.Unique || f.NaturalKey {
			sqlType := fieldTypeToSQLFor(f.Type, f.EnumValues, driver)
			colName := generatedColumnName(f.Name)
			gc := fmt.Sprintf("%s %s GENERATED ALWAYS AS (%s->>'%s') STORED",
				colName, sqlType, extCol, f.Name)

			alterIdx := fmt.Sprintf(
				"ALTER TABLE %s ADD COLUMN %s;",
				targetTable, gc)

			if f.Unique || f.NaturalKey {
				idxName := fmt.Sprintf("idx_uq_ext_%s_%s", ns, f.Name)
				// Same rule as the CREATE TABLE path: an optional natural key's
				// blank value means "not set", so uniqueness must skip it or the
				// second blank row is refused. The two paths must agree, or an
				// entity created by ALTER enforces a different rule than one
				// created with its table.
				blankPredicate := ""
				if f.NaturalKey && !f.Required {
					blankPredicate = colName + " IS NOT NULL AND " + colName + " != ''"
				}
				predicate := "WHERE deleted_at IS NULL"
				if blankPredicate != "" {
					predicate += " AND " + blankPredicate
				}
				alterIdx += fmt.Sprintf("\nCREATE UNIQUE INDEX %s ON %s (%s) %s;",
					idxName, targetTable, colName, predicate)
			} else if f.Index {
				idxName := fmt.Sprintf("idx_ext_%s_%s", ns, f.Name)
				alterIdx += fmt.Sprintf("\nCREATE INDEX %s ON %s (%s);",
					idxName, targetTable, colName)
			}

			info.CreateIndexSQL = append(info.CreateIndexSQL, alterIdx)
		}
	}

	return info, nil
}

// GetExtensionColumnName returns the extension column name for a given namespace.
func GetExtensionColumnName(namespace string) string {
	return "ext_" + namespace
}
