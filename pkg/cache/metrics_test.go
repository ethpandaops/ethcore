package cache

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	tests := []struct {
		name     string
		config   MetricsConfig
		expected struct {
			namespace  string
			subsystem  string
			labelNames []string
			labelCount int
		}
	}{
		{
			name: "with all fields",
			config: MetricsConfig{
				Namespace: "test",
				Subsystem: "cache",
				InstanceLabels: map[string]string{
					"cache_type": "session",
					"region":     "us-east",
				},
			},
			expected: struct {
				namespace  string
				subsystem  string
				labelNames []string
				labelCount int
			}{
				namespace:  "test",
				subsystem:  "cache",
				labelNames: []string{"cache_type", "region"},
				labelCount: 2,
			},
		},
		{
			name: "with default subsystem",
			config: MetricsConfig{
				Namespace: "test",
				InstanceLabels: map[string]string{
					"type": "node",
				},
			},
			expected: struct {
				namespace  string
				subsystem  string
				labelNames []string
				labelCount int
			}{
				namespace:  "test",
				subsystem:  "cache",
				labelNames: []string{"type"},
				labelCount: 1,
			},
		},
		{
			name: "with const labels",
			config: MetricsConfig{
				Namespace: "test",
				ConstLabels: prometheus.Labels{
					"app": "myapp",
					"env": "prod",
				},
				InstanceLabels: map[string]string{
					"cache_name": "primary",
				},
			},
			expected: struct {
				namespace  string
				subsystem  string
				labelNames []string
				labelCount int
			}{
				namespace:  "test",
				subsystem:  "cache",
				labelNames: []string{"cache_name"},
				labelCount: 1,
			},
		},
		{
			name:   "empty config",
			config: MetricsConfig{},
			expected: struct {
				namespace  string
				subsystem  string
				labelNames []string
				labelCount int
			}{
				namespace:  "",
				subsystem:  "cache",
				labelNames: []string{},
				labelCount: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMetrics(tt.config)

			assert.Equal(t, tt.expected.namespace, m.namespace)
			assert.Equal(t, tt.expected.subsystem, m.subsystem)
			assert.Equal(t, tt.expected.labelCount, len(m.labels))

			// Check label names are sorted
			assert.Equal(t, tt.expected.labelNames, m.labels)
		})
	}
}

func TestMetrics_Register(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := MetricsConfig{
			Namespace: "test",
			Subsystem: "cache",
			InstanceLabels: map[string]string{
				"cache_name": "test",
			},
		}

		m := NewMetrics(config)
		err := m.Register(registry)
		assert.NoError(t, err)
		assert.True(t, m.registered)

		// Use the metrics to ensure they appear in the registry
		m.IncInsertions("test")
		m.IncHits("test")
		m.IncMisses("test")
		m.IncEvictions("test")
		m.SetSize(1, "test")

		// Verify metrics are registered
		mfs, err := registry.Gather()
		require.NoError(t, err)

		metricNames := make(map[string]bool)
		for _, mf := range mfs {
			metricNames[mf.GetName()] = true
		}

		// Check that our metric families exist
		assert.True(t, metricNames["test_cache_insertions_total"])
		assert.True(t, metricNames["test_cache_hits_total"])
		assert.True(t, metricNames["test_cache_misses_total"])
		assert.True(t, metricNames["test_cache_evictions_total"])
		assert.True(t, metricNames["test_cache_size"])
	})

	t.Run("registration with nil registerer uses default", func(t *testing.T) {
		// Save and restore default registerer
		oldReg := prometheus.DefaultRegisterer
		defer func() { prometheus.DefaultRegisterer = oldReg }()

		registry := prometheus.NewRegistry()
		prometheus.DefaultRegisterer = registry

		config := MetricsConfig{
			Namespace: "test2",
			Subsystem: "cache",
		}

		m := NewMetrics(config)
		err := m.Register(nil)
		assert.NoError(t, err)
		assert.True(t, m.registered)
	})

	t.Run("duplicate registration returns error", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := MetricsConfig{
			Namespace: "test",
			Subsystem: "cache",
		}

		m1 := NewMetrics(config)
		m2 := NewMetrics(config)

		err := m1.Register(registry)
		assert.NoError(t, err)

		err = m2.Register(registry)
		assert.Error(t, err)
		assert.False(t, m2.registered)
	})

	t.Run("multiple register calls are idempotent", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := MetricsConfig{
			Namespace: "test",
			Subsystem: "cache",
		}

		m := NewMetrics(config)

		err := m.Register(registry)
		assert.NoError(t, err)
		assert.True(t, m.registered)

		// Second registration should succeed (no-op)
		err = m.Register(registry)
		assert.NoError(t, err)
		assert.True(t, m.registered)
	})
}

func TestMetrics_Unregister(t *testing.T) {
	t.Run("successful unregister", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := MetricsConfig{
			Namespace: "test",
			Subsystem: "cache",
		}

		m := NewMetrics(config)
		err := m.Register(registry)
		require.NoError(t, err)

		m.Unregister(registry)
		assert.False(t, m.registered)

		// Should be able to register again
		err = m.Register(registry)
		assert.NoError(t, err)
	})

	t.Run("unregister without register is safe", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := MetricsConfig{
			Namespace: "test",
			Subsystem: "cache",
		}

		m := NewMetrics(config)
		// Should not panic
		m.Unregister(registry)
		assert.False(t, m.registered)
	})

	t.Run("unregister with nil registerer", func(t *testing.T) {
		config := MetricsConfig{
			Namespace: "test",
			Subsystem: "cache",
		}

		m := NewMetrics(config)
		m.registered = true

		// Should not panic
		m.Unregister(nil)
		assert.False(t, m.registered)
	})
}

func TestMetrics_Operations(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "cache",
		InstanceLabels: map[string]string{
			"cache_name": "test",
		},
	}

	m := NewMetrics(config)
	err := m.Register(registry)
	require.NoError(t, err)

	t.Run("IncInsertions", func(t *testing.T) {
		m.IncInsertions("test")
		m.IncInsertions("test")

		value := testutil.ToFloat64(m.insertionsTotal.WithLabelValues("test"))
		assert.Equal(t, float64(2), value)
	})

	t.Run("IncHits", func(t *testing.T) {
		m.IncHits("test")
		m.IncHits("test")
		m.IncHits("test")

		value := testutil.ToFloat64(m.hitsTotal.WithLabelValues("test"))
		assert.Equal(t, float64(3), value)
	})

	t.Run("IncMisses", func(t *testing.T) {
		m.IncMisses("test")

		value := testutil.ToFloat64(m.missesTotal.WithLabelValues("test"))
		assert.Equal(t, float64(1), value)
	})

	t.Run("IncEvictions", func(t *testing.T) {
		m.IncEvictions("test")
		m.IncEvictions("test")
		m.IncEvictions("test")
		m.IncEvictions("test")

		value := testutil.ToFloat64(m.evictionsTotal.WithLabelValues("test"))
		assert.Equal(t, float64(4), value)
	})

	t.Run("SetSize", func(t *testing.T) {
		m.SetSize(10, "test")
		value := testutil.ToFloat64(m.sizeGauge.WithLabelValues("test"))
		assert.Equal(t, float64(10), value)

		m.SetSize(5, "test")
		value = testutil.ToFloat64(m.sizeGauge.WithLabelValues("test"))
		assert.Equal(t, float64(5), value)
	})
}

func TestMetrics_ExtractLabelValues(t *testing.T) {
	tests := []struct {
		name           string
		labels         []string
		instanceLabels map[string]string
		expected       []string
	}{
		{
			name:   "exact match",
			labels: []string{"cache_type", "region"},
			instanceLabels: map[string]string{
				"cache_type": "session",
				"region":     "us-east",
			},
			expected: []string{"session", "us-east"},
		},
		{
			name:   "missing label value",
			labels: []string{"cache_type", "region"},
			instanceLabels: map[string]string{
				"cache_type": "session",
			},
			expected: []string{"session", ""},
		},
		{
			name:   "extra labels in map",
			labels: []string{"cache_type"},
			instanceLabels: map[string]string{
				"cache_type": "session",
				"region":     "us-east",
				"extra":      "ignored",
			},
			expected: []string{"session"},
		},
		{
			name:           "empty labels",
			labels:         []string{},
			instanceLabels: map[string]string{"type": "test"},
			expected:       []string{},
		},
		{
			name:           "nil map",
			labels:         []string{"type"},
			instanceLabels: nil,
			expected:       []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metrics{labels: tt.labels}
			result := m.ExtractLabelValues(tt.instanceLabels)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMetrics_LabelOrdering(t *testing.T) {
	// Test that labels are consistently ordered regardless of map iteration order
	config := MetricsConfig{
		Namespace: "test",
		InstanceLabels: map[string]string{
			"z_last":   "value1",
			"a_first":  "value2",
			"m_middle": "value3",
		},
	}

	m1 := NewMetrics(config)
	m2 := NewMetrics(config)

	// Labels should be sorted alphabetically
	expectedLabels := []string{"a_first", "m_middle", "z_last"}
	assert.Equal(t, expectedLabels, m1.labels)
	assert.Equal(t, expectedLabels, m2.labels)

	// Extract values should maintain the same order
	values1 := m1.ExtractLabelValues(config.InstanceLabels)
	values2 := m2.ExtractLabelValues(config.InstanceLabels)

	expectedValues := []string{"value2", "value3", "value1"}
	assert.Equal(t, expectedValues, values1)
	assert.Equal(t, expectedValues, values2)
}

func TestMetrics_ThreadSafety(t *testing.T) {
	// Test concurrent registration/unregistration
	registry := prometheus.NewRegistry()
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "cache",
	}

	m := NewMetrics(config)

	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			_ = m.Register(registry)
			m.Unregister(registry)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = m.Register(registry)
			m.Unregister(registry)
		}
		done <- true
	}()

	<-done
	<-done

	// Should end in a consistent state
	assert.False(t, m.registered)
}
