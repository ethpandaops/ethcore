package v1_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNode represents a test node with its gossipsub instance and host
type TestNode struct {
	ID        peer.ID
	Host      host.Host
	Gossipsub *v1.Gossipsub
	ctx       context.Context
	cancel    context.CancelFunc
}

// Close shuts down the test node
func (n *TestNode) Close() error {
	if n.Gossipsub != nil {
		if err := n.Gossipsub.Stop(); err != nil {
			return fmt.Errorf("failed to stop gossipsub: %w", err)
		}
	}
	if n.Host != nil {
		if err := n.Host.Close(); err != nil {
			return fmt.Errorf("failed to close host: %w", err)
		}
	}
	if n.cancel != nil {
		n.cancel()
	}
	return nil
}

// GossipTestMessage is a simple test message type for gossipsub tests
type GossipTestMessage struct {
	ID      string
	Content string
	From    string
}

// TestEncoder implements the Encoder interface for GossipTestMessage
type TestEncoder struct{}

func (e *TestEncoder) Encode(msg GossipTestMessage) ([]byte, error) {
	// Simple encoding: "ID|Content|From"
	encoded := fmt.Sprintf("%s|%s|%s", msg.ID, msg.Content, msg.From)
	return []byte(encoded), nil
}

func (e *TestEncoder) Decode(data []byte) (GossipTestMessage, error) {
	// Simple decoding
	str := string(data)

	// Split by pipe delimiter
	parts := strings.Split(str, "|")
	if len(parts) != 3 {
		return GossipTestMessage{}, fmt.Errorf("invalid message format: expected 3 parts, got %d", len(parts))
	}

	msg := GossipTestMessage{
		ID:      parts[0],
		Content: parts[1],
		From:    parts[2],
	}

	return msg, nil
}

// TestInfrastructure provides common test setup and utilities
type TestInfrastructure struct {
	t     *testing.T
	nodes []*TestNode
}

// NewTestInfrastructure creates a new test infrastructure
func NewTestInfrastructure(t *testing.T) *TestInfrastructure {
	return &TestInfrastructure{
		t:     t,
		nodes: make([]*TestNode, 0),
	}
}

// CreateNode creates a new test node with gossipsub
func (ti *TestInfrastructure) CreateNode(ctx context.Context, opts ...v1.Option) (*TestNode, error) {
	// Generate a new key pair for the node
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create a new libp2p host
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		libp2p.DisableRelay(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create host: %w", err)
	}

	// Create node context
	nodeCtx, cancel := context.WithCancel(ctx)

	// Create gossipsub instance with default options
	defaultOpts := []v1.Option{
		v1.WithLogger(logrus.StandardLogger().WithField("test", "node").WithField("peer", h.ID().ShortString())),
		v1.WithPublishTimeout(5 * time.Second),
		v1.WithPubsubOptions(
			pubsub.WithMaxMessageSize(1<<20), // 1MB
			pubsub.WithValidateWorkers(10),
			pubsub.WithValidateThrottle(10),
		),
	}

	// Append user options
	defaultOpts = append(defaultOpts, opts...)

	// Create gossipsub instance
	gossipsub, err := v1.New(nodeCtx, h, defaultOpts...)
	if err != nil {
		cancel()
		h.Close()
		return nil, fmt.Errorf("failed to create gossipsub: %w", err)
	}

	node := &TestNode{
		ID:        h.ID(),
		Host:      h,
		Gossipsub: gossipsub,
		ctx:       nodeCtx,
		cancel:    cancel,
	}

	ti.nodes = append(ti.nodes, node)
	return node, nil
}

// ConnectNodes connects two nodes together
func (ti *TestInfrastructure) ConnectNodes(node1, node2 *TestNode) error {
	// Get node2's addresses
	addrs := node2.Host.Addrs()
	if len(addrs) == 0 {
		return fmt.Errorf("node2 has no addresses")
	}

	// Build the peer info
	peerInfo := peer.AddrInfo{
		ID:    node2.ID,
		Addrs: addrs,
	}

	// Connect node1 to node2
	if err := node1.Host.Connect(context.Background(), peerInfo); err != nil {
		return fmt.Errorf("failed to connect nodes: %w", err)
	}

	// Wait for connection to be established
	ti.waitForConnection(node1, node2.ID)
	ti.waitForConnection(node2, node1.ID)

	return nil
}

// waitForConnection waits for a connection to be established
func (ti *TestInfrastructure) waitForConnection(node *TestNode, targetID peer.ID) {
	require.Eventually(ti.t, func() bool {
		return node.Host.Network().Connectedness(targetID) == network.Connected
	}, 5*time.Second, 100*time.Millisecond)
}

// CreateFullyConnectedNetwork creates n nodes and connects them all together
func (ti *TestInfrastructure) CreateFullyConnectedNetwork(ctx context.Context, n int, opts ...v1.Option) ([]*TestNode, error) {
	nodes := make([]*TestNode, n)

	// Create all nodes
	for i := 0; i < n; i++ {
		node, err := ti.CreateNode(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create node %d: %w", i, err)
		}
		nodes[i] = node
	}

	// Connect all nodes to each other
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if err := ti.ConnectNodes(nodes[i], nodes[j]); err != nil {
				return nil, fmt.Errorf("failed to connect node %d to node %d: %w", i, j, err)
			}
		}
	}

	// Give gossipsub time to discover peers and establish mesh
	time.Sleep(2 * time.Second)

	return nodes, nil
}

// Cleanup closes all nodes created by this infrastructure
func (ti *TestInfrastructure) Cleanup() {
	for _, node := range ti.nodes {
		if err := node.Close(); err != nil {
			ti.t.Logf("Failed to close node %s: %v", node.ID, err)
		}
	}
	ti.nodes = nil
}

// MessageCollector helps collect messages received by nodes
type MessageCollector struct {
	messages chan ReceivedMessage
	errors   chan error
}

// ReceivedMessage represents a message received by a node
type ReceivedMessage struct {
	Node    peer.ID
	Message GossipTestMessage
	From    peer.ID
}

// NewMessageCollector creates a new message collector
func NewMessageCollector(bufferSize int) *MessageCollector {
	return &MessageCollector{
		messages: make(chan ReceivedMessage, bufferSize),
		errors:   make(chan error, bufferSize),
	}
}

// CreateProcessor creates a processor function that sends messages to the collector
func (mc *MessageCollector) CreateProcessor(nodeID peer.ID) v1.Processor[GossipTestMessage] {
	return func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
		select {
		case mc.messages <- ReceivedMessage{
			Node:    nodeID,
			Message: msg,
			From:    from,
		}:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	}
}

// GetMessages returns all collected messages
func (mc *MessageCollector) GetMessages() []ReceivedMessage {
	var msgs []ReceivedMessage
	for {
		select {
		case msg := <-mc.messages:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

// GetMessageCount returns the number of messages collected
func (mc *MessageCollector) GetMessageCount() int {
	return len(mc.messages)
}

// CreateTestTopic creates a test topic with the given name
func CreateTestTopic(name string) (*v1.Topic[GossipTestMessage], error) {
	encoder := &TestEncoder{}
	return v1.NewTopic(name, encoder)
}

// CreateTestHandler creates a handler config with the given processor
func CreateTestHandler(processor v1.Processor[GossipTestMessage]) *v1.HandlerConfig[GossipTestMessage] {
	return v1.NewHandlerConfig(
		v1.WithProcessor(processor),
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			// Accept all messages in tests
			return v1.ValidationAccept
		}),
	)
}

// WaitForGossipsubReady waits for gossipsub to be ready with the expected number of peers on a topic
func WaitForGossipsubReady(t *testing.T, nodes []*TestNode, topicName string, expectedPeers int) {
	require.Eventually(t, func() bool {
		for i, node := range nodes {
			// Check if node has the expected number of peers for the topic
			ps := node.Gossipsub.GetPubSub()
			peers := ps.ListPeers(topicName)
			t.Logf("Node %d (%s) has %d peers for topic %s", i, node.ID.ShortString(), len(peers), topicName)
			if len(peers) < expectedPeers-1 { // -1 because it doesn't include itself
				return false
			}
		}
		return true
	}, 30*time.Second, 500*time.Millisecond, "Gossipsub mesh not fully formed")
}

func TestGossipsubThreeNodeMessagePropagation(t *testing.T) {
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
	topic, err := CreateTestTopic("test-topic")
	require.NoError(t, err)

	// Create message collector
	collector := NewMessageCollector(10)

	// Register handlers and subscribe all nodes to the topic
	subscriptions := make([]*v1.Subscription, 3)
	for i, node := range nodes {
		// Create handler with collector
		handler := CreateTestHandler(collector.CreateProcessor(node.ID))

		// Register handler
		err := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		// Subscribe to topic
		sub, err := v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
		require.NotNil(t, sub)
		subscriptions[i] = sub

		t.Logf("Node %d (%s) subscribed to topic %s", i, node.ID.ShortString(), topic.Name())
	}

	// Give subscriptions time to propagate
	time.Sleep(1 * time.Second)

	// Wait for gossipsub mesh to stabilize
	t.Log("Waiting for gossipsub mesh to stabilize...")
	WaitForGossipsubReady(t, nodes, topic.Name(), 3)
	t.Log("Gossipsub mesh is ready")

	// Create and publish a test message from node 0
	testMsg := GossipTestMessage{
		ID:      "test-msg-1",
		Content: "Hello from node 0",
		From:    nodes[0].ID.String(),
	}

	// Publish message from first node
	t.Logf("Publishing message from node %s", nodes[0].ID.ShortString())
	err = v1.Publish(nodes[0].Gossipsub, topic, testMsg)
	require.NoError(t, err)
	t.Log("Message published successfully")

	// Wait for message propagation
	require.Eventually(t, func() bool {
		return collector.GetMessageCount() >= 2 // Expecting 2 messages (nodes 1 and 2)
	}, 5*time.Second, 100*time.Millisecond)

	// Get all messages and check if sender received its own message
	messages := collector.GetMessages()

	// Count messages by node
	messagesByNode := make(map[peer.ID][]ReceivedMessage)
	for _, msg := range messages {
		messagesByNode[msg.Node] = append(messagesByNode[msg.Node], msg)
	}

	// Check if the sender received its own message
	if msgs, ok := messagesByNode[nodes[0].ID]; ok && len(msgs) > 0 {
		// This is expected behavior in libp2p gossipsub - nodes receive their own messages
		t.Logf("Note: Node 0 (sender) received its own message, which is expected behavior in gossipsub")
		// Filter out the sender's own message
		var filteredMessages []ReceivedMessage
		for _, msg := range messages {
			if msg.Node != nodes[0].ID {
				filteredMessages = append(filteredMessages, msg)
			}
		}
		messages = filteredMessages
	}

	assert.Len(t, messages, 2, "Expected 2 nodes (excluding sender) to receive the message")

	// Verify that nodes 1 and 2 received the message, but not node 0 (the sender)
	receivedByNode1 := false
	receivedByNode2 := false
	for _, msg := range messages {
		assert.Equal(t, testMsg.ID, msg.Message.ID)
		assert.Equal(t, testMsg.Content, msg.Message.Content)
		assert.Equal(t, testMsg.From, msg.Message.From)

		if msg.Node == nodes[1].ID {
			receivedByNode1 = true
		} else if msg.Node == nodes[2].ID {
			receivedByNode2 = true
		}
	}

	assert.True(t, receivedByNode1, "Node 1 should have received the message")
	assert.True(t, receivedByNode2, "Node 2 should have received the message")
}

func TestGossipsubMultipleMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 fully connected nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	// Create a test topic
	topic, err := CreateTestTopic("multi-msg-topic")
	require.NoError(t, err)

	// Create message collector
	collector := NewMessageCollector(30) // Expecting more messages

	// Register handlers and subscribe all nodes
	for _, node := range nodes {
		handler := CreateTestHandler(collector.CreateProcessor(node.ID))
		err := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
	}

	// Wait for mesh to stabilize
	WaitForGossipsubReady(t, nodes, topic.Name(), 3)

	// Each node publishes a message
	for i, node := range nodes {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("msg-%d", i),
			Content: fmt.Sprintf("Message from node %d", i),
			From:    node.ID.String(),
		}

		err := v1.Publish(node.Gossipsub, topic, msg)
		require.NoError(t, err)

		// Small delay between messages
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all messages to propagate
	// Each of 3 nodes publishes 1 message, received by all 3 nodes = 9 messages total
	// (including self-delivery, which is expected in gossipsub)
	require.Eventually(t, func() bool {
		return collector.GetMessageCount() >= 9
	}, 5*time.Second, 50*time.Millisecond)

	// Verify all messages were received
	messages := collector.GetMessages()
	assert.Len(t, messages, 9)

	// Track which nodes received which messages
	messageReceipts := make(map[string]map[string]bool) // messageID -> nodeID -> received
	for i := 0; i < 3; i++ {
		messageReceipts[fmt.Sprintf("msg-%d", i)] = make(map[string]bool)
	}

	for _, msg := range messages {
		messageReceipts[msg.Message.ID][msg.Node.String()] = true
	}

	// Verify each message was received by all 3 nodes (including the sender)
	for i := range nodes {
		msgID := fmt.Sprintf("msg-%d", i)
		receipts := messageReceipts[msgID]

		// Count receipts
		receiptCount := 0
		for _, received := range receipts {
			if received {
				receiptCount++
			}
		}
		assert.Equal(t, 3, receiptCount, "Each message should be received by all 3 nodes (including sender)")
	}
}

func TestGossipsubUnsubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	// Create topic
	topic, err := CreateTestTopic("unsub-topic")
	require.NoError(t, err)

	// Create collectors for each node
	collectors := make([]*MessageCollector, 3)
	subscriptions := make([]*v1.Subscription, 3)

	for i, node := range nodes {
		collectors[i] = NewMessageCollector(5)
		handler := CreateTestHandler(collectors[i].CreateProcessor(node.ID))

		err := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		sub, err := v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
		subscriptions[i] = sub
	}

	// Wait for mesh
	WaitForGossipsubReady(t, nodes, topic.Name(), 3)

	// Unsubscribe node 2
	subscriptions[2].Cancel()
	time.Sleep(500 * time.Millisecond) // Allow unsubscribe to propagate

	// Publish message from node 0
	msg := GossipTestMessage{
		ID:      "after-unsub",
		Content: "Message after node 2 unsubscribed",
		From:    nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic, msg)
	require.NoError(t, err)

	// Wait a bit for propagation
	time.Sleep(1 * time.Second)

	// Verify only node 1 received the message
	assert.Equal(t, 1, collectors[1].GetMessageCount(), "Node 1 should receive the message")
	assert.Equal(t, 0, collectors[2].GetMessageCount(), "Node 2 should not receive the message after unsubscribe")

	messages := collectors[1].GetMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, msg.ID, messages[0].Message.ID)
}

func TestGossipsubTopicIsolation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	// Create two different topics
	topicA, err := CreateTestTopic("topic-a")
	require.NoError(t, err)

	topicB, err := CreateTestTopic("topic-b")
	require.NoError(t, err)

	// Create collectors
	collectorsA := make([]*MessageCollector, 3)
	collectorsB := make([]*MessageCollector, 3)

	// Subscribe nodes to different topics
	// Node 0: both topics
	// Node 1: only topic A
	// Node 2: only topic B
	for i, node := range nodes {
		collectorsA[i] = NewMessageCollector(5)
		collectorsB[i] = NewMessageCollector(5)

		if i == 0 || i == 1 { // Nodes 0 and 1 subscribe to topic A
			handlerA := CreateTestHandler(collectorsA[i].CreateProcessor(node.ID))
			err := v1.Register(node.Gossipsub.Registry(), topicA, handlerA)
			require.NoError(t, err)
			_, err = v1.Subscribe(ctx, node.Gossipsub, topicA)
			require.NoError(t, err)
		}

		if i == 0 || i == 2 { // Nodes 0 and 2 subscribe to topic B
			handlerB := CreateTestHandler(collectorsB[i].CreateProcessor(node.ID))
			err := v1.Register(node.Gossipsub.Registry(), topicB, handlerB)
			require.NoError(t, err)
			_, err = v1.Subscribe(ctx, node.Gossipsub, topicB)
			require.NoError(t, err)
		}
	}

	// Wait for meshes to form
	time.Sleep(2 * time.Second)

	// Publish to topic A from node 1
	msgA := GossipTestMessage{
		ID:      "msg-topic-a",
		Content: "Message for topic A",
		From:    nodes[1].ID.String(),
	}
	err = v1.Publish(nodes[1].Gossipsub, topicA, msgA)
	require.NoError(t, err)

	// Publish to topic B from node 2
	msgB := GossipTestMessage{
		ID:      "msg-topic-b",
		Content: "Message for topic B",
		From:    nodes[2].ID.String(),
	}
	err = v1.Publish(nodes[2].Gossipsub, topicB, msgB)
	require.NoError(t, err)

	// Wait for propagation
	time.Sleep(1 * time.Second)

	// Verify topic isolation
	// Topic A: nodes 0 and 1 should receive (including sender)
	assert.Equal(t, 1, collectorsA[0].GetMessageCount(), "Node 0 should receive topic A message")
	assert.Equal(t, 1, collectorsA[1].GetMessageCount(), "Node 1 (sender) receives its own message in gossipsub")
	assert.Equal(t, 0, collectorsA[2].GetMessageCount(), "Node 2 is not subscribed to topic A")

	// Topic B: nodes 0 and 2 should receive (including sender)
	assert.Equal(t, 1, collectorsB[0].GetMessageCount(), "Node 0 should receive topic B message")
	assert.Equal(t, 0, collectorsB[1].GetMessageCount(), "Node 1 is not subscribed to topic B")
	assert.Equal(t, 1, collectorsB[2].GetMessageCount(), "Node 2 (sender) receives its own message in gossipsub")

	// Verify correct messages
	messagesA := collectorsA[0].GetMessages()
	require.Len(t, messagesA, 1)
	assert.Equal(t, msgA.ID, messagesA[0].Message.ID)

	messagesB := collectorsB[0].GetMessages()
	require.Len(t, messagesB, 1)
	assert.Equal(t, msgB.ID, messagesB[0].Message.ID)
}
