package v1_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkingEncoderFailingDecoder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	topic, err := v1.NewTopic[GossipTestMessage]("working-failing-topic")
	require.NoError(t, err)

	// Create collector for invalid payloads
	collector := NewInvalidPayloadCollector()

	// DIFFERENT APPROACH: Publisher uses working encoder, receiver uses failing decoder
	// This should cause the message to encode properly but fail to decode on the receiver

	// Working encoder for publishing
	workingEncoder := &TestEncoder{}

	// Failing decoder that always fails
	failingDecoder := func(data []byte) (GossipTestMessage, error) {
		return GossipTestMessage{}, errors.New("intentional decode failure")
	}

	// Register handler on node 1 (receiver) with ONLY failing decoder and invalid payload handler
	// Do NOT provide an encoder, so it must use the decoder function
	handler1 := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithDecoder[GossipTestMessage](failingDecoder),
		v1.WithProcessor[GossipTestMessage](func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			t.Fatal("❌ Node 1 processor should not be called - decoding should fail")
			return nil
		}),
		v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
			collector.CollectWithoutTopic(ctx, data, err, from)
			t.Logf("✅ Node 1 invalid payload handler called: err=%v, data=%s, from=%s", err, string(data), from)
		}),
	)

	err = v1.Register(nodes[1].Gossipsub.Registry(), topic, handler1)
	require.NoError(t, err)
	_, err = v1.Subscribe(ctx, nodes[1].Gossipsub, topic)
	require.NoError(t, err)

	// Register handler on node 0 (publisher) with working encoder
	handler0 := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](workingEncoder),
		v1.WithProcessor[GossipTestMessage](func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, handler0)
	require.NoError(t, err)
	_, err = v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)

	// Wait for mesh formation
	time.Sleep(2 * time.Second)

	// Publish message from node 0 with working encoder
	msg := GossipTestMessage{
		ID:      "test-1",
		Content: "This will be encoded properly but fail to decode on receiver",
		From:    nodes[0].ID.String(),
	}

	t.Log("Publishing message with working encoder to receiver with failing decoder...")
	err = v1.Publish(nodes[0].Gossipsub, topic, msg)
	require.NoError(t, err)

	// Wait for invalid payload handler to be called
	require.Eventually(t, func() bool {
		return collector.Count() > 0
	}, 10*time.Second, 500*time.Millisecond, "Invalid payload handler should be called")

	// Verify invalid payload was collected
	payloads := collector.GetPayloads()
	assert.Len(t, payloads, 1)
	assert.Error(t, payloads[0].Error)
	assert.Contains(t, payloads[0].Error.Error(), "intentional decode failure")
	assert.Equal(t, nodes[0].ID, payloads[0].From)

	t.Log("SUCCESS: Invalid payload handlers working correctly")
}
