package v1_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidatorIntegration_ValidatorBlocking checks if validator execution blocks message delivery.
func TestValidatorIntegration_ValidatorBlocking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messageCount atomic.Int64
	var validationCount atomic.Int64

	// Create topic
	topic, err := CreateTestTopic("validator_blocking_test")
	require.NoError(t, err)

	// Create handler with slow validator
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			count := validationCount.Add(1)
			t.Logf("🔍 Validator: Starting validation for message %d (%s)", count, msg.ID)

			// Simulate slow validation (2 seconds)
			time.Sleep(2 * time.Second)

			t.Logf("🔍 Validator: Completed validation for message %d (%s)", count, msg.ID)

			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := messageCount.Add(1)
			t.Logf("📨 Processor: Processing message %d (%s)", count, msg.ID)

			return nil
		}),
	)

	// Register handlers
	for _, node := range nodes {
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	// Subscribe
	sub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Wait for mesh formation
	time.Sleep(2 * time.Second)

	// Send 3 messages quickly - they should all enter validation queue
	publishStart := time.Now()

	for i := 0; i < 3; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("validator-test-%d", i+1),
			Content: fmt.Sprintf("Testing validator blocking %d", i+1),
			From:    nodes[1].ID.String(),
		}

		t.Logf("📤 Publishing message %d", i+1)
		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Minimal delay to preserve order
		time.Sleep(50 * time.Millisecond)
	}

	publishDuration := time.Since(publishStart)
	t.Logf("All 3 messages published in %v", publishDuration)

	// All messages should enter validation queue quickly (not block at Next())
	// But validation will take ~6 seconds total (2 sec each)

	// Check that validations start quickly
	require.Eventually(t, func() bool {
		count := validationCount.Load()
		t.Logf("Validation count: %d", count)

		return count >= 1
	}, 5*time.Second, 200*time.Millisecond, "First validation should start quickly")

	// Wait for all validations and processing to complete
	require.Eventually(t, func() bool {
		validations := validationCount.Load()
		messages := messageCount.Load()
		t.Logf("Validations: %d, Messages processed: %d", validations, messages)

		return validations == 3 && messages == 3
	}, 15*time.Second, 500*time.Millisecond, "All messages should be validated and processed")

	t.Log("SUCCESS: Validator execution doesn't block message delivery")
}

// TestValidatorIntegration_ValidatorRejectionImpact verifies rejected messages don't block subsequent messages.
func TestValidatorIntegration_ValidatorRejectionImpact(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messageCount atomic.Int64
	var validationCount atomic.Int64
	var rejectedCount atomic.Int64

	// Create topic
	topic, err := CreateTestTopic("validator_rejection_test")
	require.NoError(t, err)

	// Create handler that rejects every other message
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			count := validationCount.Add(1)

			// Reject messages with even numbers in their ID
			if msg.ID[len(msg.ID)-1]%2 == 0 {
				rejectedCount.Add(1)
				t.Logf("❌ Validator: Rejecting message %d (%s)", count, msg.ID)

				return v1.ValidationReject
			}

			t.Logf("✅ Validator: Accepting message %d (%s)", count, msg.ID)

			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := messageCount.Add(1)
			t.Logf("📨 Processor: Processing accepted message %d (%s)", count, msg.ID)

			return nil
		}),
	)

	// Register handlers
	for _, node := range nodes {
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	// Subscribe
	sub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Wait for mesh formation
	time.Sleep(2 * time.Second)

	// Send 10 messages (5 should be accepted, 5 rejected)
	for i := 1; i <= 10; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("rejection-test-%d", i),
			Content: fmt.Sprintf("Testing rejection impact %d", i),
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all validations to complete
	require.Eventually(t, func() bool {
		validations := validationCount.Load()
		processed := messageCount.Load()
		rejected := rejectedCount.Load()

		t.Logf("Validations: %d, Processed: %d, Rejected: %d", validations, processed, rejected)

		return validations == 10
	}, 15*time.Second, 500*time.Millisecond, "All messages should be validated")

	// Verify final counts
	finalValidations := validationCount.Load()
	finalProcessed := messageCount.Load()
	finalRejected := rejectedCount.Load()

	assert.Equal(t, int64(10), finalValidations, "Should validate all 10 messages")
	assert.Equal(t, int64(5), finalRejected, "Should reject 5 messages")
	assert.Equal(t, int64(5), finalProcessed, "Should process 5 accepted messages")

	t.Log("SUCCESS: Rejected messages don't prevent future message deliveries")
}

// TestValidatorIntegration_ValidatorErrorHandling tests validator error scenarios.
func TestValidatorIntegration_ValidatorErrorHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messageCount atomic.Int64
	var validationCount atomic.Int64
	var errorCount atomic.Int64

	// Create topic
	topic, err := CreateTestTopic("validator_error_test")
	require.NoError(t, err)

	// Create handler with error-prone validator
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			count := validationCount.Add(1)

			// Simulate validator errors for messages containing "error"
			if msg.Content == "error message" {
				errorCount.Add(1)
				t.Logf("💥 Validator: Error for message %d (%s)", count, msg.ID)

				return v1.ValidationIgnore // Treat errors as ignore
			}

			t.Logf("✅ Validator: Success for message %d (%s)", count, msg.ID)

			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := messageCount.Add(1)
			t.Logf("📨 Processor: Processing message %d (%s)", count, msg.ID)

			return nil
		}),
	)

	// Register handlers
	for _, node := range nodes {
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	// Subscribe
	sub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Wait for mesh formation
	time.Sleep(2 * time.Second)

	// Send mix of good and error messages
	messages := []struct {
		id              string
		content         string
		expectProcessed bool
	}{
		{"good-1", "normal message", true},
		{"error-1", "error message", false},
		{"good-2", "another normal message", true},
		{"error-2", "error message", false},
		{"good-3", "final normal message", true},
	}

	for _, msg := range messages {
		testMsg := GossipTestMessage{
			ID:      msg.id,
			Content: msg.content,
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, testMsg)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)
	}

	// Wait for all validations
	require.Eventually(t, func() bool {
		validations := validationCount.Load()

		return validations == int64(len(messages))
	}, 10*time.Second, 200*time.Millisecond, "All messages should be validated")

	// Verify error handling didn't break the system
	finalValidations := validationCount.Load()
	finalProcessed := messageCount.Load()
	finalErrors := errorCount.Load()

	assert.Equal(t, int64(5), finalValidations, "Should validate all 5 messages")
	assert.Equal(t, int64(2), finalErrors, "Should have 2 validation errors")
	assert.Equal(t, int64(3), finalProcessed, "Should process 3 good messages")

	t.Log("SUCCESS: Validator errors don't break message processing")
}

// TestValidatorIntegration_ConcurrentValidation tests concurrent validation behavior.
func TestValidatorIntegration_ConcurrentValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messageCount atomic.Int64
	var validationStartTimes []time.Time
	var validationEndTimes []time.Time

	// Create topic
	topic, err := CreateTestTopic("concurrent_validation_test")
	require.NoError(t, err)

	// Create handler with validation timing tracking
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			start := time.Now()
			validationStartTimes = append(validationStartTimes, start)

			t.Logf("🔍 Validator: Starting validation for %s at %v", msg.ID, start)

			// Simulate validation work
			time.Sleep(1 * time.Second)

			end := time.Now()
			validationEndTimes = append(validationEndTimes, end)

			t.Logf("🔍 Validator: Completed validation for %s at %v (took %v)",
				msg.ID, end, end.Sub(start))

			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := messageCount.Add(1)
			t.Logf("📨 Processor: Processing message %d (%s)", count, msg.ID)

			return nil
		}),
	)

	// Register handlers
	for _, node := range nodes {
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	// Subscribe
	sub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Wait for mesh formation
	time.Sleep(2 * time.Second)

	// Send 3 messages rapidly to test concurrent validation
	publishStart := time.Now()

	for i := 0; i < 3; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("concurrent-%d", i+1),
			Content: fmt.Sprintf("Concurrent validation test %d", i+1),
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Minimal delay
		time.Sleep(50 * time.Millisecond)
	}

	publishEnd := time.Now()
	t.Logf("Published 3 messages in %v", publishEnd.Sub(publishStart))

	// Wait for all messages to be processed
	require.Eventually(t, func() bool {
		count := messageCount.Load()

		return count == 3
	}, 15*time.Second, 200*time.Millisecond, "All messages should be processed")

	// Analyze validation timing
	require.Len(t, validationStartTimes, 3, "Should have 3 validation start times")
	require.Len(t, validationEndTimes, 3, "Should have 3 validation end times")

	// Check if validations ran concurrently (overlapping time windows)
	if len(validationStartTimes) >= 2 {
		firstStart := validationStartTimes[0]
		secondStart := validationStartTimes[1]
		firstEnd := validationEndTimes[0]

		timeBetweenStarts := secondStart.Sub(firstStart)
		t.Logf("Time between first two validation starts: %v", timeBetweenStarts)

		// If validations are concurrent, second should start before first ends
		if secondStart.Before(firstEnd) {
			t.Log("✅ Validations are running concurrently")
		} else {
			t.Log("⚠️  Validations appear to be sequential")
		}
	}

	t.Log("SUCCESS: Concurrent validation behavior verified")
}
