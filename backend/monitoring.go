package main

import (
	"fmt"
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
)

type MetricsService struct {
	registry            *prometheus.Registry
	totalRequests       *prometheus.CounterVec
	failedRequests      *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	appMemoryBytesUsage prometheus.GaugeFunc
}

func NewMetricsService() *MetricsService {
	registry := prometheus.NewRegistry()
	ms := &MetricsService{
		registry: registry,
		totalRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Number of http requests",
			},
			[]string{"path", "method"},
		),
		failedRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_errors_total",
				Help: "Number of failed http requests",
			},
			[]string{"path", "method", "status_code"},
		),
		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_requests_duration_seconds",
				Help:    "Duration of http requests",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"path", "method"},
		),
		appMemoryBytesUsage: prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "app_memory_bytes_usage",
				Help: "Memory usage of the application",
			},
			func() float64 {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				return float64(m.Alloc)
			},
		),
	}

	ms.registerMetrics()
	return ms
}

func (p *MetricsService) Registry() *prometheus.Registry {
	return p.registry
}

func (p *MetricsService) IncrementTotalRequests(method, path string) {
	p.totalRequests.WithLabelValues(method, path).Inc()
}

func (p *MetricsService) IncrementFailedRequests(method, path, code string) {
	p.failedRequests.WithLabelValues(method, path, code).Inc()
}

func (p *MetricsService) ObserveRequestDuration(duration float64, method, path string) {
	fmt.Println("Observing request duration:", duration, "for method:", method, "and path:", path)
	p.httpRequestDuration.WithLabelValues(path, method).Observe(duration)
}

func (p *MetricsService) registerMetrics() {
	p.registry.MustRegister(
		p.totalRequests,
		p.failedRequests,
		p.httpRequestDuration,
		p.appMemoryBytesUsage,
	)
}
