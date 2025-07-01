package v1_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorInjection_ValidationErrors tests validation error scenarios
func TestErrorInjection_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h)
	require.NoError(t, err)
	defer gs.Stop()

	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("validation-errors", encoder)
	require.NoError(t, err)

	// Track validation results
	var (
		validationCalls  atomic.Int32
		acceptCount      atomic.Int32
		rejectCount      atomic.Int32
		ignoreCount      atomic.Int32
		processingCount  atomic.Int32
		validationPanics atomic.Int32
	)

	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			defer func() {
				if r := recover(); r != nil {
					validationPanics.Add(1)
				}
			}()

			validationCalls.Add(1)

			// Different validation behaviors based on message ID
			switch msg.ID {
			case "accept":
				acceptCount.Add(1)
				return v1.ValidationAccept
			case "reject":
				rejectCount.Add(1)
				return v1.ValidationReject
			case "ignore":
				ignoreCount.Add(1)
				return v1.ValidationIgnore
			case "panic":
				panic("validation panic test")
			case "slow":
				// Simulate slow validation
				select {
				case <-time.After(100 * time.Millisecond):
				case <-ctx.Done():
					return v1.ValidationIgnore
				}
				return v1.ValidationAccept
			default:
				return v1.ValidationAccept
			}
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			processingCount.Add(1)
			return nil
		}),
	)

	err = registerTopic(gs, topic, handler)
	require.NoError(t, err)

	sub, err := subscribeTopic(gs, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Test cases
	testCases := []struct {
		id               string
		expectProcessing bool
	}{
		{"accept", true},
		{"reject", false},
		{"ignore", false},
		{"panic", false},
		{"slow", true},
		{"default", true},
	}

	// Publish test messages
	for _, tc := range testCases {
		msg := GossipTestMessage{ID: tc.id, Content: "test"}
		err := publishTopic(gs, topic, msg)
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify results
	assert.Equal(t, int32(len(testCases)), validationCalls.Load(), "Validation calls mismatch")
	assert.Equal(t, int32(2), acceptCount.Load(), "Accept count mismatch")
	assert.Equal(t, int32(1), rejectCount.Load(), "Reject count mismatch")
	assert.Equal(t, int32(1), ignoreCount.Load(), "Ignore count mismatch")
	assert.Equal(t, int32(1), validationPanics.Load(), "Panic count mismatch")

	// Only accepted messages should be processed
	expectedProcessed := int32(3) // accept, slow, default
	assert.Equal(t, expectedProcessed, processingCount.Load(), "Processing count mismatch")
}

// TestErrorInjection_ProcessingErrors tests processing error scenarios
func TestErrorInjection_ProcessingErrors(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h)
	require.NoError(t, err)
	defer gs.Stop()

	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("processing-errors", encoder)
	require.NoError(t, err)

	// Track processing
	var (
		processingCalls   atomic.Int32
		processingErrors  atomic.Int32
		processingPanics  atomic.Int32
		processingSuccess atomic.Int32
	)

	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			defer func() {
				if r := recover(); r != nil {
					processingPanics.Add(1)
				}
			}()

			processingCalls.Add(1)

			switch msg.ID {
			case "success":
				processingSuccess.Add(1)
				return nil
			case "error":
				processingErrors.Add(1)
				return errors.New("processing error")
			case "panic":
				panic("processing panic test")
			case "timeout":
				// Simulate timeout
				select {
				case <-time.After(200 * time.Millisecond):
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			case "nil-error":
				processingErrors.Add(1)
				return fmt.Errorf("nil pointer: %w", nil)
			default:
				return nil
			}
		}),
	)

	err = registerTopic(gs, topic, handler)
	require.NoError(t, err)

	sub, err := subscribeTopic(gs, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Test messages
	messages := []string{"success", "error", "panic", "timeout", "nil-error", "default"}

	for _, id := range messages {
		msg := GossipTestMessage{ID: id, Content: "test"}
		err := publishTopic(gs, topic, msg)
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify results
	assert.Equal(t, int32(len(messages)), processingCalls.Load(), "Processing calls mismatch")
	assert.Equal(t, int32(1), processingSuccess.Load(), "Success count mismatch")
	assert.Equal(t, int32(2), processingErrors.Load(), "Error count mismatch")
	assert.Equal(t, int32(1), processingPanics.Load(), "Panic count mismatch")

	// v1.Gossipsub should still be running
	assert.True(t, gs.IsStarted())
}

// TestErrorInjection_EncodingErrors tests encoding/decoding error scenarios
func TestErrorInjection_EncodingErrors(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h)
	require.NoError(t, err)
	defer gs.Stop()

	// Create encoder that fails on specific conditions
	encoder := &conditionalFailEncoder{
		failOnEncode: make(map[string]bool),
		failOnDecode: make(map[string]bool),
	}

	topic, err := v1.NewTopic[GossipTestMessage]("encoding-errors", encoder)
	require.NoError(t, err)

	var (
		decodeCalls     atomic.Int32
		decodeErrors    atomic.Int32
		processingCalls atomic.Int32
	)

	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithDecoder(func(data []byte) (GossipTestMessage, error) {
			decodeCalls.Add(1)

			// Simulate various decode scenarios
			if len(data) == 0 {
				decodeErrors.Add(1)
				return GossipTestMessage{}, errors.New("empty data")
			}

			if string(data) == "corrupt" {
				decodeErrors.Add(1)
				return GossipTestMessage{}, errors.New("corrupt data")
			}

			if string(data) == "panic" {
				panic("decode panic")
			}

			// Normal decode
			return encoder.Decode(data)
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			processingCalls.Add(1)
			return nil
		}),
	)

	err = registerTopic(gs, topic, handler)
	require.NoError(t, err)

	sub, err := subscribeTopic(gs, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Test encode failures
	encoder.failOnEncode["fail-encode"] = true
	msg1 := GossipTestMessage{ID: "fail-encode", Content: "should fail"}
	err = publishTopic(gs, topic, msg1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "encode error")

	// Test successful encode but various decode scenarios
	// We'll manually publish raw data to test decode paths
	libp2pTopic := topic.Name()
	pubsubInstance := gs.GetPubSub()

	// Empty data
	err = pubsubInstance.Publish(libp2pTopic, []byte{}, pubsub.WithReadiness(pubsub.MinTopicSize(0)))
	require.NoError(t, err)

	// Corrupt data
	err = pubsubInstance.Publish(libp2pTopic, []byte("corrupt"), pubsub.WithReadiness(pubsub.MinTopicSize(0)))
	require.NoError(t, err)

	// Valid data
	validMsg := GossipTestMessage{ID: "valid", Content: "should work"}
	validData, err := encoder.Encode(validMsg)
	require.NoError(t, err)
	err = pubsubInstance.Publish(libp2pTopic, validData, pubsub.WithReadiness(pubsub.MinTopicSize(0)))
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify decode was attempted for all messages
	assert.GreaterOrEqual(t, decodeCalls.Load(), int32(3), "Decode calls mismatch")
	assert.GreaterOrEqual(t, decodeErrors.Load(), int32(2), "Decode errors mismatch")
	assert.GreaterOrEqual(t, processingCalls.Load(), int32(1), "Only valid message should be processed")
}

// TestErrorInjection_ConcurrentErrors tests error handling under concurrent load
func TestErrorInjection_ConcurrentErrors(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h, v1.WithValidationConcurrency(10))
	require.NoError(t, err)
	defer gs.Stop()

	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("concurrent-errors", encoder)
	require.NoError(t, err)

	var (
		validationErrors atomic.Int32
		processingErrors atomic.Int32
		successCount     atomic.Int32
	)

	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			// Random validation failures
			if msg.ID[0]%3 == 0 {
				validationErrors.Add(1)
				return v1.ValidationReject
			}
			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			// Random processing failures
			if msg.ID[0]%5 == 0 {
				processingErrors.Add(1)
				return errors.New("random processing error")
			}
			successCount.Add(1)
			return nil
		}),
	)

	err = registerTopic(gs, topic, handler)
	require.NoError(t, err)

	sub, err := subscribeTopic(gs, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Concurrent publishers with various error conditions
	numPublishers := 10
	messagesPerPublisher := 100
	var wg sync.WaitGroup

	for p := 0; p < numPublishers; p++ {
		wg.Add(1)
		go func(publisherID int) {
			defer wg.Done()
			for i := 0; i < messagesPerPublisher; i++ {
				msg := GossipTestMessage{
					ID:      fmt.Sprintf("%c%d-%d", byte('A'+publisherID), publisherID, i),
					Content: fmt.Sprintf("Message %d from publisher %d", i, publisherID),
				}

				// Ignore publish errors in this test
				_ = publishTopic(gs, topic, msg)

				// Small delay to avoid overwhelming
				if i%10 == 0 {
					time.Sleep(time.Millisecond)
				}
			}
		}(p)
	}

	wg.Wait()

	// Wait for all messages to be processed
	time.Sleep(2 * time.Second)

	// Log results
	t.Logf("Validation errors: %d", validationErrors.Load())
	t.Logf("Processing errors: %d", processingErrors.Load())
	t.Logf("Success count: %d", successCount.Load())

	// Verify system is still healthy
	assert.True(t, gs.IsStarted())

	// Test that we can still process messages after all the errors
	finalMsg := GossipTestMessage{ID: "final", Content: "final test"}
	err = publishTopic(gs, topic, finalMsg)
	require.NoError(t, err)
}

// TestErrorInjection_ResourceExhaustion tests behavior under resource constraints
func TestErrorInjection_ResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping resource exhaustion test in short mode")
	}

	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	// Create gossipsub with limited resources
	gs, err := v1.New(ctx, h,
		v1.WithMaxMessageSize(1024),     // Small message size
		v1.WithValidationConcurrency(2), // Limited concurrency
	)
	require.NoError(t, err)
	defer gs.Stop()

	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("resource-test", encoder)
	require.NoError(t, err)

	var (
		slowValidations atomic.Int32
		blocked         atomic.Int32
		processed       atomic.Int32
	)

	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			slowValidations.Add(1)

			// Simulate slow validation to create backpressure
			select {
			case <-time.After(50 * time.Millisecond):
			case <-ctx.Done():
				blocked.Add(1)
				return v1.ValidationIgnore
			}

			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			processed.Add(1)
			return nil
		}),
	)

	err = registerTopic(gs, topic, handler)
	require.NoError(t, err)

	sub, err := subscribeTopic(gs, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Flood with messages
	numMessages := 100
	start := time.Now()

	for i := 0; i < numMessages; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("msg-%d", i),
			Content: fmt.Sprintf("Resource test message %d", i),
		}

		// Continue publishing even if some fail
		_ = publishTopic(gs, topic, msg)
	}

	// Wait for processing to complete or timeout
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for processed.Load() < int32(numMessages) {
		select {
		case <-timeout:
			goto done
		case <-ticker.C:
			// Continue waiting
		}
	}

done:
	duration := time.Since(start)

	t.Logf("Resource exhaustion test results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Messages sent: %d", numMessages)
	t.Logf("  Slow validations: %d", slowValidations.Load())
	t.Logf("  Blocked: %d", blocked.Load())
	t.Logf("  Processed: %d", processed.Load())

	// System should still be functional
	assert.True(t, gs.IsStarted())
}

// conditionalFailEncoder is an encoder that can be configured to fail
type conditionalFailEncoder struct {
	mu           sync.RWMutex
	failOnEncode map[string]bool
	failOnDecode map[string]bool
}

func (e *conditionalFailEncoder) Encode(msg GossipTestMessage) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.failOnEncode[msg.ID] {
		return nil, fmt.Errorf("encode error for ID: %s", msg.ID)
	}

	// Simple encoding
	return []byte(msg.ID + "|" + msg.Content), nil
}

func (e *conditionalFailEncoder) Decode(data []byte) (GossipTestMessage, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	str := string(data)
	for i, b := range data {
		if b == '|' {
			msg := GossipTestMessage{
				ID:      str[:i],
				Content: str[i+1:],
			}

			if e.failOnDecode[msg.ID] {
				return GossipTestMessage{}, fmt.Errorf("decode error for ID: %s", msg.ID)
			}

			return msg, nil
		}
	}

	return GossipTestMessage{}, errors.New("invalid message format")
}

// TestErrorInjection_HandlerPanic tests recovery from handler panics
func TestErrorInjection_HandlerPanic(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h)
	require.NoError(t, err)
	defer gs.Stop()

	encoder := &GossipTestEncoder{}

	// Test panic in handler registration
	t.Run("registration panic", func(t *testing.T) {
		topic, err := v1.NewTopic[GossipTestMessage]("panic-test", encoder)
		require.NoError(t, err)

		// This should not panic the whole system
		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
				if msg.ID == "init-panic" {
					panic("initialization panic")
				}
				return v1.ValidationAccept
			}),
		)

		err = registerTopic(gs, topic, handler)
		require.NoError(t, err)

		// System should still be functional
		assert.True(t, gs.IsStarted())
	})

	// Test panic during message processing
	t.Run("processing panic chain", func(t *testing.T) {
		topic, err := v1.NewTopic[GossipTestMessage]("panic-chain", encoder)
		require.NoError(t, err)

		panicCount := 0
		recoveryCount := 0

		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
				defer func() {
					if r := recover(); r != nil {
						recoveryCount++
						panic("re-panic in validator")
					}
				}()

				if msg.ID == "validator-panic" {
					panicCount++
					panic("validator panic")
				}
				return v1.ValidationAccept
			}),
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				defer func() {
					if r := recover(); r != nil {
						recoveryCount++
						panic("re-panic in processor")
					}
				}()

				if msg.ID == "processor-panic" {
					panicCount++
					panic("processor panic")
				}
				return nil
			}),
		)

		err = registerTopic(gs, topic, handler)
		require.NoError(t, err)

		sub, err := subscribeTopic(gs, topic)
		require.NoError(t, err)
		defer sub.Cancel()

		// Trigger panics
		messages := []string{"validator-panic", "processor-panic", "normal"}
		for _, id := range messages {
			msg := GossipTestMessage{ID: id, Content: "panic test"}
			err := publishTopic(gs, topic, msg)
			require.NoError(t, err)
		}

		time.Sleep(500 * time.Millisecond)

		// System should survive all panics
		assert.True(t, gs.IsStarted())
	})
}
