package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/chuckpreslar/emission"
	"github.com/stretchr/testify/assert"
)

// createTestHandlerRegistry creates a handler registry for testing
func createTestHandlerRegistry() *handlerRegistry {
	logger := testLogger()
	metrics := NewMetrics("test")
	emitter := emission.NewEmitter()
	return newHandlerRegistry(logger, metrics, emitter)
}

func TestHandlerRegistry(t *testing.T) {
	hr := createTestHandlerRegistry()
	testTopic := "test-topic"

	// Initially no handler
	handler := hr.get(testTopic)
	assert.Nil(t, handler)

	// Register handler
	callCount := 0
	testHandler := func(ctx context.Context, msg *Message) error {
		callCount++
		return nil
	}

	hr.register(testTopic, testHandler)

	// Verify handler is registered
	handler = hr.get(testTopic)
	assert.NotNil(t, handler)

	// Test the handler
	testMsg := createTestMessage(testTopic, []byte("test"))
	err := handler(context.Background(), testMsg)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Unregister handler
	hr.unregister(testTopic)

	// Verify handler is removed
	handler = hr.get(testTopic)
	assert.Nil(t, handler)
}

func TestHandlerRegistryNilHandler(t *testing.T) {
	hr := createTestHandlerRegistry()
	testTopic := "test-topic"

	// Register nil handler (should be ignored)
	hr.register(testTopic, nil)

	// Verify no handler is registered
	handler := hr.get(testTopic)
	assert.Nil(t, handler)
}

func TestMessageHandling(t *testing.T) {
	hr := createTestHandlerRegistry()
	testTopic := "test-topic"
	testData := []byte("test message")

	// Test processing message without handler
	testMsg := createTestMessage(testTopic, testData)
	err := hr.processMessage(context.Background(), testMsg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handler registered")

	// Register handler and test successful processing
	var receivedMsg *Message
	testHandler := func(ctx context.Context, msg *Message) error {
		receivedMsg = msg
		return nil
	}

	hr.register(testTopic, testHandler)

	err = hr.processMessage(context.Background(), testMsg)
	assert.NoError(t, err)
	assert.NotNil(t, receivedMsg)
	assert.Equal(t, testTopic, receivedMsg.Topic)
	assert.Equal(t, testData, receivedMsg.Data)

	// Test handler that returns error
	errorHandler := func(ctx context.Context, msg *Message) error {
		return errors.New("handler error")
	}

	hr.register(testTopic, errorHandler)

	err = hr.processMessage(context.Background(), testMsg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler error")
}

func TestHandlerErrors(t *testing.T) {
	hr := createTestHandlerRegistry()
	testTopic := "test-topic"
	testMsg := createTestMessage(testTopic, []byte("test"))

	// Test handler that panics
	panicHandler := func(ctx context.Context, msg *Message) error {
		panic("handler panic")
	}

	hr.register(testTopic, panicHandler)

	// Should not panic the test
	err := hr.processMessage(context.Background(), testMsg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler panicked")

	// Test handler timeout
	slowHandler := func(ctx context.Context, msg *Message) error {
		select {
		case <-time.After(35 * time.Second): // Longer than handler timeout
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	hr.register(testTopic, slowHandler)

	// Use a shorter context timeout for testing
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = hr.processMessage(timeoutCtx, testMsg)
	duration := time.Since(start)

	// Should timeout quickly
	assert.True(t, duration < 5*time.Second, "Handler should timeout quickly")
	assert.Error(t, err)
}

func TestConcurrentHandlers(t *testing.T) {
	hr := createTestHandlerRegistry()
	const numGoroutines = 100
	const numTopics = 10

	var wg sync.WaitGroup
	var mu sync.Mutex
	handlerCalls := make(map[string]int)

	// Register handlers for multiple topics
	for i := 0; i < numTopics; i++ {
		topic := fmt.Sprintf("topic-%d", i)
		handler := func(topicName string) MessageHandler {
			return func(ctx context.Context, msg *Message) error {
				mu.Lock()
				handlerCalls[topicName]++
				mu.Unlock()
				// Small delay to simulate work
				time.Sleep(1 * time.Millisecond)
				return nil
			}
		}(topic)

		hr.register(topic, handler)
	}

	// Process messages concurrently
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			topic := fmt.Sprintf("topic-%d", id%numTopics)
			msg := createTestMessage(topic, []byte(fmt.Sprintf("message-%d", id)))

			err := hr.processMessage(context.Background(), msg)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify all handlers were called
	mu.Lock()
	totalCalls := 0
	for _, count := range handlerCalls {
		totalCalls += count
	}
	assert.Equal(t, numGoroutines, totalCalls)
	mu.Unlock()
}

func TestHandlerUtilities(t *testing.T) {
	testTopic := "test-topic"
	testData := []byte("test message")
	testMsg := createTestMessage(testTopic, testData)

	t.Run("LoggingHandler", func(t *testing.T) {
		logger := testLogger()
		callCount := 0

		baseHandler := func(ctx context.Context, msg *Message) error {
			callCount++
			return nil
		}

		wrappedHandler := LoggingHandler(logger, baseHandler)

		err := wrappedHandler(context.Background(), testMsg)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)

		// Test with error
		errorHandler := func(ctx context.Context, msg *Message) error {
			return errors.New("test error")
		}

		wrappedErrorHandler := LoggingHandler(logger, errorHandler)
		err = wrappedErrorHandler(context.Background(), testMsg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test error")
	})

	t.Run("MetricsHandler", func(t *testing.T) {
		metrics := NewMetrics("test")
		callCount := 0

		baseHandler := func(ctx context.Context, msg *Message) error {
			callCount++
			return nil
		}

		wrappedHandler := MetricsHandler(metrics, baseHandler)

		err := wrappedHandler(context.Background(), testMsg)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)

		// Test with nil metrics
		wrappedHandlerNilMetrics := MetricsHandler(nil, baseHandler)
		err = wrappedHandlerNilMetrics(context.Background(), testMsg)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount) // Called again
	})

	t.Run("AsyncHandler", func(t *testing.T) {
		logger := testLogger()
		var receivedMsg *Message
		var mu sync.Mutex
		processedCh := make(chan struct{})

		baseHandler := func(ctx context.Context, msg *Message) error {
			mu.Lock()
			receivedMsg = msg
			mu.Unlock()
			close(processedCh)
			return nil
		}

		wrappedHandler := AsyncHandler(logger, baseHandler)

		// Async handler should return immediately
		err := wrappedHandler(context.Background(), testMsg)
		assert.NoError(t, err)

		// Wait for async processing
		select {
		case <-processedCh:
			// Message processed
		case <-time.After(5 * time.Second):
			t.Fatal("Async handler did not process message within timeout")
		}

		mu.Lock()
		assert.NotNil(t, receivedMsg)
		assert.Equal(t, testTopic, receivedMsg.Topic)
		assert.Equal(t, testData, receivedMsg.Data)
		// Data should be copied correctly
		assert.Equal(t, testData, receivedMsg.Data)
		mu.Unlock()
	})

	t.Run("FilterHandler", func(t *testing.T) {
		callCount := 0

		baseHandler := func(ctx context.Context, msg *Message) error {
			callCount++
			return nil
		}

		// Filter that only allows messages with "allow" in data
		filter := func(msg *Message) bool {
			return string(msg.Data) == "allow"
		}

		wrappedHandler := FilterHandler(filter, baseHandler)

		// Test message that should be filtered out
		err := wrappedHandler(context.Background(), testMsg)
		assert.NoError(t, err)
		assert.Equal(t, 0, callCount) // Handler should not be called

		// Test message that should pass filter
		allowMsg := createTestMessage(testTopic, []byte("allow"))
		err = wrappedHandler(context.Background(), allowMsg)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount) // Handler should be called
	})
}

func TestHandlerEventEmission(t *testing.T) {
	hr := createTestHandlerRegistry()
	testTopic := "test-topic"
	testMsg := createTestMessage(testTopic, []byte("test"))

	// Track emitted events
	var handledEvents []string
	var errorEvents []string
	var mu sync.Mutex

	hr.emitter.On(MessageHandledEvent, func(topic string, success bool) {
		mu.Lock()
		handledEvents = append(handledEvents, fmt.Sprintf("%s:%t", topic, success))
		mu.Unlock()
	})

	hr.emitter.On(HandlerErrorEvent, func(topic string, err error) {
		mu.Lock()
		errorEvents = append(errorEvents, fmt.Sprintf("%s:%s", topic, err.Error()))
		mu.Unlock()
	})

	// Test successful handler
	successHandler := func(ctx context.Context, msg *Message) error {
		return nil
	}
	hr.register(testTopic, successHandler)

	err := hr.processMessage(context.Background(), testMsg)
	assert.NoError(t, err)

	// Give events time to fire
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	assert.Len(t, handledEvents, 1)
	assert.Contains(t, handledEvents[0], "true")
	assert.Empty(t, errorEvents)
	mu.Unlock()

	// Test failing handler
	errorHandler := func(ctx context.Context, msg *Message) error {
		return errors.New("test handler error")
	}
	hr.register(testTopic, errorHandler)

	err = hr.processMessage(context.Background(), testMsg)
	assert.Error(t, err)

	// Give events time to fire
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	assert.Len(t, handledEvents, 2) // One more handled event
	assert.Contains(t, handledEvents[1], "false")
	assert.Len(t, errorEvents, 1) // One error event
	assert.Contains(t, errorEvents[0], "test handler error")
	mu.Unlock()
}

func TestHandlerMetrics(t *testing.T) {
	hr := createTestHandlerRegistry()
	testTopic := "test-topic"
	testMsg := createTestMessage(testTopic, []byte("test"))

	// Test with metrics
	successHandler := func(ctx context.Context, msg *Message) error {
		// Add small delay to test duration recording
		time.Sleep(1 * time.Millisecond)
		return nil
	}
	hr.register(testTopic, successHandler)

	err := hr.processMessage(context.Background(), testMsg)
	assert.NoError(t, err)

	// Test with nil metrics (should not panic)
	hr.metrics = nil
	err = hr.processMessage(context.Background(), testMsg)
	assert.NoError(t, err)
}

func TestSafeMessageProcessing(t *testing.T) {
	hr := createTestHandlerRegistry()
	testMsg := createTestMessage("test-topic", []byte("test"))

	// Test normal execution
	normalHandler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	err := hr.safeProcessMessage(context.Background(), testMsg, normalHandler)
	assert.NoError(t, err)

	// Test panic recovery
	panicHandler := func(ctx context.Context, msg *Message) error {
		panic("intentional panic")
	}

	err = hr.safeProcessMessage(context.Background(), testMsg, panicHandler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler panicked")
	assert.Contains(t, err.Error(), "intentional panic")

	// Test error return
	errorHandler := func(ctx context.Context, msg *Message) error {
		return errors.New("intentional error")
	}

	err = hr.safeProcessMessage(context.Background(), testMsg, errorHandler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "intentional error")
}

func TestHandlerTimeout(t *testing.T) {
	hr := createTestHandlerRegistry()
	testTopic := "test-topic"
	testMsg := createTestMessage(testTopic, []byte("test"))

	// Create a handler that takes longer than the timeout
	slowHandler := func(ctx context.Context, msg *Message) error {
		select {
		case <-time.After(35 * time.Second): // Longer than handler timeout (30s)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	hr.register(testTopic, slowHandler)

	// Use a shorter context timeout for testing
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := hr.processMessage(timeoutCtx, testMsg)
	duration := time.Since(start)

	// Should timeout quickly
	assert.True(t, duration < 5*time.Second, "Handler should timeout quickly")
	assert.Error(t, err)
}

func TestConcurrentHandlerRegistration(t *testing.T) {
	hr := createTestHandlerRegistry()
	const numGoroutines = 100

	var wg sync.WaitGroup

	// Concurrent registrations
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			topic := fmt.Sprintf("topic-%d", id)
			handler := func(ctx context.Context, msg *Message) error {
				return nil
			}

			hr.register(topic, handler)

			// Verify handler is registered
			retrievedHandler := hr.get(topic)
			assert.NotNil(t, retrievedHandler)

			// Unregister
			hr.unregister(topic)

			// Verify handler is removed
			retrievedHandler = hr.get(topic)
			assert.Nil(t, retrievedHandler)
		}(i)
	}

	wg.Wait()

	// Verify no handlers remain
	for i := 0; i < numGoroutines; i++ {
		topic := fmt.Sprintf("topic-%d", i)
		handler := hr.get(topic)
		assert.Nil(t, handler)
	}
}

func TestMessageCopyInAsyncHandler(t *testing.T) {
	logger := testLogger()
	originalData := []byte("original message")
	testMsg := createTestMessage("test-topic", originalData)

	var receivedData []byte
	var mu sync.Mutex
	processedCh := make(chan struct{})

	baseHandler := func(ctx context.Context, msg *Message) error {
		mu.Lock()
		receivedData = msg.Data
		mu.Unlock()
		close(processedCh)
		return nil
	}

	asyncHandler := AsyncHandler(logger, baseHandler)

	// Modify original data after calling async handler
	err := asyncHandler(context.Background(), testMsg)
	assert.NoError(t, err)

	// Modify the original data
	originalData[0] = 'X'

	// Wait for processing
	select {
	case <-processedCh:
		// Message processed
	case <-time.After(5 * time.Second):
		t.Fatal("Async handler did not process message within timeout")
	}

	// Verify received data is not affected by modification to original
	mu.Lock()
	assert.NotEqual(t, originalData[0], receivedData[0])
	assert.Equal(t, byte('o'), receivedData[0]) // Should still be 'o' from "original"
	mu.Unlock()
}

// Benchmark tests for handler performance
func BenchmarkHandlerProcessing(b *testing.B) {
	hr := createTestHandlerRegistry()
	testTopic := "bench-topic"

	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}
	hr.register(testTopic, handler)

	testMsg := createTestMessage(testTopic, []byte("benchmark message"))
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = hr.processMessage(ctx, testMsg)
		}
	})
}

func BenchmarkHandlerRegistration(b *testing.B) {
	hr := createTestHandlerRegistry()

	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic := fmt.Sprintf("topic-%d", i)
		hr.register(topic, handler)
	}
}

func BenchmarkAsyncHandler(b *testing.B) {
	logger := testLogger()
	baseHandler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	asyncHandler := AsyncHandler(logger, baseHandler)
	testMsg := createTestMessage("bench-topic", []byte("benchmark message"))
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = asyncHandler(ctx, testMsg)
		}
	})
}
