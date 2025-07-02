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

func TestSimpleMalformedMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	topic, err := v1.NewTopic[GossipTestMessage]("fail-topic")
	require.NoError(t, err)

	// Create collector for invalid payloads
	collector := NewInvalidPayloadCollector()

	// Create an encoder that encodes successfully but always fails to decode
	malformedEncoder := &MalformedEncoder{
		EncodeValid:      true,  // Allow publishing
		DecodeSuccess:    false, // Cause decode errors
		DecodeError:      errors.New("test decode failure"),
		ProduceMalformed: true, // Produce malformed data that will fail decoding
	}

	// Register handler on node 1 (receiver) with invalid payload handler
	handler1 := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](malformedEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
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

	// Register handler on node 0 (publisher) for publishing
	handler0 := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](malformedEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, handler0)
	require.NoError(t, err)

	// Wait for mesh formation
	time.Sleep(1 * time.Second)

	// Publish message from node 0
	msg := GossipTestMessage{
		ID:      "test-1",
		Content: "This will be encoded OK but fail to decode",
		From:    nodes[0].ID.String(),
	}

	t.Log("Publishing message with failing decoder...")
	err = v1.Publish(nodes[0].Gossipsub, topic, msg)
	require.NoError(t, err)

	// Wait for invalid payload handler to be called
	require.Eventually(t, func() bool {
		return collector.Count() > 0
	}, 5*time.Second, 100*time.Millisecond, "Invalid payload handler should be called")

	// Verify invalid payload was collected
	payloads := collector.GetPayloads()
	assert.Len(t, payloads, 1)
	assert.Equal(t, []byte("malformed|data"), payloads[0].Data)
	assert.Error(t, payloads[0].Error)
	assert.Contains(t, payloads[0].Error.Error(), "test decode failure")
	assert.Equal(t, nodes[0].ID, payloads[0].From)

	t.Log("SUCCESS: Invalid payload handlers working correctly")
}
