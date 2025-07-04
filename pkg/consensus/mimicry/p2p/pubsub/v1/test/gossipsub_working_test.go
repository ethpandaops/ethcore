package v1_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestWorkingMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Create a working topic
	topic, err := CreateTestTopic("working-topic")
	require.NoError(t, err)

	// Track messages
	var messageReceived atomic.Bool

	// Create handler
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			t.Logf("Received message: %+v from %s", msg, from)
			messageReceived.Store(true)

			return nil
		}),
	)

	// Subscribe both nodes
	for i, node := range nodes {
		regErr := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, regErr)
		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
		t.Logf("Node %d subscribed", i)
	}

	// Wait for mesh
	time.Sleep(2 * time.Second)

	// Publish message
	msg := GossipTestMessage{
		ID:      "test-1",
		Content: "This should work",
		From:    nodes[0].ID.String(),
	}

	t.Log("Publishing message...")
	err = v1.Publish(nodes[0].Gossipsub, topic, msg)
	require.NoError(t, err)

	// Wait for propagation
	time.Sleep(2 * time.Second)

	require.True(t, messageReceived.Load(), "Message should have been received")
}
