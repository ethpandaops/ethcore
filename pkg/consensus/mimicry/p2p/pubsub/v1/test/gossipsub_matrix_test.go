package v1_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMatrix tests various combinations of node counts, message sizes, and configurations
func TestGossipsubMatrix(t *testing.T) {
	testCases := []struct {
		name             string
		nodeCount        int
		messageCount     int
		messageSize      int
		maxMessageSize   int
		publishTimeout   time.Duration
		validationResult v1.ValidationResult
		expectReceive    bool
	}{
		{
			name:             "small_network_small_messages",
			nodeCount:        3,
			messageCount:     5,
			messageSize:      100,
			maxMessageSize:   1 << 20, // 1MB
			publishTimeout:   5 * time.Second,
			validationResult: v1.ValidationAccept,
			expectReceive:    true,
		},
		{
			name:             "medium_network_medium_messages",
			nodeCount:        5,
			messageCount:     10,
			messageSize:      10000,
			maxMessageSize:   1 << 20,
			publishTimeout:   5 * time.Second,
			validationResult: v1.ValidationAccept,
			expectReceive:    true,
		},
		{
			name:             "large_network_small_messages",
			nodeCount:        10,
			messageCount:     20,
			messageSize:      100,
			maxMessageSize:   1 << 20,
			publishTimeout:   10 * time.Second,
			validationResult: v1.ValidationAccept,
			expectReceive:    true,
		},
		{
			name:             "validation_reject",
			nodeCount:        3,
			messageCount:     1,
			messageSize:      100,
			maxMessageSize:   1 << 20,
			publishTimeout:   5 * time.Second,
			validationResult: v1.ValidationReject,
			expectReceive:    false,
		},
		{
			name:             "validation_ignore",
			nodeCount:        3,
			messageCount:     1,
			messageSize:      100,
			maxMessageSize:   1 << 20,
			publishTimeout:   5 * time.Second,
			validationResult: v1.ValidationIgnore,
			expectReceive:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create test infrastructure
			ti := NewTestInfrastructure(t)
			defer ti.Cleanup()

			// Create nodes with custom options
			opts := []v1.Option{
				v1.WithPubsubOptions(pubsub.WithMaxMessageSize(tc.maxMessageSize)),
				v1.WithPublishTimeout(tc.publishTimeout),
			}

			nodes, err := ti.CreateFullyConnectedNetwork(ctx, tc.nodeCount, opts...)
			require.NoError(t, err)

			// Create topic
			topic, err := CreateTestTopic(fmt.Sprintf("matrix-topic-%s", tc.name))
			require.NoError(t, err)

			// Create collectors
			collectors := make([]*MessageCollector, tc.nodeCount)
			for i := range collectors {
				collectors[i] = NewMessageCollector(tc.messageCount * tc.nodeCount)
			}

			// Register handlers with custom validation
			for i, node := range nodes {
				processor := collectors[i].CreateProcessor(node.ID)
				handler := v1.NewHandlerConfig(
					v1.WithProcessor(processor),
					v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
						return tc.validationResult
					}),
				)

				err := v1.Register(node.Gossipsub.Registry(), topic, handler)
				require.NoError(t, err)

				_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
				require.NoError(t, err)
			}

			// Wait for mesh to stabilize
			WaitForGossipsubReady(t, nodes, topic.Name(), tc.nodeCount)

			// Publish messages from different nodes
			publishedCount := 0
			for i := 0; i < tc.messageCount; i++ {
				senderIdx := i % tc.nodeCount
				msg := GossipTestMessage{
					ID:      fmt.Sprintf("msg-%d", i),
					Content: generateContent(tc.messageSize),
					From:    nodes[senderIdx].ID.String(),
				}

				err := v1.Publish(nodes[senderIdx].Gossipsub, topic, msg)
				if err == nil {
					publishedCount++
				}
			}

			// Wait for propagation
			time.Sleep(2 * time.Second)

			// Verify results
			if tc.expectReceive {
				// Each message should be received by (nodeCount - 1) nodes
				expectedTotal := publishedCount * (tc.nodeCount - 1)
				actualTotal := 0
				for _, collector := range collectors {
					actualTotal += collector.GetMessageCount()
				}

				// Allow some tolerance for large networks
				tolerance := 0.9
				if tc.nodeCount > 5 {
					tolerance = 0.8
				}

				minExpected := int(float64(expectedTotal) * tolerance)
				assert.GreaterOrEqual(t, actualTotal, minExpected,
					"Expected at least %d messages (%.0f%% of %d), got %d",
					minExpected, tolerance*100, expectedTotal, actualTotal)
			} else {
				// No messages should be received if validation fails
				for i, collector := range collectors {
					count := collector.GetMessageCount()
					assert.Equal(t, 0, count,
						"Node %d should not receive messages when validation fails", i)
				}
			}
		})
	}
}

// TestGossipsubWithDifferentParams tests gossipsub with different parameter configurations
func TestGossipsubWithDifferentParams(t *testing.T) {
	testCases := []struct {
		name   string
		params pubsub.GossipSubParams
	}{
		{
			name: "default_params",
			params: pubsub.GossipSubParams{
				D:                         6,
				Dlo:                       5,
				Dhi:                       12,
				Dscore:                    4,
				Dout:                      2,
				HistoryLength:             5,
				HistoryGossip:             3,
				Dlazy:                     6,
				GossipFactor:              0.25,
				GossipRetransmission:      3,
				HeartbeatInitialDelay:     100 * time.Millisecond,
				HeartbeatInterval:         1 * time.Second,
				SlowHeartbeatWarning:      0.1,
				FanoutTTL:                 60 * time.Second,
				PrunePeers:                16,
				PruneBackoff:              1 * time.Minute,
				UnsubscribeBackoff:        10 * time.Second,
				Connectors:                8,
				MaxPendingConnections:     128,
				ConnectionTimeout:         30 * time.Second,
				DirectConnectTicks:        uint64(300),
				DirectConnectInitialDelay: 1 * time.Second,
				OpportunisticGraftTicks:   uint64(60),
				OpportunisticGraftPeers:   2,
				GraftFloodThreshold:       10 * time.Second,
				MaxIHaveLength:            5000,
				MaxIHaveMessages:          10,
				IWantFollowupTime:         3 * time.Second,
			},
		},
		{
			name: "low_latency_params",
			params: pubsub.GossipSubParams{
				D:                         8,
				Dlo:                       6,
				Dhi:                       12,
				Dscore:                    5,
				Dout:                      3,
				HistoryLength:             10,
				HistoryGossip:             5,
				Dlazy:                     8,
				GossipFactor:              0.5,
				GossipRetransmission:      5,
				HeartbeatInitialDelay:     50 * time.Millisecond,
				HeartbeatInterval:         500 * time.Millisecond,
				FanoutTTL:                 60 * time.Second,
				PrunePeers:                16,
				PruneBackoff:              1 * time.Minute,
				UnsubscribeBackoff:        10 * time.Second,
				Connectors:                8,
				MaxPendingConnections:     128,
				ConnectionTimeout:         30 * time.Second,
				DirectConnectTicks:        uint64(300),
				DirectConnectInitialDelay: 1 * time.Second,
				OpportunisticGraftTicks:   uint64(60),
				OpportunisticGraftPeers:   2,
				GraftFloodThreshold:       10 * time.Second,
				MaxIHaveLength:            5000,
				MaxIHaveMessages:          10,
				IWantFollowupTime:         3 * time.Second,
			},
		},
		{
			name: "high_reliability_params",
			params: pubsub.GossipSubParams{
				D:                         10,
				Dlo:                       8,
				Dhi:                       15,
				Dscore:                    6,
				Dout:                      4,
				HistoryLength:             20,
				HistoryGossip:             10,
				Dlazy:                     12,
				GossipFactor:              0.75,
				GossipRetransmission:      10,
				HeartbeatInitialDelay:     100 * time.Millisecond,
				HeartbeatInterval:         2 * time.Second,
				FanoutTTL:                 60 * time.Second,
				PrunePeers:                16,
				PruneBackoff:              1 * time.Minute,
				UnsubscribeBackoff:        10 * time.Second,
				Connectors:                8,
				MaxPendingConnections:     128,
				ConnectionTimeout:         30 * time.Second,
				DirectConnectTicks:        uint64(300),
				DirectConnectInitialDelay: 1 * time.Second,
				OpportunisticGraftTicks:   uint64(60),
				OpportunisticGraftPeers:   2,
				GraftFloodThreshold:       10 * time.Second,
				MaxIHaveLength:            5000,
				MaxIHaveMessages:          10,
				IWantFollowupTime:         3 * time.Second,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ti := NewTestInfrastructure(t)
			defer ti.Cleanup()

			// Create nodes with custom gossipsub params
			opts := []v1.Option{
				v1.WithGossipSubParams(tc.params),
			}

			nodes, err := ti.CreateFullyConnectedNetwork(ctx, 5, opts...)
			require.NoError(t, err)

			// Create topic
			topic, err := CreateTestTopic(fmt.Sprintf("params-topic-%s", tc.name))
			require.NoError(t, err)

			// Setup collectors and handlers
			collectors := make([]*MessageCollector, 5)
			for i, node := range nodes {
				collectors[i] = NewMessageCollector(50)
				handler := CreateTestHandler(collectors[i].CreateProcessor(node.ID))

				err := v1.Register(node.Gossipsub.Registry(), topic, handler)
				require.NoError(t, err)

				_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
				require.NoError(t, err)
			}

			// Wait for mesh
			WaitForGossipsubReady(t, nodes, topic.Name(), 5)

			// Send test messages
			for i := 0; i < 10; i++ {
				msg := GossipTestMessage{
					ID:      fmt.Sprintf("params-msg-%d", i),
					Content: fmt.Sprintf("Message %d with %s params", i, tc.name),
					From:    nodes[i%5].ID.String(),
				}

				err := v1.Publish(nodes[i%5].Gossipsub, topic, msg)
				require.NoError(t, err)
			}

			// Wait for propagation
			time.Sleep(3 * time.Second)

			// Verify all nodes received messages
			totalReceived := 0
			for _, collector := range collectors {
				totalReceived += collector.GetMessageCount()
			}

			// Each of 10 messages should be received by 4 nodes (not the sender)
			expectedTotal := 10 * 4
			assert.GreaterOrEqual(t, totalReceived, int(float64(expectedTotal)*0.8),
				"Expected at least 80%% of messages to be delivered with %s params", tc.name)
		})
	}
}

// TestConcurrentOperations tests concurrent subscribe/unsubscribe/publish operations
func TestConcurrentOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create a network of 5 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 5)
	require.NoError(t, err)

	// Create multiple topics
	topicCount := 3
	topics := make([]*v1.Topic[GossipTestMessage], topicCount)
	for i := 0; i < topicCount; i++ {
		topic, err := CreateTestTopic(fmt.Sprintf("concurrent-topic-%d", i))
		require.NoError(t, err)
		topics[i] = topic
	}

	// Register handlers for all topics on all nodes
	for _, node := range nodes {
		for _, topic := range topics {
			handler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				// Simple processor that just returns nil
				return nil
			})

			err := v1.Register(node.Gossipsub.Registry(), topic, handler)
			require.NoError(t, err)
		}
	}

	// Run concurrent operations
	errChan := make(chan error, 100)
	doneChan := make(chan bool)

	// Start concurrent operations
	go func() {
		defer close(doneChan)

		// Concurrent subscribes/unsubscribes
		for i := 0; i < 10; i++ {
			go func(idx int) {
				nodeIdx := idx % len(nodes)
				topicIdx := idx % len(topics)

				// Subscribe
				sub, err := v1.Subscribe(ctx, nodes[nodeIdx].Gossipsub, topics[topicIdx])
				if err != nil {
					select {
					case errChan <- fmt.Errorf("subscribe error: %w", err):
					default:
					}
					return
				}

				// Wait a bit
				time.Sleep(100 * time.Millisecond)

				// Publish a message
				msg := GossipTestMessage{
					ID:      fmt.Sprintf("concurrent-msg-%d", idx),
					Content: "Concurrent test message",
					From:    nodes[nodeIdx].ID.String(),
				}

				if err := v1.Publish(nodes[nodeIdx].Gossipsub, topics[topicIdx], msg); err != nil {
					select {
					case errChan <- fmt.Errorf("publish error: %w", err):
					default:
					}
				}

				// Wait a bit more
				time.Sleep(100 * time.Millisecond)

				// Unsubscribe
				sub.Cancel()
			}(i)
		}

		// Wait for all operations to complete
		time.Sleep(3 * time.Second)
	}()

	// Wait for completion or timeout
	select {
	case <-doneChan:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Concurrent operations timed out")
	}

	// Check for errors
	close(errChan)
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	assert.Empty(t, errors, "Concurrent operations should not produce errors")
}

// generateContent generates a string of the specified size
func generateContent(size int) string {
	if size <= 0 {
		return ""
	}

	// Create a repeating pattern
	pattern := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, size)

	for i := 0; i < size; i++ {
		result[i] = pattern[i%len(pattern)]
	}

	return string(result)
}
