package crawler

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	// Save the default registerer and restore it after each test
	originalRegisterer := prometheus.DefaultRegisterer
	defer func() {
		prometheus.DefaultRegisterer = originalRegisterer
	}()

	tests := []struct {
		name      string
		namespace string
	}{
		{
			name:      "metrics with namespace",
			namespace: "test_crawler",
		},
		{
			name:      "metrics with different namespace",
			namespace: "another_crawler",
		},
		{
			name:      "metrics with empty namespace",
			namespace: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new registry for each test to avoid conflicts
			reg := prometheus.NewRegistry()
			prometheus.DefaultRegisterer = reg

			// Create metrics using the actual NewMetrics function
			m := NewMetrics(tt.namespace)

			require.NotNil(t, m)
			assert.NotNil(t, m.NodesProcessed)
			assert.NotNil(t, m.PendingDials)
			assert.NotNil(t, m.FailedCrawls)
			assert.NotNil(t, m.SuccessfulCrawls)
			assert.NotNil(t, m.FailedRequests)

			// Verify that we can use the metrics without panicking
			// This also ensures they're properly registered
			m.RecordNodeProcessed()
			m.RecordPendingDials(5)
			m.RecordFailedCrawl("test_reason")
			m.RecordSuccessfulCrawl("test_agent")
			m.RecordFailedRequest("test_protocol", "test_error")

			// Verify metrics are properly registered and have expected values
			assert.Equal(t, float64(1), testutil.ToFloat64(m.NodesProcessed))
			assert.Equal(t, float64(5), testutil.ToFloat64(m.PendingDials))

			counter, err := m.FailedCrawls.GetMetricWithLabelValues("test_reason")
			require.NoError(t, err)
			assert.Equal(t, float64(1), testutil.ToFloat64(counter))

			counter, err = m.SuccessfulCrawls.GetMetricWithLabelValues("test_agent")
			require.NoError(t, err)
			assert.Equal(t, float64(1), testutil.ToFloat64(counter))

			counter, err = m.FailedRequests.GetMetricWithLabelValues("test_protocol", "test_error")
			require.NoError(t, err)
			assert.Equal(t, float64(1), testutil.ToFloat64(counter))
		})
	}
}

func TestMetrics_RecordNodeProcessed(t *testing.T) {
	originalRegisterer := prometheus.DefaultRegisterer
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	defer func() {
		prometheus.DefaultRegisterer = originalRegisterer
	}()

	m := NewMetrics("test")

	// Initial count should be 0
	assert.Equal(t, float64(0), testutil.ToFloat64(m.NodesProcessed))

	// Record a processed node
	m.RecordNodeProcessed()
	assert.Equal(t, float64(1), testutil.ToFloat64(m.NodesProcessed))

	// Record multiple nodes
	for i := 0; i < 5; i++ {
		m.RecordNodeProcessed()
	}
	assert.Equal(t, float64(6), testutil.ToFloat64(m.NodesProcessed))
}

func TestMetrics_RecordPendingDials(t *testing.T) {
	originalRegisterer := prometheus.DefaultRegisterer
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	defer func() {
		prometheus.DefaultRegisterer = originalRegisterer
	}()

	m := NewMetrics("test")

	tests := []struct {
		name     string
		count    int
		expected float64
	}{
		{"zero pending dials", 0, 0},
		{"single pending dial", 1, 1},
		{"multiple pending dials", 10, 10},
		{"update pending dials", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.RecordPendingDials(tt.count)
			assert.Equal(t, tt.expected, testutil.ToFloat64(m.PendingDials))
		})
	}
}

func TestMetrics_RecordFailedCrawl(t *testing.T) {
	originalRegisterer := prometheus.DefaultRegisterer
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	defer func() {
		prometheus.DefaultRegisterer = originalRegisterer
	}()

	m := NewMetrics("test")

	reasons := []string{"timeout", "connection_refused", "invalid_response"}

	// Test different failure reasons
	for _, reason := range reasons {
		m.RecordFailedCrawl(reason)
		counter, err := m.FailedCrawls.GetMetricWithLabelValues(reason)
		require.NoError(t, err)
		assert.Equal(t, float64(1), testutil.ToFloat64(counter))
	}

	// Test incrementing same reason multiple times
	m.RecordFailedCrawl("timeout")
	counter, err := m.FailedCrawls.GetMetricWithLabelValues("timeout")
	require.NoError(t, err)
	assert.Equal(t, float64(2), testutil.ToFloat64(counter))
}

func TestMetrics_RecordSuccessfulCrawl(t *testing.T) {
	originalRegisterer := prometheus.DefaultRegisterer
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	defer func() {
		prometheus.DefaultRegisterer = originalRegisterer
	}()

	m := NewMetrics("test")

	agents := []string{"lighthouse", "prysm", "teku", "nimbus"}

	// Test different agents
	for _, agent := range agents {
		m.RecordSuccessfulCrawl(agent)
		counter, err := m.SuccessfulCrawls.GetMetricWithLabelValues(agent)
		require.NoError(t, err)
		assert.Equal(t, float64(1), testutil.ToFloat64(counter))
	}

	// Test incrementing same agent multiple times
	for i := 0; i < 3; i++ {
		m.RecordSuccessfulCrawl("lighthouse")
	}
	counter, err := m.SuccessfulCrawls.GetMetricWithLabelValues("lighthouse")
	require.NoError(t, err)
	assert.Equal(t, float64(4), testutil.ToFloat64(counter))
}

func TestMetrics_RecordFailedRequest(t *testing.T) {
	originalRegisterer := prometheus.DefaultRegisterer
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	defer func() {
		prometheus.DefaultRegisterer = originalRegisterer
	}()

	m := NewMetrics("test")

	tests := []struct {
		name      string
		protocol  string
		errorType string
	}{
		{"http timeout", "http", "timeout"},
		{"http connection refused", "http", "connection_refused"},
		{"websocket error", "websocket", "handshake_failed"},
		{"grpc deadline", "grpc", "deadline_exceeded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.RecordFailedRequest(tt.protocol, tt.errorType)
			counter, err := m.FailedRequests.GetMetricWithLabelValues(tt.protocol, tt.errorType)
			require.NoError(t, err)
			assert.Equal(t, float64(1), testutil.ToFloat64(counter))
		})
	}
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	originalRegisterer := prometheus.DefaultRegisterer
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	defer func() {
		prometheus.DefaultRegisterer = originalRegisterer
	}()

	m := NewMetrics("test")

	// Test concurrent access to ensure thread safety
	done := make(chan struct{})
	workers := 10
	iterations := 100

	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer func() { done <- struct{}{} }()

			for j := 0; j < iterations; j++ {
				m.RecordNodeProcessed()
				m.RecordPendingDials(workerID)
				m.RecordFailedCrawl("concurrent_test")
				m.RecordSuccessfulCrawl("test_agent")
				m.RecordFailedRequest("http", "test_error")
			}
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < workers; i++ {
		<-done
	}

	// Verify final counts
	assert.Equal(t, float64(workers*iterations), testutil.ToFloat64(m.NodesProcessed))

	counter, err := m.FailedCrawls.GetMetricWithLabelValues("concurrent_test")
	require.NoError(t, err)
	assert.Equal(t, float64(workers*iterations), testutil.ToFloat64(counter))

	counter, err = m.SuccessfulCrawls.GetMetricWithLabelValues("test_agent")
	require.NoError(t, err)
	assert.Equal(t, float64(workers*iterations), testutil.ToFloat64(counter))

	counter, err = m.FailedRequests.GetMetricWithLabelValues("http", "test_error")
	require.NoError(t, err)
	assert.Equal(t, float64(workers*iterations), testutil.ToFloat64(counter))
}

func TestMetrics_PanicOnDuplicateRegistration(t *testing.T) {
	originalRegisterer := prometheus.DefaultRegisterer
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	defer func() {
		prometheus.DefaultRegisterer = originalRegisterer
	}()

	// First registration should succeed
	_ = NewMetrics("test")

	// Second registration with same namespace should panic
	assert.Panics(t, func() {
		_ = NewMetrics("test")
	}, "Expected panic on duplicate metric registration")
}
