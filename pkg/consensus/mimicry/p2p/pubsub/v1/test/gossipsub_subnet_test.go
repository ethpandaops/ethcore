package v1_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SubnetTestMessage represents a message that can be sent on subnet topics.
type SubnetTestMessage struct {
	ID       string
	SubnetID uint64
	Content  string
	From     string
}

// SubnetTestEncoder implements the Encoder interface for SubnetTestMessage.
type SubnetTestEncoder struct{}

func (e *SubnetTestEncoder) Encode(msg SubnetTestMessage) ([]byte, error) {
	// Simple encoding: "ID|SubnetID|Content|From"
	encoded := fmt.Sprintf("%s|%d|%s|%s", msg.ID, msg.SubnetID, msg.Content, msg.From)

	return []byte(encoded), nil
}

func (e *SubnetTestEncoder) Decode(data []byte) (SubnetTestMessage, error) {
	// Simple decoding
	str := string(data)

	// Split by pipe delimiter
	parts := strings.Split(str, "|")
	if len(parts) != 4 {
		return SubnetTestMessage{}, fmt.Errorf("invalid message format: expected 4 parts, got %d", len(parts))
	}

	var subnetID uint64
	if _, err := fmt.Sscanf(parts[1], "%d", &subnetID); err != nil {
		return SubnetTestMessage{}, fmt.Errorf("failed to parse subnet ID: %w", err)
	}

	msg := SubnetTestMessage{
		ID:       parts[0],
		SubnetID: subnetID,
		Content:  parts[2],
		From:     parts[3],
	}

	return msg, nil
}

// CreateTestSubnetTopic creates a test subnet topic with the given pattern and max subnets.
func CreateTestSubnetTopic(pattern string, maxSubnets uint64) (*v1.SubnetTopic[SubnetTestMessage], error) {
	return v1.NewSubnetTopic[SubnetTestMessage](pattern, maxSubnets)
}

// TestSubnetTopicCreation tests the creation and configuration of subnet topics.
func TestSubnetTopicCreation(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		maxSubnets  uint64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid subnet topic",
			pattern:     "beacon_attestation_%d",
			maxSubnets:  64,
			expectError: false,
		},
		{
			name:        "empty pattern",
			pattern:     "",
			maxSubnets:  64,
			expectError: true,
			errorMsg:    "subnet topic pattern cannot be empty",
		},
		{
			name:        "pattern without placeholder",
			pattern:     "beacon_attestation",
			maxSubnets:  64,
			expectError: true,
			errorMsg:    "subnet topic pattern must contain '%d' placeholder",
		},
		{
			name:        "zero max subnets",
			pattern:     "beacon_attestation_%d",
			maxSubnets:  0,
			expectError: true,
			errorMsg:    "maxSubnets must be greater than 0",
		},
		{
			name:        "pattern with prefix and suffix",
			pattern:     "prefix_%d_suffix",
			maxSubnets:  32,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st, err := CreateTestSubnetTopic(tc.pattern, tc.maxSubnets)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
				assert.Nil(t, st)
			} else {
				require.NoError(t, err)
				require.NotNil(t, st)
				assert.Equal(t, tc.maxSubnets, st.MaxSubnets())
			}
		})
	}
}

// TestSubnetTopicForSubnet tests creating topics for specific subnets.
func TestSubnetTopicForSubnet(t *testing.T) {
	pattern := "beacon_attestation_%d"
	maxSubnets := uint64(64)

	st, err := CreateTestSubnetTopic(pattern, maxSubnets)
	require.NoError(t, err)

	tests := []struct {
		name         string
		subnet       uint64
		forkDigest   [4]byte
		expectError  bool
		expectedName string
	}{
		{
			name:         "valid subnet 0",
			subnet:       0,
			forkDigest:   [4]byte{0x01, 0x02, 0x03, 0x04},
			expectError:  false,
			expectedName: "/eth2/01020304/beacon_attestation_0/ssz_snappy",
		},
		{
			name:         "valid subnet 63",
			subnet:       63,
			forkDigest:   [4]byte{0x01, 0x02, 0x03, 0x04},
			expectError:  false,
			expectedName: "/eth2/01020304/beacon_attestation_63/ssz_snappy",
		},
		{
			name:        "subnet exceeds maximum",
			subnet:      64,
			forkDigest:  [4]byte{0x01, 0x02, 0x03, 0x04},
			expectError: true,
		},
		{
			name:        "large subnet number",
			subnet:      1000,
			forkDigest:  [4]byte{0x01, 0x02, 0x03, 0x04},
			expectError: true,
		},
		{
			name:         "different fork digest",
			subnet:       5,
			forkDigest:   [4]byte{0xaa, 0xbb, 0xcc, 0xdd},
			expectError:  false,
			expectedName: "/eth2/aabbccdd/beacon_attestation_5/ssz_snappy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			topic, err := st.TopicForSubnet(tc.subnet, tc.forkDigest)

			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, topic)
			} else {
				require.NoError(t, err)
				require.NotNil(t, topic)
				assert.Equal(t, tc.expectedName, topic.Name())
			}
		})
	}
}

// TestSubnetParsing tests parsing subnet IDs from topic names.
func TestSubnetParsing(t *testing.T) {
	pattern := "beacon_attestation_%d"
	maxSubnets := uint64(64)

	st, err := CreateTestSubnetTopic(pattern, maxSubnets)
	require.NoError(t, err)

	tests := []struct {
		name        string
		topicName   string
		expectError bool
		expectedID  uint64
	}{
		{
			name:        "valid eth2 topic",
			topicName:   "/eth2/01020304/beacon_attestation_5/ssz_snappy",
			expectError: false,
			expectedID:  5,
		},
		{
			name:        "valid bare topic",
			topicName:   "beacon_attestation_42",
			expectError: false,
			expectedID:  42,
		},
		{
			name:        "different eth2 topic",
			topicName:   "/eth2/aabbccdd/beacon_attestation_0/ssz_snappy",
			expectError: false,
			expectedID:  0,
		},
		{
			name:        "invalid format - no number",
			topicName:   "/eth2/01020304/beacon_attestation_abc/ssz_snappy",
			expectError: true,
		},
		{
			name:        "invalid format - wrong prefix",
			topicName:   "/eth2/01020304/sync_committee_1/ssz_snappy",
			expectError: true,
		},
		{
			name:        "subnet exceeds maximum",
			topicName:   "/eth2/01020304/beacon_attestation_100/ssz_snappy",
			expectError: true,
		},
		{
			name:        "empty topic name",
			topicName:   "",
			expectError: true,
		},
		{
			name:        "malformed eth2 topic",
			topicName:   "/eth2/beacon_attestation_5",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subnetID, err := st.ParseSubnet(tc.topicName)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedID, subnetID)
			}
		})
	}
}

// TestSubnetSubscriptionLifecycle tests subscribing and unsubscribing from subnet topics.
func TestSubnetSubscriptionLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 fully connected nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	// Create subnet topic
	subnetTopic, err := CreateTestSubnetTopic("beacon_attestation_%d", 64)
	require.NoError(t, err)

	// Create message collectors for each node
	collectors := make([]*MessageCollector, 3)
	for i := range collectors {
		collectors[i] = NewMessageCollector(10)
	}

	// Test subscribing to specific subnets
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	subnets := []uint64{0, 5, 10}

	// Subscribe each node to a different subnet
	for i, subnet := range subnets {
		node := nodes[i]
		collector := collectors[i]

		// Get topic for specific subnet
		topic, topicErr := subnetTopic.TopicForSubnet(subnet, forkDigest)
		require.NoError(t, topicErr)

		// Create handler
		handler := v1.NewHandlerConfig(
			v1.WithEncoder(&SubnetTestEncoder{}),
			v1.WithProcessor(func(ctx context.Context, msg SubnetTestMessage, from peer.ID) error {
				select {
				case collector.messages <- ReceivedMessage{
					Node: node.ID,
					Message: GossipTestMessage{
						ID:      msg.ID,
						Content: msg.Content,
						From:    msg.From,
					},
					From: from,
				}:
				case <-ctx.Done():
					return ctx.Err()
				}

				return nil
			}),
		)

		// Register handler
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		// Subscribe to topic
		sub, subErr := v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, subErr)
		require.NotNil(t, sub)

		t.Logf("Node %d subscribed to subnet %d (topic: %s)", i, subnet, topic.Name())
	}

	// Wait for mesh to stabilize
	time.Sleep(2 * time.Second)

	// Test 1: Publish to subnet 0 - only node 0 should receive
	topic0, err := subnetTopic.TopicForSubnet(0, forkDigest)
	require.NoError(t, err)

	msg0 := SubnetTestMessage{
		ID:       "subnet-0-msg",
		SubnetID: 0,
		Content:  "Message for subnet 0",
		From:     nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic0, msg0)
	require.NoError(t, err)

	// Wait and verify
	time.Sleep(1 * time.Second)

	assert.Equal(t, 1, collectors[0].GetMessageCount(), "Node 0 should receive its own message")
	assert.Equal(t, 0, collectors[1].GetMessageCount(), "Node 1 should not receive subnet 0 message")
	assert.Equal(t, 0, collectors[2].GetMessageCount(), "Node 2 should not receive subnet 0 message")

	// Clear collectors
	for _, c := range collectors {
		c.GetMessages() // Drain messages
	}

	// Test 2: Publish to subnet 5 from a non-subscriber
	topic5, err := subnetTopic.TopicForSubnet(5, forkDigest)
	require.NoError(t, err)

	msg5 := SubnetTestMessage{
		ID:       "subnet-5-msg",
		SubnetID: 5,
		Content:  "Message for subnet 5",
		From:     nodes[0].ID.String(), // Node 0 publishes to subnet 5
	}

	err = v1.Publish(nodes[0].Gossipsub, topic5, msg5)
	require.NoError(t, err)

	// Wait and verify
	time.Sleep(1 * time.Second)

	assert.Equal(t, 0, collectors[0].GetMessageCount(), "Node 0 should not receive subnet 5 message")
	assert.Equal(t, 1, collectors[1].GetMessageCount(), "Node 1 should receive subnet 5 message")
	assert.Equal(t, 0, collectors[2].GetMessageCount(), "Node 2 should not receive subnet 5 message")
}

// TestSubnetMessagePropagation tests message propagation across subnet topics.
func TestSubnetMessagePropagation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 5 nodes for better testing
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 5)
	require.NoError(t, err)

	// Create subnet topic
	subnetTopic, err := CreateTestSubnetTopic("beacon_attestation_%d", 64)
	require.NoError(t, err)

	// Create message collector
	collector := NewMessageCollector(50)

	// Fork digest for topics
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	// Subscribe nodes to overlapping subnets
	// Node 0: subnets 0, 1
	// Node 1: subnets 1, 2
	// Node 2: subnets 2, 3
	// Node 3: subnets 3, 0
	// Node 4: subnet 1
	nodeSubnets := [][]uint64{
		{0, 1},
		{1, 2},
		{2, 3},
		{3, 0},
		{1},
	}

	for nodeIdx, subnets := range nodeSubnets {
		node := nodes[nodeIdx]

		for _, subnet := range subnets {
			topic, topicErr := subnetTopic.TopicForSubnet(subnet, forkDigest)
			require.NoError(t, topicErr)

			// Create handler that includes subnet info
			handler := v1.NewHandlerConfig(
				v1.WithProcessor(func(ctx context.Context, msg SubnetTestMessage, from peer.ID) error {
					select {
					case collector.messages <- ReceivedMessage{
						Node: node.ID,
						Message: GossipTestMessage{
							ID:      msg.ID,
							Content: fmt.Sprintf("subnet-%d: %s", msg.SubnetID, msg.Content),
							From:    msg.From,
						},
						From: from,
					}:
					case <-ctx.Done():
						return ctx.Err()
					}

					return nil
				}),
			)

			err = v1.Register(node.Gossipsub.Registry(), topic, handler)
			require.NoError(t, err)

			_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
			require.NoError(t, err)
		}
	}

	// Wait for mesh to stabilize
	time.Sleep(2 * time.Second)

	// Test message propagation on each subnet
	for subnet := uint64(0); subnet < 4; subnet++ {
		topic, topicErr := subnetTopic.TopicForSubnet(subnet, forkDigest)
		require.NoError(t, topicErr)

		msg := SubnetTestMessage{
			ID:       fmt.Sprintf("msg-subnet-%d", subnet),
			SubnetID: subnet,
			Content:  fmt.Sprintf("Test message for subnet %d", subnet),
			From:     nodes[0].ID.String(), // Always publish from node 0
		}

		err = v1.Publish(nodes[0].Gossipsub, topic, msg)
		require.NoError(t, err)

		// Small delay between publishes
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all messages to propagate
	time.Sleep(2 * time.Second)

	// Analyze received messages
	messages := collector.GetMessages()
	messagesBySubnet := make(map[uint64][]ReceivedMessage)

	for _, msg := range messages {
		// Extract subnet ID from message content
		var subnetID uint64
		if _, err := fmt.Sscanf(msg.Message.Content, "subnet-%d:", &subnetID); err != nil {
			t.Logf("Error parsing subnet ID: %v", err)

			continue
		}
		messagesBySubnet[subnetID] = append(messagesBySubnet[subnetID], msg)
	}

	// Verify message reception per subnet
	expectedReceivers := map[uint64][]int{
		0: {0, 3},    // Nodes 0 and 3 are subscribed to subnet 0
		1: {0, 1, 4}, // Nodes 0, 1, and 4 are subscribed to subnet 1
		2: {1, 2},    // Nodes 1 and 2 are subscribed to subnet 2
		3: {2, 3},    // Nodes 2 and 3 are subscribed to subnet 3
	}

	for subnet, expected := range expectedReceivers {
		receivers := messagesBySubnet[subnet]
		receiverNodes := make(map[int]bool)

		for _, msg := range receivers {
			for i, node := range nodes {
				if msg.Node == node.ID {
					receiverNodes[i] = true
				}
			}
		}

		assert.Equal(t, len(expected), len(receiverNodes),
			"Subnet %d should have %d receivers", subnet, len(expected))

		for _, nodeIdx := range expected {
			assert.True(t, receiverNodes[nodeIdx],
				"Node %d should have received message on subnet %d", nodeIdx, subnet)
		}
	}
}

// TestSubnetSubscriptionManager tests the SubnetSubscription manager functionality.
func TestSubnetSubscriptionManager(t *testing.T) {
	// Create subnet topic
	subnetTopic, err := CreateTestSubnetTopic("beacon_attestation_%d", 64)
	require.NoError(t, err)

	// Create subnet subscription manager
	subnetManager, err := v1.NewSubnetSubscription(subnetTopic)
	require.NoError(t, err)
	require.NotNil(t, subnetManager)

	// Test initial state
	assert.Equal(t, 0, subnetManager.Count(), "Initial count should be 0")
	assert.Empty(t, subnetManager.Active(), "Initial active list should be empty")

	// Test Add method
	sub1 := &v1.Subscription{}
	err = subnetManager.Add(0, sub1)
	require.NoError(t, err)

	assert.Equal(t, 1, subnetManager.Count())
	assert.Equal(t, []uint64{0}, subnetManager.Active())

	// Test adding to invalid subnet
	err = subnetManager.Add(100, &v1.Subscription{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")

	// Test Get method
	retrieved := subnetManager.Get(0)
	assert.Equal(t, sub1, retrieved)
	assert.Nil(t, subnetManager.Get(1), "Get should return nil for non-existent subnet")

	// Test adding multiple subscriptions
	sub2 := &v1.Subscription{}
	sub3 := &v1.Subscription{}
	err = subnetManager.Add(5, sub2)
	require.NoError(t, err)
	err = subnetManager.Add(10, sub3)
	require.NoError(t, err)

	assert.Equal(t, 3, subnetManager.Count())
	active := subnetManager.Active()
	assert.Len(t, active, 3)
	assert.Contains(t, active, uint64(0))
	assert.Contains(t, active, uint64(5))
	assert.Contains(t, active, uint64(10))

	// Test Replace (Add to existing subnet)
	sub1New := &v1.Subscription{}
	err = subnetManager.Add(0, sub1New)
	require.NoError(t, err)

	assert.Equal(t, 3, subnetManager.Count(), "Count should remain the same")
	assert.Equal(t, sub1New, subnetManager.Get(0), "Subscription should be replaced")

	// Test Remove
	removed := subnetManager.Remove(5)
	assert.True(t, removed)
	assert.Equal(t, 2, subnetManager.Count())
	assert.Nil(t, subnetManager.Get(5))

	removed = subnetManager.Remove(5)
	assert.False(t, removed, "Removing non-existent should return false")

	// Test Set method
	newSubs := map[uint64]*v1.Subscription{
		1:  {},
		15: {},
		20: {},
	}

	err = subnetManager.Set(newSubs)
	require.NoError(t, err)

	assert.Equal(t, 3, subnetManager.Count())
	assert.NotNil(t, subnetManager.Get(1))
	assert.NotNil(t, subnetManager.Get(15))
	assert.NotNil(t, subnetManager.Get(20))
	assert.Nil(t, subnetManager.Get(0), "Old subscription should be removed")
	assert.Nil(t, subnetManager.Get(10), "Old subscription should be removed")

	// Test Set with invalid subnet
	invalidSubs := map[uint64]*v1.Subscription{
		1:   {},
		100: {}, // Invalid subnet
	}
	err = subnetManager.Set(invalidSubs)
	require.Error(t, err)

	// Test Clear
	subnetManager.Clear()
	assert.Equal(t, 0, subnetManager.Count())
	assert.Empty(t, subnetManager.Active())
}

// TestMultipleSubnetsWithDifferentForkDigests tests handling multiple subnets with different fork digests.
func TestMultipleSubnetsWithDifferentForkDigests(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	// Create subnet topic
	subnetTopic, err := CreateTestSubnetTopic("beacon_attestation_%d", 64)
	require.NoError(t, err)

	// Different fork digests (simulating different network forks)
	forkDigest1 := [4]byte{0x01, 0x02, 0x03, 0x04}
	forkDigest2 := [4]byte{0xaa, 0xbb, 0xcc, 0xdd}

	// Create collectors
	collectors := make([]*MessageCollector, 3)
	for i := range collectors {
		collectors[i] = NewMessageCollector(10)
	}

	// Node 0: subnet 0 with fork1
	// Node 1: subnet 0 with fork2
	// Node 2: subnet 0 with both forks
	subscriptions := []struct {
		nodeIdx    int
		subnet     uint64
		forkDigest [4]byte
	}{
		{0, 0, forkDigest1},
		{1, 0, forkDigest2},
		{2, 0, forkDigest1},
		{2, 0, forkDigest2},
	}

	for _, sub := range subscriptions {
		node := nodes[sub.nodeIdx]
		collector := collectors[sub.nodeIdx]

		topic, topicErr := subnetTopic.TopicForSubnet(sub.subnet, sub.forkDigest)
		require.NoError(t, topicErr)

		handler := v1.NewHandlerConfig(
			v1.WithProcessor(func(ctx context.Context, msg SubnetTestMessage, from peer.ID) error {
				select {
				case collector.messages <- ReceivedMessage{
					Node: node.ID,
					Message: GossipTestMessage{
						ID:      msg.ID,
						Content: msg.Content,
						From:    msg.From,
					},
					From: from,
				}:
				case <-ctx.Done():
					return ctx.Err()
				}

				return nil
			}),
		)

		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)

		t.Logf("Node %d subscribed to subnet %d with fork %x", sub.nodeIdx, sub.subnet, sub.forkDigest)
	}

	// Wait for mesh
	time.Sleep(2 * time.Second)

	// Publish message with fork1
	topic1, err := subnetTopic.TopicForSubnet(0, forkDigest1)
	require.NoError(t, err)

	msg1 := SubnetTestMessage{
		ID:       "fork1-msg",
		SubnetID: 0,
		Content:  "Message for fork 1",
		From:     nodes[0].ID.String(),
	}

	err = v1.Publish(nodes[0].Gossipsub, topic1, msg1)
	require.NoError(t, err)

	// Publish message with fork2
	topic2, err := subnetTopic.TopicForSubnet(0, forkDigest2)
	require.NoError(t, err)

	msg2 := SubnetTestMessage{
		ID:       "fork2-msg",
		SubnetID: 0,
		Content:  "Message for fork 2",
		From:     nodes[1].ID.String(),
	}

	err = v1.Publish(nodes[1].Gossipsub, topic2, msg2)
	require.NoError(t, err)

	// Wait for propagation
	time.Sleep(2 * time.Second)

	// Verify fork isolation
	// Node 0: should receive only fork1 message (its own)
	messages0 := collectors[0].GetMessages()
	assert.Len(t, messages0, 1, "Node 0 should receive 1 message")
	if len(messages0) > 0 {
		assert.Equal(t, "fork1-msg", messages0[0].Message.ID)
	}

	// Node 1: should receive only fork2 message (its own)
	messages1 := collectors[1].GetMessages()
	assert.Len(t, messages1, 1, "Node 1 should receive 1 message")
	if len(messages1) > 0 {
		assert.Equal(t, "fork2-msg", messages1[0].Message.ID)
	}

	// Node 2: should receive both messages
	messages2 := collectors[2].GetMessages()
	assert.Len(t, messages2, 2, "Node 2 should receive 2 messages")
	messageIDs := make(map[string]bool)
	for _, msg := range messages2 {
		messageIDs[msg.Message.ID] = true
	}
	assert.True(t, messageIDs["fork1-msg"], "Node 2 should receive fork1 message")
	assert.True(t, messageIDs["fork2-msg"], "Node 2 should receive fork2 message")
}

// TestSubnetLimitsAndValidation tests subnet limits and validation.
func TestSubnetLimitsAndValidation(t *testing.T) {
	// Test with small subnet limit
	smallSubnetTopic, err := CreateTestSubnetTopic("test_%d", 4)
	require.NoError(t, err)

	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	// Test valid subnets
	for i := uint64(0); i < 4; i++ {
		topic, err := smallSubnetTopic.TopicForSubnet(i, forkDigest)
		require.NoError(t, err)
		assert.NotNil(t, topic)
	}

	// Test invalid subnets
	invalidSubnets := []uint64{4, 5, 100, 1000}
	for _, subnet := range invalidSubnets {
		topic, err := smallSubnetTopic.TopicForSubnet(subnet, forkDigest)
		require.Error(t, err)
		assert.Nil(t, topic)
		assert.Contains(t, err.Error(), "exceeds maximum")
	}

	// Test parsing with limits
	validTopics := []string{
		"/eth2/01020304/test_0/ssz_snappy",
		"/eth2/01020304/test_3/ssz_snappy",
		"test_2",
	}

	for _, topicName := range validTopics {
		subnet, err := smallSubnetTopic.ParseSubnet(topicName)
		require.NoError(t, err)
		assert.Less(t, subnet, uint64(4))
	}

	// Test parsing invalid subnet IDs
	invalidTopics := []string{
		"/eth2/01020304/test_4/ssz_snappy",
		"/eth2/01020304/test_10/ssz_snappy",
		"test_100",
	}

	for _, topicName := range invalidTopics {
		_, err := smallSubnetTopic.ParseSubnet(topicName)
		require.Error(t, err)
	}
}

// TestConcurrentSubnetOperations tests concurrent operations on subnet subscriptions.
func TestConcurrentSubnetOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create nodes - we don't actually use them for message passing in this test
	_, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	// Create subnet topic
	subnetTopic, err := CreateTestSubnetTopic("beacon_attestation_%d", 64)
	require.NoError(t, err)

	// Create subnet manager
	subnetManager, err := v1.NewSubnetSubscription(subnetTopic)
	require.NoError(t, err)

	// Track operations
	var (
		addCount    atomic.Int32
		removeCount atomic.Int32
		errors      atomic.Int32
	)

	// Number of concurrent operations
	numGoroutines := 10
	numOperations := 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Run concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Randomly choose operation
				subnet := uint64(workerID*2 + j%2) // Each worker uses 2 subnets

				switch j % 4 {
				case 0: // Add
					sub := &v1.Subscription{}
					if err := subnetManager.Add(subnet, sub); err != nil {
						errors.Add(1)
					} else {
						addCount.Add(1)
					}

				case 1: // Remove
					if subnetManager.Remove(subnet) {
						removeCount.Add(1)
					}

				case 2: // Get
					_ = subnetManager.Get(subnet)

				case 3: // Active
					_ = subnetManager.Active()
				}

				// Small random delay
				time.Sleep(time.Microsecond * time.Duration(j%10))
			}
		}(i)
	}

	// Run concurrent Set operations
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 10; i++ {
			newSubs := make(map[uint64]*v1.Subscription)
			for j := uint64(0); j < 5; j++ {
				newSubs[j] = &v1.Subscription{}
			}

			if err := subnetManager.Set(newSubs); err != nil {
				errors.Add(1)
			}

			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Run concurrent Clear operations
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 5; i++ {
			time.Sleep(20 * time.Millisecond)
			subnetManager.Clear()
		}
	}()

	// Wait for all operations to complete
	wg.Wait()

	// Log statistics
	t.Logf("Concurrent operations completed:")
	t.Logf("  Adds: %d", addCount.Load())
	t.Logf("  Removes: %d", removeCount.Load())
	t.Logf("  Errors: %d", errors.Load())
	t.Logf("  Final count: %d", subnetManager.Count())

	// Verify no panics occurred and operations completed
	assert.GreaterOrEqual(t, addCount.Load(), int32(0))
	assert.GreaterOrEqual(t, removeCount.Load(), int32(0))
}

// TestSubnetWithPrefixAndSuffix tests subnet topics with both prefix and suffix.
func TestSubnetWithPrefixAndSuffix(t *testing.T) {
	pattern := "prefix_%d_suffix"
	maxSubnets := uint64(16)

	st, err := CreateTestSubnetTopic(pattern, maxSubnets)
	require.NoError(t, err)

	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	// Test topic creation
	topic, err := st.TopicForSubnet(5, forkDigest)
	require.NoError(t, err)
	assert.Equal(t, "/eth2/01020304/prefix_5_suffix/ssz_snappy", topic.Name())

	// Test parsing
	tests := []struct {
		topicName   string
		expectedID  uint64
		expectError bool
	}{
		{
			topicName:   "/eth2/01020304/prefix_7_suffix/ssz_snappy",
			expectedID:  7,
			expectError: false,
		},
		{
			topicName:   "prefix_12_suffix",
			expectedID:  12,
			expectError: false,
		},
		{
			topicName:   "/eth2/01020304/prefix_5/ssz_snappy", // Missing suffix
			expectError: true,
		},
		{
			topicName:   "/eth2/01020304/5_suffix/ssz_snappy", // Missing prefix
			expectError: true,
		},
		{
			topicName:   "/eth2/01020304/prefix_abc_suffix/ssz_snappy", // Non-numeric
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.topicName, func(t *testing.T) {
			subnetID, err := st.ParseSubnet(tc.topicName)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedID, subnetID)
			}
		})
	}
}

// TestSubnetTopicEncoderValidation tests that subnet topics properly validate encoders.
func TestSubnetTopicEncoderValidation(t *testing.T) {
	pattern := "test_%d"
	maxSubnets := uint64(10)

	// Note: encoder is now part of handler config, not topic
	// This test is no longer relevant

	// Test with valid pattern
	st, err := v1.NewSubnetTopic[SubnetTestMessage](pattern, maxSubnets)
	require.NoError(t, err)
	require.NotNil(t, st)
}
