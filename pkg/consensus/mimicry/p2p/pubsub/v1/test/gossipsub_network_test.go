package v1_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DisconnectNodes disconnects two nodes from each other.
func (ti *TestInfrastructure) DisconnectNodes(node1, node2 *TestNode) error {
	// Close the connection from both sides
	if err := node1.Host.Network().ClosePeer(node2.ID); err != nil {
		return fmt.Errorf("failed to disconnect node1 from node2: %w", err)
	}

	// Wait for disconnection to be confirmed
	ti.waitForDisconnection(node1, node2.ID)
	ti.waitForDisconnection(node2, node1.ID)

	return nil
}

// waitForDisconnection waits for a disconnection to be confirmed.
func (ti *TestInfrastructure) waitForDisconnection(node *TestNode, targetID peer.ID) {
	require.Eventually(ti.t, func() bool {
		return node.Host.Network().Connectedness(targetID) == network.NotConnected
	}, 5*time.Second, 100*time.Millisecond)
}

// ReconnectNodes reconnects two previously connected nodes.
func (ti *TestInfrastructure) ReconnectNodes(node1, node2 *TestNode) error {
	return ti.ConnectNodes(node1, node2)
}

// TestGossipsubMessageDeliveryWithDisconnect tests message delivery when nodes disconnect/reconnect.
func TestGossipsubMessageDeliveryWithDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 fully connected nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)
	require.Len(t, nodes, 3)

	// Create a test topic
	topic, err := CreateTestTopic("disconnect-test-topic")
	require.NoError(t, err)

	// Create message collectors for each node
	collectors := make([]*MessageCollector, 3)
	for i := range collectors {
		collectors[i] = NewMessageCollector(20)
	}

	// Register handlers and subscribe all nodes to the topic
	for i, node := range nodes {
		handler := CreateTestHandler(collectors[i].CreateProcessor(node.ID))
		regErr := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, regErr)

		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
	}

	// Wait for gossipsub mesh to stabilize
	WaitForGossipsubReady(t, nodes, topic.Name(), 3)

	// Test 1: Send message while all nodes are connected
	msg1 := GossipTestMessage{
		ID:      "msg-before-disconnect",
		Content: "Message before disconnection",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, msg1)
	require.NoError(t, err)

	// Wait for message propagation
	require.Eventually(t, func() bool {
		return collectors[1].GetMessageCount() >= 1 && collectors[2].GetMessageCount() >= 1
	}, 5*time.Second, 100*time.Millisecond)

	// Verify message delivery using count only (don't drain messages yet)
	assert.Equal(t, 1, collectors[1].GetMessageCount(), "Node 1 should have received 1 message")
	assert.Equal(t, 1, collectors[2].GetMessageCount(), "Node 2 should have received 1 message")

	// Test 2: Disconnect node 2 from the network
	t.Log("Disconnecting node 2 from the network")
	err = ti.DisconnectNodes(nodes[0], nodes[2])
	require.NoError(t, err)
	err = ti.DisconnectNodes(nodes[1], nodes[2])
	require.NoError(t, err)

	// Give gossipsub time to detect the disconnection
	time.Sleep(2 * time.Second)

	// Send message while node 2 is disconnected
	msg2 := GossipTestMessage{
		ID:      "msg-during-disconnect",
		Content: "Message during disconnection",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, msg2)
	require.NoError(t, err)

	// Wait and verify only node 1 receives the message
	time.Sleep(2 * time.Second)

	// Check collector 1 has 2 messages (before and during disconnect)
	assert.Equal(t, 2, collectors[1].GetMessageCount(), "Node 1 should have received 2 messages")

	// Check collector 2 still has only 1 message
	assert.Equal(t, 1, collectors[2].GetMessageCount(), "Node 2 should still have only 1 message")

	// Test 3: Reconnect node 2
	t.Log("Reconnecting node 2 to the network")
	err = ti.ReconnectNodes(nodes[0], nodes[2])
	require.NoError(t, err)
	err = ti.ReconnectNodes(nodes[1], nodes[2])
	require.NoError(t, err)

	// Wait for mesh reformation
	time.Sleep(3 * time.Second)

	// Send message after reconnection
	msg3 := GossipTestMessage{
		ID:      "msg-after-reconnect",
		Content: "Message after reconnection",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, msg3)
	require.NoError(t, err)

	// Wait for message propagation
	require.Eventually(t, func() bool {
		return collectors[2].GetMessageCount() >= 2
	}, 5*time.Second, 100*time.Millisecond)

	// Final verification - get messages for validation but save counts first
	node1Count := collectors[1].GetMessageCount()
	node2Count := collectors[2].GetMessageCount()

	// Verify node 2 received the new message after reconnection
	messages := collectors[2].GetMessages()
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		assert.Equal(t, msg3.ID, lastMsg.Message.ID)
	}

	// Final count verification
	assert.Equal(t, 3, node1Count, "Node 1 should have received all 3 messages")
	assert.Equal(t, 2, node2Count, "Node 2 should have received 2 messages (missed one during disconnect)")
}

// TestGossipsubPeerOfflineMidTransmission tests behavior when peer goes offline mid-transmission.
func TestGossipsubPeerOfflineMidTransmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 4 nodes for better mesh coverage
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 4)
	require.NoError(t, err)

	topic, err := CreateTestTopic("offline-mid-transmission")
	require.NoError(t, err)

	collectors := make([]*MessageCollector, 4)
	for i := range collectors {
		collectors[i] = NewMessageCollector(10)
	}

	// Set up subscriptions with a delayed processor for node 1
	for i, node := range nodes {
		var processor v1.Processor[GossipTestMessage]

		if i == 1 {
			// Add delay to simulate processing during disconnection
			processor = func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				time.Sleep(500 * time.Millisecond)

				return collectors[i].CreateProcessor(node.ID)(ctx, msg, from)
			}
		} else {
			processor = collectors[i].CreateProcessor(node.ID)
		}

		handler := CreateTestHandler(processor)
		regErr := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, regErr)

		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
	}

	WaitForGossipsubReady(t, nodes, topic.Name(), 4)

	// Send first 2 messages while connected
	for i := 0; i < 2; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("msg-%d", i),
			Content: fmt.Sprintf("Message %d", i),
			From:    nodes[0].ID.String(),
		}

		err = v1.Publish(nodes[0].Gossipsub, topic, msg)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)
	}

	// Wait for initial messages to propagate
	require.Eventually(t, func() bool {
		return collectors[1].GetMessageCount() >= 2
	}, 2*time.Second, 100*time.Millisecond, "Node 1 should receive first 2 messages")

	// Disconnect node 1 mid-transmission
	t.Log("Disconnecting node 1 mid-transmission")
	err = ti.DisconnectNodes(nodes[0], nodes[1])
	require.NoError(t, err)
	err = ti.DisconnectNodes(nodes[2], nodes[1])
	require.NoError(t, err)
	err = ti.DisconnectNodes(nodes[3], nodes[1])
	require.NoError(t, err)

	// Send remaining messages while node 1 is disconnected
	for i := 2; i < 5; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("msg-%d", i),
			Content: fmt.Sprintf("Message %d", i),
			From:    nodes[0].ID.String(),
		}

		err = v1.Publish(nodes[0].Gossipsub, topic, msg)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)
	}

	// Wait for remaining messages to propagate to connected nodes
	time.Sleep(1 * time.Second)

	// Verify message delivery
	// Node 1 should have received only the first 2 messages (sent before disconnection)
	node1Messages := collectors[1].GetMessageCount()
	t.Logf("Node 1 received %d messages before disconnection", node1Messages)
	assert.Equal(t, 2, node1Messages, "Node 1 should have received exactly 2 messages (sent before disconnection)")

	// Other nodes should have received all messages
	for i := 2; i < 4; i++ {
		count := collectors[i].GetMessageCount()
		assert.Equal(t, 5, count, "Node %d should have received all 5 messages", i)
	}
}

// TestGossipsubMeshReformationAfterNetworkChanges tests mesh reformation after network topology changes.
func TestGossipsubMeshReformationAfterNetworkChanges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 6 nodes for testing mesh changes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 6)
	require.NoError(t, err)

	topic, err := CreateTestTopic("mesh-reformation")
	require.NoError(t, err)

	collectors := make([]*MessageCollector, 6)
	for i := range collectors {
		collectors[i] = NewMessageCollector(20)
		handler := CreateTestHandler(collectors[i].CreateProcessor(nodes[i].ID))
		regErr := v1.Register(nodes[i].Gossipsub.Registry(), topic, handler)
		require.NoError(t, regErr)

		_, err = v1.Subscribe(ctx, nodes[i].Gossipsub, topic)
		require.NoError(t, err)
	}

	WaitForGossipsubReady(t, nodes, topic.Name(), 6)

	// Test 1: Initial message propagation
	msg1 := GossipTestMessage{
		ID:      "initial-mesh",
		Content: "Testing initial mesh",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, msg1)
	require.NoError(t, err)

	// Verify all nodes receive the message
	require.Eventually(t, func() bool {
		for i := 1; i < 6; i++ {
			if collectors[i].GetMessageCount() < 1 {
				return false
			}
		}

		return true
	}, 5*time.Second, 100*time.Millisecond)

	// Test 2: Create a partition - disconnect nodes 0,1,2 from nodes 3,4,5
	t.Log("Creating network partition")
	for i := 0; i < 3; i++ {
		for j := 3; j < 6; j++ {
			err = ti.DisconnectNodes(nodes[i], nodes[j])
			require.NoError(t, err)
		}
	}

	// Wait for mesh to adapt
	time.Sleep(3 * time.Second)

	// Test message propagation within partitions
	msg2 := GossipTestMessage{
		ID:      "partition-1-msg",
		Content: "Message in partition 1",
		From:    nodes[0].ID.String(),
	}

	msg3 := GossipTestMessage{
		ID:      "partition-2-msg",
		Content: "Message in partition 2",
		From:    nodes[3].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, msg2)
	require.NoError(t, err)

	err = v1.Publish(nodes[3].Gossipsub, topic, msg3)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	// Verify partition 1 nodes only received msg2
	for i := 0; i < 3; i++ {
		messages := collectors[i].GetMessages()
		hasMsg2 := false
		hasMsg3 := false
		for _, msg := range messages {
			if msg.Message.ID == msg2.ID {
				hasMsg2 = true
			}
			if msg.Message.ID == msg3.ID {
				hasMsg3 = true
			}
		}
		assert.True(t, hasMsg2, "Node %d in partition 1 should have msg2", i)
		assert.False(t, hasMsg3, "Node %d in partition 1 should not have msg3", i)
	}

	// Verify partition 2 nodes only received msg3
	for i := 3; i < 6; i++ {
		messages := collectors[i].GetMessages()
		hasMsg2 := false
		hasMsg3 := false
		for _, msg := range messages {
			if msg.Message.ID == msg2.ID {
				hasMsg2 = true
			}
			if msg.Message.ID == msg3.ID {
				hasMsg3 = true
			}
		}
		assert.False(t, hasMsg2, "Node %d in partition 2 should not have msg2", i)
		assert.True(t, hasMsg3, "Node %d in partition 2 should have msg3", i)
	}

	// Test 3: Heal the partition
	t.Log("Healing network partition")
	for i := 0; i < 3; i++ {
		for j := 3; j < 6; j++ {
			err = ti.ReconnectNodes(nodes[i], nodes[j])
			require.NoError(t, err)
		}
	}

	// Wait for mesh reformation
	time.Sleep(5 * time.Second)

	// Test message propagation after healing
	msg4 := GossipTestMessage{
		ID:      "post-heal-msg",
		Content: "Message after partition heal",
		From:    nodes[2].ID.String(),
	}

	err = v1.Publish(nodes[2].Gossipsub, topic, msg4)
	require.NoError(t, err)

	// Verify all nodes receive the post-heal message
	require.Eventually(t, func() bool {
		for i := 0; i < 6; i++ {
			hasMsg4 := false
			messages := collectors[i].GetMessages()
			for _, msg := range messages {
				if msg.Message.ID == msg4.ID {
					hasMsg4 = true

					break
				}
			}
			if !hasMsg4 {
				return false
			}
		}

		return true
	}, 5*time.Second, 100*time.Millisecond)
}

// TestGossipsubIntermittentConnectivity tests message delivery with intermittent connectivity.
func TestGossipsubIntermittentConnectivity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	topic, err := CreateTestTopic("intermittent-connectivity")
	require.NoError(t, err)

	collectors := make([]*MessageCollector, 3)
	for i := range collectors {
		collectors[i] = NewMessageCollector(50)
		handler := CreateTestHandler(collectors[i].CreateProcessor(nodes[i].ID))
		regErr := v1.Register(nodes[i].Gossipsub.Registry(), topic, handler)
		require.NoError(t, regErr)

		_, err = v1.Subscribe(ctx, nodes[i].Gossipsub, topic)
		require.NoError(t, err)
	}

	WaitForGossipsubReady(t, nodes, topic.Name(), 3)

	// Start a goroutine to cause intermittent connectivity for node 2
	stopIntermittent := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		disconnected := false
		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if disconnected {
					// Reconnect
					t.Log("Reconnecting node 2")
					if reconnectErr := ti.ReconnectNodes(nodes[0], nodes[2]); reconnectErr != nil {
						t.Logf("Error reconnecting nodes: %v", reconnectErr)
					}
					if reconnectErr := ti.ReconnectNodes(nodes[1], nodes[2]); reconnectErr != nil {
						t.Logf("Error reconnecting nodes: %v", reconnectErr)
					}
					disconnected = false
				} else {
					// Disconnect
					t.Log("Disconnecting node 2")
					if disconnectErr := ti.DisconnectNodes(nodes[0], nodes[2]); disconnectErr != nil {
						t.Logf("Error disconnecting nodes: %v", disconnectErr)
					}
					if disconnectErr := ti.DisconnectNodes(nodes[1], nodes[2]); disconnectErr != nil {
						t.Logf("Error disconnecting nodes: %v", disconnectErr)
					}
					disconnected = true
				}
			case <-stopIntermittent:
				// Ensure we're connected before returning
				if disconnected {
					if reconnectErr := ti.ReconnectNodes(nodes[0], nodes[2]); reconnectErr != nil {
						t.Logf("Error reconnecting nodes: %v", reconnectErr)
					}
					if reconnectErr := ti.ReconnectNodes(nodes[1], nodes[2]); reconnectErr != nil {
						t.Logf("Error reconnecting nodes: %v", reconnectErr)
					}
				}

				return
			}
		}
	}()

	// Publish messages continuously
	messageCount := 10
	for i := 0; i < messageCount; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("intermittent-msg-%d", i),
			Content: fmt.Sprintf("Message %d with intermittent connectivity", i),
			From:    nodes[0].ID.String(),
		}

		err = v1.Publish(nodes[0].Gossipsub, topic, msg)
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)
	}

	// Stop intermittent connectivity
	close(stopIntermittent)
	wg.Wait()

	// Wait for any pending messages
	time.Sleep(3 * time.Second)

	// Analyze results
	node1Count := collectors[1].GetMessageCount()
	node2Count := collectors[2].GetMessageCount()

	t.Logf("Node 1 received %d/%d messages", node1Count, messageCount)
	t.Logf("Node 2 received %d/%d messages (intermittent)", node2Count, messageCount)

	// Node 1 should have received all messages (always connected to node 0)
	assert.Equal(t, messageCount, node1Count, "Node 1 should receive all messages")

	// Node 2 should have missed some messages due to intermittent connectivity
	assert.Less(t, node2Count, messageCount, "Node 2 should have missed some messages")
	assert.Greater(t, node2Count, 0, "Node 2 should have received at least some messages")

	// Verify message order for received messages
	messages := collectors[2].GetMessages()
	for i := 1; i < len(messages); i++ {
		prevID := messages[i-1].Message.ID
		currID := messages[i].Message.ID

		// Extract message numbers
		var prevNum, currNum int
		if _, err := fmt.Sscanf(prevID, "intermittent-msg-%d", &prevNum); err != nil {
			t.Logf("Error parsing prevID: %v", err)

			continue
		}
		if _, err := fmt.Sscanf(currID, "intermittent-msg-%d", &currNum); err != nil {
			t.Logf("Error parsing currID: %v", err)

			continue
		}

		assert.Less(t, prevNum, currNum, "Messages should be received in order")
	}
}

// TestGossipsubNetworkPartitionRecovery tests gossipsub recovery after network partition.
func TestGossipsubNetworkPartitionRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 8 nodes for a more complex partition scenario
	opts := []v1.Option{
		v1.WithLogger(logrus.StandardLogger().WithField("test", "partition-recovery")),
		v1.WithPubsubOptions(pubsub.WithMaxMessageSize(1 << 20)),
		v1.WithPublishTimeout(5 * time.Second),
	}

	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 8, opts...)
	require.NoError(t, err)

	topic, err := CreateTestTopic("partition-recovery")
	require.NoError(t, err)

	collectors := make([]*MessageCollector, 8)
	for i := range collectors {
		collectors[i] = NewMessageCollector(30)
		handler := CreateTestHandler(collectors[i].CreateProcessor(nodes[i].ID))
		regErr := v1.Register(nodes[i].Gossipsub.Registry(), topic, handler)
		require.NoError(t, regErr)

		_, err = v1.Subscribe(ctx, nodes[i].Gossipsub, topic)
		require.NoError(t, err)
	}

	WaitForGossipsubReady(t, nodes, topic.Name(), 8)

	// Test 1: Verify full connectivity
	initialMsg := GossipTestMessage{
		ID:      "pre-partition",
		Content: "Message before partition",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, initialMsg)
	require.NoError(t, err)

	// All nodes should receive the message
	require.Eventually(t, func() bool {
		for i := 1; i < 8; i++ {
			if collectors[i].GetMessageCount() < 1 {
				return false
			}
		}

		return true
	}, 5*time.Second, 100*time.Millisecond)

	// Test 2: Create two partitions
	// Partition A: nodes 0-3
	// Partition B: nodes 4-7
	t.Log("Creating network partition: A=[0,1,2,3], B=[4,5,6,7]")
	for i := 0; i < 4; i++ {
		for j := 4; j < 8; j++ {
			err = ti.DisconnectNodes(nodes[i], nodes[j])
			require.NoError(t, err)
		}
	}

	// Wait for partitions to stabilize
	time.Sleep(3 * time.Second)

	// Test 3: Send messages in each partition
	var wg sync.WaitGroup
	var messagesMutex sync.Mutex
	messagesSent := make(map[string][]string)
	messagesSent["A"] = []string{}
	messagesSent["B"] = []string{}

	// Partition A messages
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			msg := GossipTestMessage{
				ID:      fmt.Sprintf("partition-A-msg-%d", i),
				Content: fmt.Sprintf("Message %d from partition A", i),
				From:    nodes[i%4].ID.String(),
			}
			if pubErr := v1.Publish(nodes[i%4].Gossipsub, topic, msg); pubErr == nil {
				messagesMutex.Lock()
				messagesSent["A"] = append(messagesSent["A"], msg.ID)
				messagesMutex.Unlock()
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	// Partition B messages
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			msg := GossipTestMessage{
				ID:      fmt.Sprintf("partition-B-msg-%d", i),
				Content: fmt.Sprintf("Message %d from partition B", i),
				From:    nodes[4+i%4].ID.String(),
			}
			if pubErr := v1.Publish(nodes[4+i%4].Gossipsub, topic, msg); pubErr == nil {
				messagesMutex.Lock()
				messagesSent["B"] = append(messagesSent["B"], msg.ID)
				messagesMutex.Unlock()
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	wg.Wait()
	time.Sleep(2 * time.Second)

	// Verify partition isolation
	for i := 0; i < 4; i++ {
		messages := collectors[i].GetMessages()
		for _, msg := range messages {
			if msg.Message.ID != initialMsg.ID {
				assert.Contains(t, msg.Message.ID, "partition-A",
					"Node %d in partition A should only have partition A messages", i)
			}
		}
	}

	for i := 4; i < 8; i++ {
		messages := collectors[i].GetMessages()
		for _, msg := range messages {
			if msg.Message.ID != initialMsg.ID {
				assert.Contains(t, msg.Message.ID, "partition-B",
					"Node %d in partition B should only have partition B messages", i)
			}
		}
	}

	// Test 4: Gradually heal the partition
	t.Log("Beginning gradual partition healing")

	// First, connect one node from each partition (bridge)
	t.Log("Creating bridge: connecting node 3 to node 4")
	err = ti.ReconnectNodes(nodes[3], nodes[4])
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	// Send a message to test bridge propagation
	bridgeMsg := GossipTestMessage{
		ID:      "bridge-test",
		Content: "Testing bridge connection",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, bridgeMsg)
	require.NoError(t, err)

	// The message should eventually reach all nodes through the bridge
	require.Eventually(t, func() bool {
		// Check if at least one node from partition B received the bridge message
		for i := 4; i < 8; i++ {
			messages := collectors[i].GetMessages()
			for _, msg := range messages {
				if msg.Message.ID == bridgeMsg.ID {
					return true
				}
			}
		}

		return false
	}, 10*time.Second, 500*time.Millisecond)

	// Test 5: Fully heal the partition
	t.Log("Fully healing the partition")
	for i := 0; i < 4; i++ {
		for j := 4; j < 8; j++ {
			if i == 3 && j == 4 {
				continue // Already connected
			}
			err = ti.ReconnectNodes(nodes[i], nodes[j])
			require.NoError(t, err)
		}
	}

	// Wait for mesh to fully reform
	time.Sleep(5 * time.Second)

	// Test 6: Send final message to verify full recovery
	recoveryMsg := GossipTestMessage{
		ID:      "post-recovery",
		Content: "Network fully recovered",
		From:    nodes[7].ID.String(),
	}

	err = v1.Publish(nodes[7].Gossipsub, topic, recoveryMsg)
	require.NoError(t, err)

	// All nodes should receive the recovery message
	require.Eventually(t, func() bool {
		for i := 0; i < 8; i++ {
			found := false
			messages := collectors[i].GetMessages()
			for _, msg := range messages {
				if msg.Message.ID == recoveryMsg.ID {
					found = true

					break
				}
			}
			if !found {
				return false
			}
		}

		return true
	}, 5*time.Second, 100*time.Millisecond)

	t.Log("Network partition recovery test completed successfully")
}

// TestGossipsubRapidConnectDisconnect tests behavior with rapid connect/disconnect cycles.
func TestGossipsubRapidConnectDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 4 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 4)
	require.NoError(t, err)

	topic, err := CreateTestTopic("rapid-connect-disconnect")
	require.NoError(t, err)

	collectors := make([]*MessageCollector, 4)
	messagesSent := make([]string, 0)
	var messagesMutex sync.Mutex

	for i := range collectors {
		collectors[i] = NewMessageCollector(100)
		handler := CreateTestHandler(collectors[i].CreateProcessor(nodes[i].ID))
		regErr := v1.Register(nodes[i].Gossipsub.Registry(), topic, handler)
		require.NoError(t, regErr)

		_, err = v1.Subscribe(ctx, nodes[i].Gossipsub, topic)
		require.NoError(t, err)
	}

	WaitForGossipsubReady(t, nodes, topic.Name(), 4)

	// Start rapid connect/disconnect cycles for node 3
	stopCycles := make(chan struct{})
	var cycleWg sync.WaitGroup
	cycleWg.Add(1)

	go func() {
		defer cycleWg.Done()
		cycleCount := 0
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cycleCount++
				if cycleCount%2 == 0 {
					// Disconnect phase
					t.Logf("Cycle %d: Disconnecting node 3", cycleCount)
					for i := 0; i < 3; i++ {
						if disconnectErr := ti.DisconnectNodes(nodes[i], nodes[3]); disconnectErr != nil {
							t.Logf("Error disconnecting nodes: %v", disconnectErr)
						}
					}
				} else {
					// Reconnect phase
					t.Logf("Cycle %d: Reconnecting node 3", cycleCount)
					for i := 0; i < 3; i++ {
						if reconnectErr := ti.ReconnectNodes(nodes[i], nodes[3]); reconnectErr != nil {
							t.Logf("Error reconnecting nodes: %v", reconnectErr)
						}
					}
				}
			case <-stopCycles:
				// Ensure we're connected before exiting
				for i := 0; i < 3; i++ {
					if reconnectErr := ti.ReconnectNodes(nodes[i], nodes[3]); reconnectErr != nil {
						t.Logf("Error reconnecting nodes: %v", reconnectErr)
					}
				}

				return
			}
		}
	}()

	// Publish messages during the chaos
	var publishWg sync.WaitGroup
	publishWg.Add(1)

	go func() {
		defer publishWg.Done()
		for i := 0; i < 20; i++ {
			msg := GossipTestMessage{
				ID:      fmt.Sprintf("rapid-msg-%d", i),
				Content: fmt.Sprintf("Message %d during rapid cycles", i),
				From:    nodes[i%3].ID.String(),
			}

			if pubErr := v1.Publish(nodes[i%3].Gossipsub, topic, msg); pubErr == nil {
				messagesMutex.Lock()
				messagesSent = append(messagesSent, msg.ID)
				messagesMutex.Unlock()
			} else {
				t.Logf("Failed to publish message %d: %v", i, pubErr)
			}

			time.Sleep(150 * time.Millisecond)
		}
	}()

	// Wait for publishing to complete
	publishWg.Wait()

	// Stop the chaos
	close(stopCycles)
	cycleWg.Wait()

	// Wait for network to stabilize
	time.Sleep(3 * time.Second)

	// Analyze message delivery
	messagesMutex.Lock()
	totalSent := len(messagesSent)
	messagesMutex.Unlock()

	t.Logf("Total messages sent: %d", totalSent)

	// Nodes 0, 1, 2 should have received most messages (they stayed connected)
	for i := 0; i < 3; i++ {
		count := collectors[i].GetMessageCount()
		t.Logf("Node %d received %d/%d messages", i, count, totalSent)

		// These nodes should receive at least 80% of messages
		minExpected := int(float64(totalSent) * 0.8)
		assert.GreaterOrEqual(t, count, minExpected,
			"Node %d should have received at least %d messages", i, minExpected)
	}

	// Node 3 should have received fewer messages due to rapid disconnections
	node3Count := collectors[3].GetMessageCount()
	t.Logf("Node 3 (rapid disconnect) received %d/%d messages", node3Count, totalSent)

	assert.Less(t, node3Count, totalSent, "Node 3 should have missed some messages")
	assert.Greater(t, node3Count, 0, "Node 3 should have received at least some messages")

	// Test final connectivity by sending one more message
	finalMsg := GossipTestMessage{
		ID:      "final-stability-check",
		Content: "Testing final network stability",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, finalMsg)
	require.NoError(t, err)

	// All nodes should receive this final message
	require.Eventually(t, func() bool {
		for i := 0; i < 4; i++ {
			found := false
			messages := collectors[i].GetMessages()
			for _, msg := range messages {
				if msg.Message.ID == finalMsg.ID {
					found = true

					break
				}
			}
			if !found {
				return false
			}
		}

		return true
	}, 5*time.Second, 100*time.Millisecond)

	t.Log("Rapid connect/disconnect test completed successfully")
}
