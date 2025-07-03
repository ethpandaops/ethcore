package v1_test

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// TestWithPubsubOptions tests that custom pubsub options can be provided
func TestWithPubsubOptions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Custom pubsub options
	customOpts := []pubsub.Option{
		pubsub.WithFloodPublish(true),
		pubsub.WithDiscovery(nil),
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
	}

	// Create a node with custom options
	node, err := ti.CreateNode(ctx, v1.WithPubsubOptions(customOpts...))
	require.NoError(t, err)
	require.NotNil(t, node)

	// Verify the gossipsub instance was created successfully
	require.True(t, node.Gossipsub.IsStarted())
}

// TestWithPubsubOptionsMultiple tests that multiple WithPubsubOptions calls append options
func TestWithPubsubOptionsMultiple(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create a node with multiple option calls
	node, err := ti.CreateNode(ctx,
		v1.WithPubsubOptions(pubsub.WithFloodPublish(true)),
		v1.WithPubsubOptions(pubsub.WithMessageSigning(false)),
		v1.WithPubsubOptions(pubsub.WithMaxMessageSize(5<<20)), // 5MB
	)
	require.NoError(t, err)
	require.NotNil(t, node)

	// Verify the gossipsub instance was created successfully
	require.True(t, node.Gossipsub.IsStarted())
}

// TestWithPubsubOptionsScoring tests providing peer scoring options
func TestWithPubsubOptionsScoring(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Peer score params
	peerScoreParams := &pubsub.PeerScoreParams{
		AppSpecificScore: func(p peer.ID) float64 {
			return 0
		},
		DecayInterval: time.Second,
		DecayToZero:   0.01,
	}

	peerScoreThresholds := &pubsub.PeerScoreThresholds{
		GossipThreshold:             -100,
		PublishThreshold:            -200,
		GraylistThreshold:           -300,
		AcceptPXThreshold:           100,
		OpportunisticGraftThreshold: 1,
	}

	// Create a node with peer scoring enabled
	node, err := ti.CreateNode(ctx,
		v1.WithPubsubOptions(
			pubsub.WithPeerScore(peerScoreParams, peerScoreThresholds),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, node)

	// Verify the gossipsub instance was created successfully
	require.True(t, node.Gossipsub.IsStarted())
}

// TestWithPubsubOptionsIntegration tests pubsub options work with actual pub/sub
func TestWithPubsubOptionsIntegration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes with flood publish enabled
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2,
		v1.WithPubsubOptions(pubsub.WithFloodPublish(true)),
	)
	require.NoError(t, err)

	// Create topic
	topic, err := CreateTestTopic("pubsub-options-test")
	require.NoError(t, err)

	// Create message collector
	collector := NewMessageCollector(10)

	// Register handlers on both nodes
	for _, node := range nodes {
		handler := CreateTestHandler(collector.CreateProcessor(node.ID))
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	// Subscribe both nodes
	for _, node := range nodes {
		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
	}

	// Wait for mesh to stabilize
	time.Sleep(2 * time.Second)

	// Publish a message
	msg := GossipTestMessage{
		ID:      "test-1",
		Content: "Testing with custom pubsub options",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, msg)
	require.NoError(t, err)

	// Wait for propagation
	time.Sleep(1 * time.Second)

	// Both nodes should receive the message (flood publish ensures this)
	messages := collector.GetMessages()
	require.GreaterOrEqual(t, len(messages), 2, "Both nodes should receive the message with flood publish")
}
