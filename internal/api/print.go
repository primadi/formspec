package api

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// HandlePrint returns a GET /_ui/print/{module}/{name}/{id}?format=pdf
// handler that renders a kind: Print document server-side (todo 5.13.2).
//
// The frontend PrintRenderer handles `format: html` via window.print(); this
// endpoint covers `format: pdf` — the same declarative Print manifest
// (header/body/footer) rendered to a PDF with the go-pdf/fpdf library, so a
// printable document can be generated without a browser.
func (b *RouterBuilder) HandlePrint() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		module := r.PathValue("module")
		name := r.PathValue("name")
		id := r.PathValue("id")
		if module == "" || name == "" || id == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST",
				"missing module, name, or id")
			return
		}

		if b.uiRegistry == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"UI registry not configured")
			return
		}

		// Resolve the Print manifest by name within the module.
		var printSpec *spec.PrintSpec
		for _, e := range b.uiRegistry.Prints {
			if e.Name == name && e.Module == module {
				printSpec = e.Spec
				break
			}
		}
		if printSpec == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				fmt.Sprintf("print %q not found in module %q", name, module))
			return
		}

		// Resolve the entity + load the record.
		entityModule, entityName := module, printSpec.Entity
		if i := strings.LastIndexByte(printSpec.Entity, '.'); i > 0 {
			entityModule, entityName = printSpec.Entity[:i], printSpec.Entity[i+1:]
		}
		store, err := b.registry.GetEntityStore(entityModule, entityName)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				fmt.Sprintf("entity %q not found", printSpec.Entity))
			return
		}
		workspaceID := workspaceFromContext(r.Context())
		rec, err := store.GetByID(r.Context(), db.GetByIDParams{
			WorkspaceID: workspaceID,
			ID:          id,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "record not found")
			return
		}

		pdf, err := renderPrintPDF(printSpec, rec.Data)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "PRINT_ERROR",
				fmt.Sprintf("pdf render: %v", err))
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf("attachment; filename=%q", name+".pdf"))
		w.Write(pdf)
	}
}

// renderPrintPDF renders a Print manifest + record to a PDF byte slice.
// Mirrors the frontend PrintRenderer's declarative layout: header title/
// subtitle, body fields (with dot-path resolution), and child_table rows.
func renderPrintPDF(ps *spec.PrintSpec, record map[string]any) ([]byte, error) {
	paper := "A4"
	if ps.Output != nil && ps.Output.Paper != nil && ps.Output.Paper.Size != "" {
		paper = ps.Output.Paper.Size
	}
	pdf := fpdf.New("P", "mm", paper, "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// ── Header ──
	if ps.Header != nil {
		if ps.Header.Title != "" {
			pdf.SetFont("Helvetica", "B", 16)
			pdf.CellFormat(0, 10, interpolatePrint(ps.Header.Title, record), "", 1, "L", false, 0, "")
		}
		if ps.Header.Subtitle != "" {
			pdf.SetFont("Helvetica", "", 11)
			pdf.SetTextColor(100, 100, 100)
			pdf.CellFormat(0, 7, interpolatePrint(ps.Header.Subtitle, record), "", 1, "L", false, 0, "")
		}
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(4)
	}

	// ── Body ──
	for _, item := range ps.Body {
		switch {
		case len(item.Fields) > 0:
			// Field list — label/value rows.
			pdf.SetFont("Helvetica", "", 10)
			for _, f := range item.Fields {
				label := strings.ReplaceAll(f, "_", " ")
				label = strings.ToUpper(label[:1]) + label[1:]
				value := resolvePrintPath(record, f)
				pdf.CellFormat(50, 7, label+":", "", 0, "L", false, 0, "")
				pdf.CellFormat(0, 7, value, "", 1, "L", false, 0, "")
			}
			pdf.Ln(2)
		case item.ChildTable != nil:
			// Child table — header row + rows.
			child := item.ChildTable
			rows, ok := record[child.Field].([]any)
			if !ok {
				if arr, ok2 := record[child.Field].([]map[string]any); ok2 {
					rows = make([]any, len(arr))
					for i, m := range arr {
						rows[i] = m
					}
				}
			}
			pdf.SetFont("Helvetica", "B", 10)
			for _, c := range child.Columns {
				pdf.CellFormat(0, 7, strings.ToUpper(c), "", 0, "L", false, 0, "")
			}
			pdf.Ln(7)
			pdf.SetFont("Helvetica", "", 10)
			for _, row := range rows {
				rm, ok := row.(map[string]any)
				if !ok {
					continue
				}
				for _, c := range child.Columns {
					pdf.CellFormat(0, 7, fmt.Sprintf("%v", rm[c]), "", 0, "L", false, 0, "")
				}
				pdf.Ln(7)
			}
			pdf.Ln(2)
		}
	}

	// ── Footer ──
	if ps.Footer != nil {
		pdf.SetY(-20)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 5, interpolatePrint(ps.Footer.Text, record), "", 1, "C", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// interpolatePrint replaces `{path}` tokens in a string with record values
// (dot-path resolved), mirroring the frontend's interpolate().
func interpolatePrint(tmpl string, record map[string]any) string {
	out := tmpl
	for {
		start := strings.Index(out, "{")
		if start < 0 {
			break
		}
		end := strings.Index(out[start:], "}")
		if end < 0 {
			break
		}
		path := out[start+1 : start+end]
		out = out[:start] + resolvePrintPath(record, path) + out[start+end+1:]
	}
	return out
}

// resolvePrintPath resolves a dot-path (e.g. "customer.name") against a
// record, returning "" for missing segments.
func resolvePrintPath(record map[string]any, path string) string {
	var cur any = record
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = m[part]
		if !ok {
			return ""
		}
	}
	if cur == nil {
		return ""
	}
	switch v := cur.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
