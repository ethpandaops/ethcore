package v1_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBeaconBlock represents a test beacon block message
type TestBeaconBlock struct {
	Slot       uint64
	ParentRoot []byte
	StateRoot  []byte
	BodyRoot   []byte
}

// TestBeaconBlockEncoder implements encoding/decoding for TestBeaconBlock
type TestBeaconBlockEncoder struct{}

func (e *TestBeaconBlockEncoder) Encode(msg *TestBeaconBlock) ([]byte, error) {
	// Simple encoding: "slot|parentroot|stateroot|bodyroot"
	encoded := fmt.Sprintf("%d|%x|%x|%x", msg.Slot, msg.ParentRoot, msg.StateRoot, msg.BodyRoot)
	return []byte(encoded), nil
}

func (e *TestBeaconBlockEncoder) Decode(data []byte) (*TestBeaconBlock, error) {
	str := string(data)
	parts := strings.Split(str, "|")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid beacon block format: expected 4 parts, got %d", len(parts))
	}

	var slot uint64
	if _, err := fmt.Sscanf(parts[0], "%d", &slot); err != nil {
		return nil, fmt.Errorf("invalid slot: %w", err)
	}

	var parentRoot, stateRoot, bodyRoot []byte
	var err error
	if parentRoot, err = decodeHex(parts[1]); err != nil {
		return nil, fmt.Errorf("invalid parent root: %w", err)
	}
	if stateRoot, err = decodeHex(parts[2]); err != nil {
		return nil, fmt.Errorf("invalid state root: %w", err)
	}
	if bodyRoot, err = decodeHex(parts[3]); err != nil {
		return nil, fmt.Errorf("invalid body root: %w", err)
	}

	return &TestBeaconBlock{
		Slot:       slot,
		ParentRoot: parentRoot,
		StateRoot:  stateRoot,
		BodyRoot:   bodyRoot,
	}, nil
}

func decodeHex(s string) ([]byte, error) {
	if len(s) == 0 {
		return []byte{}, nil
	}
	result := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var b byte
		if _, err := fmt.Sscanf(s[i:i+2], "%02x", &b); err != nil {
			return nil, err
		}
		result[i/2] = b
	}
	return result, nil
}

// TestSubscriptionLifecycle_MultipleMessages tests that a subscription can receive multiple messages sequentially
// This is the core test for the reported issue where Next() blocks after the first message
func TestSubscriptionLifecycle_MultipleMessages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Message counter
	var messageCount atomic.Int64
	var processedMessages []string
	var mu sync.Mutex

	// Create a topic for beacon blocks
	topic, err := v1.NewTopic[*TestBeaconBlock]("beacon_block")
	require.NoError(t, err)

	// Create encoder
	encoder := &TestBeaconBlockEncoder{}

	// Create handler that counts messages
	handler := v1.NewHandlerConfig[*TestBeaconBlock](
		v1.WithEncoder[*TestBeaconBlock](encoder),
		v1.WithProcessor[*TestBeaconBlock](func(ctx context.Context, data *TestBeaconBlock, from peer.ID) error {
			count := messageCount.Add(1)
			t.Logf("Received message %d: slot=%d from=%s", count, data.Slot, from.String()[:8])

			mu.Lock()
			processedMessages = append(processedMessages, fmt.Sprintf("slot-%d", data.Slot))
			mu.Unlock()

			return nil
		}),
	)

	// Register the handler on both nodes
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)
	err = v1.Register(nodes[1].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)

	// Subscribe to the topic on node 0
	t.Log("Subscribing to topic...")
	sub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Allow time for subscription to propagate
	time.Sleep(2 * time.Second)

	// Wait for mesh to form - check active topics
	t.Log("Waiting for mesh to form...")
	require.Eventually(t, func() bool {
		topics0 := nodes[0].Gossipsub.ActiveTopics()
		topics1 := nodes[1].Gossipsub.ActiveTopics()
		t.Logf("Node0 topics: %v, Node1 topics: %v", topics0, topics1)
		return len(topics0) > 0
	}, 10*time.Second, 500*time.Millisecond, "Expected topics to be active")

	// Publish multiple messages with delays
	messageSlots := []uint64{100, 101, 102, 103, 104}

	for i, slot := range messageSlots {
		msg := &TestBeaconBlock{
			Slot:       slot,
			ParentRoot: []byte(fmt.Sprintf("parent-%d", slot)),
			StateRoot:  []byte(fmt.Sprintf("state-%d", slot)),
			BodyRoot:   []byte(fmt.Sprintf("body-%d", slot)),
		}

		t.Logf("Publishing message %d (slot %d)...", i+1, slot)
		err := v1.Publish(nodes[1].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Small delay between messages to see if processing continues
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all messages to be processed
	t.Log("Waiting for all messages to be processed...")
	require.Eventually(t, func() bool {
		count := messageCount.Load()
		t.Logf("Processed %d/%d messages", count, len(messageSlots))
		return count == int64(len(messageSlots))
	}, 15*time.Second, 500*time.Millisecond, "Expected all messages to be processed")

	// Verify all messages were received in order
	mu.Lock()
	expectedMessages := []string{"slot-100", "slot-101", "slot-102", "slot-103", "slot-104"}
	assert.Equal(t, expectedMessages, processedMessages, "Messages should be processed in order")
	mu.Unlock()

	t.Logf("SUCCESS: All %d messages were processed correctly", len(messageSlots))
}

// TestSubscriptionLifecycle_SubscriptionCleanup verifies that subscriptions are properly cleaned up
func TestSubscriptionLifecycle_SubscriptionCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messageCount atomic.Int64

	topic, err := v1.NewTopic[*TestBeaconBlock]("cleanup_test")
	require.NoError(t, err)

	encoder := &TestBeaconBlockEncoder{}

	handler := v1.NewHandlerConfig[*TestBeaconBlock](
		v1.WithEncoder[*TestBeaconBlock](encoder),
		v1.WithProcessor[*TestBeaconBlock](func(ctx context.Context, data *TestBeaconBlock, from peer.ID) error {
			messageCount.Add(1)
			t.Logf("Received message: slot=%d", data.Slot)
			return nil
		}),
	)

	// Register handlers
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)
	err = v1.Register(nodes[1].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)

	// Subscribe and receive one message
	sub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)

	// Wait for mesh formation
	time.Sleep(2 * time.Second)

	// Publish first message
	msg1 := &TestBeaconBlock{Slot: 200}
	err = v1.Publish(nodes[1].Gossipsub, topic, msg1)
	require.NoError(t, err)

	// Wait for message to be processed
	require.Eventually(t, func() bool {
		return messageCount.Load() == 1
	}, 5*time.Second, 100*time.Millisecond)

	// Cancel the subscription
	t.Log("Cancelling subscription...")
	sub.Cancel()
	assert.True(t, sub.IsCancelled())

	// Reset counter
	messageCount.Store(0)

	// Publish second message - should not be received
	t.Log("Publishing message after cancellation...")
	msg2 := &TestBeaconBlock{Slot: 201}
	err = v1.Publish(nodes[1].Gossipsub, topic, msg2)
	require.NoError(t, err)

	// Wait a bit and verify no message was processed
	time.Sleep(2 * time.Second)
	assert.Equal(t, int64(0), messageCount.Load(), "No messages should be processed after cancellation")

	t.Log("SUCCESS: Subscription cleanup works correctly")
}

// TestSubscriptionLifecycle_ResubscribeAfterCancel tests re-subscribing to a topic after cancellation
func TestSubscriptionLifecycle_ResubscribeAfterCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	var messageCount atomic.Int64

	topic, err := v1.NewTopic[*TestBeaconBlock]("resubscribe_test")
	require.NoError(t, err)

	encoder := &TestBeaconBlockEncoder{}

	handler := v1.NewHandlerConfig[*TestBeaconBlock](
		v1.WithEncoder[*TestBeaconBlock](encoder),
		v1.WithProcessor[*TestBeaconBlock](func(ctx context.Context, data *TestBeaconBlock, from peer.ID) error {
			messageCount.Add(1)
			t.Logf("Received message: slot=%d", data.Slot)
			return nil
		}),
	)

	// Register handlers
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)
	err = v1.Register(nodes[1].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)

	// First subscription
	t.Log("Creating first subscription...")
	sub1, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)

	time.Sleep(2 * time.Second) // Allow mesh formation

	// Publish and receive first message
	msg1 := &TestBeaconBlock{Slot: 300}
	err = v1.Publish(nodes[1].Gossipsub, topic, msg1)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return messageCount.Load() == 1
	}, 5*time.Second, 100*time.Millisecond)

	// Cancel first subscription
	t.Log("Cancelling first subscription...")
	sub1.Cancel()
	assert.True(t, sub1.IsCancelled())

	// Reset counter
	messageCount.Store(0)

	// Re-subscribe to same topic
	t.Log("Creating second subscription...")
	sub2, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)
	defer sub2.Cancel()

	time.Sleep(2 * time.Second) // Allow mesh reformation

	// Publish second message
	t.Log("Publishing message to new subscription...")
	msg2 := &TestBeaconBlock{Slot: 301}
	err = v1.Publish(nodes[1].Gossipsub, topic, msg2)
	require.NoError(t, err)

	// Should receive the message on new subscription
	require.Eventually(t, func() bool {
		return messageCount.Load() == 1
	}, 5*time.Second, 100*time.Millisecond, "New subscription should receive message")

	t.Log("SUCCESS: Re-subscription after cancel works correctly")
}

// TestSubscriptionLifecycle_SubscriptionState tests that subscription state is managed correctly
func TestSubscriptionLifecycle_SubscriptionState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 1 node for this test
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 1)
	require.NoError(t, err)

	topic, err := v1.NewTopic[*TestBeaconBlock]("state_test")
	require.NoError(t, err)

	encoder := &TestBeaconBlockEncoder{}

	handler := v1.NewHandlerConfig[*TestBeaconBlock](
		v1.WithEncoder[*TestBeaconBlock](encoder),
		v1.WithProcessor[*TestBeaconBlock](func(ctx context.Context, data *TestBeaconBlock, from peer.ID) error {
			return nil
		}),
	)

	// Register handler
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)

	// Test initial state
	sub, err := v1.Subscribe(ctx, nodes[0].Gossipsub, topic)
	require.NoError(t, err)

	assert.Equal(t, "state_test", sub.Topic())
	assert.False(t, sub.IsCancelled())

	// Test state after cancellation
	sub.Cancel()
	assert.True(t, sub.IsCancelled())

	// Test multiple cancellations (should be safe)
	sub.Cancel()
	sub.Cancel()
	assert.True(t, sub.IsCancelled())

	t.Log("SUCCESS: Subscription state management works correctly")
}

// createTestHostPair creates two connected libp2p hosts for testing
func createTestHostPair(t *testing.T, ctx context.Context) (host.Host, host.Host, func()) {
	// Create private keys
	priv1, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	require.NoError(t, err)

	priv2, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	require.NoError(t, err)

	// Create hosts
	host1, err := libp2p.New(
		libp2p.Identity(priv1),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	require.NoError(t, err)

	host2, err := libp2p.New(
		libp2p.Identity(priv2),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	require.NoError(t, err)

	// Connect the hosts
	err = host1.Connect(ctx, host2.Peerstore().PeerInfo(host2.ID()))
	require.NoError(t, err)

	cleanup := func() {
		host1.Close()
		host2.Close()
	}

	return host1, host2, cleanup
}
