package v1_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNetworkPartition_MessageDeliveryDuringPartition tests message delivery during network partition
func TestNetworkPartition_MessageDeliveryDuringPartition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create test infrastructure with 3 nodes
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 nodes: A, B, C where we'll partition A from B+C
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	nodeA, nodeB, nodeC := nodes[0], nodes[1], nodes[2]

	// Message counters for each node
	var countA, countB, countC atomic.Int64

	// Create topic
	topic, err := CreateTestTopic("partition_test")
	require.NoError(t, err)

	// Create handlers that count messages
	handlerA := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := countA.Add(1)
			t.Logf("Node A received message %d: %s", count, msg.ID)
			return nil
		}),
	)

	handlerB := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := countB.Add(1)
			t.Logf("Node B received message %d: %s", count, msg.ID)
			return nil
		}),
	)

	handlerC := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := countC.Add(1)
			t.Logf("Node C received message %d: %s", count, msg.ID)
			return nil
		}),
	)

	// Register handlers on all nodes
	for i, handler := range []*v1.HandlerConfig[GossipTestMessage]{handlerA, handlerB, handlerC} {
		err = v1.Register(nodes[i].Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	// Subscribe all nodes
	subA, err := v1.Subscribe(ctx, nodeA.Gossipsub, topic)
	require.NoError(t, err)
	defer subA.Cancel()

	subB, err := v1.Subscribe(ctx, nodeB.Gossipsub, topic)
	require.NoError(t, err)
	defer subB.Cancel()

	subC, err := v1.Subscribe(ctx, nodeC.Gossipsub, topic)
	require.NoError(t, err)
	defer subC.Cancel()

	// Wait for mesh formation
	time.Sleep(3 * time.Second)

	// Phase 1: Send message when all connected - should reach all nodes
	t.Log("=== PHASE 1: All nodes connected ===")
	msg1 := GossipTestMessage{
		ID:      "before-partition",
		Content: "Message before partition",
		From:    nodeA.ID.String(),
	}

	err = v1.Publish(nodeA.Gossipsub, topic, msg1)
	require.NoError(t, err)

	// Wait for propagation
	require.Eventually(t, func() bool {
		a, b, c := countA.Load(), countB.Load(), countC.Load()
		t.Logf("Phase 1 counts: A=%d, B=%d, C=%d", a, b, c)
		// nodeA publishes so gets its own message, B and C should receive it
		return a >= 1 && b >= 1 && c >= 1
	}, 10*time.Second, 500*time.Millisecond, "All nodes should receive message before partition")

	// Phase 2: Create partition - disconnect A from B and C
	t.Log("=== PHASE 2: Creating partition (A isolated from B+C) ===")
	err = ti.DisconnectNodes(nodeA, nodeB)
	require.NoError(t, err)
	err = ti.DisconnectNodes(nodeA, nodeC)
	require.NoError(t, err)

	// Wait for partition to take effect
	time.Sleep(3 * time.Second)

	// Reset counters for clearer tracking
	countA.Store(0)
	countB.Store(0)
	countC.Store(0)

	// Send message from B to C (should work)
	t.Log("=== PHASE 2a: B->C message (should work) ===")
	msg2 := GossipTestMessage{
		ID:      "during-partition-bc",
		Content: "Message from B to C during partition",
		From:    nodeB.ID.String(),
	}

	err = v1.Publish(nodeB.Gossipsub, topic, msg2)
	require.NoError(t, err)

	// B and C should receive this, A should not
	require.Eventually(t, func() bool {
		a, b, c := countA.Load(), countB.Load(), countC.Load()
		t.Logf("Phase 2a counts: A=%d, B=%d, C=%d", a, b, c)
		return b >= 1 && c >= 1
	}, 5*time.Second, 500*time.Millisecond, "B and C should receive B->C message")

	// Verify A didn't receive it (wait a bit to be sure)
	time.Sleep(2 * time.Second)
	assert.Equal(t, int64(0), countA.Load(), "Node A should not receive message during partition")

	// Send message from A (should only reach A)
	t.Log("=== PHASE 2b: A message (should only reach A) ===")
	msg3 := GossipTestMessage{
		ID:      "during-partition-a",
		Content: "Message from A during partition",
		From:    nodeA.ID.String(),
	}

	err = v1.Publish(nodeA.Gossipsub, topic, msg3)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)
	a, b, c := countA.Load(), countB.Load(), countC.Load()
	t.Logf("Phase 2b final counts: A=%d, B=%d, C=%d", a, b, c)

	assert.Equal(t, int64(1), a, "Node A should receive its own message")
	assert.Equal(t, int64(1), b, "Node B should only have previous message")
	assert.Equal(t, int64(1), c, "Node C should only have previous message")

	// Phase 3: Heal partition - reconnect A to B and C
	t.Log("=== PHASE 3: Healing partition ===")
	err = ti.ReconnectNodes(nodeA, nodeB)
	require.NoError(t, err)
	err = ti.ReconnectNodes(nodeA, nodeC)
	require.NoError(t, err)

	// Wait for mesh to reform
	time.Sleep(5 * time.Second)

	// Reset counters
	countA.Store(0)
	countB.Store(0)
	countC.Store(0)

	// Send message after healing - should reach all nodes again
	t.Log("=== PHASE 3: Post-healing message ===")
	msg4 := GossipTestMessage{
		ID:      "after-partition",
		Content: "Message after partition healing",
		From:    nodeA.ID.String(),
	}

	err = v1.Publish(nodeA.Gossipsub, topic, msg4)
	require.NoError(t, err)

	// All nodes should receive this
	require.Eventually(t, func() bool {
		a, b, c := countA.Load(), countB.Load(), countC.Load()
		t.Logf("Phase 3 counts: A=%d, B=%d, C=%d", a, b, c)
		return a >= 1 && b >= 1 && c >= 1
	}, 10*time.Second, 500*time.Millisecond, "All nodes should receive message after healing")

	t.Log("SUCCESS: Network partition and recovery test completed")
}

// TestNetworkPartition_SubscriptionSurvival tests that subscriptions survive network partitions
func TestNetworkPartition_SubscriptionSurvival(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	nodeA, nodeB := nodes[0], nodes[1]

	var messageCount atomic.Int64

	// Create topic and handler
	topic, err := CreateTestTopic("survival_test")
	require.NoError(t, err)

	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := messageCount.Add(1)
			t.Logf("Received message %d: %s", count, msg.ID)
			return nil
		}),
	)

	// Register and subscribe
	for _, node := range nodes {
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	subA, err := v1.Subscribe(ctx, nodeA.Gossipsub, topic)
	require.NoError(t, err)
	defer subA.Cancel()

	subB, err := v1.Subscribe(ctx, nodeB.Gossipsub, topic)
	require.NoError(t, err)
	defer subB.Cancel()

	// Wait for initial mesh
	time.Sleep(2 * time.Second)

	// Send initial message
	msg1 := GossipTestMessage{ID: "before-disconnect", Content: "Before disconnect", From: nodeA.ID.String()}
	err = v1.Publish(nodeA.Gossipsub, topic, msg1)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return messageCount.Load() >= 2 // Both nodes should receive
	}, 5*time.Second, 200*time.Millisecond)

	// Verify subscriptions are active
	assert.False(t, subA.IsCancelled(), "Subscription A should be active")
	assert.False(t, subB.IsCancelled(), "Subscription B should be active")

	// Disconnect nodes
	t.Log("Disconnecting nodes...")
	err = ti.DisconnectNodes(nodeA, nodeB)
	require.NoError(t, err)

	// Wait during disconnection
	time.Sleep(3 * time.Second)

	// Verify subscriptions are still active (not cancelled due to disconnect)
	assert.False(t, subA.IsCancelled(), "Subscription A should survive disconnect")
	assert.False(t, subB.IsCancelled(), "Subscription B should survive disconnect")

	// Reconnect
	t.Log("Reconnecting nodes...")
	err = ti.ReconnectNodes(nodeA, nodeB)
	require.NoError(t, err)

	// Wait for mesh reformation
	time.Sleep(3 * time.Second)

	// Reset counter and send message after reconnection
	messageCount.Store(0)
	msg2 := GossipTestMessage{ID: "after-reconnect", Content: "After reconnect", From: nodeB.ID.String()}
	err = v1.Publish(nodeB.Gossipsub, topic, msg2)
	require.NoError(t, err)

	// Should receive messages again
	require.Eventually(t, func() bool {
		count := messageCount.Load()
		t.Logf("Messages after reconnect: %d", count)
		return count >= 2
	}, 10*time.Second, 500*time.Millisecond, "Messages should flow after reconnection")

	// Verify subscriptions are still active
	assert.False(t, subA.IsCancelled(), "Subscription A should remain active")
	assert.False(t, subB.IsCancelled(), "Subscription B should remain active")

	t.Log("SUCCESS: Subscriptions survived network partition")
}

// TestNetworkPartition_RapidPartitionRecovery tests rapid partition/recovery cycles
func TestNetworkPartition_RapidPartitionRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	nodeA, nodeB := nodes[0], nodes[1]

	var messageCount atomic.Int64

	// Create topic and handler
	topic, err := CreateTestTopic("rapid_test")
	require.NoError(t, err)

	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			count := messageCount.Add(1)
			t.Logf("Received message %d: %s", count, msg.ID)
			return nil
		}),
	)

	// Register and subscribe
	for _, node := range nodes {
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)
	}

	subA, err := v1.Subscribe(ctx, nodeA.Gossipsub, topic)
	require.NoError(t, err)
	defer subA.Cancel()

	subB, err := v1.Subscribe(ctx, nodeB.Gossipsub, topic)
	require.NoError(t, err)
	defer subB.Cancel()

	// Wait for initial mesh
	time.Sleep(2 * time.Second)

	// Perform rapid partition/recovery cycles
	for i := 0; i < 3; i++ {
		t.Logf("=== Partition/Recovery Cycle %d ===", i+1)

		// Disconnect
		err = ti.DisconnectNodes(nodeA, nodeB)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// Reconnect
		err = ti.ReconnectNodes(nodeA, nodeB)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Test message delivery after each cycle
		messageCount.Store(0)
		testMsg := GossipTestMessage{
			ID:      fmt.Sprintf("cycle-%d", i+1),
			Content: fmt.Sprintf("Test message after cycle %d", i+1),
			From:    nodeA.ID.String(),
		}

		err = v1.Publish(nodeA.Gossipsub, topic, testMsg)
		require.NoError(t, err)

		// Should receive messages
		require.Eventually(t, func() bool {
			count := messageCount.Load()
			return count >= 2
		}, 8*time.Second, 500*time.Millisecond,
			fmt.Sprintf("Messages should flow after cycle %d", i+1))

		// Verify subscriptions are still active
		assert.False(t, subA.IsCancelled(), "Subscription A should remain active")
		assert.False(t, subB.IsCancelled(), "Subscription B should remain active")
	}

	t.Log("SUCCESS: Rapid partition/recovery cycles completed")
}
