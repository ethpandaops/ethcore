package pubsub

import (
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	metrics := NewMetrics("test")
	
	assert.NotNil(t, metrics)
	
	// Verify metrics are initialized
	assert.NotNil(t, metrics.MessagesReceived)
	assert.NotNil(t, metrics.MessagesPublished)
	assert.NotNil(t, metrics.MessagesValidated)
	assert.NotNil(t, metrics.MessagesHandled)
	assert.NotNil(t, metrics.ValidationErrors)
	assert.NotNil(t, metrics.HandlerErrors)
	assert.NotNil(t, metrics.PublishErrors)
	assert.NotNil(t, metrics.ValidationDuration)
	assert.NotNil(t, metrics.HandlerDuration)
	assert.NotNil(t, metrics.PublishDuration)
	assert.NotNil(t, metrics.ActiveSubscriptions)
	assert.NotNil(t, metrics.PeersPerTopic)
	assert.NotNil(t, metrics.MessageQueueSize)
	assert.NotNil(t, metrics.PeerScores)
	assert.NotNil(t, metrics.PeersConnected)
}

func TestMetricsRecordMethods(t *testing.T) {
	metrics := NewMetrics("test")

	// Test recording messages
	metrics.RecordMessageReceived("test_topic")
	metrics.RecordMessagePublished("test_topic", true)
	metrics.RecordMessagePublished("test_topic", false)
	metrics.RecordMessageValidated("test_topic", ValidationAccept)
	metrics.RecordMessageValidated("test_topic", ValidationReject)
	
	// Test recording with duration
	duration := 100 * time.Millisecond
	metrics.RecordMessageHandled("test_topic", true, duration)
	metrics.RecordMessageHandled("test_topic", false, duration)

	// Test recording errors
	metrics.RecordValidationError("test_topic")
	metrics.RecordHandlerError("test_topic")
	metrics.RecordPublishError("test_topic")

	// Test recording durations
	metrics.RecordValidationDuration("test_topic", duration)
	metrics.RecordHandlerDuration("test_topic", duration)
	metrics.RecordPublishDuration("test_topic", duration)

	// Test updating state metrics
	metrics.SetActiveSubscriptions(5)
	metrics.SetPeersPerTopic("test_topic", 10)
	metrics.SetMessageQueueSize("test_topic", 100)
	metrics.SetPeerScore("peer1", 0.95)
	metrics.SetPeersConnected(15)
}

func TestMetricsRecordMessageReceived(t *testing.T) {
	metrics := NewMetrics("test")
	topic := "test/topic"

	// Record some messages
	for i := 0; i < 5; i++ {
		metrics.RecordMessageReceived(topic)
	}

	// Check counter
	metric := &dto.Metric{}
	err := metrics.MessagesReceived.WithLabelValues(topic).Write(metric)
	require.NoError(t, err)
	assert.Equal(t, float64(5), *metric.Counter.Value)
}

func TestMetricsRecordMessagePublished(t *testing.T) {
	metrics := NewMetrics("test")
	topic := "test/topic"

	// Record successful and failed publishes
	metrics.RecordMessagePublished(topic, true)
	metrics.RecordMessagePublished(topic, true)
	metrics.RecordMessagePublished(topic, false)

	// Check success counter
	successMetric := &dto.Metric{}
	err := metrics.MessagesPublished.WithLabelValues(topic, "success").Write(successMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(2), *successMetric.Counter.Value)

	// Check error counter
	errorMetric := &dto.Metric{}
	err = metrics.MessagesPublished.WithLabelValues(topic, "error").Write(errorMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(1), *errorMetric.Counter.Value)
}

func TestMetricsRecordMessageValidated(t *testing.T) {
	metrics := NewMetrics("test")
	topic := "test/topic"

	// Record validations
	metrics.RecordMessageValidated(topic, ValidationAccept)
	metrics.RecordMessageValidated(topic, ValidationAccept)
	metrics.RecordMessageValidated(topic, ValidationReject)
	metrics.RecordMessageValidated(topic, ValidationIgnore)

	// Check accept counter
	acceptMetric := &dto.Metric{}
	err := metrics.MessagesValidated.WithLabelValues(topic, "accept").Write(acceptMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(2), *acceptMetric.Counter.Value)

	// Check reject counter
	rejectMetric := &dto.Metric{}
	err = metrics.MessagesValidated.WithLabelValues(topic, "reject").Write(rejectMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(1), *rejectMetric.Counter.Value)

	// Check ignore counter
	ignoreMetric := &dto.Metric{}
	err = metrics.MessagesValidated.WithLabelValues(topic, "ignore").Write(ignoreMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(1), *ignoreMetric.Counter.Value)
}

func TestMetricsRecordMessageHandled(t *testing.T) {
	metrics := NewMetrics("test")
	topic := "test/topic"
	duration := 50 * time.Millisecond

	// Record handled messages
	metrics.RecordMessageHandled(topic, true, duration)
	metrics.RecordMessageHandled(topic, false, duration)
	metrics.RecordMessageHandled(topic, true, duration*2)

	// Check success counter
	successMetric := &dto.Metric{}
	err := metrics.MessagesHandled.WithLabelValues(topic, "success").Write(successMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(2), *successMetric.Counter.Value)

	// Check error counter
	errorMetric := &dto.Metric{}
	err = metrics.MessagesHandled.WithLabelValues(topic, "error").Write(errorMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(1), *errorMetric.Counter.Value)

	// Check duration histogram
	histMetric := &dto.Metric{}
	err = metrics.HandlerDuration.WithLabelValues(topic).(interface {
		Write(*dto.Metric) error
	}).Write(histMetric)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), *histMetric.Histogram.SampleCount)
}

func TestMetricsRecordErrors(t *testing.T) {
	metrics := NewMetrics("test")
	topic := "test/topic"

	// Record various errors
	metrics.RecordValidationError(topic)
	metrics.RecordValidationError(topic)
	metrics.RecordHandlerError(topic)
	metrics.RecordPublishError(topic)
	metrics.RecordPublishError(topic)
	metrics.RecordPublishError(topic)

	// Check validation errors
	valErrorMetric := &dto.Metric{}
	err := metrics.ValidationErrors.WithLabelValues(topic).Write(valErrorMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(2), *valErrorMetric.Counter.Value)

	// Check handler errors
	handlerErrorMetric := &dto.Metric{}
	err = metrics.HandlerErrors.WithLabelValues(topic).Write(handlerErrorMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(1), *handlerErrorMetric.Counter.Value)

	// Check publish errors
	pubErrorMetric := &dto.Metric{}
	err = metrics.PublishErrors.WithLabelValues(topic).Write(pubErrorMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(3), *pubErrorMetric.Counter.Value)
}

func TestMetricsRecordDurations(t *testing.T) {
	metrics := NewMetrics("test")
	topic := "test/topic"

	durations := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		15 * time.Millisecond,
	}

	// Record validation durations
	for _, d := range durations {
		metrics.RecordValidationDuration(topic, d)
	}

	// Check validation histogram
	valHistMetric := &dto.Metric{}
	err := metrics.ValidationDuration.WithLabelValues(topic).(interface {
		Write(*dto.Metric) error
	}).Write(valHistMetric)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), *valHistMetric.Histogram.SampleCount)

	// Record handler durations
	for _, d := range durations {
		metrics.RecordHandlerDuration(topic, d)
	}

	// Check handler histogram
	handlerHistMetric := &dto.Metric{}
	err = metrics.HandlerDuration.WithLabelValues(topic).(interface {
		Write(*dto.Metric) error
	}).Write(handlerHistMetric)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), *handlerHistMetric.Histogram.SampleCount)

	// Record publish durations
	for _, d := range durations {
		metrics.RecordPublishDuration(topic, d)
	}

	// Check publish histogram
	pubHistMetric := &dto.Metric{}
	err = metrics.PublishDuration.WithLabelValues(topic).(interface {
		Write(*dto.Metric) error
	}).Write(pubHistMetric)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), *pubHistMetric.Histogram.SampleCount)
}

func TestMetricsSetStateMetrics(t *testing.T) {
	metrics := NewMetrics("test")
	topic := "test/topic"

	// Set active subscriptions
	metrics.SetActiveSubscriptions(5)
	activeSubsMetric := &dto.Metric{}
	err := metrics.ActiveSubscriptions.WithLabelValues().Write(activeSubsMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(5), *activeSubsMetric.Gauge.Value)

	// Set peers per topic
	metrics.SetPeersPerTopic(topic, 10)
	peersPerTopicMetric := &dto.Metric{}
	err = metrics.PeersPerTopic.WithLabelValues(topic).Write(peersPerTopicMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(10), *peersPerTopicMetric.Gauge.Value)

	// Set message queue size
	metrics.SetMessageQueueSize(topic, 100)
	queueSizeMetric := &dto.Metric{}
	err = metrics.MessageQueueSize.WithLabelValues(topic).Write(queueSizeMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(100), *queueSizeMetric.Gauge.Value)
}

func TestMetricsSetPeerMetrics(t *testing.T) {
	metrics := NewMetrics("test")
	peerID := "peer123"

	// Set peer score
	metrics.SetPeerScore(peerID, 0.95)
	peerScoreMetric := &dto.Metric{}
	err := metrics.PeerScores.WithLabelValues(peerID).Write(peerScoreMetric)
	require.NoError(t, err)
	assert.Equal(t, 0.95, *peerScoreMetric.Gauge.Value)

	// Update peer score
	metrics.SetPeerScore(peerID, -0.1)
	err = metrics.PeerScores.WithLabelValues(peerID).Write(peerScoreMetric)
	require.NoError(t, err)
	assert.Equal(t, -0.1, *peerScoreMetric.Gauge.Value)

	// Set peers connected
	metrics.SetPeersConnected(15)
	peersConnectedMetric := &dto.Metric{}
	err = metrics.PeersConnected.Write(peersConnectedMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(15), *peersConnectedMetric.Gauge.Value)
}

func TestMetricsRegister(t *testing.T) {
	metrics := NewMetrics("test")
	registry := prometheus.NewRegistry()

	// Register should succeed
	err := metrics.Register(registry)
	assert.NoError(t, err)

	// Second registration should fail (already registered)
	err = metrics.Register(registry)
	assert.Error(t, err)
}

func TestMetricsConcurrency(t *testing.T) {
	metrics := NewMetrics("test")
	topic := "test/topic"

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				metrics.RecordMessageReceived(topic)
				metrics.RecordMessagePublished(topic, true)
				metrics.RecordMessageValidated(topic, ValidationAccept)
				metrics.RecordMessageHandled(topic, true, time.Millisecond)
				metrics.RecordValidationDuration(topic, time.Millisecond)
				metrics.SetPeersPerTopic(topic, j)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check that metrics were recorded
	receivedMetric := &dto.Metric{}
	err := metrics.MessagesReceived.WithLabelValues(topic).Write(receivedMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(1000), *receivedMetric.Counter.Value)
}

func TestMetricsNilSafety(t *testing.T) {
	// Create metrics but set some fields to nil to test safety
	metrics := &Metrics{}

	// All methods should handle nil fields gracefully
	assert.NotPanics(t, func() {
		metrics.RecordMessageReceived("topic")
		metrics.RecordMessagePublished("topic", true)
		metrics.RecordMessageValidated("topic", ValidationAccept)
		metrics.RecordMessageHandled("topic", true, time.Second)
		metrics.RecordValidationError("topic")
		metrics.RecordHandlerError("topic")
		metrics.RecordPublishError("topic")
		metrics.RecordValidationDuration("topic", time.Second)
		metrics.RecordHandlerDuration("topic", time.Second)
		metrics.RecordPublishDuration("topic", time.Second)
		metrics.SetActiveSubscriptions(5)
		metrics.SetPeersPerTopic("topic", 10)
		metrics.SetMessageQueueSize("topic", 100)
		metrics.SetPeerScore("peer", 0.5)
		metrics.SetPeersConnected(15)
	})
}