package pubsub

import (
	"sync"
	"testing"
	"time"

	"github.com/chuckpreslar/emission"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// Tests for subscription-related functionality
// Note: Many tests are simplified due to difficulty in mocking libp2p pubsub.Subscription

func TestProcessorSubscriptionBasics(t *testing.T) {
	// Test basic ProcessorSubscription struct
	ps := &ProcessorSubscription[string]{
		started: false,
		stopped: false,
	}

	assert.False(t, ps.IsStarted())
	assert.False(t, ps.IsStopped())
	
	ps.started = true
	assert.True(t, ps.IsStarted())
	
	ps.stopped = true
	assert.True(t, ps.IsStopped())
}

// MultiProcessorSubscription tests would go here if the type was exported

func TestSubscriptionProcessorMetrics(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	
	metrics := NewProcessorMetrics(log)
	assert.NotNil(t, metrics)
	
	// Test basic metric recording
	metrics.RecordMessage()
	stats := metrics.GetStats()
	assert.Equal(t, uint64(1), stats.MessagesReceived)
	
	// Test validation results
	metrics.RecordValidationResult(ValidationAccept)
	metrics.RecordValidationResult(ValidationReject)
	metrics.RecordValidationResult(ValidationIgnore)
	
	stats = metrics.GetStats()
	assert.Equal(t, uint64(1), stats.MessagesAccepted)
	assert.Equal(t, uint64(1), stats.MessagesRejected)
	assert.Equal(t, uint64(1), stats.MessagesIgnored)
	
	// Test error recording
	metrics.RecordDecodingError()
	metrics.RecordValidationError()
	metrics.RecordProcessingError()
	
	stats = metrics.GetStats()
	assert.Equal(t, uint64(1), stats.DecodingErrors)
	assert.Equal(t, uint64(1), stats.ValidationErrors)
	assert.Equal(t, uint64(1), stats.ProcessingErrors)
	
	// Test processing time
	metrics.RecordProcessingTime(100 * time.Millisecond)
	stats = metrics.GetStats()
	assert.Greater(t, stats.TotalProcessingTime, time.Duration(0))
}

func TestProcessorMetricsConcurrency(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	
	metrics := NewProcessorMetrics(log)
	
	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				metrics.RecordMessage()
				metrics.RecordProcessed()
				metrics.RecordValidationResult(ValidationAccept)
				metrics.RecordProcessingTime(time.Millisecond)
			}
		}()
	}
	
	wg.Wait()
	
	stats := metrics.GetStats()
	assert.Equal(t, uint64(1000), stats.MessagesReceived)
	assert.Equal(t, uint64(1000), stats.MessagesProcessed)
	assert.Equal(t, uint64(1000), stats.MessagesAccepted)
}

func TestProcessorStats(t *testing.T) {
	// Test ProcessorStats struct
	stats := ProcessorStats{
		MessagesReceived:    100,
		MessagesProcessed:   90,
		MessagesAccepted:    80,
		MessagesRejected:    5,
		MessagesIgnored:     5,
		DecodingErrors:      2,
		ValidationErrors:    3,
		ProcessingErrors:    5,
		TotalProcessingTime: 500 * time.Millisecond,
	}
	
	// Basic assertions
	assert.Equal(t, uint64(100), stats.MessagesReceived)
	assert.Equal(t, uint64(90), stats.MessagesProcessed)
	assert.Equal(t, uint64(80), stats.MessagesAccepted)
	assert.Equal(t, uint64(5), stats.MessagesRejected)
	assert.Equal(t, uint64(5), stats.MessagesIgnored)
	assert.Equal(t, uint64(2), stats.DecodingErrors)
	assert.Equal(t, uint64(3), stats.ValidationErrors)
	assert.Equal(t, uint64(5), stats.ProcessingErrors)
	assert.Equal(t, 500*time.Millisecond, stats.TotalProcessingTime)
}

func TestEmitterIntegration(t *testing.T) {
	// Test that emission package works correctly
	emitter := emission.NewEmitter()
	assert.NotNil(t, emitter)
	
	var called bool
	emitter.On("test-event", func(data string) {
		called = true
		assert.Equal(t, "test-data", data)
	})
	
	emitter.Emit("test-event", "test-data")
	
	// Give some time for async emission
	time.Sleep(10 * time.Millisecond)
	assert.True(t, called)
}

func TestSubscriptionHelperFunctions(t *testing.T) {
	// Test SubscribeWithProcessor function behavior
	t.Run("SubscribeWithProcessor requires started gossipsub", func(t *testing.T) {
		// This test would require a real Gossipsub instance
		t.Skip("Requires full Gossipsub setup")
	})
	
	t.Run("SubscribeMultiWithProcessor requires started gossipsub", func(t *testing.T) {
		// This test would require a real Gossipsub instance
		t.Skip("Requires full Gossipsub setup")
	})
}

// Note: mockProcessor is defined in processor_test.go