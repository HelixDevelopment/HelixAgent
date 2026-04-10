package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestHTTPMetrics creates an HTTPMetrics with an isolated registry
// so parallel tests do not conflict.
func newTestHTTPMetrics(t *testing.T) (*HTTPMetrics, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	m := NewHTTPMetricsWithRegistry(reg)
	return m, reg
}

// setupTestRouter builds a minimal Gin engine wired with the metrics
// middleware and a single test route.
func setupTestRouter(
	m *HTTPMetrics, method, path string, status int, body string,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(m.Middleware())
	handler := func(c *gin.Context) {
		c.String(status, body)
	}
	switch method {
	case http.MethodGet:
		r.GET(path, handler)
	case http.MethodPost:
		r.POST(path, handler)
	case http.MethodPut:
		r.PUT(path, handler)
	case http.MethodDelete:
		r.DELETE(path, handler)
	default:
		r.GET(path, handler)
	}
	return r
}

// gatherCounter collects the named counter from the registry and returns
// the sample value for the given label set.
func gatherCounter(
	t *testing.T, reg *prometheus.Registry, name string, labels map[string]string,
) float64 {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, fam := range families {
		if fam.GetName() != name {
			continue
		}
		for _, m := range fam.GetMetric() {
			if matchLabels(m.GetLabel(), labels) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// gatherHistogramCount collects the named histogram from the registry and
// returns the sample count for the given label set.
func gatherHistogramCount(
	t *testing.T, reg *prometheus.Registry, name string, labels map[string]string,
) uint64 {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, fam := range families {
		if fam.GetName() != name {
			continue
		}
		for _, m := range fam.GetMetric() {
			if matchLabels(m.GetLabel(), labels) {
				return m.GetHistogram().GetSampleCount()
			}
		}
	}
	return 0
}

func matchLabels(
	pairs []*io_prometheus_client.LabelPair, expected map[string]string,
) bool {
	if len(pairs) != len(expected) {
		return false
	}
	for _, lp := range pairs {
		v, ok := expected[lp.GetName()]
		if !ok || v != lp.GetValue() {
			return false
		}
	}
	return true
}

func TestHTTPMetrics_Middleware_RecordsDuration(t *testing.T) {
	t.Parallel()
	m, reg := newTestHTTPMetrics(t)
	r := setupTestRouter(m, http.MethodGet, "/v1/health", http.StatusOK, "ok")

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	labels := map[string]string{
		"method": "GET", "path": "/v1/health", "status_code": "200",
	}
	count := gatherHistogramCount(t, reg, "helixagent_http_request_duration_seconds", labels)
	assert.Equal(t, uint64(1), count, "expected one observation in the histogram")
}

func TestHTTPMetrics_Middleware_IncrementsTotal(t *testing.T) {
	t.Parallel()
	m, reg := newTestHTTPMetrics(t)
	r := setupTestRouter(m, http.MethodPost, "/v1/debate", http.StatusCreated, "created")

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/debate", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	}

	labels := map[string]string{
		"method": "POST", "path": "/v1/debate", "status_code": "201",
	}
	total := gatherCounter(t, reg, "helixagent_http_requests_total", labels)
	assert.Equal(t, float64(3), total)
}

func TestHTTPMetrics_Middleware_TracksErrors(t *testing.T) {
	t.Parallel()
	m, reg := newTestHTTPMetrics(t)
	r := setupTestRouter(
		m, http.MethodGet, "/v1/failing",
		http.StatusInternalServerError, "error",
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/failing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	errLabels := map[string]string{"method": "GET", "path": "/v1/failing"}
	errCount := gatherCounter(t, reg, "helixagent_http_request_errors_total", errLabels)
	assert.Equal(t, float64(1), errCount)
}

func TestHTTPMetrics_Middleware_NoErrorFor4xx(t *testing.T) {
	t.Parallel()
	m, reg := newTestHTTPMetrics(t)
	r := setupTestRouter(
		m, http.MethodGet, "/v1/notfound",
		http.StatusNotFound, "not found",
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/notfound", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	errLabels := map[string]string{"method": "GET", "path": "/v1/notfound"}
	errCount := gatherCounter(t, reg, "helixagent_http_request_errors_total", errLabels)
	assert.Equal(t, float64(0), errCount, "4xx should not count as server error")
}

func TestHTTPMetrics_Middleware_UnmatchedRouteUsesUnknown(t *testing.T) {
	t.Parallel()
	m, reg := newTestHTTPMetrics(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(m.Middleware())
	// Register a route that will NOT match
	r.GET("/v1/exists", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/v1/does-not-exist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Gin returns 404 for unmatched routes
	assert.Equal(t, http.StatusNotFound, w.Code)

	labels := map[string]string{
		"method": "GET", "path": "unknown", "status_code": "404",
	}
	total := gatherCounter(t, reg, "helixagent_http_requests_total", labels)
	assert.Equal(t, float64(1), total)
}

func TestHTTPMetrics_Middleware_PathParam(t *testing.T) {
	t.Parallel()
	m, reg := newTestHTTPMetrics(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/v1/users/:id", func(c *gin.Context) {
		c.String(http.StatusOK, c.Param("id"))
	})

	// Two requests with different path params should collapse to the pattern
	for _, id := range []string{"123", "456"} {
		req := httptest.NewRequest(http.MethodGet, "/v1/users/"+id, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	labels := map[string]string{
		"method": "GET", "path": "/v1/users/:id", "status_code": "200",
	}
	total := gatherCounter(t, reg, "helixagent_http_requests_total", labels)
	assert.Equal(t, float64(2), total,
		"both requests should share the same route pattern label")
}

func TestHTTPMetrics_Middleware_MultipleStatuses(t *testing.T) {
	t.Parallel()
	m, reg := newTestHTTPMetrics(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/v1/status", func(c *gin.Context) {
		code := 200
		if q := c.Query("code"); q != "" {
			switch q {
			case "500":
				code = 500
			case "400":
				code = 400
			}
		}
		c.String(code, "response")
	})

	codes := []string{"200", "400", "500"}
	for _, code := range codes {
		req := httptest.NewRequest(http.MethodGet, "/v1/status?code="+code, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}

	for _, code := range codes {
		labels := map[string]string{
			"method": "GET", "path": "/v1/status", "status_code": code,
		}
		total := gatherCounter(t, reg, "helixagent_http_requests_total", labels)
		assert.Equal(t, float64(1), total, "expected 1 request with status %s", code)
	}

	errLabels := map[string]string{"method": "GET", "path": "/v1/status"}
	errCount := gatherCounter(t, reg, "helixagent_http_request_errors_total", errLabels)
	assert.Equal(t, float64(1), errCount, "only the 500 should count as error")
}

func BenchmarkHTTPMetrics_Middleware(b *testing.B) {
	reg := prometheus.NewRegistry()
	m := NewHTTPMetricsWithRegistry(reg)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/v1/bench", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/v1/bench", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
