package v1_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicValidation tests that validation works correctly.
func TestBasicValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Create topic
	topic, err := CreateTestTopic("validation-test")
	require.NoError(t, err)

	// Track validation calls
	var validationCount atomic.Int32
	var rejectedCount atomic.Int32
	var processedCount atomic.Int32

	// Create handler with validator that rejects messages with "reject" in content
	handler := v1.NewHandlerConfig(
		v1.WithEncoder(&TestEncoder{}),
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			validationCount.Add(1)
			if msg.Content == "reject" {
				rejectedCount.Add(1)
				return v1.ValidationReject
			}
			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			processedCount.Add(1)
			t.Logf("Processed message: %s", msg.Content)
			return nil
		}),
	)

	// Register handler on both nodes
	for _, node := range nodes {
		regErr := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, regErr)
		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
	}

	// Wait for mesh
	time.Sleep(2 * time.Second)

	// Test 1: Valid message should be processed
	validMsg := GossipTestMessage{
		ID:      "valid-1",
		Content: "valid message",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, validMsg)
	require.NoError(t, err)

	// Test 2: Invalid message should be rejected
	invalidMsg := GossipTestMessage{
		ID:      "invalid-1",
		Content: "reject",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, invalidMsg)
	// This should fail because validation rejects it
	assert.Error(t, err, "Publishing rejected message should fail")

	// Wait for processing
	time.Sleep(2 * time.Second)

	// Verify validation occurred
	assert.Greater(t, validationCount.Load(), int32(0), "Validation should have been called")
	// Since rejected message fails at publish time, rejectedCount might be 0
	assert.Equal(t, int32(2), processedCount.Load(), "Valid message should be processed by both nodes")
}
