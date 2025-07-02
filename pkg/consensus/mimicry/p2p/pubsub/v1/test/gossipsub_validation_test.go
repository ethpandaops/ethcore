package v1_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	// pubsub "github.com/libp2p/go-libp2p-pubsub" // TODO: Uncomment when implementing score params test
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLibp2pValidation tests that validation happens at the libp2p level before propagation
func TestLibp2pValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 nodes: publisher, subscriber with validator, and observer
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	publisher := nodes[0]
	validator := nodes[1]
	observer := nodes[2]

	// Create topic
	topic, err := CreateTestTopic("validation-test")
	require.NoError(t, err)

	// Track validation calls
	var validationCount atomic.Int32
	var rejectedCount atomic.Int32

	// Create handler with validator that rejects messages with "reject" in content
	validatorHandler := v1.NewHandlerConfig(
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			validationCount.Add(1)
			if msg.Content == "reject" {
				rejectedCount.Add(1)
				return v1.ValidationReject
			}
			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			// This should only be called for accepted messages
			t.Logf("Validator node processed message: %s", msg.Content)
			return nil
		}),
	)

	// Observer handler - needs a validator too to enforce validation at libp2p level
	observerCollector := NewMessageCollector(10)
	observerHandler := v1.NewHandlerConfig(
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			// Observer uses same validation logic - reject "reject" messages
			if msg.Content == "reject" {
				return v1.ValidationReject
			}
			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			select {
			case observerCollector.messages <- ReceivedMessage{
				Node:    observer.ID,
				Message: msg,
				From:    from,
			}:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		}),
	)

	// Register handlers
	err = v1.Register(validator.Gossipsub.Registry(), topic, validatorHandler)
	require.NoError(t, err)
	err = v1.Register(observer.Gossipsub.Registry(), topic, observerHandler)
	require.NoError(t, err)

	// Subscribe validator and observer
	_, err = v1.Subscribe(ctx, validator.Gossipsub, topic)
	require.NoError(t, err)
	_, err = v1.Subscribe(ctx, observer.Gossipsub, topic)
	require.NoError(t, err)

	// Wait for mesh to stabilize
	time.Sleep(2 * time.Second)

	// Test 1: Publish accepted message
	acceptMsg := GossipTestMessage{
		ID:      "msg-1",
		Content: "accept",
		From:    publisher.ID.String(),
	}
	err = v1.Publish(publisher.Gossipsub, topic, acceptMsg)
	require.NoError(t, err)

	// Wait for propagation
	time.Sleep(1 * time.Second)

	// Check observer received the accepted message
	messages := observerCollector.GetMessages()
	assert.Equal(t, 1, len(messages), "Observer should receive accepted message")
	if len(messages) > 0 {
		assert.Equal(t, "accept", messages[0].Message.Content)
	}

	// Test 2: Publish rejected message
	rejectMsg := GossipTestMessage{
		ID:      "msg-2",
		Content: "reject",
		From:    publisher.ID.String(),
	}
	err = v1.Publish(publisher.Gossipsub, topic, rejectMsg)
	require.NoError(t, err)

	// Wait for potential propagation
	time.Sleep(1 * time.Second)

	// Check that observer did NOT receive the rejected message
	messages = observerCollector.GetMessages()
	assert.Equal(t, 0, len(messages), "Observer should NOT receive rejected message")

	// Verify validation was called
	assert.GreaterOrEqual(t, validationCount.Load(), int32(2), "Validator should be called for both messages")
	assert.GreaterOrEqual(t, rejectedCount.Load(), int32(1), "At least one message should be rejected")

	t.Logf("Validation count: %d, Rejected count: %d", validationCount.Load(), rejectedCount.Load())
}

// TestValidationWithScoreParams tests that score parameters are properly applied
// TODO: This test requires peer scoring to be enabled in the pubsub router
/*
func TestValidationWithScoreParams(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Create topic
	topic, err := CreateTestTopic("score-params-test")
	require.NoError(t, err)

	// Create score parameters
	scoreParams := &pubsub.TopicScoreParams{
		TopicWeight:                     1.0,
		TimeInMeshWeight:                0.01,
		TimeInMeshQuantum:               time.Second,
		TimeInMeshCap:                   3600,
		FirstMessageDeliveriesWeight:    1.0,
		FirstMessageDeliveriesDecay:     0.5,
		FirstMessageDeliveriesCap:       100,
		MeshMessageDeliveriesWeight:     -1.0,
		MeshMessageDeliveriesDecay:      0.5,
		MeshMessageDeliveriesCap:        100,
		MeshMessageDeliveriesThreshold:  20,
		MeshMessageDeliveriesWindow:     10 * time.Millisecond,
		MeshMessageDeliveriesActivation: 1 * time.Second,
		MeshFailurePenaltyWeight:        -1.0,
		MeshFailurePenaltyDecay:         0.5,
		InvalidMessageDeliveriesWeight:  -1.0,
		InvalidMessageDeliveriesDecay:   0.5,
	}

	// Create handler with score parameters
	handler := v1.NewHandlerConfig(
		v1.WithScoreParams[GossipTestMessage](scoreParams),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)

	// Register handler
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)

	// Subscribe - this should apply the score parameters
	_, err = v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)

	// The score parameters should now be applied to the topic
	// We can't directly verify this without accessing internal libp2p state,
	// but the fact that Subscribe didn't error means SetScoreParams was called successfully
	t.Log("Score parameters applied successfully")
}
*/

// TestValidationBeforeDecoding tests that validation metrics are recorded even for decode failures
func TestValidationBeforeDecoding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes with global invalid payload handler
	var globalInvalidCount atomic.Int32
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2,
		v1.WithGlobalInvalidPayloadHandler(func(ctx context.Context, data []byte, err error, from peer.ID, topic string) {
			globalInvalidCount.Add(1)
			t.Logf("Global invalid payload handler called: topic=%s, from=%s, data=%q, err=%v", topic, from, string(data), err)
		}))
	require.NoError(t, err)

	// Create topic with a broken encoder that will fail to decode certain messages
	brokenEncoder := &BrokenEncoder{
		failOnDecode: func(data []byte) bool {
			return string(data) == "INVALID"
		},
	}
	topic, err := v1.NewTopic[GossipTestMessage]("/test/broken-decode/1.0.0")
	require.NoError(t, err)

	// Track invalid payload handler calls
	var invalidPayloadCount atomic.Int32

	// Create handler with invalid payload handler
	handler := v1.NewHandlerConfig(
		v1.WithEncoder[GossipTestMessage](brokenEncoder),
		v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
			invalidPayloadCount.Add(1)
			t.Logf("Invalid payload handler called from %s: data=%q, err=%v", from, string(data), err)
		}),
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			// This should not be called for messages that fail to decode
			t.Logf("Validator called (this is expected with libp2p validation)")
			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			// This should not be called for messages that fail to decode
			t.Error("Processor called for message that should have failed decoding")
			return nil
		}),
	)

	// Register handler
	err = v1.Register(nodes[1].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)

	// Subscribe
	_, err = v1.Subscribe(ctx, nodes[1].Gossipsub, topic)
	require.NoError(t, err)

	// Wait for mesh
	time.Sleep(2 * time.Second)

	// To test decode failure at the validator level, we need to use a different approach
	// Since we can't easily publish raw bytes through the typed API, we'll have node[0]
	// publish a valid message that the broken encoder will fail to decode

	// First, let's have node 0 join the topic so it can publish
	pubsubTopic := nodes[0].Gossipsub.GetPubSub()
	topicHandle, err := pubsubTopic.Join(topic.Name())
	require.NoError(t, err)

	// Publish raw bytes that will fail decoding
	err = topicHandle.Publish(ctx, []byte("INVALID"))
	require.NoError(t, err)

	// Wait for message handling
	time.Sleep(1 * time.Second)

	// Verify invalid payload handler was called
	totalInvalidCalls := invalidPayloadCount.Load() + globalInvalidCount.Load()
	assert.GreaterOrEqual(t, totalInvalidCalls, int32(1), "Invalid payload handler (topic or global) should be called at least once")
	t.Logf("Invalid payload counts - Topic: %d, Global: %d", invalidPayloadCount.Load(), globalInvalidCount.Load())
}

// BrokenEncoder is a test encoder that can be configured to fail
type BrokenEncoder struct {
	failOnDecode func([]byte) bool
}

func (e *BrokenEncoder) Encode(msg GossipTestMessage) ([]byte, error) {
	return []byte(fmt.Sprintf("%s|%s|%s", msg.ID, msg.Content, msg.From)), nil
}

func (e *BrokenEncoder) Decode(data []byte) (GossipTestMessage, error) {
	if e.failOnDecode != nil && e.failOnDecode(data) {
		return GossipTestMessage{}, fmt.Errorf("decode failure triggered")
	}
	// Normal decode logic - same as TestEncoder
	str := string(data)
	parts := strings.Split(str, "|")
	if len(parts) != 3 {
		return GossipTestMessage{}, fmt.Errorf("invalid message format")
	}
	return GossipTestMessage{
		ID:      parts[0],
		Content: parts[1],
		From:    parts[2],
	}, nil
}

// TestMultipleValidators tests that multiple subscribers with different validators work correctly
func TestMultipleValidators(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 4 nodes: publisher + 3 subscribers with different validators
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 4)
	require.NoError(t, err)

	publisher := nodes[0]

	// Create topic
	topic, err := CreateTestTopic("multi-validator-test")
	require.NoError(t, err)

	// Create different validators for each subscriber
	validators := []struct {
		name      string
		node      *TestNode
		acceptIDs []string
		collector *MessageCollector
	}{
		{
			name:      "odd-validator",
			node:      nodes[1],
			acceptIDs: []string{"1", "3", "5"},
			collector: NewMessageCollector(10),
		},
		{
			name:      "even-validator",
			node:      nodes[2],
			acceptIDs: []string{"2", "4", "6"},
			collector: NewMessageCollector(10),
		},
		{
			name:      "all-validator",
			node:      nodes[3],
			acceptIDs: []string{"1", "2", "3", "4", "5", "6"},
			collector: NewMessageCollector(10),
		},
	}

	// Setup validators
	for _, v := range validators {
		validator := v // capture loop variable

		handler := v1.NewHandlerConfig(
			v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
				for _, acceptID := range validator.acceptIDs {
					if msg.ID == acceptID {
						return v1.ValidationAccept
					}
				}
				return v1.ValidationReject
			}),
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				select {
				case validator.collector.messages <- ReceivedMessage{
					Node:    validator.node.ID,
					Message: msg,
					From:    from,
				}:
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			}),
		)

		err = v1.Register(validator.node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		_, err = v1.Subscribe(ctx, validator.node.Gossipsub, topic)
		require.NoError(t, err)
	}

	// Wait for mesh
	time.Sleep(2 * time.Second)

	// Publish messages with IDs 1-6
	for i := 1; i <= 6; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("%d", i),
			Content: fmt.Sprintf("Message %d", i),
			From:    publisher.ID.String(),
		}
		err = v1.Publish(publisher.Gossipsub, topic, msg)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond) // Small delay between messages
	}

	// Wait for all messages to propagate
	time.Sleep(2 * time.Second)

	// Verify each validator received only the messages they accepted
	oddMessages := validators[0].collector.GetMessages()
	assert.Equal(t, 3, len(oddMessages), "Odd validator should receive 3 messages")
	for _, msg := range oddMessages {
		id := msg.Message.ID
		assert.Contains(t, []string{"1", "3", "5"}, id, "Odd validator should only receive odd IDs")
	}

	evenMessages := validators[1].collector.GetMessages()
	assert.Equal(t, 3, len(evenMessages), "Even validator should receive 3 messages")
	for _, msg := range evenMessages {
		id := msg.Message.ID
		assert.Contains(t, []string{"2", "4", "6"}, id, "Even validator should only receive even IDs")
	}

	allMessages := validators[2].collector.GetMessages()
	assert.Equal(t, 6, len(allMessages), "All validator should receive all 6 messages")
}
