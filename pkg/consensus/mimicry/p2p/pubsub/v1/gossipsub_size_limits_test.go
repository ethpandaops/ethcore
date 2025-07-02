package v1_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPublishMessageExceedingMaxSize tests publishing a message that exceeds the maximum size limit
func TestPublishMessageExceedingMaxSize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Set a small max message size (1KB)
	maxSize := 1024
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2,
		v1.WithPubsubOptions(pubsub.WithMaxMessageSize(maxSize)),
		v1.WithLogger(logrus.StandardLogger().WithField("test", "message-size-limit")),
	)
	require.NoError(t, err)
	require.Len(t, nodes, 2)

	// Create a test topic
	topic, err := CreateTestTopic("size-limit-topic")
	require.NoError(t, err)

	// Create message collector
	collector := NewMessageCollector(10)

	// Register handlers and subscribe all nodes
	for _, node := range nodes {
		handler := CreateTestHandler(collector.CreateProcessor(node.ID))
		err := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
	}

	// Wait for mesh to stabilize
	WaitForGossipsubReady(t, nodes, topic.Name(), 2)

	// Create an oversized message (2KB content)
	// Note: The actual message size includes encoding overhead and pubsub headers
	oversizedContent := strings.Repeat("A", 2048)
	oversizedMsg := GossipTestMessage{
		ID:      "oversized-msg",
		Content: oversizedContent,
		From:    nodes[0].ID.String(),
	}

	// Attempt to publish the oversized message
	// Note: libp2p doesn't reject at publish time, but during transmission
	err = v1.Publish(nodes[0].Gossipsub, topic, oversizedMsg)
	require.NoError(t, err, "Publish should succeed locally")

	// Wait a bit to ensure no propagation
	time.Sleep(1 * time.Second)

	// Verify no messages were received by other nodes due to size limit
	messages := collector.GetMessages()
	receiversCount := 0
	for _, msg := range messages {
		if msg.Node != nodes[0].ID {
			receiversCount++
		}
	}
	assert.Equal(t, 0, receiversCount, "Oversized messages should not propagate to other nodes")
}

// TestReceivingOversizedMessages tests the behavior when receiving oversized messages from peers
func TestReceivingOversizedMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create nodes with different max message sizes
	// Node 0: sender with large limit (10MB)
	// Node 1: receiver with small limit (1KB)
	node0, err := ti.CreateNode(ctx,
		v1.WithPubsubOptions(pubsub.WithMaxMessageSize(10<<20)), // 10MB
		v1.WithLogger(logrus.StandardLogger().WithField("test", "sender")),
	)
	require.NoError(t, err)

	node1, err := ti.CreateNode(ctx,
		v1.WithPubsubOptions(pubsub.WithMaxMessageSize(1024)), // 1KB
		v1.WithLogger(logrus.StandardLogger().WithField("test", "receiver")),
	)
	require.NoError(t, err)

	// Connect the nodes
	err = ti.ConnectNodes(node0, node1)
	require.NoError(t, err)

	// Create a test topic
	topic, err := CreateTestTopic("receive-size-topic")
	require.NoError(t, err)

	// Create message collectors
	collector0 := NewMessageCollector(5)
	collector1 := NewMessageCollector(5)

	// Register handlers
	handler0 := CreateTestHandler(collector0.CreateProcessor(node0.ID))
	err = v1.Register(node0.Gossipsub.Registry(), topic, handler0)
	require.NoError(t, err)

	handler1 := CreateTestHandler(collector1.CreateProcessor(node1.ID))
	err = v1.Register(node1.Gossipsub.Registry(), topic, handler1)
	require.NoError(t, err)

	// Subscribe both nodes
	_, err = v1.Subscribe(ctx, node0.Gossipsub, topic)
	require.NoError(t, err)
	_, err = v1.Subscribe(ctx, node1.Gossipsub, topic)
	require.NoError(t, err)

	// Wait for mesh to form
	time.Sleep(2 * time.Second)

	// Create a message that's within node0's limit but exceeds node1's limit
	largeContent := strings.Repeat("B", 1500) // 1.5KB
	largeMsg := GossipTestMessage{
		ID:      "large-msg",
		Content: largeContent,
		From:    node0.ID.String(),
	}

	// Publish from node0 (should succeed locally)
	err = v1.Publish(node0.Gossipsub, topic, largeMsg)
	require.NoError(t, err, "Node0 should be able to publish within its limit")

	// Wait for potential propagation
	time.Sleep(1 * time.Second)

	// Node1 should not receive the message due to its size limit
	assert.Equal(t, 0, collector1.GetMessageCount(), "Node1 should not receive messages exceeding its size limit")

	// Node0 might receive its own message (gossipsub behavior)
	// This is implementation-specific and may vary
}

// TestCustomMaxMessageSizeConfiguration tests custom message size configuration
func TestCustomMaxMessageSizeConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		maxSize     int
		messageSize int
		shouldWork  bool
	}{
		{
			name:        "small limit with small message",
			maxSize:     512,
			messageSize: 256,
			shouldWork:  true,
		},
		{
			name:        "exact limit message",
			maxSize:     1024,
			messageSize: 950, // Leave room for encoding overhead
			shouldWork:  true,
		},
		{
			name:        "over limit message",
			maxSize:     1024,
			messageSize: 2048,
			shouldWork:  false,
		},
		{
			name:        "large limit with large message",
			maxSize:     1 << 20, // 1MB
			messageSize: 512000,  // 500KB
			shouldWork:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create test infrastructure
			ti := NewTestInfrastructure(t)
			defer ti.Cleanup()

			// Create nodes with custom max size
			nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2,
				v1.WithPubsubOptions(pubsub.WithMaxMessageSize(tc.maxSize)),
			)
			require.NoError(t, err)

			// Create topic and setup
			topic, err := CreateTestTopic(fmt.Sprintf("custom-size-%s", tc.name))
			require.NoError(t, err)

			collector := NewMessageCollector(5)
			for _, node := range nodes {
				handler := CreateTestHandler(collector.CreateProcessor(node.ID))
				err := v1.Register(node.Gossipsub.Registry(), topic, handler)
				require.NoError(t, err)

				_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
				require.NoError(t, err)
			}

			// Wait for mesh
			WaitForGossipsubReady(t, nodes, topic.Name(), 2)

			// Create message with specified size
			content := strings.Repeat("X", tc.messageSize)
			msg := GossipTestMessage{
				ID:      fmt.Sprintf("msg-%s", tc.name),
				Content: content,
				From:    nodes[0].ID.String(),
			}

			// Attempt to publish
			err = v1.Publish(nodes[0].Gossipsub, topic, msg)

			if tc.shouldWork {
				assert.NoError(t, err, "Message within limit should publish successfully")

				// Wait for propagation
				require.Eventually(t, func() bool {
					return collector.GetMessageCount() >= 1 // At least the receiver should get it
				}, 2*time.Second, 100*time.Millisecond)
			} else {
				// Oversized messages don't fail at publish time
				assert.NoError(t, err, "Publish succeeds locally even for oversized messages")

				// Wait and verify no propagation
				time.Sleep(1 * time.Second)
				messages := collector.GetMessages()
				receiversCount := 0
				for _, msg := range messages {
					if msg.Node != nodes[0].ID {
						receiversCount++
					}
				}
				assert.Equal(t, 0, receiversCount, "Oversized messages should not propagate")
			}
		})
	}
}

// TestMessageSizeLimitEdgeCases tests edge cases around message size limits
func TestMessageSizeLimitEdgeCases(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Set a specific max message size
	maxSize := 1024
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2,
		v1.WithPubsubOptions(pubsub.WithMaxMessageSize(maxSize)),
	)
	require.NoError(t, err)

	// Create topic
	topic, err := CreateTestTopic("edge-case-topic")
	require.NoError(t, err)

	// Setup subscriptions
	collector := NewMessageCollector(10)
	for _, node := range nodes {
		handler := CreateTestHandler(collector.CreateProcessor(node.ID))
		err := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
	}

	// Wait for mesh
	WaitForGossipsubReady(t, nodes, topic.Name(), 2)

	// Test cases for edge conditions
	testCases := []struct {
		name          string
		contentSize   int
		expectSuccess bool
		description   string
	}{
		{
			name:          "empty message",
			contentSize:   0,
			expectSuccess: true,
			description:   "Empty messages should always work",
		},
		{
			name:          "single byte",
			contentSize:   1,
			expectSuccess: true,
			description:   "Minimal message should work",
		},
		{
			name:          "near limit",
			contentSize:   900, // Account for encoding overhead
			expectSuccess: true,
			description:   "Message near but under limit should work",
		},
		{
			name:          "at encoding boundary",
			contentSize:   980, // Very close to limit with encoding
			expectSuccess: true,
			description:   "Message at encoding boundary should work",
		},
		{
			name:          "just over limit",
			contentSize:   1025, // 1 byte over
			expectSuccess: false,
			description:   "Message just over limit should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear previous messages
			collector.GetMessages() // Drain collector

			// Create message
			content := strings.Repeat("E", tc.contentSize)
			msg := GossipTestMessage{
				ID:      fmt.Sprintf("edge-%s", tc.name),
				Content: content,
				From:    nodes[0].ID.String(),
			}

			// Attempt to publish
			err := v1.Publish(nodes[0].Gossipsub, topic, msg)

			if tc.expectSuccess {
				assert.NoError(t, err, tc.description)

				// Verify propagation
				require.Eventually(t, func() bool {
					return collector.GetMessageCount() >= 1
				}, 2*time.Second, 50*time.Millisecond, "Message should propagate")
			} else {
				// Publish succeeds locally
				assert.NoError(t, err, "Publish succeeds locally")

				// Verify no propagation
				time.Sleep(1 * time.Second)
				messages := collector.GetMessages()
				receiversCount := 0
				for _, msg := range messages {
					if msg.Node != nodes[0].ID {
						receiversCount++
					}
				}
				assert.Equal(t, 0, receiversCount, tc.description)
			}
		})
	}
}

// TestDefaultMessageSizeLimit tests the default message size limit behavior
func TestDefaultMessageSizeLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create nodes without specifying max message size (should use default)
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Create topic
	topic, err := CreateTestTopic("default-size-topic")
	require.NoError(t, err)

	// Setup subscriptions
	collector := NewMessageCollector(5)
	for _, node := range nodes {
		handler := CreateTestHandler(collector.CreateProcessor(node.ID))
		err := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
	}

	// Wait for mesh
	WaitForGossipsubReady(t, nodes, topic.Name(), 2)

	// Test with a large message that should work with default limit (10MB)
	largeMsgSize := 5 << 20 // 5MB
	largeContent := strings.Repeat("D", largeMsgSize)
	largeMsg := GossipTestMessage{
		ID:      "default-large",
		Content: largeContent,
		From:    nodes[0].ID.String(),
	}

	// Should succeed with default limit
	err = v1.Publish(nodes[0].Gossipsub, topic, largeMsg)
	require.NoError(t, err, "5MB message should work with default 10MB limit")

	// Wait for propagation
	require.Eventually(t, func() bool {
		return collector.GetMessageCount() >= 1
	}, 5*time.Second, 100*time.Millisecond)

	// Clear collector
	collector.GetMessages()

	// Test with a message exceeding default limit
	// Note: The actual libp2p message includes headers and metadata, so we need a smaller payload
	oversizedContent := strings.Repeat("O", 15<<20) // 15MB - definitely over 10MB limit
	oversizedMsg := GossipTestMessage{
		ID:      "default-oversized",
		Content: oversizedContent,
		From:    nodes[0].ID.String(),
	}

	// Should fail to propagate with default limit
	err = v1.Publish(nodes[0].Gossipsub, topic, oversizedMsg)
	assert.NoError(t, err, "Publish succeeds locally")

	// Wait and verify no propagation
	time.Sleep(2 * time.Second)
	messages := collector.GetMessages()
	receiversCount := 0
	for _, msg := range messages {
		if msg.Node != nodes[0].ID {
			receiversCount++
		}
	}
	assert.Equal(t, 0, receiversCount, "15MB message should not propagate with default 10MB limit")
}

// VerboseEncoder is a test encoder that adds significant overhead to messages
type VerboseEncoder struct{}

func (e *VerboseEncoder) Encode(msg GossipTestMessage) ([]byte, error) {
	// Add verbose encoding with metadata
	encoded := fmt.Sprintf("VERSION:1|ID:%s|CONTENT:%s|FROM:%s|CHECKSUM:12345",
		msg.ID, msg.Content, msg.From)
	return []byte(encoded), nil
}

func (e *VerboseEncoder) Decode(data []byte) (GossipTestMessage, error) {
	// Simple parsing for test
	str := string(data)
	// Extract content between CONTENT: and |FROM:
	contentStart := strings.Index(str, "CONTENT:") + 8
	contentEnd := strings.Index(str[contentStart:], "|FROM:")
	if contentEnd == -1 {
		return GossipTestMessage{}, fmt.Errorf("invalid format")
	}

	content := str[contentStart : contentStart+contentEnd]

	// Extract ID
	idStart := strings.Index(str, "ID:") + 3
	idEnd := strings.Index(str[idStart:], "|")
	id := str[idStart : idStart+idEnd]

	// Extract FROM
	fromStart := strings.Index(str, "FROM:") + 5
	fromEnd := strings.Index(str[fromStart:], "|")
	if fromEnd == -1 {
		fromEnd = len(str) - fromStart
	}
	from := str[fromStart : fromStart+fromEnd]

	return GossipTestMessage{
		ID:      id,
		Content: content,
		From:    from,
	}, nil
}

// TestMessageSizeWithDifferentEncodings tests size limits with different message encodings
func TestMessageSizeWithDifferentEncodings(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Set max message size
	maxSize := 1024
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2,
		v1.WithPubsubOptions(pubsub.WithMaxMessageSize(maxSize)),
	)
	require.NoError(t, err)

	// Create topic with verbose encoder
	verboseEncoder := &VerboseEncoder{}
	verboseTopic, err := v1.NewTopic[GossipTestMessage]("verbose-encoding-topic")
	require.NoError(t, err)

	// Setup subscriptions
	collector := NewMessageCollector(5)
	for _, node := range nodes {
		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithEncoder[GossipTestMessage](verboseEncoder),
			v1.WithProcessor(collector.CreateProcessor(node.ID)),
			v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
		)
		err := v1.Register(node.Gossipsub.Registry(), verboseTopic, handler)
		require.NoError(t, err)

		_, err = v1.Subscribe(ctx, node.Gossipsub, verboseTopic)
		require.NoError(t, err)
	}

	// Wait for mesh
	time.Sleep(2 * time.Second)

	// Test with a message that would fit with simple encoding but not with verbose
	content := strings.Repeat("V", 900) // Content alone is 900 bytes
	msg := GossipTestMessage{
		ID:      "verbose-test",
		Content: content,
		From:    nodes[0].ID.String(),
	}

	// The verbose encoding adds significant overhead
	encoded, err := verboseEncoder.Encode(msg)
	require.NoError(t, err)
	t.Logf("Encoded message size: %d bytes", len(encoded))

	// Attempt to publish - should not propagate due to encoding overhead
	err = v1.Publish(nodes[0].Gossipsub, verboseTopic, msg)
	assert.NoError(t, err, "Publish succeeds locally")

	// Wait and verify no propagation
	time.Sleep(1 * time.Second)
	messages := collector.GetMessages()
	receiversCount := 0
	for _, m := range messages {
		if m.Node != nodes[0].ID {
			receiversCount++
		}
	}
	assert.Equal(t, 0, receiversCount, "Message with verbose encoding should not propagate")

	// Test with smaller content that fits even with verbose encoding
	smallContent := strings.Repeat("S", 100)
	smallMsg := GossipTestMessage{
		ID:      "small-verbose",
		Content: smallContent,
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, verboseTopic, smallMsg)
	assert.NoError(t, err, "Small message should work even with verbose encoding")

	// Verify propagation
	require.Eventually(t, func() bool {
		return collector.GetMessageCount() >= 1
	}, 2*time.Second, 100*time.Millisecond)
}

// TestInvalidMaxMessageSizeConfiguration tests invalid configuration handling
func TestInvalidMaxMessageSizeConfiguration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Test creating node with invalid max message size
	testCases := []struct {
		name    string
		maxSize int
	}{
		{
			name:    "zero size",
			maxSize: 0,
		},
		{
			name:    "negative size",
			maxSize: -1024,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ti.CreateNode(ctx,
				v1.WithPubsubOptions(pubsub.WithMaxMessageSize(tc.maxSize)),
			)
			assert.Error(t, err, "Creating node with invalid max message size should fail")
			assert.Contains(t, err.Error(), "max message size must be positive", "Error should indicate invalid size")
		})
	}
}

// TestConcurrentMessageSizeLimit tests size limits under concurrent publishing
func TestConcurrentMessageSizeLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create nodes with moderate size limit
	// Note: The libp2p message size includes protocol overhead, so the actual
	// payload limit is less than the configured max
	maxSize := 5000 // 5KB limit to ensure our 20KB messages are rejected
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3,
		v1.WithPubsubOptions(pubsub.WithMaxMessageSize(maxSize)),
	)
	require.NoError(t, err)

	// Create topic
	topic, err := CreateTestTopic("concurrent-size-topic")
	require.NoError(t, err)

	// Setup subscriptions
	collectors := make([]*MessageCollector, 3)
	for i, node := range nodes {
		collectors[i] = NewMessageCollector(50)
		handler := CreateTestHandler(collectors[i].CreateProcessor(node.ID))
		err := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
	}

	// Wait for mesh
	WaitForGossipsubReady(t, nodes, topic.Name(), 3)

	// Concurrently publish messages of different sizes
	type publishResult struct {
		nodeIdx int
		msgSize int
		err     error
	}

	results := make(chan publishResult, 9)

	// Launch concurrent publishers
	for i, node := range nodes {
		nodeIdx := i
		go func(n *TestNode) {
			// Each node publishes 3 messages of different sizes
			sizes := []int{
				100,  // Small - should succeed
				3000, // Medium - should succeed (under 5KB limit)
				8000, // Large - should fail (exceeds 5KB limit)
			}

			for j, size := range sizes {
				content := strings.Repeat(fmt.Sprintf("%d", nodeIdx), size)
				msg := GossipTestMessage{
					ID:      fmt.Sprintf("concurrent-%d-%d", nodeIdx, j),
					Content: content,
					From:    n.ID.String(),
				}

				err := v1.Publish(n.Gossipsub, topic, msg)
				results <- publishResult{
					nodeIdx: nodeIdx,
					msgSize: size,
					err:     err,
				}

				// Small delay between messages
				time.Sleep(50 * time.Millisecond)
			}
		}(node)
	}

	// Collect results
	successCount := 0
	largeCount := 0

	for i := 0; i < 9; i++ {
		result := <-results
		assert.NoError(t, result.err, "All publishes should succeed locally")

		if result.msgSize < 5000 { // Under the 5KB limit
			successCount++
		} else {
			largeCount++
		}
	}

	// Verify expected results
	assert.Equal(t, 6, successCount, "Should have 6 messages within limit")
	assert.Equal(t, 3, largeCount, "Should have 3 messages over limit")

	// Wait for propagation
	time.Sleep(2 * time.Second)

	// Verify message reception
	// Count messages that were actually delivered (not oversized)
	totalDelivered := 0
	oversizedBlocked := 0

	for i, collector := range collectors {
		messages := collector.GetMessages()
		for _, msg := range messages {
			// Check if this is an oversized message based on content length
			if len(msg.Message.Content) >= 8000 {
				oversizedBlocked++
				t.Logf("ERROR: Received oversized message %s with content length %d", msg.Message.ID, len(msg.Message.Content))
			} else {
				totalDelivered++
			}
		}
		t.Logf("Node %d received %d messages", i, len(messages))
	}

	// We should see 6 messages within limit delivered to all nodes
	// The 3 oversized messages might be delivered to the sender (self-delivery)
	// but should not propagate to other nodes
	t.Logf("Total delivered: %d, Oversized blocked: %d", totalDelivered, oversizedBlocked)
	assert.GreaterOrEqual(t, totalDelivered, 12, "Should deliver at least 12 messages (6 valid messages * 2 receivers)")
	assert.LessOrEqual(t, totalDelivered, 18, "Should deliver at most 18 messages (6 valid messages * 3 nodes)")

	// The oversized messages (3 total) may appear due to self-delivery
	// but should not propagate beyond the sender
	assert.LessOrEqual(t, oversizedBlocked, 3, "At most 3 oversized messages (self-delivery only)")
}
