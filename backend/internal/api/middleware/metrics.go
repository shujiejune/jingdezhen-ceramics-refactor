package middleware

import (
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics for the Fiber API. Exposed at GET /metrics (promhttp)
// and scraped by the Prometheus sidecar in docker-compose.prod.yml.
//
// Metrics:
//   - http_requests_total{status, method, route} — request counter
//   - http_request_duration_seconds{route}        — latency histogram (s)
//   - http_requests_in_flight                     — current in-flight gauge
//   - fiber_errors_total{type}                    — panics recovered
var (
	reqTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed.",
		},
		[]string{"status", "method", "route"},
	)
	reqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"route"},
	)
	reqInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of in-flight HTTP requests.",
		},
	)
	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fiber_errors_total",
			Help: "Total number of recovered panics.",
		},
		[]string{"type"},
	)
)

var registerOnce sync.Once

func registerMetrics() {
	registerOnce.Do(func() {
		prometheus.MustRegister(reqTotal, reqDuration, reqInFlight, errorsTotal)
	})
}

// PrometheusMiddleware records per-request metrics for Prometheus.
// It wraps the response writer to capture status code and timing.
func PrometheusMiddleware() fiber.Handler {
	registerMetrics()
	return func(c *fiber.Ctx) error {
		start := time.Now()

		reqInFlight.Inc()
		defer reqInFlight.Dec()

		// Process request
		err := c.Next()

		elapsed := time.Since(start).Seconds()
		status := strconv.Itoa(c.Response().StatusCode())
		route := c.Route().Path
		if route == "" {
			route = "unknown"
		}

		reqTotal.WithLabelValues(status, c.Method(), route).Inc()
		reqDuration.WithLabelValues(route).Observe(elapsed)

		return err
	}
}

// PrometheusHandler returns the Fiber handler that serves the /metrics
// endpoint. It bridges Fiber's fasthttp to net/http via a lightweight
// ResponseWriter shim so promhttp can write its text-formatted output.
func PrometheusHandler() fiber.Handler {
	registerMetrics()
	handler := promhttp.Handler()
	return func(c *fiber.Ctx) error {
		rw := &fiberResponseWriter{c: c}
		req := &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/metrics"},
			Header: http.Header{},
		}
		handler.ServeHTTP(rw, req)
		return nil
	}
}

// fiberResponseWriter adapts fiber.Ctx to net/http.ResponseWriter so
// promhttp.Handler() can write its text exposition format output.
type fiberResponseWriter struct {
	c       *fiber.Ctx
	status  int
	headers http.Header
}

func (w *fiberResponseWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = http.Header{}
	}
	return w.headers
}

func (w *fiberResponseWriter) Write(data []byte) (int, error) {
	return w.c.Write(data)
}

func (w *fiberResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.c.Status(statusCode)
}

// RecoverPanicCounter increments the panic-recovery counter. Call from a
// recover middleware or defer in handlers to track recovered panics.
func RecoverPanicCounter() {
	errorsTotal.WithLabelValues("panic").Inc()
}
