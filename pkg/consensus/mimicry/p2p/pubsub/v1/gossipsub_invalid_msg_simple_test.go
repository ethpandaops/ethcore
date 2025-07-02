package v1_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SimpleMalformedEncoder always encodes successfully but fails to decode
type SimpleMalformedEncoder struct {
	decodeError error
}

func (e *SimpleMalformedEncoder) Encode(msg GossipTestMessage) ([]byte, error) {
	// Always encode successfully
	encoded := fmt.Sprintf("%s|%s|%s", msg.ID, msg.Content, msg.From)
	return []byte(encoded), nil
}

func (e *SimpleMalformedEncoder) Decode(data []byte) (GossipTestMessage, error) {
	// Always fail to decode
	return GossipTestMessage{}, e.decodeError
}

func TestSimpleMalformedMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Create encoder that always fails decoding
	failEncoder := &SimpleMalformedEncoder{
		decodeError: errors.New("always fails"),
	}

	topic, err := v1.NewTopic("fail-topic", failEncoder)
	require.NoError(t, err)

	// Track if handler was called on each node
	node0HandlerCalled := false
	node1HandlerCalled := false
	var capturedError error
	var capturedData []byte

	// Register handler with invalid payload handler on node 0
	handler0 := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			t.Fatal("Node 0 processor should not be called for invalid messages")
			return nil
		}),
		v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
			node0HandlerCalled = true
			capturedError = err
			capturedData = data
			t.Logf("Node 0 invalid payload handler called: err=%v, data=%s, from=%s", err, string(data), from)
		}),
	)

	// Register handler with invalid payload handler on node 1
	handler1 := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			t.Fatal("Node 1 processor should not be called for invalid messages")
			return nil
		}),
		v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
			node1HandlerCalled = true
			capturedError = err
			capturedData = data
			t.Logf("Node 1 invalid payload handler called: err=%v, data=%s, from=%s", err, string(data), from)
		}),
	)

	// Subscribe both nodes
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, handler0)
	require.NoError(t, err)
	_, err = v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)
	t.Log("Node 0 subscribed")

	err = v1.Register(nodes[1].Gossipsub.Registry(), topic, handler1)
	require.NoError(t, err)
	_, err = v1.Subscribe(ctx, nodes[1].Gossipsub, topic)
	require.NoError(t, err)
	t.Log("Node 1 subscribed")

	// Wait for mesh
	time.Sleep(2 * time.Second)

	// Verify connectivity
	t.Logf("Node 0 peers: %v", nodes[0].Host.Network().Peers())
	t.Logf("Node 1 peers: %v", nodes[1].Host.Network().Peers())
	
	// Check topic peers
	ps0 := nodes[0].Gossipsub.GetPubSub()
	ps1 := nodes[1].Gossipsub.GetPubSub()
	t.Logf("Node 0 topic peers: %v", ps0.ListPeers("fail-topic"))
	t.Logf("Node 1 topic peers: %v", ps1.ListPeers("fail-topic"))

	// Publish message from node 0
	msg := GossipTestMessage{
		ID:      "test-1",
		Content: "This will fail to decode",
		From:    nodes[0].ID.String(),
	}

	t.Log("Publishing message...")
	err = v1.Publish(nodes[0].Gossipsub, topic, msg)
	require.NoError(t, err)

	// Wait longer for propagation
	time.Sleep(3 * time.Second)

	// Check if handlers were called
	assert.True(t, node0HandlerCalled || node1HandlerCalled, "At least one invalid payload handler should have been called")
	if node0HandlerCalled {
		t.Log("Node 0 received its own message and failed to decode")
	}
	if node1HandlerCalled {
		t.Log("Node 1 received the message and failed to decode")
	}
	assert.Error(t, capturedError)
	assert.Contains(t, capturedError.Error(), "always fails")
	assert.NotNil(t, capturedData)
}