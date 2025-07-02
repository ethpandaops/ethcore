package v1_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// TestMinimalReproduction_BeaconBlockBlocking is the minimal test to reproduce the specific
// issue where libp2p's Next() method blocks indefinitely after receiving one beacon block message.
// This test directly mirrors the reported problem scenario.
func TestMinimalReproduction_BeaconBlockBlocking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes to simulate the real scenario
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Message counter to track exactly how many messages are processed
	var messageCount atomic.Int64

	// Create a beacon block topic (using the exact pattern from the reported issue)
	topic, err := CreateTestTopic("beacon_block")
	require.NoError(t, err)

	// Create handler with minimal processing to isolate the issue
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := messageCount.Add(1)

			// Add detailed logging to track the exact behavior
			t.Logf("=== PROCESSOR: Received beacon block message #%d ===", count)
			t.Logf("Message ID: %s", msg.ID)
			t.Logf("From peer: %s", from.String()[:8])
			t.Logf("Context error: %v", ctx.Err())
			t.Logf("=== END PROCESSOR MESSAGE #%d ===", count)

			return nil
		}),
	)

	// Register handlers on both nodes
	for i, node := range nodes {
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
		t.Logf("Registered handler on node %d", i)
	}

	// Subscribe to beacon block topic on node 0 (receiver)
	t.Log("=== SUBSCRIBING TO BEACON BLOCK TOPIC ===")
	sub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Wait for gossipsub mesh formation
	t.Log("=== WAITING FOR MESH FORMATION ===")
	time.Sleep(3 * time.Second)

	// Verify mesh is ready
	activeTopics0 := nodes[0].Gossipsub.ActiveTopics()
	activeTopics1 := nodes[1].Gossipsub.ActiveTopics()
	t.Logf("Node 0 active topics: %v", activeTopics0)
	t.Logf("Node 1 active topics: %v", activeTopics1)
	require.Greater(t, len(activeTopics0), 0, "Node 0 should have active topics")

	// ============================================================
	// CRITICAL TEST: Send first beacon block message
	// ============================================================
	t.Log("=== PUBLISHING FIRST BEACON BLOCK MESSAGE ===")
	firstMessage := GossipTestMessage{
		ID:      "beacon-block-1",
		Content: "First beacon block that should work",
		From:    nodes[1].ID.String(),
	}

	err = v1.Publish(nodes[1].Gossipsub, topic, firstMessage)
	require.NoError(t, err)
	t.Log("First message published successfully")

	// Wait for first message to be processed
	t.Log("=== WAITING FOR FIRST MESSAGE PROCESSING ===")
	require.Eventually(t, func() bool {
		count := messageCount.Load()
		t.Logf("Current message count: %d", count)
		return count == 1
	}, 10*time.Second, 200*time.Millisecond, "First message should be processed")

	t.Log("=== FIRST MESSAGE PROCESSED SUCCESSFULLY ===")

	// Small delay to simulate real-world timing
	time.Sleep(1 * time.Second)

	// ============================================================
	// CRITICAL TEST: Send second beacon block message
	// This is where the blocking issue typically manifests
	// ============================================================
	t.Log("=== PUBLISHING SECOND BEACON BLOCK MESSAGE ===")
	secondMessage := GossipTestMessage{
		ID:      "beacon-block-2",
		Content: "Second beacon block - this is where blocking occurs",
		From:    nodes[1].ID.String(),
	}

	publishStart := time.Now()
	err = v1.Publish(nodes[1].Gossipsub, topic, secondMessage)
	require.NoError(t, err)
	publishDuration := time.Since(publishStart)

	t.Logf("Second message published in %v", publishDuration)

	// This is the critical assertion - if Next() is blocking, this will timeout
	t.Log("=== WAITING FOR SECOND MESSAGE PROCESSING (CRITICAL TEST) ===")

	// Use a shorter timeout to fail fast if blocking occurs
	startWait := time.Now()
	require.Eventually(t, func() bool {
		count := messageCount.Load()
		elapsed := time.Since(startWait)
		t.Logf("Waiting for second message... count=%d, elapsed=%v", count, elapsed)

		// Log subscription state
		if !sub.IsCancelled() {
			t.Logf("Subscription still active for topic: %s", sub.Topic())
		}

		return count == 2
	}, 20*time.Second, 500*time.Millisecond,
		"CRITICAL: Second message should be processed - if this fails, Next() is likely blocking!")

	waitDuration := time.Since(startWait)
	t.Logf("=== SECOND MESSAGE PROCESSED AFTER %v ===", waitDuration)

	// ============================================================
	// VERIFICATION: Send a third message to confirm recovery
	// ============================================================
	t.Log("=== PUBLISHING THIRD BEACON BLOCK MESSAGE (VERIFICATION) ===")
	thirdMessage := GossipTestMessage{
		ID:      "beacon-block-3",
		Content: "Third beacon block to verify system recovery",
		From:    nodes[1].ID.String(),
	}

	err = v1.Publish(nodes[1].Gossipsub, topic, thirdMessage)
	require.NoError(t, err)

	// Verify third message is processed quickly (system should be working normally)
	require.Eventually(t, func() bool {
		count := messageCount.Load()
		return count == 3
	}, 5*time.Second, 200*time.Millisecond, "Third message should be processed quickly")

	finalCount := messageCount.Load()
	t.Logf("=== FINAL RESULT: All %d messages processed successfully ===", finalCount)

	if finalCount == 3 {
		t.Log("✅ SUCCESS: No blocking detected - all beacon block messages processed")
	} else {
		t.Fatalf("❌ FAILURE: Expected 3 messages, got %d - possible blocking issue", finalCount)
	}
}

// TestMinimalReproduction_NextMethodDirectInspection provides direct debugging of the Next() method
func TestMinimalReproduction_NextMethodDirectInspection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Track timing of Next() calls
	var nextCallTimes []time.Time
	var processingTimes []time.Duration

	// Create topic
	topic, err := CreateTestTopic("next_inspection")
	require.NoError(t, err)

	// Create handler that tracks timing
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			processStart := time.Now()

			t.Logf("🔍 Processing started for message: %s at %v", msg.ID, processStart)

			// Simulate minimal processing time
			time.Sleep(10 * time.Millisecond)

			processDuration := time.Since(processStart)
			processingTimes = append(processingTimes, processDuration)

			t.Logf("🔍 Processing completed for message: %s (took %v)", msg.ID, processDuration)

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

	// Wait for mesh
	time.Sleep(2 * time.Second)

	// Send messages with precise timing measurement
	messages := []string{"timing-1", "timing-2", "timing-3", "timing-4"}

	for i, msgID := range messages {
		nextCallStart := time.Now()
		nextCallTimes = append(nextCallTimes, nextCallStart)

		msg := GossipTestMessage{
			ID:      msgID,
			Content: fmt.Sprintf("Timing test message %d", i+1),
			From:    nodes[1].ID.String(),
		}

		t.Logf("🚀 Publishing message %s at %v", msgID, nextCallStart)

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Wait for this specific message to be processed before sending next
		require.Eventually(t, func() bool {
			return len(processingTimes) > i
		}, 10*time.Second, 100*time.Millisecond,
			fmt.Sprintf("Message %s should be processed", msgID))

		// Calculate time between publish and processing
		if len(processingTimes) > i {
			deliveryTime := time.Since(nextCallStart)
			t.Logf("📊 Message %s delivery time: %v", msgID, deliveryTime)
		}

		// Delay before next message
		time.Sleep(500 * time.Millisecond)
	}

	// Analyze timing patterns
	t.Log("📊 TIMING ANALYSIS:")
	for i, processTime := range processingTimes {
		if i < len(nextCallTimes) {
			totalTime := time.Since(nextCallTimes[i])
			t.Logf("Message %d: Processing=%v, Total=%v", i+1, processTime, totalTime)
		}
	}

	// Check for abnormal delays that might indicate blocking
	for i, duration := range processingTimes {
		if duration > 1*time.Second {
			t.Errorf("⚠️  Message %d took unusually long to process: %v", i+1, duration)
		}
	}

	t.Log("✅ Next() method timing inspection completed")
}

// TestMinimalReproduction_SubscriptionStateMonitoring monitors subscription state during message flow
func TestMinimalReproduction_SubscriptionStateMonitoring(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messageCount atomic.Int64

	// Create topic
	topic, err := CreateTestTopic("state_monitoring")
	require.NoError(t, err)

	// Create handler
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := messageCount.Add(1)
			t.Logf("📨 State Monitor: Processing message %d (%s)", count, msg.ID)
			return nil
		}),
	)

	// Register handlers
	for _, node := range nodes {
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	// Subscribe with state monitoring
	sub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Start a goroutine to monitor subscription state
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if sub.IsCancelled() {
					t.Log("⚠️  Subscription cancelled unexpectedly!")
					return
				}

				activeTopics := nodes[0].Gossipsub.ActiveTopics()
				count := messageCount.Load()

				t.Logf("🔍 State: Topic=%s, Active=%v, Messages=%d",
					sub.Topic(), activeTopics, count)
			}
		}
	}()

	// Wait for mesh
	time.Sleep(2 * time.Second)

	// Send messages with state monitoring
	for i := 0; i < 3; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("monitor-%d", i+1),
			Content: fmt.Sprintf("State monitoring message %d", i+1),
			From:    nodes[1].ID.String(),
		}

		t.Logf("📤 Publishing monitored message %d", i+1)

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Wait for message with state verification
		require.Eventually(t, func() bool {
			count := messageCount.Load()
			isActive := !sub.IsCancelled()
			t.Logf("Waiting: count=%d, subscription_active=%v", count, isActive)
			return count == int64(i+1)
		}, 10*time.Second, 200*time.Millisecond,
			fmt.Sprintf("Message %d should be processed", i+1))

		time.Sleep(1 * time.Second)
	}

	// Final state check
	require.False(t, sub.IsCancelled(), "Subscription should remain active")
	require.Equal(t, int64(3), messageCount.Load(), "All messages should be processed")

	t.Log("✅ Subscription state monitoring completed successfully")
}
