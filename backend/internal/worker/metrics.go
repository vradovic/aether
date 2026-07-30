package worker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	StageDuration *prometheus.HistogramVec
}

func InitMetrics() (*prometheus.Registry, *Metrics) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	stageDuration := promauto.With(registry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "stage_duration_seconds",
		Help:    "Latency for a stage.",
		Buckets: []float64{.0001, .0005, .001, .0025, .005, .0075, .01, .025, .05},
	}, []string{"stage"})

	return registry, &Metrics{
		StageDuration: stageDuration,
	}
}
