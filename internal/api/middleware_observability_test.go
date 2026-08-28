package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/primadi/formspec/internal/observability"
)

// TestCORSMiddleware_AllowList proves the production CORS contract
// (todo 8.1.5): only allow-listed origins get the header; `*` only when
// explicitly configured (dev).
func TestCORSMiddleware_AllowList(t *testing.T) {
	handler := NewCORSMiddleware([]string{"https://app.example.com", "https://admin.example.com"})(http.NotFoundHandler())

	// Allowed origin → echoed.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("allowed origin → ACAO = %q", got)
	}

	// Disallowed origin → no CORS header.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin → ACAO = %q, want empty", got)
	}

	// Dev default (nil list) → permissive *.
	dev := NewCORSMiddleware(nil)(http.NotFoundHandler())
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://anything.example.com")
	dev.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("dev default → ACAO = %q, want *", got)
	}
}

// TestRequestIDMiddleware_Upstream proves an upstream X-Request-ID is
// forwarded, not regenerated (todo 8.2.3, spec §2.3).
func TestRequestIDMiddleware_Upstream(t *testing.T) {
	var captured string
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = observability.RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Request-ID", "upstream-123")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if captured != "upstream-123" {
		t.Errorf("upstream request id not forwarded: %q", captured)
	}

	// No upstream header → generated.
	req = httptest.NewRequest("GET", "/x", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if captured == "" || captured == "upstream-123" {
		t.Errorf("expected a generated request id, got %q", captured)
	}
}

// TestClassifyRoute proves route classes are bounded (spec §3.2) —
// per-record paths collapse into the same class.
func TestClassifyRoute(t *testing.T) {
	cases := []struct{ path, want string }{
		{"/health", observability.RouteClassHealth},
		{"/default/_ui/entity/invoice", observability.RouteClassAdmin},
		{"/default/api/v1/billing/invoices", observability.RouteClassEntityCRUD},
		{"/default/api/v1/billing/invoices/actions/submit", observability.RouteClassAction},
		{"/default/_ui/_ws", observability.RouteClassWebsocket},
		{"/unknown", observability.RouteClassOther},
	}
	for _, c := range cases {
		if got := ClassifyRoute(c.path); got != c.want {
			t.Errorf("ClassifyRoute(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestMetricsMiddleware proves request instrumentation records bounded
// labels (todo 8.2.4/8.2.5).
func TestMetricsMiddleware(t *testing.T) {
	m := observability.NewMetrics()
	handler := MetricsMiddleware(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}))

	req := httptest.NewRequest("POST", "/default/api/v1/billing/invoices", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	found := false
	for _, mf := range families {
		if mf.GetName() != "http_requests_total" {
			continue
		}
		found = true
		for _, ms := range mf.GetMetric() {
			labels := map[string]string{}
			for _, lp := range ms.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			if labels["method"] != "POST" || labels["status_class"] != "2xx" {
				t.Errorf("unexpected labels: %v", labels)
			}
			if labels["route_class"] != observability.RouteClassEntityCRUD {
				t.Errorf("route_class = %q, want entity_crud", labels["route_class"])
			}
			// Cardinality discipline: raw path must never be a label value.
			for _, v := range labels {
				if v == req.URL.Path {
					t.Errorf("raw URL path leaked as label value: %q", v)
				}
			}
		}
	}
	if !found {
		t.Fatal("http_requests_total not registered")
	}
}
