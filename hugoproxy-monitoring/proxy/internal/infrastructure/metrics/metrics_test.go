package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// TestMetrics_Init проверяет создание метрик и их регистрацию
func TestMetrics_Init(t *testing.T) {
	tests := []struct {
		name     string
		metric   prometheus.Collector
		wantType string
	}{
		{
			name:     "http request duration histogram",
			metric:   httpRequestDuration,
			wantType: "Histogram",
		},
		{
			name:     "http requests total counter",
			metric:   httpRequestsTotal,
			wantType: "Counter",
		},
		{
			name:     "cache request duration histogram",
			metric:   cacheRequestDuration,
			wantType: "Histogram",
		},
		{
			name:     "cache requests total counter",
			metric:   cacheRequestsTotal,
			wantType: "Counter",
		},
		{
			name:     "db request duration histogram",
			metric:   dbRequestDuration,
			wantType: "Histogram",
		},
		{
			name:     "db requests total counter",
			metric:   dbRequestsTotal,
			wantType: "Counter",
		},
		{
			name:     "external API request duration histogram",
			metric:   externalAPIRequestDuration,
			wantType: "Histogram",
		},
		{
			name:     "external API requests total counter",
			metric:   externalAPIRequestsTotal,
			wantType: "Counter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.metric, "Metric should not be nil")

			// Проверяем тип метрики через reflect
			var isCorrectType bool
			switch tt.wantType {
			case "Histogram":
				_, isCorrectType = tt.metric.(*prometheus.HistogramVec)
			case "Counter":
				_, isCorrectType = tt.metric.(*prometheus.CounterVec)
			}
			assert.True(t, isCorrectType, "Metric should be of type %s", tt.wantType)
		})
	}
}

// TestObserveHTTPRequest проверяет функцию ObserveHTTPRequest
func TestObserveHTTPRequest(t *testing.T) {
	// Очищаем метрики перед тестом
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	path := "/api/test"
	method := "GET"
	statusCode := "200"
	duration := 100 * time.Millisecond

	// Вызываем функцию наблюдения
	ObserveHTTPRequest(path, method, statusCode, duration)

	// Проверяем, что счётчик увеличился
	counter := httpRequestsTotal.WithLabelValues(path, method, statusCode)
	counterValue := testutil.ToFloat64(counter)
	assert.Equal(t, float64(1), counterValue, "Counter should be incremented")
}

// TestObserveHTTPRequest_MultipleRequests проверяет несколько запросов
func TestObserveHTTPRequest_MultipleRequests(t *testing.T) {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	// Выполняем несколько запросов
	for i := 0; i < 5; i++ {
		ObserveHTTPRequest("/api/test", "GET", "200", 50*time.Millisecond)
	}

	counter := httpRequestsTotal.WithLabelValues("/api/test", "GET", "200")
	counterValue := testutil.ToFloat64(counter)
	assert.Equal(t, float64(5), counterValue, "Counter should be 5")
}

// TestObserveCacheRequest проверяет функцию ObserveCacheRequest
func TestObserveCacheRequest(t *testing.T) {
	cacheRequestsTotal.Reset()
	cacheRequestDuration.Reset()

	method := "GET"
	hit := true
	duration := 10 * time.Millisecond

	ObserveCacheRequest(method, hit, duration)

	counter := cacheRequestsTotal.WithLabelValues(method, "true")
	counterValue := testutil.ToFloat64(counter)
	assert.Equal(t, float64(1), counterValue, "Cache counter should be incremented")
}

// TestObserveCacheRequest_Miss проверяетcache miss
func TestObserveCacheRequest_Miss(t *testing.T) {
	cacheRequestsTotal.Reset()
	cacheRequestDuration.Reset()

	method := "SET"
	hit := false
	duration := 5 * time.Millisecond

	ObserveCacheRequest(method, hit, duration)

	counter := cacheRequestsTotal.WithLabelValues(method, "false")
	counterValue := testutil.ToFloat64(counter)
	assert.Equal(t, float64(1), counterValue, "Cache miss counter should be incremented")
}

// TestObserveDBRequest проверяет функцию ObserveDBRequest
func TestObserveDBRequest(t *testing.T) {
	dbRequestsTotal.Reset()
	dbRequestDuration.Reset()

	method := "SELECT"
	duration := 20 * time.Millisecond

	ObserveDBRequest(method, duration)

	counter := dbRequestsTotal.WithLabelValues(method)
	counterValue := testutil.ToFloat64(counter)
	assert.Equal(t, float64(1), counterValue, "DB counter should be incremented")
}

// TestObserveExternalAPIRequest проверяет функцию ObserveExternalAPIRequest
func TestObserveExternalAPIRequest(t *testing.T) {
	externalAPIRequestsTotal.Reset()
	externalAPIRequestDuration.Reset()

	method := "POST"
	duration := 500 * time.Millisecond

	ObserveExternalAPIRequest(method, duration)

	counter := externalAPIRequestsTotal.WithLabelValues(method)
	counterValue := testutil.ToFloat64(counter)
	assert.Equal(t, float64(1), counterValue, "External API counter should be incremented")
}

// TestHTTPMetricsMiddleware проверяет middleware для записи метрик
func TestHTTPMetricsMiddleware(t *testing.T) {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	// Создаём тестовый handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Применяем middleware
	middleware := HTTPMetricsMiddleware(testHandler)

	// Выполняем запрос
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	// Проверяем, что запрос прошёл успешно
	assert.Equal(t, http.StatusOK, rr.Code, "Request should succeed")

	// Проверяем, что метрики записаны
	counter := httpRequestsTotal.WithLabelValues("/api/test", "GET", "200")
	counterValue := testutil.ToFloat64(counter)
	assert.Equal(t, float64(1), counterValue, "HTTP requests counter should be incremented")
}

// TestHTTPMetricsMiddleware_MultipleRequests проверяет несколько запросов через middleware
func TestHTTPMetricsMiddleware_MultipleRequests(t *testing.T) {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	// Создаём тестовый handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPMetricsMiddleware(testHandler)

	// Выполняем несколько запросов
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
	}

	counter := httpRequestsTotal.WithLabelValues("/api/test", "GET", "200")
	counterValue := testutil.ToFloat64(counter)
	assert.Equal(t, float64(3), counterValue, "Should have 3 requests")
}

// TestHTTPMetricsMiddleware_DifferentPaths проверяет метрики для разных путей
func TestHTTPMetricsMiddleware_DifferentPaths(t *testing.T) {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPMetricsMiddleware(testHandler)

	// Запрос на /api/users
	req1 := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rr1 := httptest.NewRecorder()
	middleware.ServeHTTP(rr1, req1)

	// Запрос на /api/posts
	req2 := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)

	// Проверяем метрики для первого пути
	counter1 := httpRequestsTotal.WithLabelValues("/api/users", "GET", "200")
	assert.Equal(t, float64(1), testutil.ToFloat64(counter1), "Should have 1 request to /api/users")

	// Проверяем метрики для второго пути
	counter2 := httpRequestsTotal.WithLabelValues("/api/posts", "GET", "200")
	assert.Equal(t, float64(1), testutil.ToFloat64(counter2), "Should have 1 request to /api/posts")
}

// TestHTTPMetricsMiddleware_DifferentMethods проверяет метрики для разных HTTP методов
func TestHTTPMetricsMiddleware_DifferentMethods(t *testing.T) {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPMetricsMiddleware(testHandler)

	// GET запрос
	req1 := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr1 := httptest.NewRecorder()
	middleware.ServeHTTP(rr1, req1)

	// POST запрос
	req2 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)

	// Проверяем GET счётчик
	counterGET := httpRequestsTotal.WithLabelValues("/api/test", "GET", "200")
	assert.Equal(t, float64(1), testutil.ToFloat64(counterGET), "Should have 1 GET request")

	// Проверяем POST счётчик
	counterPOST := httpRequestsTotal.WithLabelValues("/api/test", "POST", "200")
	assert.Equal(t, float64(1), testutil.ToFloat64(counterPOST), "Should have 1 POST request")
}

// TestHTTPMetricsMiddleware_DifferentStatusCodes проверяет метрики для разных статус кодов
func TestHTTPMetricsMiddleware_DifferentStatusCodes(t *testing.T) {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Возвращаем 404
		w.WriteHeader(http.StatusNotFound)
	})

	middleware := HTTPMetricsMiddleware(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/notfound", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	// Проверяем, что записан статус 404
	counter := httpRequestsTotal.WithLabelValues("/api/notfound", "GET", "404")
	assert.Equal(t, float64(1), testutil.ToFloat64(counter), "Should record 404 status code")
}

// TestHTTPMetricsMiddleware_PanicRecovery проверяет восстановление после паники
func TestHTTPMetricsMiddleware_PanicRecovery(t *testing.T) {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	// Handler который паникует
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	middleware := HTTPMetricsMiddleware(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/panic", nil)
	rr := httptest.NewRecorder()

	// Middleware должен восстановиться после паники
	assert.NotPanics(t, func() {
		middleware.ServeHTTP(rr, req)
	})

	// После паники должен быть записан статус 500
	counter := httpRequestsTotal.WithLabelValues("/api/panic", "GET", "500")
	assert.Equal(t, float64(1), testutil.ToFloat64(counter), "Should record 500 status after panic")
}

// TestMetrics_Export проверяет экспорт метрик в формате Prometheus
func TestMetrics_Export(t *testing.T) {
	// Создаём тестовые метрики
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	ObserveHTTPRequest("/api/test", "GET", "200", 50*time.Millisecond)

	// Создаём HTTP сервер с /metrics endpoint
	registry := prometheus.NewRegistry()
	registry.MustRegister(httpRequestsTotal)
	registry.MustRegister(httpRequestDuration)

	// Создаём handler для /metrics
	metricsHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	// Выполняем запрос к /metrics
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	metricsHandler.ServeHTTP(rr, req)

	// Проверяем статус код
	assert.Equal(t, http.StatusOK, rr.Code, "Metrics endpoint should return 200")

	// Проверяем Content-Type
	assert.Equal(t, "text/plain; version=0.0.0; charset=utf-8", rr.Header().Get("Content-Type"))

	// Проверяем, что тело содержит метрики
	body, _ := io.ReadAll(rr.Body)
	bodyStr := string(body)

	// Проверяем наличие метрик http_requests_total
	assert.True(t, strings.Contains(bodyStr, "http_requests_total"), "Should contain http_requests_total")

	// Проверяем наличие метрик http_request_duration_seconds
	assert.True(t, strings.Contains(bodyStr, "http_request_duration_seconds"), "Should contain http_request_duration_seconds")

	// Проверяем конкретные значения
	assert.True(t, strings.Contains(bodyStr, `http_requests_total{method="GET",path="/api/test",status_code="200"}`), "Should contain specific metric labels")
}

// TestMetrics_Export_EmptyMetrics проверяет экспорт пустых метрик
func TestMetrics_Export_EmptyMetrics(t *testing.T) {
	// Очищаем метрики
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	registry := prometheus.NewRegistry()
	registry.MustRegister(httpRequestsTotal)
	registry.MustRegister(httpRequestDuration)

	metricsHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	metricsHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Should return 200 even with empty metrics")
}

// TestMetrics_Labels проверяет правильность меток (labels)
func TestMetrics_Labels(t *testing.T) {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	path := "/api/users/123"
	method := "POST"
	statusCode := "201"
	duration := 75 * time.Millisecond

	ObserveHTTPRequest(path, method, statusCode, duration)

	// Проверяем, что метки установлены правильно
	counter := httpRequestsTotal.WithLabelValues(path, method, statusCode)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter), "Counter should have correct labels")
}

// TestCacheMetrics_CacheHitAndMiss проверяет метрики кэша hit/miss
func TestCacheMetrics_CacheHitAndMiss(t *testing.T) {
	cacheRequestsTotal.Reset()
	cacheRequestDuration.Reset()

	// Cache hit
	ObserveCacheRequest("GET", true, 5*time.Millisecond)

	// Cache miss
	ObserveCacheRequest("GET", false, 10*time.Millisecond)

	// Проверяем cache hit counter
	hitCounter := cacheRequestsTotal.WithLabelValues("GET", "true")
	assert.Equal(t, float64(1), testutil.ToFloat64(hitCounter), "Should have 1 cache hit")

	// Проверяем cache miss counter
	missCounter := cacheRequestsTotal.WithLabelValues("GET", "false")
	assert.Equal(t, float64(1), testutil.ToFloat64(missCounter), "Should have 1 cache miss")
}
