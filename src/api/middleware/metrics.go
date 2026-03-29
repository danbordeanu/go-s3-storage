package middleware

import (
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Define metric variables without initialization so we can register them later.
	httpRequestsTotal  *prometheus.CounterVec
	requestLatency     *prometheus.HistogramVec
	httpResponseSize   *prometheus.SummaryVec
	httpRequestSize    *prometheus.SummaryVec
	concurrentRequests *prometheus.GaugeVec
	methodCounts       *prometheus.CounterVec
	mu                 sync.Mutex // Mutex for safe concurrent access to shared metrics
)

// NewRegister creates and returns a new Prometheus registry with default Go and process collectors,
// as well as application-specific metrics for tracking HTTP requests, latency, response sizes, and concurrent requests.
// This registry can be used to expose metrics to Prometheus for monitoring and analysis.
func NewRegister() *prometheus.Registry {
	registry := prometheus.NewRegistry()

	// Register default Go and process collectors.
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	registerAppMetrics(registry)
	return registry
}

// registerAppMetrics initializes and registers application-specific Prometheus metrics for tracking HTTP requests, latency, response sizes, and concurrent requests.
// The metrics are labeled with "app_name" to allow for differentiation between multiple applications or services.
// This function is called during the setup of the Prometheus registry to ensure that all necessary metrics are registered and ready for use.
func registerAppMetrics(registry *prometheus.Registry) {
	// Initialize metrics with app_name as a label.

	// httpRequestsTotal counts the total number of HTTP requests.
	// This metric is labeled by HTTP method, endpoint, and status code.
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"app_name", "method", "endpoint", "status_code"},
	)

	// requestLatency records the latency distribution of HTTP requests in seconds.
	// It is labeled by HTTP method, endpoint, and status code.
	requestLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Request latency distribution in seconds",
			Buckets: prometheus.DefBuckets, // Default latency buckets, or customize as needed
		},
		[]string{"app_name", "method", "endpoint", "status_code"},
	)

	// httpResponseSize measures the size of HTTP responses in bytes.
	// It is labeled by HTTP method, endpoint, and status code.
	httpResponseSize = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_response_size_bytes",
			Help: "Size of HTTP responses in bytes",
		},
		[]string{"app_name", "method", "endpoint", "status_code"},
	)

	// httpRequestSize measures the size of HTTP requests in bytes.
	// It is labeled by HTTP method and endpoint.
	httpRequestSize = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_request_size_bytes",
			Help: "Size of HTTP requests in bytes",
		},
		[]string{"app_name", "method", "endpoint"},
	)

	// concurrentRequests tracks the current number of concurrent HTTP requests.
	// This metric is useful for understanding the load on the server.
	concurrentRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_concurrent_requests",
			Help: "Current number of concurrent requests",
		},
		[]string{"app_name"},
	)

	// methodCounts counts the number of HTTP requests made by each method.
	// This metric is labeled by the HTTP method.
	methodCounts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_request_method_counts",
			Help: "Count of HTTP requests by method",
		},
		[]string{"app_name", "method"},
	)
	registry.MustRegister(httpRequestsTotal, requestLatency, httpResponseSize, httpRequestSize, concurrentRequests, methodCounts)

}

// PrometheusMiddleware is a Gin middleware that tracks various HTTP request metrics,
// including total requests, request methods, concurrent requests, and request latency.
// It records the size of both incoming requests and outgoing responses.
// If used, this middleware facilitates performance monitoring and analysis
// of HTTP endpoints by utilizing Prometheus metrics.
//
// Returns a Gin middleware function that collects and observes metrics for each HTTP request.
func PrometheusMiddleware(appName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		mu.Lock() // Lock before modifying shared metrics
		methodCounts.WithLabelValues(appName, c.Request.Method).Inc()
		concurrentRequests.WithLabelValues(appName).Inc()
		mu.Unlock() // Unlock after modification

		defer func() {
			mu.Lock() // Lock before decrementing concurrent requests
			concurrentRequests.WithLabelValues(appName).Dec()
			mu.Unlock() // Unlock after modification
		}()

		// Start the timer
		timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
			// This will observe request duration after completion
			mu.Lock()
			requestLatency.WithLabelValues(appName, c.Request.Method, c.FullPath(), fmt.Sprintf("%d", c.Writer.Status())).Observe(v)
			mu.Unlock()
		}))

		// Process the request
		c.Next()

		// Record the response status code after processing
		statusCode := c.Writer.Status()
		// Record the response size
		responseSize := c.Writer.Size()
		// Record the size of the request body
		requestSize := c.Request.ContentLength

		// update counters
		mu.Lock()
		httpRequestsTotal.WithLabelValues(appName, c.Request.Method, c.FullPath(), fmt.Sprintf("%d", statusCode)).Inc()
		httpResponseSize.WithLabelValues(appName, c.Request.Method, c.FullPath(), fmt.Sprintf("%d", statusCode)).Observe(float64(responseSize))
		httpRequestSize.WithLabelValues(appName, c.Request.Method, c.FullPath()).Observe(float64(requestSize))
		mu.Unlock()

		// Stop the timer and observe the duration
		timer.ObserveDuration()
	}
}
