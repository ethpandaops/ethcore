package v1_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessorState_ContextHandling verifies processor context isn't being cancelled prematurely
func TestProcessorState_ContextHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Track context state
	var contextChecks []bool
	var mu sync.Mutex

	// Create topic
	topic, err := CreateTestTopic("context_test")
	require.NoError(t, err)

	// Create handler that checks context state
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			// Check if context is still valid
			mu.Lock()
			contextChecks = append(contextChecks, ctx.Err() == nil)
			mu.Unlock()

			t.Logf("Processing message %s, context valid: %v", msg.ID, ctx.Err() == nil)
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

	// Send multiple messages
	for i := 0; i < 5; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("context-test-%d", i),
			Content: fmt.Sprintf("Testing context %d", i),
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Small delay between messages
		time.Sleep(200 * time.Millisecond)
	}

	// Wait for all messages to be processed
	require.Eventually(t, func() bool {
		mu.Lock()
		count := len(contextChecks)
		mu.Unlock()
		return count == 5
	}, 10*time.Second, 100*time.Millisecond, "Expected all messages to be processed")

	// Verify all context checks were valid
	mu.Lock()
	for i, valid := range contextChecks {
		assert.True(t, valid, "Context should be valid for message %d", i)
	}
	mu.Unlock()

	t.Log("SUCCESS: Processor context remains valid for multiple messages")
}

// TestProcessorState_ConcurrentMessageProcessing tests if concurrent processing blocks subsequent messages
func TestProcessorState_ConcurrentMessageProcessing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Message tracking
	var messagesReceived atomic.Int64
	var processingTimes []time.Time
	var mu sync.Mutex

	// Create topic
	topic, err := CreateTestTopic("concurrent_test")
	require.NoError(t, err)

	// Create handler that takes time to process
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			received := messagesReceived.Add(1)

			mu.Lock()
			processingTimes = append(processingTimes, time.Now())
			mu.Unlock()

			t.Logf("Started processing message %s (#%d)", msg.ID, received)

			// Simulate slow processing (but not too slow to avoid timeout)
			time.Sleep(500 * time.Millisecond)

			t.Logf("Finished processing message %s (#%d)", msg.ID, received)
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

	// Send 5 messages rapidly
	publishStart := time.Now()
	for i := 0; i < 5; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("concurrent-test-%d", i),
			Content: fmt.Sprintf("Testing concurrent processing %d", i),
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Minimal delay to ensure messages are published in order
		time.Sleep(50 * time.Millisecond)
	}
	publishEnd := time.Now()

	t.Logf("Published 5 messages in %v", publishEnd.Sub(publishStart))

	// Wait for all messages to be received (not necessarily processed)
	require.Eventually(t, func() bool {
		count := messagesReceived.Load()
		t.Logf("Received %d/5 messages", count)
		return count == 5
	}, 15*time.Second, 100*time.Millisecond, "Expected all messages to be received")

	// Check that messages were received quickly (delivery shouldn't block on processing)
	mu.Lock()
	if len(processingTimes) >= 2 {
		// The first two messages should be received close together
		timeDiff := processingTimes[1].Sub(processingTimes[0])
		t.Logf("Time between first two message receptions: %v", timeDiff)

		// If processing were blocking, this would be >500ms. We expect <200ms for good delivery
		assert.Less(t, timeDiff, 200*time.Millisecond,
			"Messages should be received quickly even with slow processing")
	}
	mu.Unlock()

	t.Log("SUCCESS: Concurrent message processing doesn't block message delivery")
}

// TestProcessorState_ProcessorGoroutineLifecycle tests processor goroutine lifecycle
func TestProcessorState_ProcessorGoroutineLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messageCount atomic.Int64

	// Create topic
	topic, err := CreateTestTopic("lifecycle_test")
	require.NoError(t, err)

	// Create handler
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			messageCount.Add(1)
			t.Logf("Processed message: %s", msg.ID)
			return nil
		}),
	)

	// Register handlers
	for _, node := range nodes {
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	// Count goroutines before subscription
	initialGoroutines := runtime.NumGoroutine()

	// Subscribe
	sub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)

	// Wait for mesh formation
	time.Sleep(2 * time.Second)

	// Send a message to ensure processor is running
	msg := GossipTestMessage{
		ID:      "lifecycle-test-1",
		Content: "Testing lifecycle",
		From:    nodes[1].ID.String(),
	}

	err = v1.Publish(nodes[1].Gossipsub, topic, msg)
	require.NoError(t, err)

	// Wait for message processing
	require.Eventually(t, func() bool {
		return messageCount.Load() == 1
	}, 5*time.Second, 100*time.Millisecond)

	// Cancel subscription
	t.Log("Cancelling subscription...")
	sub.Cancel()

	// Wait for cleanup
	time.Sleep(2 * time.Second)

	// Check goroutine count (should not increase significantly)
	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - initialGoroutines

	t.Logf("Goroutines before: %d, after: %d, difference: %d",
		initialGoroutines, finalGoroutines, goroutineDiff)

	// Allow some tolerance for test infrastructure goroutines
	assert.Less(t, goroutineDiff, 10, "Should not have significant goroutine leaks")

	t.Log("SUCCESS: Processor goroutine lifecycle managed correctly")
}

// TestProcessorState_MessageOrderPreservation tests that message processing preserves order
func TestProcessorState_MessageOrderPreservation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Track processing order
	var processedOrder []string
	var mu sync.Mutex

	// Create topic
	topic, err := CreateTestTopic("order_test")
	require.NoError(t, err)

	// Create handler that tracks order
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			mu.Lock()
			processedOrder = append(processedOrder, msg.ID)
			mu.Unlock()

			t.Logf("Processed message: %s", msg.ID)
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

	// Send messages in specific order
	expectedOrder := []string{"order-1", "order-2", "order-3", "order-4", "order-5"}

	for _, id := range expectedOrder {
		msg := GossipTestMessage{
			ID:      id,
			Content: fmt.Sprintf("Order test message %s", id),
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Small delay to ensure ordering
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all messages to be processed
	require.Eventually(t, func() bool {
		mu.Lock()
		count := len(processedOrder)
		mu.Unlock()
		return count == len(expectedOrder)
	}, 10*time.Second, 100*time.Millisecond, "Expected all messages to be processed")

	// Verify order is preserved
	mu.Lock()
	assert.Equal(t, expectedOrder, processedOrder, "Message processing order should be preserved")
	mu.Unlock()

	t.Log("SUCCESS: Message processing order is preserved")
}
