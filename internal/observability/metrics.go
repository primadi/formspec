package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics implements the mandatory metric set (spec §3.1) with bounded
// cardinality labels (spec §3.2).
//
// Allowed label dimensions: route_class, method, status_class, module,
// action, error_code. Forbidden as labels (normative §3.2): entity instance
// ID, request_id, actor, raw URL path, business field values.
type Metrics struct {
	registry *prometheus.Registry

	HTTPRequestsTotal      *prometheus.CounterVec
	HTTPRequestErrorsTotal *prometheus.CounterVec
	HTTPRequestDuration    *prometheus.HistogramVec
	ActionDuration         *prometheus.HistogramVec
	ActionErrorsTotal      *prometheus.CounterVec
	OutboxPending          prometheus.Gauge
	OutboxLagSeconds       prometheus.Gauge
	WSConnections          prometheus.Gauge
	DBPoolOpen             prometheus.Gauge
	DBPoolIdle             prometheus.Gauge
	DBPoolWaitTotal        prometheus.Counter
	SnapshotAgeSeconds     prometheus.Gauge
}

// Route classes group routes by class, not per-record path (spec §3.2).
const (
	RouteClassEntityCRUD = "entity_crud"
	RouteClassAction     = "action_invoke"
	RouteClassAdmin      = "admin_panel"
	RouteClassWebsocket  = "websocket"
	RouteClassHealth     = "health"
	RouteClassOther      = "other"
)

// NewMetrics creates the metric set on a fresh registry (never the global
// default — the admin listener must not leak Go runtime metrics unless the
// operator opts in).
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{registry: reg}

	m.HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by route_class, method and status_class.",
	}, []string{"route_class", "method", "status_class"})
	m.HTTPRequestErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_request_errors_total",
		Help: "Failed HTTP requests by route_class and error_code.",
	}, []string{"route_class", "error_code"})
	m.HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency by route_class.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route_class"})
	m.ActionDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "action_duration_seconds",
		Help:    "Action execution latency by module and action.",
		Buckets: prometheus.DefBuckets,
	}, []string{"module", "action"})
	m.ActionErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "action_errors_total",
		Help: "Failed action executions by module, action and error_code.",
	}, []string{"module", "action", "error_code"})
	m.OutboxPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "outbox_pending",
		Help: "Number of outbox entries not yet flushed.",
	})
	m.OutboxLagSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "outbox_lag_seconds",
		Help: "Age of the oldest unflushed outbox entry.",
	})
	m.WSConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ws_connections",
		Help: "Active websocket connections.",
	})
	m.DBPoolOpen = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_pool_open",
		Help: "Open datastore pool connections.",
	})
	m.DBPoolIdle = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_pool_idle",
		Help: "Idle datastore pool connections.",
	})
	m.DBPoolWaitTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "db_pool_wait_total",
		Help: "Total waits for a pool connection.",
	})
	m.SnapshotAgeSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "snapshot_age_seconds",
		Help: "Age of the last Plane Protocol snapshot.",
	})

	reg.MustRegister(
		m.HTTPRequestsTotal, m.HTTPRequestErrorsTotal, m.HTTPRequestDuration,
		m.ActionDuration, m.ActionErrorsTotal, m.OutboxPending,
		m.OutboxLagSeconds, m.WSConnections, m.DBPoolOpen, m.DBPoolIdle,
		m.DBPoolWaitTotal, m.SnapshotAgeSeconds,
	)
	return m
}

// Registry returns the underlying Prometheus registry.
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

// Handler returns the /metrics HTTP handler (text exposition).
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// StatusClass maps an HTTP status code to its class (2xx/4xx/5xx — never
// the raw status, spec §3.2).
func StatusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	}
	return "other"
}

// ObserveRequest records one HTTP request. routeClass must be a bounded
// class constant; error_code may be empty.
func (m *Metrics) ObserveRequest(routeClass, method string, status int, seconds float64, errorCode string) {
	sc := StatusClass(status)
	m.HTTPRequestsTotal.WithLabelValues(routeClass, method, sc).Inc()
	m.HTTPRequestDuration.WithLabelValues(routeClass).Observe(seconds)
	if status >= 500 || (errorCode != "" && status >= 400) {
		ec := errorCode
		if ec == "" {
			ec = "HTTP_" + sc
		}
		m.HTTPRequestErrorsTotal.WithLabelValues(routeClass, ec).Inc()
	}
}

// ObserveAction records one action execution.
func (m *Metrics) ObserveAction(module, action string, seconds float64, errorCode string) {
	m.ActionDuration.WithLabelValues(module, action).Observe(seconds)
	if errorCode != "" {
		m.ActionErrorsTotal.WithLabelValues(module, action, errorCode).Inc()
	}
}
