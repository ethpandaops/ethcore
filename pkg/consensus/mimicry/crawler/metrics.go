package crawler

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	NodesProcessed   prometheus.Counter
	PendingDials     prometheus.Gauge
	FailedCrawls     *prometheus.CounterVec
	SuccessfulCrawls *prometheus.CounterVec
	FailedRequests   *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	namespace := "crawler"

	m := &Metrics{
		NodesProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "nodes_processed_total",
			Help:      "Number of nodes processed",
			Namespace: namespace,
		}),
		PendingDials: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:      "pending_dials",
			Help:      "Number of pending dials",
			Namespace: namespace,
		}),
		FailedCrawls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "failed_total",
			Help:      "Number of failed crawls",
			Namespace: namespace,
		}, []string{"reason"}),
		SuccessfulCrawls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "successful_total",
			Help:      "Number of successful crawls",
			Namespace: namespace,
		}, []string{"agent"}),
		FailedRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "failed_requests_total",
			Help:      "Number of failed requests",
			Namespace: namespace,
		}, []string{"protocol", "error"}),
	}

	prometheus.MustRegister(m.NodesProcessed)
	prometheus.MustRegister(m.PendingDials)
	prometheus.MustRegister(m.FailedCrawls)
	prometheus.MustRegister(m.SuccessfulCrawls)
	prometheus.MustRegister(m.FailedRequests)

	return m
}

func (m *Metrics) RecordNodeProcessed() {
	m.NodesProcessed.Inc()
}

func (m *Metrics) RecordPendingDials(count int) {
	m.PendingDials.Set(float64(count))
}

func (m *Metrics) RecordFailedCrawl(reason string) {
	m.FailedCrawls.WithLabelValues(reason).Inc()
}

func (m *Metrics) RecordSuccessfulCrawl(agent string) {
	m.SuccessfulCrawls.WithLabelValues(agent).Inc()
}

func (m *Metrics) RecordFailedRequest(protocol, errorType string) {
	m.FailedRequests.WithLabelValues(protocol, errorType).Inc()
}
