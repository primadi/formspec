package formspec

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// doAuthedWithHeaders is doAuthed plus extra request headers — needed where a
// request only executes when a header is present (strict-mode `If-Match`) and
// an assertions-only test would otherwise pass without the write ever running.
func doAuthedWithHeaders(t *testing.T, app *App, method, path, token string, body any, headers map[string]string) (int, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out
}
