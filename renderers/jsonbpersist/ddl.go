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
	columns = append(columns,
		fmt.Sprintf("id          %s", dl.uuidPK),
		fmt.Sprintf("tenant_id   %s   NOT NULL", dl.uuid),
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
		fmt.Sprintf("created_by  %s", dl.uuid),
		fmt.Sprintf("updated_by  %s", dl.uuid),
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
				gc := generateGeneratedColumn(f.Name, "uuid", driver)
				columns = append(columns, gc)
				if f.Index || f.Unique {
					idx := generateIndexConstraint(ti.TableName, f.Name, f.Unique)
					indexes = append(indexes, idx)
				}
			}
			continue
		}

		// Indexed fields get generated columns
		if f.Index || f.Unique || f.NaturalKey {
			sqlType := fieldTypeToSQL(f.Type, f.EnumValues)
			gc := generateGeneratedColumn(f.Name, sqlType, driver)
			columns = append(columns, gc)
		}

		// Unique constraint
		if f.Unique || f.NaturalKey {
			colName := generatedColumnName(f.Name)
			idxName := fmt.Sprintf("idx_uq_%s_%s", ti.TableName, f.Name)

			if driver == DriverSQLite {
				// SQLite: partial unique constraints must be CREATE UNIQUE INDEX.
				// Natural key uniqueness is scoped per tenant (tenant_id, _field)
				// per 01-core-basic.md §2: "unique constraint per tenant".
				if softDelete {
					indexes = append(indexes,
						fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (tenant_id, %s) WHERE deleted_at IS NULL;",
							idxName, ti.TableName, colName))
				} else {
					indexes = append(indexes,
						fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (tenant_id, %s);",
							idxName, ti.TableName, colName))
				}
			} else {
				// PostgreSQL: inline UNIQUE constraint with WHERE
				if softDelete {
					constraints = append(constraints,
						fmt.Sprintf("UNIQUE (tenant_id, %s) WHERE deleted_at IS NULL", colName))
				} else {
					constraints = append(constraints,
						fmt.Sprintf("UNIQUE (tenant_id, %s)", colName))
				}
			}
		}

		// Standalone index (not already handled by unique)
		if f.Index && !f.Unique && !f.NaturalKey {
			colName := generatedColumnName(f.Name)
			idx := fmt.Sprintf("CREATE INDEX idx_%s_%s ON %s (%s);",
				ti.TableName, f.Name, ti.TableName, colName)
			indexes = append(indexes, idx)
		}

		// Enum CHECK constraint — use json_extract since the generated column
		// may not exist if the field isn't indexed
		if f.Type == spec.FieldEnum && len(f.EnumValues) > 0 {
			var vals []string
			for _, v := range f.EnumValues {
				vals = append(vals, fmt.Sprintf("'%s'", v))
			}
			constraints = append(constraints,
				fmt.Sprintf("CHECK (json_extract(data, '$.%s') IN (%s))", f.Name, strings.Join(vals, ", ")))
		}
	}

	// 4. Additional indexes from EntitySpec.Indexes
	if entity.Persist != nil {
		for _, idx := range entity.Persist.Indexes {
			var colNames []string
			for _, f := range idx.Fields {
				colNames = append(colNames, generatedColumnName(f))
			}
			idxName := fmt.Sprintf("idx_%s_%s", ti.TableName, strings.Join(idx.Fields, "_"))
			if idx.Unique {
				indexes = append(indexes,
					fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s);", idxName, ti.TableName, strings.Join(colNames, ", ")))
			} else {
				indexes = append(indexes,
					fmt.Sprintf("CREATE INDEX %s ON %s (%s);", idxName, ti.TableName, strings.Join(colNames, ", ")))
			}
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

// generateGeneratedColumn creates a generated column for indexed/unique fields.
// PostgreSQL: data->>'field'  —  SQLite: json_extract(data, '$.field')
func generateGeneratedColumn(fieldName string, sqlType string, driver DriverType) string {
	colName := generatedColumnName(fieldName)
	var expr string
	if driver == DriverPostgres {
		expr = fmt.Sprintf("data->>'%s'", fieldName)
	} else {
		expr = fmt.Sprintf("json_extract(data, '$.%s')", fieldName)
	}
	return fmt.Sprintf("%s %s GENERATED ALWAYS AS (%s) STORED",
		colName, sqlType, expr)
}

// generatedColumnName returns the generated column name for a field.
func generatedColumnName(fieldName string) string {
	return "_" + fieldName
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
	switch ft {
	case spec.FieldString:
		return "text"
	case spec.FieldInteger:
		return "bigint"
	case spec.FieldDecimal, spec.FieldNumber:
		return "numeric(20,8)"
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
// names. Manifest names stay kebab-case everywhere else (routes, permissions).
func sanitizeIdent(s string) string {
	return strings.ReplaceAll(s, "-", "_")
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
func GenerateExtensionDDL(meta spec.Metadata, entity *spec.EntitySpec, driver DriverType) (*ExtensionDDLInfo, error) {
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
			sqlType := fieldTypeToSQL(f.Type, f.EnumValues)
			colName := generatedColumnName(f.Name)
			gc := fmt.Sprintf("%s %s GENERATED ALWAYS AS (%s->>'%s') STORED",
				colName, sqlType, extCol, f.Name)

			alterIdx := fmt.Sprintf(
				"ALTER TABLE %s ADD COLUMN %s;",
				targetTable, gc)

			if f.Unique || f.NaturalKey {
				idxName := fmt.Sprintf("idx_uq_ext_%s_%s", ns, f.Name)
				softDelete := true // default
				if driver == DriverSQLite {
					if softDelete {
						alterIdx += fmt.Sprintf("\nCREATE UNIQUE INDEX %s ON %s (%s) WHERE deleted_at IS NULL;",
							idxName, targetTable, colName)
					} else {
						alterIdx += fmt.Sprintf("\nCREATE UNIQUE INDEX %s ON %s (%s);",
							idxName, targetTable, colName)
					}
				} else {
					// PostgreSQL: inline UNIQUE is messy with ALTER TABLE ADD COLUMN
					// Use separate CREATE UNIQUE INDEX
					if softDelete {
						alterIdx += fmt.Sprintf("\nCREATE UNIQUE INDEX %s ON %s (%s) WHERE deleted_at IS NULL;",
							idxName, targetTable, colName)
					} else {
						alterIdx += fmt.Sprintf("\nCREATE UNIQUE INDEX %s ON %s (%s);",
							idxName, targetTable, colName)
					}
				}
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
