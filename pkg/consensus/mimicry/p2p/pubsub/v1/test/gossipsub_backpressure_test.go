package v1_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackpressure_SlowConsumer tests behavior when message processing is slower than delivery
func TestBackpressure_SlowConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messagesReceived atomic.Int64
	var messagesProcessed atomic.Int64
	var processingStartTimes []time.Time
	var processingEndTimes []time.Time
	var mu sync.Mutex

	// Create topic
	topic, err := CreateTestTopic("backpressure_slow")
	require.NoError(t, err)

	// Create handler with intentionally slow processing
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			received := messagesReceived.Add(1)
			processStart := time.Now()

			mu.Lock()
			processingStartTimes = append(processingStartTimes, processStart)
			mu.Unlock()

			t.Logf("🐌 Started processing message %d (%s) at %v", received, msg.ID, processStart)

			// Simulate slow processing (2 seconds per message)
			time.Sleep(2 * time.Second)

			processEnd := time.Now()
			processed := messagesProcessed.Add(1)

			mu.Lock()
			processingEndTimes = append(processingEndTimes, processEnd)
			mu.Unlock()

			t.Logf("✅ Finished processing message %d (%s) at %v (took %v)",
				processed, msg.ID, processEnd, processEnd.Sub(processStart))

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

	// Send messages faster than they can be processed
	const numMessages = 10
	publishStart := time.Now()

	t.Log("🚀 Publishing messages faster than processing...")
	for i := 0; i < numMessages; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("slow-consumer-%d", i),
			Content: fmt.Sprintf("Backpressure test message %d", i),
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Publish every 200ms (much faster than 2s processing time)
		time.Sleep(200 * time.Millisecond)
	}

	publishDuration := time.Since(publishStart)
	t.Logf("📤 Published %d messages in %v (avg: %v per message)",
		numMessages, publishDuration, publishDuration/numMessages)

	// Verify that messages are received quickly even though processing is slow
	t.Log("⏱️  Checking message reception rate...")
	require.Eventually(t, func() bool {
		received := messagesReceived.Load()
		t.Logf("Received %d/%d messages", received, numMessages)
		return received == numMessages
	}, 15*time.Second, 500*time.Millisecond, "All messages should be received quickly")

	receptionDuration := time.Since(publishStart)
	t.Logf("📨 All messages received in %v (much faster than processing)", receptionDuration)

	// Now wait for all processing to complete
	t.Log("⏳ Waiting for all processing to complete...")
	require.Eventually(t, func() bool {
		processed := messagesProcessed.Load()
		t.Logf("Processed %d/%d messages", processed, numMessages)
		return processed == numMessages
	}, 60*time.Second, 2*time.Second, "All messages should eventually be processed")

	totalDuration := time.Since(publishStart)
	t.Logf("✅ All messages processed in %v", totalDuration)

	// Analyze timing to ensure no blocking occurred
	mu.Lock()
	if len(processingStartTimes) >= 2 {
		// Check that processing started for multiple messages before first one finished
		firstStart := processingStartTimes[0]
		secondStart := processingStartTimes[1]
		firstEnd := processingEndTimes[0]

		timeBetweenStarts := secondStart.Sub(firstStart)
		t.Logf("⏱️  Time between first two processing starts: %v", timeBetweenStarts)

		// Second message should start processing before first finishes (concurrent processing)
		if secondStart.Before(firstEnd) {
			t.Log("✅ Concurrent processing confirmed - no blocking detected")
		} else {
			t.Log("⚠️  Sequential processing detected")
		}

		// Message reception should be much faster than processing
		assert.Less(t, timeBetweenStarts, 1*time.Second,
			"Messages should start processing quickly despite slow processing")
	}
	mu.Unlock()

	// Verify subscription survived the backpressure
	assert.False(t, sub.IsCancelled(), "Subscription should survive backpressure")

	t.Log("SUCCESS: Slow consumer test completed - no blocking detected")
}

// TestBackpressure_MessageBuffering tests message buffering behavior
func TestBackpressure_MessageBuffering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var totalReceived atomic.Int64
	var batchProcessed atomic.Int64
	var processingActive atomic.Bool

	// Create topic
	topic, err := CreateTestTopic("backpressure_buffer")
	require.NoError(t, err)

	// Create handler that can be paused/resumed
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			totalReceived.Add(1)

			// Wait if processing is paused
			for !processingActive.Load() {
				time.Sleep(10 * time.Millisecond)
			}

			processed := batchProcessed.Add(1)
			t.Logf("📦 Buffered message processed: %d (%s)", processed, msg.ID)

			// Minimal processing time
			time.Sleep(10 * time.Millisecond)

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

	// Phase 1: Send messages while processing is paused (simulating buffer buildup)
	t.Log("📥 Phase 1: Sending messages with processing paused...")
	processingActive.Store(false)

	const batchSize = 20
	for i := 0; i < batchSize; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("buffered-%d", i),
			Content: fmt.Sprintf("Buffered message %d", i),
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)
	}

	// Wait for all messages to be received (but not processed)
	require.Eventually(t, func() bool {
		received := totalReceived.Load()
		processed := batchProcessed.Load()
		t.Logf("Received: %d, Processed: %d", received, processed)
		return received == batchSize
	}, 15*time.Second, 500*time.Millisecond, "All messages should be received")

	// Verify processing hasn't started yet
	assert.Equal(t, int64(0), batchProcessed.Load(), "No messages should be processed while paused")

	// Phase 2: Resume processing and verify all buffered messages are processed
	t.Log("▶️  Phase 2: Resuming processing...")
	processingActive.Store(true)

	// All buffered messages should now be processed
	require.Eventually(t, func() bool {
		processed := batchProcessed.Load()
		t.Logf("Processing resumed: %d/%d messages processed", processed, batchSize)
		return processed == batchSize
	}, 30*time.Second, 500*time.Millisecond, "All buffered messages should be processed")

	t.Log("SUCCESS: Message buffering test completed")
}

// TestBackpressure_HighThroughputSustained tests sustained high throughput
func TestBackpressure_HighThroughputSustained(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messagesProcessed atomic.Int64
	var lastReportTime atomic.Value
	lastReportTime.Store(time.Now())

	// Create topic
	topic, err := CreateTestTopic("backpressure_throughput")
	require.NoError(t, err)

	// Create handler that tracks throughput
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := messagesProcessed.Add(1)

			// Report throughput every 100 messages
			if count%100 == 0 {
				now := time.Now()
				lastReport := lastReportTime.Load().(time.Time)
				if !lastReport.IsZero() {
					duration := now.Sub(lastReport)
					rate := float64(100) / duration.Seconds()
					t.Logf("📊 Processed %d messages, current rate: %.2f msg/sec", count, rate)
				}
				lastReportTime.Store(now)
			}

			// Minimal processing to test pure throughput
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

	// Sustained high throughput test
	const testDuration = 30 * time.Second
	const targetRate = 100 // messages per second

	startTime := time.Now()
	messagesSent := 0

	t.Logf("🚀 Starting sustained throughput test for %v at %d msg/sec target rate",
		testDuration, targetRate)

	// Send messages at target rate
	ticker := time.NewTicker(time.Second / targetRate)
	defer ticker.Stop()

	testTimer := time.NewTimer(testDuration)
	defer testTimer.Stop()

publishLoop:
	for {
		select {
		case <-testTimer.C:
			break publishLoop
		case <-ticker.C:
			msg := GossipTestMessage{
				ID:      fmt.Sprintf("throughput-%d", messagesSent),
				Content: fmt.Sprintf("Sustained throughput message %d", messagesSent),
				From:    nodes[1].ID.String(),
			}

			err = v1.Publish(nodes[1].Gossipsub, topic, msg)
			if err != nil {
				t.Errorf("Failed to publish message %d: %v", messagesSent, err)
				continue
			}

			messagesSent++
		case <-ctx.Done():
			break publishLoop
		}
	}

	publishDuration := time.Since(startTime)
	actualPublishRate := float64(messagesSent) / publishDuration.Seconds()

	t.Logf("📤 Published %d messages in %v (rate: %.2f msg/sec)",
		messagesSent, publishDuration, actualPublishRate)

	// Wait for all messages to be processed
	t.Log("⏳ Waiting for processing to complete...")
	require.Eventually(t, func() bool {
		processed := messagesProcessed.Load()
		rate := float64(processed) / time.Since(startTime).Seconds()
		t.Logf("Processed %d/%d messages (rate: %.2f msg/sec)", processed, messagesSent, rate)
		return processed == int64(messagesSent)
	}, 60*time.Second, 2*time.Second, "All messages should be processed")

	totalDuration := time.Since(startTime)
	overallRate := float64(messagesSent) / totalDuration.Seconds()

	t.Logf("✅ Sustained throughput test completed:")
	t.Logf("   Messages: %d", messagesSent)
	t.Logf("   Duration: %v", totalDuration)
	t.Logf("   Overall rate: %.2f msg/sec", overallRate)

	// Verify system maintained reasonable performance
	assert.Greater(t, overallRate, float64(targetRate)*0.8,
		"Should maintain at least 80% of target throughput")

	// Verify subscription is still healthy
	assert.False(t, sub.IsCancelled(), "Subscription should remain active after throughput test")

	t.Log("SUCCESS: Sustained high throughput test completed")
}

// TestBackpressure_ProcessorFailureRecovery tests recovery from processor failures
func TestBackpressure_ProcessorFailureRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messagesProcessed atomic.Int64
	var processingErrors atomic.Int64
	var shouldFail atomic.Bool

	// Create topic
	topic, err := CreateTestTopic("backpressure_failure")
	require.NoError(t, err)

	// Create handler that can simulate failures
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			if shouldFail.Load() {
				errors := processingErrors.Add(1)
				t.Logf("💥 Simulated processor failure %d for message: %s", errors, msg.ID)
				return fmt.Errorf("simulated processing failure")
			}

			processed := messagesProcessed.Add(1)
			t.Logf("✅ Successfully processed message %d: %s", processed, msg.ID)
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

	// Phase 1: Send messages with processor failures enabled
	t.Log("💥 Phase 1: Sending messages with processor failures...")
	shouldFail.Store(true)

	for i := 0; i < 10; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("failure-test-%d", i),
			Content: fmt.Sprintf("Failure test message %d", i),
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
	}

	// Wait for failure processing
	time.Sleep(3 * time.Second)

	failureCount := processingErrors.Load()
	t.Logf("Phase 1 complete: %d processing failures occurred", failureCount)
	assert.Greater(t, failureCount, int64(0), "Some failures should have occurred")

	// Phase 2: Disable failures and send recovery messages
	t.Log("🔧 Phase 2: Disabling failures and testing recovery...")
	shouldFail.Store(false)

	for i := 0; i < 5; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("recovery-test-%d", i),
			Content: fmt.Sprintf("Recovery test message %d", i),
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
	}

	// Wait for recovery processing
	require.Eventually(t, func() bool {
		processed := messagesProcessed.Load()
		t.Logf("Recovery phase: %d messages processed successfully", processed)
		return processed >= 5
	}, 10*time.Second, 500*time.Millisecond, "Recovery messages should be processed")

	finalProcessed := messagesProcessed.Load()
	finalErrors := processingErrors.Load()

	t.Logf("Final results: %d processed, %d errors", finalProcessed, finalErrors)

	// Verify system recovered and is still functional
	assert.GreaterOrEqual(t, finalProcessed, int64(5), "Should process recovery messages")
	assert.False(t, sub.IsCancelled(), "Subscription should survive processor failures")

	t.Log("SUCCESS: Processor failure recovery test completed")
}
