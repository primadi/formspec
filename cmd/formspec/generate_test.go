package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func writeSpecFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "entities"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "entities", name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateTypeScript_NoExposedEntities(t *testing.T) {
	dir := t.TempDir()
	writeSpecFile(t, dir, "widget.yaml", `
apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: widget, module: inventory }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: sku, type: string, rules: [required] }
`)
	reg, err := loadRegistryForCodegen(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_, err = generateTypeScript(reg)
	if err == nil || !strings.Contains(err.Error(), "no exposed entities") {
		t.Fatalf("expected 'no exposed entities' error, got %v", err)
	}
}

func TestGenerateTypeScript_FullEntity(t *testing.T) {
	dir := t.TempDir()
	writeSpecFile(t, dir, "invoice.yaml", `
apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: invoice, module: billing }
spec:
  version: v1
  characteristic: master
  expose:
    - { type: rest, actions: [list, find, create, update, delete] }
  fields:
    - { name: customer_id, type: relation, relation: { type: belongs_to, resource: billing.customer }, rules: [required] }
    - { name: status, type: enum, enum_values: [draft, submitted], rules: [required] }
    - { name: total, type: decimal, rules: [required] }
    - { name: note, type: string }
    - name: items
      type: child
      child:
        storage: jsonb
        fields:
          - { name: sku, type: string, rules: [required] }
          - { name: qty, type: integer }
  actions:
    - name: approve
      required_permission: billing.invoices.approve
      params:
        validate:
          - { field: note, rules: [] }
      impl: { type: script_ref, ref: billing/approve }
`)
	reg, err := loadRegistryForCodegen(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ts, err := generateTypeScript(reg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	checks := []string{
		`import { FormaClient, FormaRecord, ListOptions, ListResult } from "@formspec/client";`,
		`export interface BillingInvoice {`,
		`"customer_id": string;`,                                             // relation -> string, quoted literal wire key
		`"status": "draft" | "submitted";`,                                   // enum union
		`"total": string;`,                                                   // decimal -> string, never number
		`"note"?: string | null;`,                                            // optional non-required field
		`"items"?: Array<{ "sku": string; "qty"?: number | null; }> | null;`, // child inline type, literal keys
		`export interface BillingInvoiceCreateInput {`,
		`export type BillingInvoiceUpdateInput = Partial<BillingInvoiceCreateInput>;`,
		`export interface BillingInvoiceApproveParams {`,
		`"note"?: unknown;`,
		`export function createApi(client: FormaClient) {`,
		`billing: {`,
		`invoices: {`,
		`list: (opts?: ListOptions): Promise<ListResult<FormaRecord<BillingInvoice>>> =>`,
		`client.list<FormaRecord<BillingInvoice>>("billing", "invoices", opts),`,
		`delete: (id: string): Promise<void> => client.delete("billing", "invoices", id),`,
		`approve: (id: string, params: BillingInvoiceApproveParams): Promise<unknown> =>`,
		`client.action("billing", "invoices", id, "approve", params),`,
	}
	for _, want := range checks {
		if !strings.Contains(ts, want) {
			t.Errorf("generated output missing %q\n--- full output ---\n%s", want, ts)
		}
	}

	// Field keys must never be camelCased — they must match the literal
	// wire JSON key exactly (renderers/jsonb-persist EntityRecord.MarshalJSON spreads
	// Data verbatim, no case transform).
	if strings.Contains(ts, "customerId") {
		t.Error("field name was camelCased — must stay literal to match the wire format")
	}
}

func TestFieldIsRequired(t *testing.T) {
	cases := []struct {
		name string
		f    spec.Field
		want bool
	}{
		{"top-level required", spec.Field{Required: true}, true},
		{"rules required", spec.Field{Rules: []spec.ValidationRule{{Name: "required"}}}, true},
		{"neither", spec.Field{Rules: []spec.ValidationRule{{Name: "email"}}}, false},
		{"empty", spec.Field{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fieldIsRequired(tc.f); got != tc.want {
				t.Errorf("fieldIsRequired(%+v) = %v, want %v", tc.f, got, tc.want)
			}
		})
	}
}

// TestTsFieldType_MoneyAndFile verifies the money and file/attachment field
// types map to their wire representation (todo 3.3.3): money amount is a
// string (arbitrary precision, never number), and file is a ctx.storage
// reference with canonical metadata.
func TestTsFieldType_MoneyAndFile(t *testing.T) {
	cases := []struct {
		name string
		f    spec.Field
		want string
	}{
		{"money", spec.Field{Type: spec.FieldMoney}, "{ amount: string; currency: string }"},
		{"file", spec.Field{Type: spec.FieldFile}, "{ key: string; filename: string; content_type: string; size: number; checksum: string }"},
		{"attachment alias", spec.Field{Type: spec.FieldAttachment}, "{ key: string; filename: string; content_type: string; size: number; checksum: string }"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tsFieldType(tc.f); got != tc.want {
				t.Errorf("tsFieldType(%s) = %q, want %q", tc.f.Type, got, tc.want)
			}
		})
	}
}

func TestPascalCase(t *testing.T) {
	cases := map[string]string{
		"invoice":        "Invoice",
		"general-ledger": "GeneralLedger",
		"gl_balance":     "GlBalance",
		"stock-movement": "StockMovement",
	}
	for in, want := range cases {
		if got := pascalCase(in); got != want {
			t.Errorf("pascalCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTsIdent(t *testing.T) {
	cases := map[string]string{
		"invoice":        "invoice",
		"general-ledger": "generalLedger",
		"stock_movement": "stockMovement",
	}
	for in, want := range cases {
		if got := tsIdent(in); got != want {
			t.Errorf("tsIdent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTsFieldType_DecimalNeverNumber(t *testing.T) {
	if got := tsFieldType(spec.Field{Type: spec.FieldDecimal}); got != "string" {
		t.Errorf("decimal -> %q, want string (money must never be a JS number)", got)
	}
	if got := tsFieldType(spec.Field{Type: spec.FieldNumber}); got != "string" {
		t.Errorf("number (deprecated decimal alias) -> %q, want string", got)
	}
}

func TestTsFieldType_EnumEmptyValuesFallsBackToString(t *testing.T) {
	if got := tsFieldType(spec.Field{Type: spec.FieldEnum}); got != "string" {
		t.Errorf("enum with no values -> %q, want string fallback", got)
	}
}
