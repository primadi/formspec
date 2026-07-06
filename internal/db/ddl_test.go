package db

import (
	"strings"
	"testing"

	"github.com/forma/forma/pkg/spec"
)

func customerEntity() (spec.Metadata, *spec.EntitySpec) {
	return spec.Metadata{
			Name:   "customer",
			Module: "billing",
		}, &spec.EntitySpec{
			Version:         "v1",
			Characteristics: []spec.Characteristic{spec.CharMaster},
			Fields: []spec.Field{
				{Name: "name", Type: spec.FieldString, Required: true},
				{Name: "email", Type: spec.FieldString, Unique: true, Index: true, Required: true},
				{Name: "phone", Type: spec.FieldString},
				{Name: "is_blacklisted", Type: spec.FieldBoolean, Default: false},
				{Name: "member_tier", Type: spec.FieldEnum, EnumValues: []string{"regular", "silver", "gold"}, Default: "regular"},
				{Name: "notes", Type: spec.FieldString},
			},
		}
}

func orderEntity() (spec.Metadata, *spec.EntitySpec) {
	return spec.Metadata{
			Name:   "order",
			Module: "billing",
		}, &spec.EntitySpec{
			Version:         "v1",
			Characteristics: []spec.Characteristic{spec.CharTransaction},
			Fields: []spec.Field{
				{
					Name: "number", Type: spec.FieldString,
					NaturalKey: true, Unique: true, Index: true, Immutable: true,
					NaturalKeyRule: &spec.NaturalKeyRuleDecl{
						Strategy: "sequence",
						Format:   "{prefix}-{year}-{seq:06d}",
						Prefix:   &spec.NaturalKeyPrefix{Config: "billing.order_number_prefix", Default: "ORD"},
						Reset:    "yearly",
					},
				},
				{
					Name: "customer_id", Type: spec.FieldUUID,
					Relation: &spec.RelationDecl{Type: "belongs_to", Resource: "customer"},
					Required: true,
				},
				{
					Name: "items", Type: spec.FieldChild,
					Child: &spec.ChildDecl{Storage: "jsonb"},
				},
				{Name: "total", Type: spec.FieldNumber, Required: true},
				{Name: "member_tier", Type: spec.FieldEnum, EnumValues: []string{"regular", "silver", "gold"}},
				{
					Name: "status", Type: spec.FieldEnum,
					EnumValues: []string{"draft", "awaiting_payment", "paid", "fulfilled", "void"},
					Index:      true,
				},
			},
		}
}

func TestGenerateEntityDDL_Customer(t *testing.T) {
	meta, entity := customerEntity()
	ti, err := GenerateEntityDDL(meta, entity, DriverSQLite)
	if err != nil {
		t.Fatalf("GenerateEntityDDL failed: %v", err)
	}

	// Verify table name
	expectedTable := "billing_customers"
	if ti.TableName != expectedTable {
		t.Errorf("expected table %q, got %q", expectedTable, ti.TableName)
	}

	// Verify CREATE TABLE contains normative columns
	checks := []string{
		"CREATE TABLE billing_customers",
		"PRIMARY KEY",
		"tenant_id",
		"version",
		"created_at",
		"deleted_at",
	}
	for _, c := range checks {
		if !strings.Contains(ti.CreateTableSQL, c) {
			t.Errorf("expected CREATE TABLE to contain %q", c)
		}
	}

	// Verify generated columns for indexed/unique fields
	if !strings.Contains(ti.CreateTableSQL, "_email text GENERATED ALWAYS AS (json_extract(data, '$.email')) STORED") {
		t.Error("expected generated column for email")
	}

	// Unique constraints go into CREATE UNIQUE INDEX for SQLite
	hasUniqueIdx := false
	for _, idx := range ti.CreateIndexSQL {
		if strings.Contains(idx, "idx_uq_billing_customers_email") {
			hasUniqueIdx = true
			break
		}
	}
	if !hasUniqueIdx {
		t.Error("expected UNIQUE INDEX for email")
	}

	// Verify child tables (none for customer)
	if len(ti.ChildTables) != 0 {
		t.Errorf("expected 0 child tables, got %d", len(ti.ChildTables))
	}

	t.Logf("Customer DDL:\n%s", ti.CreateTableSQL)
	for _, idx := range ti.CreateIndexSQL {
		t.Logf("Index: %s", idx)
	}
}

func TestGenerateEntityDDL_Order(t *testing.T) {
	meta, entity := orderEntity()
	ti, err := GenerateEntityDDL(meta, entity, DriverSQLite)
	if err != nil {
		t.Fatalf("GenerateEntityDDL failed: %v", err)
	}

	// Verify child tables
	if len(ti.ChildTables) != 0 {
		t.Logf("Child tables: %d", len(ti.ChildTables))
		for _, ct := range ti.ChildTables {
			t.Logf("Child DDL:\n%s", ct.CreateTableSQL)
		}
	}

	// Natural key is unique (as CREATE UNIQUE INDEX for SQLite)
	hasUniqueNK := false
	for _, idx := range ti.CreateIndexSQL {
		if strings.Contains(idx, "idx_uq_billing_orders_number") {
			hasUniqueNK = true
			break
		}
	}
	if !hasUniqueNK {
		t.Error("expected UNIQUE INDEX for natural key number")
	}

	// Enum CHECK constraint uses json_extract
	if !strings.Contains(ti.CreateTableSQL, "CHECK (json_extract(data, '$.status') IN") {
		t.Error("expected CHECK constraint for status enum")
	}

	// Generated columns
	if !strings.Contains(ti.CreateTableSQL, "_number text GENERATED ALWAYS AS (json_extract(data, '$.number')) STORED") {
		t.Error("expected generated column for number")
	}
	if !strings.Contains(ti.CreateTableSQL, "_status varchar(50) GENERATED ALWAYS AS (json_extract(data, '$.status')) STORED") {
		t.Error("expected generated column for status")
	}

	t.Logf("Order DDL:\n%s", ti.CreateTableSQL)
}

func TestGenerateEntityDDL_PostgresSchema(t *testing.T) {
	meta, entity := customerEntity()
	entity.Persist = &spec.PersistSpec{
		Category: "financial",
	}

	ti, err := GenerateEntityDDL(meta, entity, DriverPostgres)
	if err != nil {
		t.Fatalf("GenerateEntityDDL failed: %v", err)
	}

	if ti.Schema != "financial" {
		t.Errorf("expected schema 'financial', got %q", ti.Schema)
	}

	if !strings.Contains(ti.CreateTableSQL, "financial.billing_customers") {
		t.Error("expected qualified table name with schema")
	}
}

func TestGenerateEntityDDL_ChildTable(t *testing.T) {
	meta := spec.Metadata{Name: "invoice", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "number", Type: spec.FieldString, Required: true},
			{
				Name: "lines",
				Type: spec.FieldChild,
				Child: &spec.ChildDecl{
					Storage:       "table",
					SequenceField: "line_number",
				},
			},
		},
	}

	ti, err := GenerateEntityDDL(meta, entity, DriverSQLite)
	if err != nil {
		t.Fatalf("GenerateEntityDDL failed: %v", err)
	}

	if len(ti.ChildTables) != 1 {
		t.Fatalf("expected 1 child table, got %d", len(ti.ChildTables))
	}

	ct := ti.ChildTables[0]
	if ct.TableName != "billing_invoices__lines" {
		t.Errorf("expected child table 'billing_invoices__lines', got %q", ct.TableName)
	}

	if !strings.Contains(ct.CreateTableSQL, "REFERENCES billing_invoices(id) ON DELETE CASCADE") {
		t.Error("expected FK reference in child table")
	}

	t.Logf("Parent DDL:\n%s", ti.CreateTableSQL)
	t.Logf("Child DDL:\n%s", ct.CreateTableSQL)
}

func TestPluralInflection(t *testing.T) {
	tests := []struct {
		singular string
		plural   string
	}{
		{"customer", "customers"},
		{"invoice", "invoices"},
		{"address", "addresses"},
		{"box", "boxes"},
		{"category", "categories"},
		{"entry", "entries"},
		{"status", "statuses"},
		{"order", "orders"},
	}
	for _, tt := range tests {
		got := inflectPlural(tt.singular)
		if got != tt.plural {
			t.Errorf("inflectPlural(%q) = %q, want %q", tt.singular, got, tt.plural)
		}
	}
}

func TestGeneratedColumnName(t *testing.T) {
	if got := generatedColumnName("email"); got != "_email" {
		t.Errorf("expected '_email', got %q", got)
	}
}

func TestFieldTypeToSQL(t *testing.T) {
	tests := []struct {
		ft  spec.FieldType
		sql string
	}{
		{spec.FieldString, "text"},
		{spec.FieldNumber, "numeric(20,8)"},
		{spec.FieldBoolean, "boolean"},
		{spec.FieldDate, "date"},
		{spec.FieldDateTime, "timestamptz"},
		{spec.FieldJSON, "jsonb"},
		{spec.FieldUUID, "uuid"},
		{spec.FieldEnum, "varchar(50)"},
		{spec.FieldChild, "jsonb"},
	}
	for _, tt := range tests {
		got := fieldTypeToSQL(tt.ft, nil)
		if got != tt.sql {
			t.Errorf("fieldTypeToSQL(%q) = %q, want %q", tt.ft, got, tt.sql)
		}
	}
}

func TestGenerateEntityDDL_VerifiesBuild(t *testing.T) {
	// Ensure full build works with all field types
	meta := spec.Metadata{Name: "alltypes", Module: "test"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "str_field", Type: spec.FieldString, Index: true},
			{Name: "num_field", Type: spec.FieldNumber, Index: true},
			{Name: "bool_field", Type: spec.FieldBoolean, Index: true},
			{Name: "date_field", Type: spec.FieldDate},
			{Name: "dt_field", Type: spec.FieldDateTime},
			{Name: "json_field", Type: spec.FieldJSON},
			{Name: "uuid_field", Type: spec.FieldUUID, Index: true},
			{Name: "enum_field", Type: spec.FieldEnum, EnumValues: []string{"a", "b", "c"}},
		},
	}

	ti, err := GenerateEntityDDL(meta, entity, DriverSQLite)
	if err != nil {
		t.Fatalf("GenerateEntityDDL failed: %v", err)
	}

	if ti.CreateTableSQL == "" {
		t.Fatal("expected non-empty DDL")
	}
}

func TestGenerateEntityDDL_SoftDeleteDisabled(t *testing.T) {
	meta := spec.Metadata{Name: "log", Module: "system"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Persist: &spec.PersistSpec{SoftDelete: false},
		Fields: []spec.Field{
			{Name: "message", Type: spec.FieldString},
		},
	}

	ti, err := GenerateEntityDDL(meta, entity, DriverSQLite)
	if err != nil {
		t.Fatalf("GenerateEntityDDL failed: %v", err)
	}

	if strings.Contains(ti.CreateTableSQL, "deleted_at") {
		t.Error("expected no deleted_at column when soft_delete is false")
	}
}

func TestGenerateExtensionDDL(t *testing.T) {
	meta := spec.Metadata{Name: "custext", Module: "billing"}
	entity := &spec.EntitySpec{
		Version: "v1",
		Fields: []spec.Field{
			{Name: "membership_level", Type: spec.FieldString, Default: "regular"},
			{Name: "referral_code", Type: spec.FieldString, Unique: true},
			{Name: "lifetime_value", Type: spec.FieldNumber, Index: true},
		},
		ExtendStorage: &spec.ExtendStorage{
			Target:    "billing/customer",
			Namespace: "custext",
		},
	}

	info, err := GenerateExtensionDDL(meta, entity, DriverSQLite)
	if err != nil {
		t.Fatalf("GenerateExtensionDDL failed: %v", err)
	}

	if info.TargetTable != "billing_customers" {
		t.Errorf("expected target 'billing_customers', got %q", info.TargetTable)
	}
	if info.Namespace != "custext" {
		t.Errorf("expected namespace 'custext', got %q", info.Namespace)
	}
	if info.ExtensionTable != "ext_custext" {
		t.Errorf("expected extension column 'ext_custext', got %q", info.ExtensionTable)
	}

	// Verify ALTER TABLE ADD COLUMN
	if !strings.Contains(info.AlterTableSQL, "ALTER TABLE billing_customers") {
		t.Error("expected ALTER TABLE billing_customers")
	}
	if !strings.Contains(info.AlterTableSQL, "ADD COLUMN ext_custext") {
		t.Error("expected ADD COLUMN ext_custext")
	}

	// Verify generated column for unique field (referral_code)
	hasReferralCol := false
	for _, idx := range info.CreateIndexSQL {
		if strings.Contains(idx, "_referral_code") {
			hasReferralCol = true
			if !strings.Contains(idx, "ext_custext->>'") {
				t.Error("expected generated column to reference ext_custext")
			}
			break
		}
	}
	if !hasReferralCol {
		t.Error("expected generated column for referral_code")
	}

	// Verify unique index on referral_code
	hasUniqueIdx := false
	for _, idx := range info.CreateIndexSQL {
		if strings.Contains(idx, "idx_uq_ext_custext_referral_code") {
			hasUniqueIdx = true
			break
		}
	}
	if !hasUniqueIdx {
		t.Error("expected UNIQUE INDEX for referral_code in extension")
	}

	// Index on lifetime_value
	hasIndex := false
	for _, idx := range info.CreateIndexSQL {
		if strings.Contains(idx, "idx_ext_custext_lifetime_value") {
			hasIndex = true
			break
		}
	}
	if !hasIndex {
		t.Error("expected INDEX for lifetime_value in extension")
	}

	// Verify non-indexed field (membership_level) has no generated column
	for _, idx := range info.CreateIndexSQL {
		if strings.Contains(idx, "membership_level") {
			t.Error("non-indexed field should not have generated column")
		}
	}
}
