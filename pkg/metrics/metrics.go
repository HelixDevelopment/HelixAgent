package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// LLM request metrics
	LLMRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_requests_total",
			Help: "Total number of LLM requests",
		},
		[]string{"provider", "request_type"},
	)

	LLMResponseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_response_time_seconds",
			Help:    "Response time for LLM requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"provider"},
	)

	LLMTokensUsed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_tokens_used_total",
			Help: "Total tokens used by LLM requests",
		},
		[]string{"provider"},
	)

	// Provider health metrics
	ProviderHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "llm_provider_health_status",
			Help: "Health status of LLM providers (1=healthy, 0=unhealthy)",
		},
		[]string{"provider"},
	)

	ProviderSuccessRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "llm_provider_success_rate",
			Help: "Success rate of LLM providers",
		},
		[]string{"provider"},
	)
)

func InitMetrics() {
	// Register metrics
	prometheus.MustRegister(LLMRequestsTotal)
	prometheus.MustRegister(LLMResponseTime)
	prometheus.MustRegister(LLMTokensUsed)
	prometheus.MustRegister(ProviderHealthStatus)
	prometheus.MustRegister(ProviderSuccessRate)

	log.Println("[metrics] Prometheus metrics initialized")
}

// Handler returns the Prometheus metrics HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}
