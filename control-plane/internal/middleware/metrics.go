// Package middleware provides HTTP middleware for the control plane.
package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP request metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "popsigner_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "popsigner_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Visitor metrics
	visitorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "popsigner_visitors_total",
			Help: "Total number of page visits (all requests)",
		},
	)

	visitorsUnique = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "popsigner_visitors_unique_total",
			Help: "Total number of unique visitors (by fingerprint)",
		},
	)

	activeVisitors = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "popsigner_visitors_active",
			Help: "Number of active visitors in the last 5 minutes",
		},
	)

	// Page view metrics
	pageViewsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "popsigner_page_views_total",
			Help: "Total page views by path",
		},
		[]string{"path"},
	)

	// API metrics
	apiCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "popsigner_api_calls_total",
			Help: "Total API calls by endpoint",
		},
		[]string{"method", "endpoint"},
	)

	signingOperationsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "popsigner_signing_operations_total",
			Help: "Total number of signing operations",
		},
	)

	keysCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "popsigner_keys_created_total",
			Help: "Total number of keys created",
		},
	)

	// Error metrics
	errorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "popsigner_errors_total",
			Help: "Total number of errors by type",
		},
		[]string{"type"},
	)
)

// VisitorTracker tracks unique visitors using an in-memory set with TTL.
// For production, this should use Redis for distributed tracking.
type VisitorTracker struct {
	seen    map[string]time.Time
	maxSize int
}

// NewVisitorTracker creates a new visitor tracker.
func NewVisitorTracker(maxSize int) *VisitorTracker {
	if maxSize <= 0 {
		maxSize = 100000 // Default 100k unique visitors
	}
	return &VisitorTracker{
		seen:    make(map[string]time.Time),
		maxSize: maxSize,
	}
}

// Track records a visitor and returns true if they're new.
func (vt *VisitorTracker) Track(fingerprint string) bool {
	now := time.Now()

	// Clean old entries (older than 24 hours)
	if len(vt.seen) > vt.maxSize/2 {
		cutoff := now.Add(-24 * time.Hour)
		for k, v := range vt.seen {
			if v.Before(cutoff) {
				delete(vt.seen, k)
			}
		}
	}

	if _, exists := vt.seen[fingerprint]; exists {
		return false
	}

	vt.seen[fingerprint] = now
	return true
}

// ActiveCount returns the number of visitors in the last duration.
func (vt *VisitorTracker) ActiveCount(d time.Duration) int {
	cutoff := time.Now().Add(-d)
	count := 0
	for _, t := range vt.seen {
		if t.After(cutoff) {
			count++
		}
	}
	return count
}

// Global visitor tracker instance
var visitorTracker = NewVisitorTracker(100000)

// Metrics returns a middleware that records Prometheus metrics.
func Metrics() func(next http.Handler) http.Handler {
	// Start background goroutine to update active visitors gauge
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			activeVisitors.Set(float64(visitorTracker.ActiveCount(5 * time.Minute)))
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &metricsResponseWriter{ResponseWriter: w, status: http.StatusOK}

			// Track visitor
			fingerprint := getVisitorFingerprint(r)
			visitorsTotal.Inc()

			if visitorTracker.Track(fingerprint) {
				visitorsUnique.Inc()
			}

			// Get normalized path for metrics (avoid cardinality explosion)
			path := normalizePath(r)

			// Track page views for web pages (not API or static)
			if isWebPage(r.URL.Path) {
				pageViewsTotal.WithLabelValues(path).Inc()
			}

			// Track API calls
			if strings.HasPrefix(r.URL.Path, "/v1/") {
				apiCallsTotal.WithLabelValues(r.Method, path).Inc()
			}

			// Execute handler
			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(wrapped.status)

			httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
			httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)

			// Track specific operations
			if r.Method == "POST" && strings.Contains(r.URL.Path, "/sign") && wrapped.status == http.StatusOK {
				signingOperationsTotal.Inc()
			}
			if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/keys") && wrapped.status == http.StatusCreated {
				keysCreatedTotal.Inc()
			}

			// Track errors
			if wrapped.status >= 400 {
				errorType := "client_error"
				if wrapped.status >= 500 {
					errorType = "server_error"
				}
				errorsTotal.WithLabelValues(errorType).Inc()
			}
		})
	}
}

type metricsResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *metricsResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

// getVisitorFingerprint creates a privacy-respecting fingerprint.
// Uses IP + User-Agent hash, not tracking cookies.
func getVisitorFingerprint(r *http.Request) string {
	// Get real IP (respects X-Forwarded-For from middleware)
	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = strings.Split(forwarded, ",")[0]
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		ip = realIP
	}

	// Create fingerprint from IP + User-Agent
	data := ip + "|" + r.UserAgent()
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // First 8 bytes is enough
}

// normalizePath normalizes URL paths to prevent cardinality explosion.
func normalizePath(r *http.Request) string {
	// Get route pattern from chi if available
	rctx := chi.RouteContext(r.Context())
	if rctx != nil && rctx.RoutePattern() != "" {
		return rctx.RoutePattern()
	}

	// Fallback: normalize common patterns
	path := r.URL.Path

	// Normalize UUID patterns
	// /v1/keys/550e8400-e29b-41d4-a716-446655440000 -> /v1/keys/{id}
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if len(seg) == 36 && strings.Count(seg, "-") == 4 {
			segments[i] = "{id}"
		}
		// ULID pattern (26 chars alphanumeric)
		if len(seg) == 26 && isAlphanumeric(seg) {
			segments[i] = "{id}"
		}
	}

	return strings.Join(segments, "/")
}

func isAlphanumeric(s string) bool {
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// isWebPage returns true if the path is a web page (not API or static).
func isWebPage(path string) bool {
	// Skip API routes
	if strings.HasPrefix(path, "/v1/") {
		return false
	}
	// Skip static files
	if strings.HasPrefix(path, "/static/") {
		return false
	}
	// Skip health checks
	if path == "/health" || path == "/ready" || path == "/metrics" {
		return false
	}
	// Skip OAuth callbacks (these are tracked separately)
	if strings.HasPrefix(path, "/auth/") {
		return false
	}
	return true
}

// IncrementSigningOps increments the signing operations counter.
// Call this from sign handlers for more accurate tracking.
func IncrementSigningOps() {
	signingOperationsTotal.Inc()
}

// IncrementKeysCreated increments the keys created counter.
// Call this from key handlers for more accurate tracking.
func IncrementKeysCreated() {
	keysCreatedTotal.Inc()
}


