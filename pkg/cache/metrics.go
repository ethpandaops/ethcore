package cache

import (
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus metrics for cache operations.
type Metrics struct {
	namespace string
	subsystem string
	labels    []string

	// Metrics
	insertionsTotal *prometheus.CounterVec
	hitsTotal       *prometheus.CounterVec
	missesTotal     *prometheus.CounterVec
	evictionsTotal  *prometheus.CounterVec
	sizeGauge       *prometheus.GaugeVec

	// Registration tracking
	registered bool
	mu         sync.Mutex
}

// MetricsConfig holds configuration for cache metrics.
type MetricsConfig struct {
	// Namespace is the prometheus namespace for metrics.
	Namespace string
	// Subsystem is the prometheus subsystem for metrics.
	Subsystem string
	// ConstLabels are constant labels added to all metrics.
	ConstLabels prometheus.Labels
	// InstanceLabels are the variable labels for this cache instance.
	// Example: {"cache_type": "session", "region": "us-east"}
	// The keys will be used as label names, values as label values.
	InstanceLabels map[string]string
	// UpdateInterval controls how often metrics are updated.
	// If 0, defaults to 5 seconds.
	UpdateInterval time.Duration
	// Registerer is the prometheus registerer to use.
	// If nil, the default registerer is used.
	Registerer prometheus.Registerer
}

// NewMetrics creates a new Metrics instance.
func NewMetrics(cfg MetricsConfig) *Metrics {
	if cfg.Subsystem == "" {
		cfg.Subsystem = "cache"
	}

	// Extract label names from InstanceLabels map
	labelNames := make([]string, 0, len(cfg.InstanceLabels))
	for k := range cfg.InstanceLabels {
		labelNames = append(labelNames, k)
	}
	// Sort to ensure consistent order
	sort.Strings(labelNames)

	return &Metrics{
		namespace: cfg.Namespace,
		subsystem: cfg.Subsystem,
		labels:    labelNames,
		insertionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			Name:        "insertions_total",
			Help:        "Total number of cache insertions",
			ConstLabels: cfg.ConstLabels,
		}, labelNames),
		hitsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			Name:        "hits_total",
			Help:        "Total number of cache hits",
			ConstLabels: cfg.ConstLabels,
		}, labelNames),
		missesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			Name:        "misses_total",
			Help:        "Total number of cache misses",
			ConstLabels: cfg.ConstLabels,
		}, labelNames),
		evictionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			Name:        "evictions_total",
			Help:        "Total number of cache evictions",
			ConstLabels: cfg.ConstLabels,
		}, labelNames),
		sizeGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			Name:        "size",
			Help:        "Current number of items in the cache",
			ConstLabels: cfg.ConstLabels,
		}, labelNames),
	}
}

// Register registers the metrics with the provided registerer.
// If registerer is nil, the default prometheus registerer is used.
func (m *Metrics) Register(registerer prometheus.Registerer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	collectors := []prometheus.Collector{
		m.insertionsTotal,
		m.hitsTotal,
		m.missesTotal,
		m.evictionsTotal,
		m.sizeGauge,
	}

	for _, c := range collectors {
		if err := registerer.Register(c); err != nil {
			// Try to unregister any previously registered collectors
			for i := 0; i < len(collectors); i++ {
				registerer.Unregister(collectors[i])
			}

			return err
		}
	}

	m.registered = true

	return nil
}

// Unregister unregisters the metrics from the provided registerer.
func (m *Metrics) Unregister(registerer prometheus.Registerer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.registered {
		return
	}

	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	registerer.Unregister(m.insertionsTotal)
	registerer.Unregister(m.hitsTotal)
	registerer.Unregister(m.missesTotal)
	registerer.Unregister(m.evictionsTotal)
	registerer.Unregister(m.sizeGauge)

	m.registered = false
}

// IncInsertions increments the insertions counter.
func (m *Metrics) IncInsertions(labelValues ...string) {
	m.insertionsTotal.WithLabelValues(labelValues...).Inc()
}

// IncHits increments the hits counter.
func (m *Metrics) IncHits(labelValues ...string) {
	m.hitsTotal.WithLabelValues(labelValues...).Inc()
}

// IncMisses increments the misses counter.
func (m *Metrics) IncMisses(labelValues ...string) {
	m.missesTotal.WithLabelValues(labelValues...).Inc()
}

// IncEvictions increments the evictions counter.
func (m *Metrics) IncEvictions(labelValues ...string) {
	m.evictionsTotal.WithLabelValues(labelValues...).Inc()
}

// SetSize sets the current cache size.
func (m *Metrics) SetSize(size float64, labelValues ...string) {
	m.sizeGauge.WithLabelValues(labelValues...).Set(size)
}

// ExtractLabelValues extracts label values from a map in the order of registered labels.
func (m *Metrics) ExtractLabelValues(instanceLabels map[string]string) []string {
	values := make([]string, len(m.labels))
	for i, label := range m.labels {
		values[i] = instanceLabels[label]
	}

	return values
}
