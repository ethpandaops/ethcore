package v1_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiNode_BasicCommunication tests basic message propagation between nodes
func TestMultiNode_BasicCommunication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-node test in short mode")
	}

	ctx := context.Background()
	numNodes := 5
	nodes := createTestNetwork(t, numNodes)
	defer stopTestNetwork(nodes)

	// Connect nodes in a mesh topology
	connectNodesFullMesh(t, ctx, nodes)

	// Create gossipsub instances
	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("multi-basic", encoder)
	require.NoError(t, err)

	gossipsubs := make([]*v1.Gossipsub, numNodes)
	receivedMessages := make([]chan GossipTestMessage, numNodes)

	for i, node := range nodes {
		gs, err := v1.New(ctx, node.host)
		require.NoError(t, err)
		gossipsubs[i] = gs

		ch := make(chan GossipTestMessage, 100)
		receivedMessages[i] = ch
		nodeID := i

		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				select {
				case receivedMessages[nodeID] <- msg:
				default:
				}
				return nil
			}),
		)

		err = registerTopic(gs, topic, handler)
		require.NoError(t, err)

		_, err = subscribeTopic(gs, topic)
		require.NoError(t, err)
	}

	// Wait for mesh to stabilize
	time.Sleep(1 * time.Second)

	// Each node publishes a message
	for i := 0; i < numNodes; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("node-%d-msg", i),
			Content: fmt.Sprintf("Hello from node %d", i),
		}
		err := publishTopic(gossipsubs[i], topic, msg)
		require.NoError(t, err)
	}

	// Verify each node receives messages from all other nodes
	timeout := time.After(5 * time.Second)
	receivedCounts := make([]map[string]bool, numNodes)
	for i := range receivedCounts {
		receivedCounts[i] = make(map[string]bool)
	}

	expectedPerNode := numNodes - 1 // Each node should receive from all others
	done := false

	for !done {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for message propagation")
		default:
			done = true
			for i := 0; i < numNodes; i++ {
				select {
				case msg := <-receivedMessages[i]:
					receivedCounts[i][msg.ID] = true
				default:
				}

				if len(receivedCounts[i]) < expectedPerNode {
					done = false
				}
			}
			if !done {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	// Verify results
	for i, received := range receivedCounts {
		assert.Equal(t, expectedPerNode, len(received),
			"Node %d received %d messages, expected %d", i, len(received), expectedPerNode)

		// Should not receive own message
		ownMsg := fmt.Sprintf("node-%d-msg", i)
		assert.False(t, received[ownMsg], "Node %d received its own message", i)
	}

	// Cleanup
	for _, gs := range gossipsubs {
		err := gs.Stop()
		require.NoError(t, err)
	}
}

// TestMultiNode_SubnetPropagation tests subnet-based message propagation
func TestMultiNode_SubnetPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-node test in short mode")
	}

	ctx := context.Background()
	numNodes := 6
	nodes := createTestNetwork(t, numNodes)
	defer stopTestNetwork(nodes)

	// Connect nodes in a mesh topology
	connectNodesFullMesh(t, ctx, nodes)

	// Create gossipsub instances
	encoder := &GossipTestEncoder{}
	subnetTopic, err := v1.NewSubnetTopic[GossipTestMessage]("subnet_%d", 4, encoder)
	require.NoError(t, err)

	gossipsubs := make([]*v1.Gossipsub, numNodes)
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	// Track which nodes subscribe to which subnets
	nodeSubnets := map[int][]uint64{
		0: {0, 1},    // Node 0 subscribes to subnets 0 and 1
		1: {0, 2},    // Node 1 subscribes to subnets 0 and 2
		2: {1, 3},    // Node 2 subscribes to subnets 1 and 3
		3: {2, 3},    // Node 3 subscribes to subnets 2 and 3
		4: {0, 1, 2}, // Node 4 subscribes to subnets 0, 1, and 2
		5: {1, 2, 3}, // Node 5 subscribes to subnets 1, 2, and 3
	}

	receivedMessages := make([]map[uint64]chan GossipTestMessage, numNodes)

	for i, node := range nodes {
		gs, err := v1.New(ctx, node.host)
		require.NoError(t, err)
		gossipsubs[i] = gs

		// Register subnet handler
		handler := createTestHandler[GossipTestMessage]()
		err = registerSubnetTopic(gs, subnetTopic, handler)
		require.NoError(t, err)

		receivedMessages[i] = make(map[uint64]chan GossipTestMessage)

		// Subscribe to specified subnets
		for _, subnet := range nodeSubnets[i] {
			ch := make(chan GossipTestMessage, 100)
			receivedMessages[i][subnet] = ch

			subnetID := subnet
			nodeID := i

			// Create subnet-specific handler
			subnetHandler := v1.NewHandlerConfig[GossipTestMessage](
				v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
					select {
					case receivedMessages[nodeID][subnetID] <- msg:
					default:
					}
					return nil
				}),
			)

			// Get topic for subnet
			topic, err := subnetTopic.TopicForSubnet(subnet, forkDigest)
			require.NoError(t, err)

			// Register and subscribe
			err = registerTopic(gs, topic, subnetHandler)
			require.NoError(t, err)

			_, err = subscribeTopic(gs, topic)
			require.NoError(t, err)
		}
	}

	// Wait for mesh to stabilize
	time.Sleep(1 * time.Second)

	// Publish messages to each subnet
	publishedMsgs := make(map[uint64]GossipTestMessage)
	for subnet := uint64(0); subnet < 4; subnet++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("subnet-%d-msg", subnet),
			Content: fmt.Sprintf("Message for subnet %d", subnet),
		}
		publishedMsgs[subnet] = msg

		// Find a node subscribed to this subnet to publish
		for nodeID, subnets := range nodeSubnets {
			for _, s := range subnets {
				if s == subnet {
					topic, err := subnetTopic.TopicForSubnet(subnet, forkDigest)
					require.NoError(t, err)

					err = publishTopic(gossipsubs[nodeID], topic, msg)
					require.NoError(t, err)
					goto nextSubnet
				}
			}
		}
	nextSubnet:
	}

	// Verify nodes only receive messages for their subscribed subnets
	time.Sleep(2 * time.Second)

	for nodeID, subnets := range nodeSubnets {
		for _, subnet := range subnets {
			ch := receivedMessages[nodeID][subnet]
			received := false

			select {
			case msg := <-ch:
				assert.Equal(t, publishedMsgs[subnet].ID, msg.ID,
					"Node %d received wrong message on subnet %d", nodeID, subnet)
				received = true
			default:
			}

			assert.True(t, received,
				"Node %d did not receive message on subscribed subnet %d", nodeID, subnet)
		}

		// Verify no messages on unsubscribed subnets
		for subnet := uint64(0); subnet < 4; subnet++ {
			subscribed := false
			for _, s := range subnets {
				if s == subnet {
					subscribed = true
					break
				}
			}

			if !subscribed {
				assert.Nil(t, receivedMessages[nodeID][subnet],
					"Node %d should not have channel for unsubscribed subnet %d", nodeID, subnet)
			}
		}
	}

	// Cleanup
	for _, gs := range gossipsubs {
		err := gs.Stop()
		require.NoError(t, err)
	}
}

// TestMultiNode_DynamicTopology tests nodes joining and leaving
func TestMultiNode_DynamicTopology(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-node test in short mode")
	}

	ctx := context.Background()

	// Start with 3 nodes
	initialNodes := 3
	nodes := createTestNetwork(t, initialNodes)
	defer stopTestNetwork(nodes)

	connectNodesFullMesh(t, ctx, nodes[:initialNodes])

	// Create gossipsub instances and set up message tracking
	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("dynamic-topology", encoder)
	require.NoError(t, err)

	gossipsubs := make([]*v1.Gossipsub, len(nodes))
	messageCount := make([]atomic.Int32, len(nodes))

	setupNode := func(idx int) error {
		gs, err := v1.New(ctx, nodes[idx].host)
		if err != nil {
			return err
		}
		gossipsubs[idx] = gs

		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				messageCount[idx].Add(1)
				return nil
			}),
		)

		if err := registerTopic(gs, topic, handler); err != nil {
			return err
		}

		_, err = subscribeTopic(gs, topic)
		return err
	}

	// Setup initial nodes
	for i := 0; i < initialNodes; i++ {
		err := setupNode(i)
		require.NoError(t, err)
	}

	// Wait for initial mesh
	time.Sleep(500 * time.Millisecond)

	// Publish initial messages
	for i := 0; i < initialNodes; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("initial-%d", i),
			Content: "Initial message",
		}
		err := publishTopic(gossipsubs[i], topic, msg)
		require.NoError(t, err)
	}

	// Wait and verify initial propagation
	time.Sleep(500 * time.Millisecond)
	for i := 0; i < initialNodes; i++ {
		count := messageCount[i].Load()
		assert.Equal(t, int32(initialNodes-1), count,
			"Node %d received %d messages, expected %d", i, count, initialNodes-1)
	}

	// Add new nodes one by one
	for i := initialNodes; i < len(nodes); i++ {
		// Connect new node to existing nodes
		for j := 0; j < i; j++ {
			err := nodes[i].host.Connect(ctx, peer.AddrInfo{
				ID:    nodes[j].host.ID(),
				Addrs: nodes[j].host.Addrs(),
			})
			require.NoError(t, err)
		}

		// Setup new node
		err := setupNode(i)
		require.NoError(t, err)

		// Wait for mesh update
		time.Sleep(500 * time.Millisecond)

		// New node publishes
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("new-node-%d", i),
			Content: fmt.Sprintf("Message from new node %d", i),
		}
		err = publishTopic(gossipsubs[i], topic, msg)
		require.NoError(t, err)

		// Verify all nodes receive the message
		time.Sleep(500 * time.Millisecond)

		// Reset counters
		for j := 0; j <= i; j++ {
			messageCount[j].Store(0)
		}

		// Publish test message
		testMsg := GossipTestMessage{
			ID:      fmt.Sprintf("test-after-%d", i),
			Content: "Test connectivity",
		}
		err = publishTopic(gossipsubs[0], topic, testMsg)
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		// All nodes except publisher should receive
		for j := 0; j <= i; j++ {
			expected := int32(0)
			if j != 0 {
				expected = 1
			}
			count := messageCount[j].Load()
			assert.Equal(t, expected, count,
				"After adding node %d, node %d received %d messages, expected %d",
				i, j, count, expected)
		}
	}

	// Test node departure
	departingNode := 1
	err = gossipsubs[departingNode].Stop()
	require.NoError(t, err)
	err = nodes[departingNode].host.Close()
	require.NoError(t, err)

	// Wait for topology to adjust
	time.Sleep(1 * time.Second)

	// Reset counters
	for i := range messageCount {
		messageCount[i].Store(0)
	}

	// Remaining nodes should still communicate
	msg := GossipTestMessage{
		ID:      "after-departure",
		Content: "Message after node departure",
	}
	err = publishTopic(gossipsubs[0], topic, msg)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Verify remaining nodes received the message
	for i := 0; i < len(nodes); i++ {
		if i == departingNode || i == 0 { // Departed node or publisher
			continue
		}
		count := messageCount[i].Load()
		assert.Equal(t, int32(1), count,
			"Node %d received %d messages after node %d departed", i, count, departingNode)
	}

	// Cleanup remaining nodes
	for i, gs := range gossipsubs {
		if i != departingNode && gs != nil {
			err := gs.Stop()
			require.NoError(t, err)
		}
	}
}

// TestMultiNode_NetworkPartition tests behavior during network partitions
func TestMultiNode_NetworkPartition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-node test in short mode")
	}

	ctx := context.Background()
	numNodes := 6
	nodes := createTestNetwork(t, numNodes)
	defer stopTestNetwork(nodes)

	// Initially connect all nodes
	connectNodesFullMesh(t, ctx, nodes)

	// Create gossipsub instances
	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("partition-test", encoder)
	require.NoError(t, err)

	gossipsubs := make([]*v1.Gossipsub, numNodes)
	receivedMessages := make([]chan GossipTestMessage, numNodes)

	for i, node := range nodes {
		gs, err := v1.New(ctx, node.host)
		require.NoError(t, err)
		gossipsubs[i] = gs

		ch := make(chan GossipTestMessage, 100)
		receivedMessages[i] = ch
		nodeID := i

		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				select {
				case receivedMessages[nodeID] <- msg:
				default:
				}
				return nil
			}),
		)

		err = registerTopic(gs, topic, handler)
		require.NoError(t, err)

		_, err = subscribeTopic(gs, topic)
		require.NoError(t, err)
	}

	// Wait for mesh to form
	time.Sleep(1 * time.Second)

	// Test pre-partition communication
	msg1 := GossipTestMessage{
		ID:      "pre-partition",
		Content: "Before partition",
	}
	err = publishTopic(gossipsubs[0], topic, msg1)
	require.NoError(t, err)

	// Verify all nodes receive
	received := waitForMessages(t, receivedMessages, numNodes-1, 2*time.Second)
	assert.Equal(t, numNodes-1, received, "Not all nodes received pre-partition message")

	// Create network partition: nodes 0,1,2 in one partition, 3,4,5 in another
	partition1 := []int{0, 1, 2}
	partition2 := []int{3, 4, 5}

	// Disconnect partitions from each other
	for _, n1 := range partition1 {
		for _, n2 := range partition2 {
			nodes[n1].host.Network().ClosePeer(nodes[n2].host.ID())
			nodes[n2].host.Network().ClosePeer(nodes[n1].host.ID())
		}
	}

	// Wait for partition to take effect
	time.Sleep(1 * time.Second)

	// Clear message channels
	for i := range receivedMessages {
		drainChannel(receivedMessages[i])
	}

	// Test communication within partition 1
	msg2 := GossipTestMessage{
		ID:      "partition1-msg",
		Content: "Message in partition 1",
	}
	err = publishTopic(gossipsubs[0], topic, msg2)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Only nodes in partition 1 (except publisher) should receive
	for i := 0; i < numNodes; i++ {
		select {
		case msg := <-receivedMessages[i]:
			if i == 0 || !contains(partition1, i) {
				t.Errorf("Node %d should not have received message during partition", i)
			}
			assert.Equal(t, msg2.ID, msg.ID)
		default:
			if i != 0 && contains(partition1, i) {
				t.Errorf("Node %d in partition 1 did not receive message", i)
			}
		}
	}

	// Clear channels again
	for i := range receivedMessages {
		drainChannel(receivedMessages[i])
	}

	// Test communication within partition 2
	msg3 := GossipTestMessage{
		ID:      "partition2-msg",
		Content: "Message in partition 2",
	}
	err = publishTopic(gossipsubs[3], topic, msg3)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Only nodes in partition 2 (except publisher) should receive
	for i := 0; i < numNodes; i++ {
		select {
		case msg := <-receivedMessages[i]:
			if i == 3 || !contains(partition2, i) {
				t.Errorf("Node %d should not have received message during partition", i)
			}
			assert.Equal(t, msg3.ID, msg.ID)
		default:
			if i != 3 && contains(partition2, i) {
				t.Errorf("Node %d in partition 2 did not receive message", i)
			}
		}
	}

	// Heal partition
	for _, n1 := range partition1 {
		for _, n2 := range partition2 {
			err := nodes[n1].host.Connect(ctx, peer.AddrInfo{
				ID:    nodes[n2].host.ID(),
				Addrs: nodes[n2].host.Addrs(),
			})
			require.NoError(t, err)
		}
	}

	// Wait for mesh to reform
	time.Sleep(2 * time.Second)

	// Clear channels
	for i := range receivedMessages {
		drainChannel(receivedMessages[i])
	}

	// Test post-healing communication
	msg4 := GossipTestMessage{
		ID:      "post-healing",
		Content: "After partition healed",
	}
	err = publishTopic(gossipsubs[0], topic, msg4)
	require.NoError(t, err)

	// All nodes should receive again
	received = waitForMessages(t, receivedMessages, numNodes-1, 2*time.Second)
	assert.Equal(t, numNodes-1, received, "Not all nodes received post-healing message")

	// Cleanup
	for _, gs := range gossipsubs {
		err := gs.Stop()
		require.NoError(t, err)
	}
}

// Helper types and functions

type testNode struct {
	host   host.Host
	cancel context.CancelFunc
}

func createTestNetwork(t *testing.T, numNodes int) []*testNode {
	nodes := make([]*testNode, numNodes)

	for i := 0; i < numNodes; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		h, err := libp2p.New(
			libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 9000+i)),
		)
		require.NoError(t, err)

		nodes[i] = &testNode{
			host:   h,
			cancel: cancel,
		}

		// Start a goroutine to handle the context
		go func(ctx context.Context, h host.Host) {
			<-ctx.Done()
		}(ctx, h)
	}

	return nodes
}

func stopTestNetwork(nodes []*testNode) {
	var wg sync.WaitGroup
	for _, node := range nodes {
		if node != nil {
			wg.Add(1)
			go func(n *testNode) {
				defer wg.Done()
				n.cancel()
				n.host.Close()
			}(node)
		}
	}
	wg.Wait()
}

func connectNodesFullMesh(t *testing.T, ctx context.Context, nodes []*testNode) {
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			err := nodes[i].host.Connect(ctx, peer.AddrInfo{
				ID:    nodes[j].host.ID(),
				Addrs: nodes[j].host.Addrs(),
			})
			require.NoError(t, err)
		}
	}
}

func waitForMessages(t *testing.T, channels []chan GossipTestMessage, expected int, timeout time.Duration) int {
	deadline := time.After(timeout)
	received := 0

	for received < expected {
		select {
		case <-deadline:
			return received
		default:
			for i := range channels {
				select {
				case <-channels[i]:
					received++
				default:
				}
			}
			if received < expected {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	return received
}

func drainChannel(ch chan GossipTestMessage) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func contains(slice []int, item int) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// mockConnManager is a simple connection manager for testing
type mockConnManager struct{}

func (m *mockConnManager) TagPeer(peer.ID, string, int)             {}
func (m *mockConnManager) UntagPeer(peer.ID, string)                {}
func (m *mockConnManager) UpsertTag(peer.ID, string, func(int) int) {}
func (m *mockConnManager) GetTagInfo(peer.ID) any                   { return nil }
func (m *mockConnManager) TrimOpenConns(context.Context)            {}
func (m *mockConnManager) Notifee() network.Notifiee                { return nil }
func (m *mockConnManager) Protect(peer.ID, string)                  {}
func (m *mockConnManager) Unprotect(peer.ID, string) bool           { return false }
func (m *mockConnManager) IsProtected(peer.ID, string) bool         { return false }
func (m *mockConnManager) Close() error                             { return nil }
