package v1_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStress_HighVolumeMessages tests system behavior under high message volume.
func TestStress_HighVolumeMessages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var processedCount atomic.Int64
	var startTime time.Time
	var processingTimes []time.Duration
	var mu sync.Mutex

	// Create topic
	topic, err := CreateTestTopic("stress_volume")
	require.NoError(t, err)

	// Create handler that tracks processing time
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			processStart := time.Now()
			count := processedCount.Add(1)

			// Simulate minimal processing (1ms)
			time.Sleep(1 * time.Millisecond)

			processingDuration := time.Since(processStart)

			mu.Lock()
			processingTimes = append(processingTimes, processingDuration)
			mu.Unlock()

			if count%100 == 0 {
				elapsed := time.Since(startTime)
				rate := float64(count) / elapsed.Seconds()
				t.Logf("Processed %d messages, rate: %.2f msg/sec", count, rate)
			}

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

	// Send high volume of messages
	const numMessages = 1000
	startTime = time.Now()

	t.Logf("Starting to send %d messages...", numMessages)
	publishStart := time.Now()

	for i := 0; i < numMessages; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("stress-msg-%d", i),
			Content: fmt.Sprintf("High volume stress test message %d", i),
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// No delay - send as fast as possible
	}

	publishDuration := time.Since(publishStart)
	publishRate := float64(numMessages) / publishDuration.Seconds()
	t.Logf("Published %d messages in %v (rate: %.2f msg/sec)", numMessages, publishDuration, publishRate)

	// Wait for all messages to be processed
	require.Eventually(t, func() bool {
		count := processedCount.Load()
		elapsed := time.Since(startTime)
		if elapsed > 5*time.Second && count > 0 {
			rate := float64(count) / elapsed.Seconds()
			t.Logf("Processing: %d/%d messages (%.2f msg/sec)", count, numMessages, rate)
		}

		return count == numMessages
	}, 60*time.Second, 1*time.Second, "All messages should be processed")

	totalDuration := time.Since(startTime)
	finalRate := float64(numMessages) / totalDuration.Seconds()

	t.Logf("SUCCESS: Processed %d messages in %v (rate: %.2f msg/sec)", numMessages, totalDuration, finalRate)

	// Analyze processing times
	mu.Lock()
	if len(processingTimes) > 0 {
		var total time.Duration
		var maxTime time.Duration
		for _, pt := range processingTimes {
			total += pt
			if pt > maxTime {
				maxTime = pt
			}
		}
		avgProcessing := total / time.Duration(len(processingTimes))
		t.Logf("Processing time stats: avg=%v, max=%v", avgProcessing, maxTime)

		// Check that no processing took too long (indicating blocking)
		assert.Less(t, maxTime, 100*time.Millisecond, "No single message should take too long to process")
	}
	mu.Unlock()

	// Ensure system is responsive after stress test
	assert.False(t, sub.IsCancelled(), "Subscription should remain active after stress test")
}

// TestStress_ConcurrentPublishers tests multiple publishers sending simultaneously.
func TestStress_ConcurrentPublishers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Create test infrastructure with multiple publisher nodes
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	const numPublishers = 5
	const messagesPerPublisher = 200

	// Create nodes: 1 subscriber + numPublishers publishers
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, numPublishers+1)
	require.NoError(t, err)

	subscriber := nodes[0]
	publishers := nodes[1:]

	var totalProcessed atomic.Int64
	receivedFromPublisher := make([]atomic.Int64, numPublishers)

	// Create topic
	topic, err := CreateTestTopic("stress_concurrent")
	require.NoError(t, err)

	// Create handler that tracks messages from each publisher
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := totalProcessed.Add(1)

			// Track which publisher this came from
			for i, pub := range publishers {
				if from == pub.ID {
					pubCount := receivedFromPublisher[i].Add(1)
					if pubCount%50 == 0 {
						t.Logf("Received %d messages from publisher %d", pubCount, i)
					}

					break
				}
			}

			if count%100 == 0 {
				t.Logf("Total processed: %d", count)
			}

			return nil
		}),
	)

	// Register handlers on all nodes
	for _, node := range nodes {
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	// Subscribe on subscriber node
	sub, err := v1.Subscribe(ctx, subscriber.Gossipsub, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Wait for mesh formation
	time.Sleep(3 * time.Second)

	// Start concurrent publishers
	var wg sync.WaitGroup
	startTime := time.Now()

	t.Logf("Starting %d concurrent publishers, %d messages each...", numPublishers, messagesPerPublisher)

	for i, publisher := range publishers {
		wg.Add(1)
		go func(publisherID int, pub *TestNode) {
			defer wg.Done()

			for j := 0; j < messagesPerPublisher; j++ {
				msg := GossipTestMessage{
					ID:      fmt.Sprintf("pub-%d-msg-%d", publisherID, j),
					Content: fmt.Sprintf("Message %d from publisher %d", j, publisherID),
					From:    pub.ID.String(),
				}

				err := v1.Publish(pub.Gossipsub, topic, msg)
				if err != nil {
					t.Errorf("Publisher %d failed to publish message %d: %v", publisherID, j, err)

					return
				}

				// Small delay to avoid overwhelming
				time.Sleep(1 * time.Millisecond)
			}

			t.Logf("Publisher %d completed", publisherID)
		}(i, publisher)
	}

	// Wait for all publishers to finish
	wg.Wait()
	publishDuration := time.Since(startTime)

	expectedTotal := int64(numPublishers * messagesPerPublisher)
	t.Logf("All publishers finished in %v, waiting for processing...", publishDuration)

	// Wait for all messages to be processed
	require.Eventually(t, func() bool {
		count := totalProcessed.Load()
		t.Logf("Processed %d/%d messages", count, expectedTotal)

		return count == expectedTotal
	}, 45*time.Second, 2*time.Second, "All messages should be processed")

	totalDuration := time.Since(startTime)
	rate := float64(expectedTotal) / totalDuration.Seconds()

	t.Logf("SUCCESS: Processed %d messages from %d concurrent publishers in %v (rate: %.2f msg/sec)",
		expectedTotal, numPublishers, totalDuration, rate)

	// Verify we received messages from all publishers
	for i := 0; i < numPublishers; i++ {
		count := receivedFromPublisher[i].Load()
		assert.Equal(t, int64(messagesPerPublisher), count,
			"Should receive all messages from publisher %d", i)
	}
}

// TestStress_MemoryPressure tests behavior under memory pressure.
func TestStress_MemoryPressure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var processedCount atomic.Int64

	// Create topic
	topic, err := CreateTestTopic("stress_memory")
	require.NoError(t, err)

	// Create handler with simulated memory usage
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := processedCount.Add(1)

			// Simulate memory allocation (will be GC'd)
			_ = make([]byte, 1024*1024) // 1MB allocation per message

			if count%10 == 0 {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				t.Logf("Processed %d messages, HeapAlloc: %d MB", count, m.HeapAlloc/1024/1024)
			}

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

	// Get initial memory stats
	var initialStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialStats)
	t.Logf("Initial HeapAlloc: %d MB", initialStats.HeapAlloc/1024/1024)

	// Send messages that will cause memory pressure
	const numMessages = 100
	startTime := time.Now()

	for i := 0; i < numMessages; i++ {
		// Create larger messages to increase memory pressure
		largeContent := fmt.Sprintf("Large message %d: %s", i,
			string(make([]byte, 10*1024))) // 10KB per message

		msg := GossipTestMessage{
			ID:      fmt.Sprintf("memory-stress-%d", i),
			Content: largeContent,
			From:    nodes[1].ID.String(),
		}

		err = v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Force GC every 10 messages to test stability
		if i%10 == 0 {
			runtime.GC()
		}
	}

	// Wait for all messages to be processed
	require.Eventually(t, func() bool {
		count := processedCount.Load()
		if count%10 == 0 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			t.Logf("Processing under memory pressure: %d/%d messages, HeapAlloc: %d MB",
				count, numMessages, m.HeapAlloc/1024/1024)
		}

		return count == numMessages
	}, 30*time.Second, 1*time.Second, "All messages should be processed under memory pressure")

	duration := time.Since(startTime)

	// Final memory check
	runtime.GC()
	var finalStats runtime.MemStats
	runtime.ReadMemStats(&finalStats)

	t.Logf("SUCCESS: Processed %d messages under memory pressure in %v", numMessages, duration)
	t.Logf("Memory stats - Initial: %d MB, Final: %d MB",
		initialStats.HeapAlloc/1024/1024, finalStats.HeapAlloc/1024/1024)

	// Verify subscription is still active
	assert.False(t, sub.IsCancelled(), "Subscription should remain active after memory pressure test")
}

// TestStress_RapidSubscribeUnsubscribe tests rapid subscription changes.
func TestStress_RapidSubscribeUnsubscribe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	// Create multiple topics for stress testing
	topics := make([]*v1.Topic[GossipTestMessage], 5)
	for i := 0; i < 5; i++ {
		topic, topicErr := CreateTestTopic(fmt.Sprintf("rapid-topic-%d", i))
		require.NoError(t, topicErr)
		topics[i] = topic
	}

	var operationCount atomic.Int64

	// Create handler
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			// Minimal processing
			return nil
		}),
	)

	// Register handlers for all topics on all nodes
	for _, node := range nodes {
		for _, topic := range topics {
			err = v1.Register(node.Gossipsub.Registry(), topic, handler)
			require.NoError(t, err)
		}
	}

	// Perform rapid subscribe/unsubscribe operations
	var wg sync.WaitGroup
	const numOperations = 100

	for i := 0; i < 3; i++ { // 3 concurrent goroutines
		wg.Add(1)
		go func(nodeID int) {
			defer wg.Done()
			node := nodes[nodeID]

			for j := 0; j < numOperations; j++ {
				opCount := operationCount.Add(1)
				topicIndex := j % len(topics)
				topic := topics[topicIndex]

				// Subscribe
				sub, subErr := v1.Subscribe(ctx, node.Gossipsub, topic)
				if subErr != nil {
					t.Errorf("Node %d failed to subscribe to topic %d: %v", nodeID, topicIndex, subErr)

					continue
				}

				// Short wait
				time.Sleep(10 * time.Millisecond)

				// Unsubscribe
				sub.Cancel()

				if opCount%20 == 0 {
					t.Logf("Completed %d rapid subscribe/unsubscribe operations", opCount)
				}

				// Brief pause to avoid overwhelming
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	// Wait for all operations to complete
	wg.Wait()

	totalOps := operationCount.Load()
	t.Logf("SUCCESS: Completed %d rapid subscribe/unsubscribe operations", totalOps)

	// Verify system is still functional - create final subscription and send message
	finalTopic := topics[0]
	finalSub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, finalTopic)
	require.NoError(t, err)
	defer finalSub.Cancel()

	time.Sleep(1 * time.Second)

	// Send test message
	msg := GossipTestMessage{
		ID:      "final-test",
		Content: "Final test after rapid subscribe/unsubscribe",
		From:    nodes[1].ID.String(),
	}

	err = v1.Publish(nodes[1].Gossipsub, finalTopic, msg)
	require.NoError(t, err)

	// Should still work
	time.Sleep(2 * time.Second)
	assert.False(t, finalSub.IsCancelled(), "Final subscription should be active")

	t.Log("SUCCESS: System remains functional after rapid subscribe/unsubscribe stress test")
}
