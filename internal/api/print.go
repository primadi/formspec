package api

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// printContext carries what printing a value needs beyond the record itself:
// the resolved settings namespace (currency symbol + locale for money) and the
// origin a QR payload is made absolute against.
type printContext struct {
	settings *spec.Settings
	origin   string
}

// requestOrigin rebuilds the origin the document is being printed from, so an
// `absolute` QR payload encodes a URL reachable by the phone scanning it.
// Forwarded headers are honoured because the server usually sits behind a
// proxy in production; `format: html` never reaches here (the browser uses its
// own origin).
func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

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

		// Branch on the declared output format (GAP-10 / item 7.1). Before this
		// the handler ALWAYS returned a PDF regardless of `output.format`, so a
		// `format: thermal` manifest validated green and produced a PDF named
		// .pdf — the receipt could not actually be printed to a thermal printer.
		format := "pdf"
		if printSpec.Output != nil && printSpec.Output.Format != "" {
			format = string(printSpec.Output.Format)
		}

		// Settings carry the currency symbol/locale used to format money values;
		// the origin is what an `absolute` QR payload is built from.
		pc := printContext{settings: b.settings, origin: requestOrigin(r)}

		switch format {
		case "thermal":
			bytes, err := renderPrintThermal(printSpec, rec.Data, pc)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "PRINT_ERROR",
					fmt.Sprintf("thermal render: %v", err))
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition",
				fmt.Sprintf("attachment; filename=%q", name+".escpos"))
			_, _ = w.Write(bytes)
		case "pdf", "":
			pdf, err := renderPrintPDF(printSpec, rec.Data, pc)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "PRINT_ERROR",
					fmt.Sprintf("pdf render: %v", err))
				return
			}
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition",
				fmt.Sprintf("attachment; filename=%q", name+".pdf"))
			_, _ = w.Write(pdf)
		default:
			// `html` is rendered client-side by PrintRenderer; `dotmatrix` is
			// not implemented. Rejecting here (rather than silently returning a
			// PDF) keeps the endpoint honest — the same principle as the
			// validator rejecting a format it cannot honour.
			writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
				fmt.Sprintf("print format %q is not served by this endpoint (supported: pdf, thermal; html is client-side)", format))
		}
	}
}

// renderPrintPDF renders a Print manifest + record to a PDF byte slice.
// Mirrors the frontend PrintRenderer's declarative layout: header title/
// subtitle, body fields (with dot-path resolution), child_table rows and QR
// codes, footer. Money values are formatted (§printContext) instead of being
// stringified as Go maps.
func renderPrintPDF(ps *spec.PrintSpec, record map[string]any, pc printContext) ([]byte, error) {
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
			pdf.CellFormat(0, 10, pc.interpolate(ps.Header.Title, record), "", 1, "L", false, 0, "")
		}
		if ps.Header.Subtitle != "" {
			pdf.SetFont("Helvetica", "", 11)
			pdf.SetTextColor(100, 100, 100)
			pdf.CellFormat(0, 7, pc.interpolate(ps.Header.Subtitle, record), "", 1, "L", false, 0, "")
		}
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(4)
	}

	// ── Body ──
	for idx, item := range ps.Body {
		switch {
		case len(item.Fields) > 0:
			// Field list — label/value rows.
			pdf.SetFont("Helvetica", "", 10)
			for _, f := range item.Fields {
				label := strings.ReplaceAll(f, "_", " ")
				label = strings.ToUpper(label[:1]) + label[1:]
				value := pc.path(record, f)
				pdf.CellFormat(50, 7, label+":", "", 0, "L", false, 0, "")
				pdf.CellFormat(0, 7, value, "", 1, "L", false, 0, "")
			}
			pdf.Ln(2)
		case item.Qrcode != nil:
			// QR code (gap #3 / S4). The payload is interpolated first, then
			// made absolute against the request origin when declared so. A
			// record with nothing to encode (a counter order has no guest
			// token) omits the element instead of printing a code that leads
			// nowhere.
			payload, complete := pc.interpolateStrict(item.Qrcode.Payload, record)
			if !complete {
				break
			}
			payload, err := item.Qrcode.QRPayload(payload, pc.origin)
			if err != nil {
				return nil, err
			}
			if payload == "" {
				break
			}
			png, err := qrcode.Encode(payload, qrcode.Medium, 256)
			if err != nil {
				return nil, fmt.Errorf("qrcode encode: %w", err)
			}
			if item.Qrcode.Label != "" {
				pdf.SetFont("Helvetica", "", 9)
				pdf.CellFormat(0, 6, item.Qrcode.Label, "", 1, "C", false, 0, "")
			}
			name := fmt.Sprintf("qr-%d", idx)
			pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))
			size := float64(item.Qrcode.QRSizeMM())
			wd, _ := pdf.GetPageSize()
			y := pdf.GetY()
			pdf.ImageOptions(name, (wd-size)/2, y, size, size, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
			pdf.SetY(y + size + 4)
		case item.ChildTable != nil:
			// Child table — header row + rows.
			child := item.ChildTable
			rows := childRows(record[child.Field])
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
					pdf.CellFormat(0, 7, formatPrintValue(rm[c], pc.settings), "", 0, "L", false, 0, "")
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
		pdf.CellFormat(0, 5, pc.interpolate(ps.Footer.Text, record), "", 1, "C", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// childRows normalizes a child collection to []any.
//
// A child collection reaches a renderer in one of two shapes depending on where
// the record came from: []any as the client sent it, or []map[string]any once
// ChildStore.Hydrate has read it back from its own table (`storage: table`).
// Both describe the same rows, so a layout that accepts only one of them
// silently renders nothing for the other — which is exactly what happened to
// thermal receipts (the PDF branch had a local []map[string]any fallback, the
// thermal branch did not, so every receipt with line items printed blank).
func childRows(v any) []any {
	switch rows := v.(type) {
	case []any:
		return rows
	case []map[string]any:
		out := make([]any, len(rows))
		for i, r := range rows {
			out[i] = r
		}
		return out
	default:
		return nil
	}
}

// ─── ESC/POS thermal rendering (GAP-10 / item 7.1) ───
//
// renderPrintThermal renders a Print manifest + record to an ESC/POS byte
// stream for a 58mm thermal receipt printer. It mirrors the declarative layout
// of renderPrintPDF (header title/subtitle, body fields/separator/child_table/
// totals, footer) but emits printer control codes instead of drawing a page.
//
// ESC/POS is a byte protocol, not a document format: the printer interprets
// control sequences inline with the text. The subset used here is the one every
// 58mm printer supports: initialize, alignment, emphasis, and a paper cut.

const (
	escInit      = "\x1b@"        // initialize printer
	escAlignLeft = "\x1b\x61\x00" // left align
	escAlignCtr  = "\x1b\x61\x01" // center align
	escBoldOn    = "\x1b\x45\x01" // emphasized on
	escBoldOff   = "\x1b\x45\x00" // emphasized off
	escCut       = "\x1d\x56\x00" // full cut
)

// thermalWidth is the character width of a 58mm receipt at the default font
// (32 columns is the common denominator across 58mm printers).
const thermalWidth = 32

// renderPrintThermal renders a Print manifest + record to an ESC/POS byte
// stream.
func renderPrintThermal(ps *spec.PrintSpec, record map[string]any, pc printContext) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(escInit)

	if ps.Header != nil {
		b.WriteString(escAlignCtr)
		if ps.Header.Title != "" {
			b.WriteString(escBoldOn)
			b.WriteString(pc.interpolate(ps.Header.Title, record))
			b.WriteString("\n")
			b.WriteString(escBoldOff)
		}
		if ps.Header.Subtitle != "" {
			b.WriteString(pc.interpolate(ps.Header.Subtitle, record))
			b.WriteString("\n")
		}
		b.WriteString(escAlignLeft)
	}

	for _, item := range ps.Body {
		switch {
		case item.Separator != "":
			b.WriteString(item.Separator)
			b.WriteString("\n")
		case len(item.Fields) > 0:
			for _, f := range item.Fields {
				label := titleCaseField(f)
				value := pc.path(record, f)
				b.WriteString(thermalLine(label, value))
			}
		case item.Qrcode != nil:
			// QR code (gap #3 / S4): ESC/POS "GS ( k" model 2, which 58mm
			// receipt printers implement natively — no raster bitmap to scale,
			// so the code stays sharp at any module size. Nothing to encode →
			// omit the element (see the PDF branch).
			payload, complete := pc.interpolateStrict(item.Qrcode.Payload, record)
			if !complete {
				break
			}
			payload, err := item.Qrcode.QRPayload(payload, pc.origin)
			if err != nil {
				return nil, err
			}
			if payload == "" {
				break
			}
			b.WriteString(escAlignCtr)
			if item.Qrcode.Label != "" {
				b.WriteString(item.Qrcode.Label)
				b.WriteString("\n")
			}
			b.Write(escposQR(payload, qrModuleSize(item.Qrcode.QRSizeMM())))
			b.WriteString("\n")
			b.WriteString(escAlignLeft)
		case item.ChildTable != nil:
			// Child table: one line per row. A real child collection reaches a
			// renderer as []map[string]any once ChildStore.Hydrate has read it
			// back from its own table, and as []any when it came straight from
			// the request — both describe the same rows (see childRows).
			for _, r := range childRows(record[item.ChildTable.Field]) {
				row, ok := r.(map[string]any)
				if !ok {
					continue
				}
				b.WriteString(thermalChildRow(item.ChildTable.Columns, row, pc.settings))
			}
		case item.Totals != nil:
			value := pc.path(record, item.Totals.Field)
			b.WriteString(escBoldOn)
			b.WriteString(thermalLine(titleCaseField(item.Totals.Field), value))
			b.WriteString(escBoldOff)
		}
	}

	if ps.Footer != nil && ps.Footer.Text != "" {
		b.WriteString(escAlignCtr)
		b.WriteString(pc.interpolate(ps.Footer.Text, record))
		b.WriteString("\n")
		b.WriteString(escAlignLeft)
	}

	// Feed a few lines then cut, so the receipt clears the print head.
	b.WriteString("\n\n\n")
	b.WriteString(escCut)
	return b.Bytes(), nil
}

// thermalLine renders a "label: value" line, truncating the label so the value
// stays on one line within the receipt width.
func thermalLine(label, value string) string {
	prefix := label + ": "
	if len(prefix) >= thermalWidth {
		return prefix + value + "\n"
	}
	return prefix + value + "\n"
}

// thermalChildRow renders one child-table row as a single receipt line: the
// first column is the item name, the rest are appended space-separated.
func thermalChildRow(columns []string, row map[string]any, settings *spec.Settings) string {
	if len(columns) == 0 {
		return ""
	}
	parts := make([]string, 0, len(columns))
	for _, c := range columns {
		parts = append(parts, formatPrintValue(row[c], settings))
	}
	return strings.Join(parts, " ") + "\n"
}

// escposQR builds the native QR sequence ("GS ( k", model 2, error correction
// M) for data. Native beats a raster image here: the command set is built into
// QR-capable ESC/POS printers, the symbol stays sharp at any module size, and
// nothing has to be scaled to the printer's dot width.
func escposQR(data string, moduleSize byte) []byte {
	var b bytes.Buffer
	// Select model 2.
	b.Write([]byte{0x1d, 0x28, 0x6b, 0x04, 0x00, 0x31, 0x41, 0x32, 0x00})
	// Module size (1–16 dots).
	b.Write([]byte{0x1d, 0x28, 0x6b, 0x03, 0x00, 0x31, 0x43, moduleSize})
	// Error correction level M (48=L, 49=M, 50=Q, 51=H).
	b.Write([]byte{0x1d, 0x28, 0x6b, 0x03, 0x00, 0x31, 0x45, 0x31})
	// Store the data (length = payload + 3 header bytes, little-endian).
	n := len(data) + 3
	b.Write([]byte{0x1d, 0x28, 0x6b, byte(n & 0xff), byte((n >> 8) & 0xff), 0x31, 0x50, 0x30})
	b.WriteString(data)
	// Print the stored symbol.
	b.Write([]byte{0x1d, 0x28, 0x6b, 0x03, 0x00, 0x31, 0x51, 0x30})
	return b.Bytes()
}

// qrModuleSize maps the declared millimetre size to an ESC/POS module size
// (dots per QR module). 58mm paper is ~48mm of printable width, so a QR must
// stay within ~1..8 dots per module to fit; the mapping keeps the declared size
// meaningful without pretending the thermal path has millimetre precision.
func qrModuleSize(sizeMM int) byte {
	switch {
	case sizeMM <= 20:
		return 3
	case sizeMM <= 30:
		return 4
	case sizeMM <= 40:
		return 6
	default:
		return 8
	}
}

// titleCaseField turns a snake_case field name into a Title Case label.
func titleCaseField(f string) string {
	parts := strings.Split(f, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// interpolate replaces `{path}` tokens in a string with record values
// (dot-path resolved), mirroring the frontend's interpolate().
func (pc printContext) interpolate(tmpl string, record map[string]any) string {
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
		out = out[:start] + pc.path(record, path) + out[start+end+1:]
	}
	return out
}

// path resolves a dot-path (e.g. "customer.name") against a record and renders
// the value for print, returning "" for missing segments.
func (pc printContext) path(record map[string]any, path string) string {
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
	return formatPrintValue(cur, pc.settings)
}

// interpolateStrict behaves like interpolate but also reports whether every
// `{path}` token resolved to a non-empty value.
//
// QR payloads use it: `interpolate` blanks a missing token, which would turn
// `/status/{guest_token}` into the plausible-looking `/status/` — a broken link
// printed as if it worked. A record that cannot fill a declared token means
// "nothing to encode", so the element is omitted instead.
func (pc printContext) interpolateStrict(tmpl string, record map[string]any) (string, bool) {
	out := tmpl
	resolved := true
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
		value := pc.path(record, path)
		if strings.TrimSpace(value) == "" {
			resolved = false
		}
		out = out[:start] + value + out[start+end+1:]
	}
	return out, resolved
}

// formatPrintValue renders one record value for print output. Money is the
// reason this exists: a money value is an object ({amount, currency}), and
// `%v` on it printed `map[amount:25000 currency:IDR]` on a receipt while the
// screen showed `Rp25.000`. Everything else falls back to scalar rendering.
func formatPrintValue(value any, settings *spec.Settings) string {
	if value == nil {
		return ""
	}
	if money := spec.FormatMoneyDisplay(value, settings); money != "" {
		return money
	}
	switch v := value.(type) {
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
