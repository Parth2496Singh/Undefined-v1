package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"service", "method", "path", "status"},
	)
	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Histogram of response latency (seconds) of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "path"},
	)
	DeploymentsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "velzion_deployments_total",
			Help: "Total number of deployments triggered.",
		},
		[]string{"engine", "status"},
	)
	ActiveEnvironments = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "velzion_active_environments",
			Help: "Current number of active environments.",
		},
		[]string{"engine"},
	)
	DeploymentDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "velzion_deployment_duration_seconds",
			Help:    "Histogram of deployment durations (seconds).",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"engine"},
	)
)

func init() {
	prometheus.MustRegister(HttpRequestsTotal)
	prometheus.MustRegister(HttpRequestDuration)
	prometheus.MustRegister(DeploymentsTotal)
	prometheus.MustRegister(ActiveEnvironments)
	prometheus.MustRegister(DeploymentDuration)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func MetricsMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			recorder := &statusRecorder{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			next.ServeHTTP(recorder, r)

			duration := time.Since(start).Seconds()

			HttpRequestsTotal.WithLabelValues(serviceName, r.Method, r.URL.Path, strconv.Itoa(recorder.status)).Inc()
			HttpRequestDuration.WithLabelValues(serviceName, r.URL.Path).Observe(duration)
		})
	}
}
