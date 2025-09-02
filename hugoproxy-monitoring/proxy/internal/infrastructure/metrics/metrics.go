package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP метрики
	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests.",
		Buckets: prometheus.DefBuckets,
	}, []string{"path", "method", "status_code"})

	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"path", "method", "status_code"})

	// Кэш метрики
	cacheRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cache_request_duration_seconds",
		Help:    "Duration of cache requests.",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 16), // от 0.1ms до 6.5s
	}, []string{"method", "cache_hit"})

	cacheRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_requests_total",
		Help: "Total number of cache requests.",
	}, []string{"method", "cache_hit"})

	// БД метрики
	dbRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "db_request_duration_seconds",
		Help:    "Duration of database requests.",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 16),
	}, []string{"method"})

	dbRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "db_requests_total",
		Help: "Total number of database requests.",
	}, []string{"method"})

	// Внешний API метрики
	externalAPIRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "external_api_request_duration_seconds",
		Help:    "Duration of external API requests.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 16), // от 1ms до 65s
	}, []string{"method"})

	externalAPIRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "external_api_requests_total",
		Help: "Total number of external API requests.",
	}, []string{"method"})
)

// ObserveHTTPRequest измеряет время HTTP запроса
func ObserveHTTPRequest(path, method, statusCode string, duration time.Duration) {
	httpRequestDuration.WithLabelValues(path, method, statusCode).Observe(duration.Seconds())
	httpRequestsTotal.WithLabelValues(path, method, statusCode).Inc()
}

// ObserveCacheRequest измеряет время запроса к кэшу
func ObserveCacheRequest(method string, hit bool, duration time.Duration) {
	hitStr := strconv.FormatBool(hit)
	cacheRequestDuration.WithLabelValues(method, hitStr).Observe(duration.Seconds())
	cacheRequestsTotal.WithLabelValues(method, hitStr).Inc()
}

// ObserveDBRequest измеряет время запроса к БД
func ObserveDBRequest(method string, duration time.Duration) {
	dbRequestDuration.WithLabelValues(method).Observe(duration.Seconds())
	dbRequestsTotal.WithLabelValues(method).Inc()
}

// ObserveExternalAPIRequest измеряет время запроса к внешнему API
func ObserveExternalAPIRequest(method string, duration time.Duration) {
	externalAPIRequestDuration.WithLabelValues(method).Observe(duration.Seconds())
	externalAPIRequestsTotal.WithLabelValues(method).Inc()
}
