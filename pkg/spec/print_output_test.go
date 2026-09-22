package spec

import "testing"

// TestFormatMoneyDisplay pins the print-output money formatting added with item
// 2.6: a receipt must show `Rp62.500`, not `map[amount:62500 currency:IDR]`.
func TestFormatMoneyDisplay(t *testing.T) {
	zero := 0
	two := 2
	idr := &Settings{
		Locale:   "id-ID",
		Currency: &CurrencySettings{Code: "IDR", Symbol: "Rp", DecimalPlaces: &zero},
	}
	us := &Settings{
		Locale:   "en-US",
		Currency: &CurrencySettings{Code: "USD", Symbol: "$", DecimalPlaces: &two},
	}

	cases := []struct {
		name    string
		value   any
		sett    *Settings
		want    string
		wantNot string
	}{
		{name: "id grouping + symbol", value: map[string]any{"amount": "62500", "currency": "IDR"}, sett: idr, want: "Rp62.500"},
		{name: "id grouping millions", value: map[string]any{"amount": "1234567", "currency": "IDR"}, sett: idr, want: "Rp1.234.567"},
		{name: "decimals preserved", value: Money{Amount: "1250.50", Currency: "USD"}, sett: us, want: "$1,250.50"},
		{name: "negative sign first", value: map[string]any{"amount": "-12500", "currency": "IDR"}, sett: idr, want: "-Rp12.500"},
		{name: "foreign currency uses the code", value: map[string]any{"amount": "1000", "currency": "SGD"}, sett: idr, want: "SGD 1.000"},
		{name: "nil settings still formats", value: map[string]any{"amount": "1000", "currency": "SGD"}, sett: nil, want: "SGD 1,000"},
		{name: "no currency is not money", value: map[string]any{"amount": "1000"}, sett: idr, want: ""},
		{name: "plain number is not money", value: 1000, sett: idr, want: ""},
		{name: "non-numeric amount refused", value: map[string]any{"amount": "1.2.3", "currency": "IDR"}, sett: idr, want: ""},
		{name: "empty amount refused", value: map[string]any{"amount": "", "currency": "IDR"}, sett: idr, want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatMoneyDisplay(tc.value, tc.sett)
			if got != tc.want {
				t.Errorf("FormatMoneyDisplay(%v) = %q, want %q", tc.value, got, tc.want)
			}
			if tc.wantNot != "" && got == tc.wantNot {
				t.Errorf("got the refused rendering %q", got)
			}
		})
	}
}

// TestPrintQrcode_QRPayload pins the payload rules: interpolation happens
// before this call, `absolute` prepends the origin, an already-absolute URL is
// left alone, and an empty payload is an error rather than a blank code.
func TestPrintQrcode_QRPayload(t *testing.T) {
	cases := []struct {
		name    string
		qr      PrintQrcode
		payload string
		origin  string
		want    string
		wantErr bool
	}{
		{name: "relative, not absolute", qr: PrintQrcode{Payload: "/status/{t}"}, payload: "/status/abc", origin: "https://kafe.example", want: "/status/abc"},
		{name: "absolute prepends origin", qr: PrintQrcode{Payload: "/status/{t}", Absolute: true}, payload: "/status/abc", origin: "https://kafe.example", want: "https://kafe.example/status/abc"},
		{name: "missing leading slash added", qr: PrintQrcode{Payload: "status/{t}", Absolute: true}, payload: "status/abc", origin: "https://kafe.example/", want: "https://kafe.example/status/abc"},
		{name: "already absolute left alone", qr: PrintQrcode{Payload: "https://x/y", Absolute: true}, payload: "https://x/y", origin: "https://kafe.example", want: "https://x/y"},
		{name: "raw token payload", qr: PrintQrcode{Payload: "{qr_token}"}, payload: "TBL-9", origin: "", want: "TBL-9"},
		{name: "unresolved payload is empty, not an error", qr: PrintQrcode{Payload: "{missing}"}, payload: "", origin: "https://kafe.example", want: ""},
		{name: "leftover token is never encoded", qr: PrintQrcode{Payload: "/status/{t}", Absolute: true}, payload: "/status/{t}", origin: "https://kafe.example", want: ""},
		{name: "absolute without origin is an error", qr: PrintQrcode{Payload: "/x", Absolute: true}, payload: "/x", origin: "", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.qr.QRPayload(tc.payload, tc.origin)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got payload %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("QRPayload: %v", err)
			}
			if got != tc.want {
				t.Errorf("QRPayload = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPrintQrcode_QRSizeMM pins the size default (30mm) used by every pipeline.
func TestPrintQrcode_QRSizeMM(t *testing.T) {
	var nilQR *PrintQrcode
	if got := nilQR.QRSizeMM(); got != DefaultPrintQRSizeMM {
		t.Errorf("nil QR size = %d, want %d", got, DefaultPrintQRSizeMM)
	}
	if got := (&PrintQrcode{}).QRSizeMM(); got != DefaultPrintQRSizeMM {
		t.Errorf("unset QR size = %d, want %d", got, DefaultPrintQRSizeMM)
	}
	if got := (&PrintQrcode{SizeMM: 40}).QRSizeMM(); got != 40 {
		t.Errorf("declared QR size = %d, want 40", got)
	}
}
