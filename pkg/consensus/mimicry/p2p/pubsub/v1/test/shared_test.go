package v1_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// GossipTestMessage is a simple test message type for gossipsub tests.
type GossipTestMessage struct {
	ID      string
	Content string
	From    string
}

// SSZTestMessage is a test message that implements SSZ serialization.
type SSZTestMessage struct {
	ID    uint64
	Data  []byte
	Value uint32
}

// MarshalSSZ implements SSZ marshalling.
func (m *SSZTestMessage) MarshalSSZ() ([]byte, error) {
	// Simple encoding: [ID:8][Value:4][ContentLen:4][Content:N]
	buf := make([]byte, 8+4+4+len(m.Data))

	// ID (8 bytes)
	for i := 0; i < 8; i++ {
		buf[i] = byte(m.ID >> (i * 8))
	}

	// Value (4 bytes)
	for i := 0; i < 4; i++ {
		buf[8+i] = byte(m.Value >> (i * 8))
	}

	// Content length (4 bytes)
	contentLen := uint32(len(m.Data))
	for i := 0; i < 4; i++ {
		buf[12+i] = byte(contentLen >> (i * 8))
	}

	// Content
	copy(buf[16:], m.Data)

	return buf, nil
}

// UnmarshalSSZ implements SSZ unmarshalling.
func (m *SSZTestMessage) UnmarshalSSZ(buf []byte) error {
	if len(buf) < 16 {
		return fmt.Errorf("buffer too short: need at least 16 bytes, got %d", len(buf))
	}

	// ID (8 bytes)
	m.ID = 0
	for i := 0; i < 8; i++ {
		m.ID |= uint64(buf[i]) << (i * 8)
	}

	// Value (4 bytes)
	m.Value = 0
	for i := 0; i < 4; i++ {
		m.Value |= uint32(buf[8+i]) << (i * 8)
	}

	// Content length (4 bytes)
	var contentLen uint32
	for i := 0; i < 4; i++ {
		contentLen |= uint32(buf[12+i]) << (i * 8)
	}

	if len(buf) < 16+int(contentLen) {
		return fmt.Errorf("buffer too short for content: need %d bytes, got %d", 16+contentLen, len(buf))
	}

	// Content
	m.Data = make([]byte, contentLen)
	copy(m.Data, buf[16:16+contentLen])

	return nil
}

// SizeSSZ returns the size of the SSZ encoded message.
func (m *SSZTestMessage) SizeSSZ() int {
	return 16 + len(m.Data)
}

// MarshalSSZTo implements SSZ marshalling to a destination buffer.
func (m *SSZTestMessage) MarshalSSZTo(dst []byte) ([]byte, error) {
	size := m.SizeSSZ()
	if len(dst) < size {
		dst = append(dst, make([]byte, size-len(dst))...)
	}

	// ID (8 bytes)
	for i := 0; i < 8; i++ {
		dst[i] = byte(m.ID >> (i * 8))
	}

	// Value (4 bytes)
	for i := 0; i < 4; i++ {
		dst[8+i] = byte(m.Value >> (i * 8))
	}

	// Content length (4 bytes)
	contentLen := uint32(len(m.Data))
	for i := 0; i < 4; i++ {
		dst[12+i] = byte(contentLen >> (i * 8))
	}

	// Content
	copy(dst[16:16+len(m.Data)], m.Data)

	return dst[:size], nil
}

// TestEncoder implements the Encoder interface for GossipTestMessage.
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

// TestNode represents a test gossipsub node.
type TestNode struct {
	ID        peer.ID
	Host      host.Host
	Gossipsub *v1.Gossipsub
	cancel    context.CancelFunc
}

// Close shuts down the test node.
func (n *TestNode) Close() error {
	if n.Gossipsub != nil {
		if err := n.Gossipsub.Stop(); err != nil {
			return fmt.Errorf("failed to stop gossipsub: %w", err)
		}
	}

	if n.cancel != nil {
		n.cancel()
	}

	if n.Host != nil {
		return n.Host.Close()
	}

	return nil
}

// TestInfrastructure manages test nodes and cleanup.
type TestInfrastructure struct {
	t     *testing.T
	nodes []*TestNode
}

// NewTestInfrastructure creates a new test infrastructure.
func NewTestInfrastructure(t *testing.T) *TestInfrastructure {
	t.Helper()

	return &TestInfrastructure{
		t:     t,
		nodes: make([]*TestNode, 0),
	}
}

// CreateNode creates a single test node.
func (ti *TestInfrastructure) CreateNode(ctx context.Context, opts ...v1.Option) (*TestNode, error) {
	// Generate a private key
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

	nodeCtx, cancel := context.WithCancel(ctx)

	// Create gossipsub instance
	gossipsub, err := v1.New(nodeCtx, h, opts...)
	if err != nil {
		cancel()
		h.Close()

		return nil, fmt.Errorf("failed to create gossipsub: %w", err)
	}

	node := &TestNode{
		ID:        h.ID(),
		Host:      h,
		Gossipsub: gossipsub,
		cancel:    cancel,
	}

	ti.nodes = append(ti.nodes, node)

	return node, nil
}

// CreateFullyConnectedNetwork creates a fully connected network of test nodes.
func (ti *TestInfrastructure) CreateFullyConnectedNetwork(ctx context.Context, count int, opts ...v1.Option) ([]*TestNode, error) {
	nodes := make([]*TestNode, count)

	// Create all nodes
	for i := 0; i < count; i++ {
		node, err := ti.CreateNode(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create node %d: %w", i, err)
		}
		nodes[i] = node
	}

	// Connect all nodes to each other
	for i := 0; i < count; i++ {
		for j := i + 1; j < count; j++ {
			if err := ti.connectNodes(ctx, nodes[i], nodes[j]); err != nil {
				return nil, fmt.Errorf("failed to connect nodes %d and %d: %w", i, j, err)
			}
		}
	}

	// Wait for connections to stabilize
	time.Sleep(100 * time.Millisecond)

	return nodes, nil
}

// connectNodes connects two test nodes.
func (ti *TestInfrastructure) connectNodes(ctx context.Context, node1, node2 *TestNode) error {
	// Get node2's address info
	addrs := node2.Host.Addrs()
	if len(addrs) == 0 {
		return fmt.Errorf("node2 has no addresses")
	}

	addrInfo := peer.AddrInfo{
		ID:    node2.ID,
		Addrs: addrs,
	}

	// Connect node1 to node2
	return node1.Host.Connect(ctx, addrInfo)
}

// Cleanup shuts down all test nodes.
func (ti *TestInfrastructure) Cleanup() {
	for _, node := range ti.nodes {
		if err := node.Close(); err != nil {
			ti.t.Logf("Failed to close node %s: %v", node.ID, err)
		}
	}
	ti.nodes = nil
}

// ReceivedMessage represents a message received by a test node.
type ReceivedMessage struct {
	Message GossipTestMessage
	From    peer.ID
	NodeID  peer.ID
	Node    peer.ID // Alias for NodeID for backwards compatibility
}

// MessageCollector collects messages for test verification.
type MessageCollector struct {
	messages chan ReceivedMessage
	errors   chan error
}

// NewMessageCollector creates a new message collector.
func NewMessageCollector(bufferSize int) *MessageCollector {
	return &MessageCollector{
		messages: make(chan ReceivedMessage, bufferSize),
		errors:   make(chan error, bufferSize),
	}
}

// CreateProcessor creates a processor function that sends messages to the collector.
func (mc *MessageCollector) CreateProcessor(nodeID peer.ID) v1.Processor[GossipTestMessage] {
	return func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
		select {
		case mc.messages <- ReceivedMessage{
			Message: msg,
			From:    from,
			NodeID:  nodeID,
			Node:    nodeID,
		}:
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Buffer full, drop message
		}

		return nil
	}
}

// WaitForMessages waits for a specific number of messages.
func (mc *MessageCollector) WaitForMessages(count int, timeout time.Duration) ([]ReceivedMessage, error) {
	var messages []ReceivedMessage
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for len(messages) < count {
		select {
		case msg := <-mc.messages:
			messages = append(messages, msg)
		case err := <-mc.errors:
			return messages, err
		case <-timer.C:
			return messages, fmt.Errorf("timeout waiting for messages: got %d, expected %d", len(messages), count)
		}
	}

	return messages, nil
}

// CreateTestTopic creates a test topic with the given name.
func CreateTestTopic(name string) (*v1.Topic[GossipTestMessage], error) {
	return v1.NewTopic[GossipTestMessage](name)
}

// CreateTestHandler creates a handler config with the given processor.
func CreateTestHandler(processor v1.Processor[GossipTestMessage]) *v1.HandlerConfig[GossipTestMessage] {
	return v1.NewHandlerConfig(
		v1.WithEncoder(&TestEncoder{}),
		v1.WithProcessor(processor),
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			// Accept all messages in tests
			return v1.ValidationAccept
		}),
	)
}

// GetMessageCount returns the number of messages currently in the buffer.
func (mc *MessageCollector) GetMessageCount() int {
	return len(mc.messages)
}

// GetMessages returns all collected messages.
func (mc *MessageCollector) GetMessages() []ReceivedMessage {
	var messages []ReceivedMessage
	for {
		select {
		case msg := <-mc.messages:
			messages = append(messages, msg)
		default:
			return messages
		}
	}
}

// ConnectNodes connects two nodes directly (backwards compatibility).
func (ti *TestInfrastructure) ConnectNodes(node1, node2 *TestNode) error {
	ctx := context.Background()

	return ti.connectNodes(ctx, node1, node2)
}

// WaitForGossipsubReady waits for gossipsub mesh to be ready.
func WaitForGossipsubReady(t *testing.T, nodes []*TestNode, topic string, expectedConnections int) {
	t.Helper()
	// Wait for mesh to stabilize
	time.Sleep(2 * time.Second)
}

// MalformedEncoder is an encoder that can produce malformed messages.
type MalformedEncoder struct {
	// Controls whether encoding produces valid data
	EncodeValid bool
	// Controls whether decoding succeeds
	DecodeSuccess bool
	// Custom decode error to return
	DecodeError error
	// If true, produces data that will trigger decoding errors
	ProduceMalformed bool
}

func (e *MalformedEncoder) Encode(msg GossipTestMessage) ([]byte, error) {
	if !e.EncodeValid {
		return nil, fmt.Errorf("encoding failed")
	}

	if e.ProduceMalformed {
		// Return malformed data that will fail decoding
		return []byte("malformed|data"), nil
	}

	// Normal encoding
	encoded := fmt.Sprintf("%s|%s|%s", msg.ID, msg.Content, msg.From)

	return []byte(encoded), nil
}

func (e *MalformedEncoder) Decode(data []byte) (GossipTestMessage, error) {
	if !e.DecodeSuccess {
		if e.DecodeError != nil {
			return GossipTestMessage{}, e.DecodeError
		}

		return GossipTestMessage{}, fmt.Errorf("decoding failed: malformed data")
	}

	// Use the standard test encoder logic
	encoder := &TestEncoder{}

	return encoder.Decode(data)
}

// InvalidPayloadCollector collects invalid payload handler calls.
type InvalidPayloadCollector struct {
	mu       sync.Mutex
	payloads []InvalidPayload
}

type InvalidPayload struct {
	Data  []byte
	Error error
	From  peer.ID
	Topic string
}

func NewInvalidPayloadCollector() *InvalidPayloadCollector {
	return &InvalidPayloadCollector{
		payloads: make([]InvalidPayload, 0),
	}
}

func (c *InvalidPayloadCollector) Collect(ctx context.Context, data []byte, err error, from peer.ID, topic string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payloads = append(c.payloads, InvalidPayload{
		Data:  data,
		Error: err,
		From:  from,
		Topic: topic,
	})
}

func (c *InvalidPayloadCollector) CollectWithoutTopic(ctx context.Context, data []byte, err error, from peer.ID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payloads = append(c.payloads, InvalidPayload{
		Data:  data,
		Error: err,
		From:  from,
		Topic: "", // Topic not provided in this handler type
	})
}

func (c *InvalidPayloadCollector) GetPayloads() []InvalidPayload {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]InvalidPayload, len(c.payloads))
	copy(result, c.payloads)

	return result
}

func (c *InvalidPayloadCollector) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.payloads)
}

func (c *InvalidPayloadCollector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payloads = make([]InvalidPayload, 0)
}
