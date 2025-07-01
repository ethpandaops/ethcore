package v1_test

import (
	"testing"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEncoder is a test encoder implementation.
type topicMockEncoder[T any] struct {
	encodeFunc func(T) ([]byte, error)
	decodeFunc func([]byte) (T, error)
}

func (m *topicMockEncoder[T]) Encode(msg T) ([]byte, error) {
	if m.encodeFunc != nil {
		return m.encodeFunc(msg)
	}
	return []byte("encoded"), nil
}

func (m *topicMockEncoder[T]) Decode(data []byte) (T, error) {
	var zero T
	if m.decodeFunc != nil {
		return m.decodeFunc(data)
	}
	return zero, nil
}

func TestNewTopic(t *testing.T) {
	encoder := &topicMockEncoder[string]{}

	tests := []struct {
		name        string
		topicName   string
		encoder     v1.Encoder[string]
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid topic",
			topicName:   "beacon_block",
			encoder:     encoder,
			expectError: false,
		},
		{
			name:        "empty topic name",
			topicName:   "",
			encoder:     encoder,
			expectError: true,
			errorMsg:    "topic name cannot be empty",
		},
		{
			name:        "nil encoder",
			topicName:   "beacon_block",
			encoder:     nil,
			expectError: true,
			errorMsg:    "encoder cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic, err := v1.NewTopic(tt.topicName, tt.encoder)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, topic)
			} else {
				require.NoError(t, err)
				require.NotNil(t, topic)
				assert.Equal(t, tt.topicName, topic.Name())
				assert.Equal(t, tt.encoder, topic.Encoder())
			}
		})
	}
}

func TestTopicWithForkDigest(t *testing.T) {
	encoder := &topicMockEncoder[string]{}
	topic, err := v1.NewTopic("beacon_block", encoder)
	require.NoError(t, err)

	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	topicWithFork := topic.WithForkDigest(forkDigest)

	assert.Equal(t, "/eth2/01020304/beacon_block/ssz_snappy", topicWithFork.Name())
	assert.Equal(t, encoder, topicWithFork.Encoder())
}

func TestNewSubnetTopic(t *testing.T) {
	encoder := &topicMockEncoder[string]{}

	tests := []struct {
		name        string
		pattern     string
		maxSubnets  uint64
		encoder     v1.Encoder[string]
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid subnet topic",
			pattern:     "beacon_attestation_%d",
			maxSubnets:  64,
			encoder:     encoder,
			expectError: false,
		},
		{
			name:        "empty pattern",
			pattern:     "",
			maxSubnets:  64,
			encoder:     encoder,
			expectError: true,
			errorMsg:    "subnet topic pattern cannot be empty",
		},
		{
			name:        "pattern without placeholder",
			pattern:     "beacon_attestation_",
			maxSubnets:  64,
			encoder:     encoder,
			expectError: true,
			errorMsg:    "subnet topic pattern must contain '%d' placeholder",
		},
		{
			name:        "zero max subnets",
			pattern:     "beacon_attestation_%d",
			maxSubnets:  0,
			encoder:     encoder,
			expectError: true,
			errorMsg:    "maxSubnets must be greater than 0",
		},
		{
			name:        "nil encoder",
			pattern:     "beacon_attestation_%d",
			maxSubnets:  64,
			encoder:     nil,
			expectError: true,
			errorMsg:    "encoder cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnetTopic, err := v1.NewSubnetTopic(tt.pattern, tt.maxSubnets, tt.encoder)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, subnetTopic)
			} else {
				require.NoError(t, err)
				require.NotNil(t, subnetTopic)
				assert.Equal(t, tt.maxSubnets, subnetTopic.MaxSubnets())
				assert.Equal(t, tt.encoder, subnetTopic.Encoder())
			}
		})
	}
}

func TestSubnetTopicForSubnet(t *testing.T) {
	encoder := &topicMockEncoder[string]{}
	subnetTopic, err := v1.NewSubnetTopic("beacon_attestation_%d", 64, encoder)
	require.NoError(t, err)

	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	tests := []struct {
		name         string
		subnet       uint64
		expectError  bool
		expectedName string
	}{
		{
			name:         "valid subnet 0",
			subnet:       0,
			expectError:  false,
			expectedName: "/eth2/01020304/beacon_attestation_0/ssz_snappy",
		},
		{
			name:         "valid subnet 63",
			subnet:       63,
			expectError:  false,
			expectedName: "/eth2/01020304/beacon_attestation_63/ssz_snappy",
		},
		{
			name:        "subnet equals max",
			subnet:      64,
			expectError: true,
		},
		{
			name:        "subnet exceeds max",
			subnet:      100,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic, err := subnetTopic.TopicForSubnet(tt.subnet, forkDigest)
			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, topic)
			} else {
				require.NoError(t, err)
				require.NotNil(t, topic)
				assert.Equal(t, tt.expectedName, topic.Name())
				assert.Equal(t, encoder, topic.Encoder())
			}
		})
	}
}

func TestSubnetParseSubnet(t *testing.T) {
	encoder := &topicMockEncoder[string]{}
	subnetTopic, err := v1.NewSubnetTopic("beacon_attestation_%d", 64, encoder)
	require.NoError(t, err)

	tests := []struct {
		name           string
		topicName      string
		expectedSubnet uint64
		expectError    bool
		errorMsg       string
	}{
		{
			name:           "parse from full eth2 topic",
			topicName:      "/eth2/01020304/beacon_attestation_42/ssz_snappy",
			expectedSubnet: 42,
			expectError:    false,
		},
		{
			name:           "parse from base topic name",
			topicName:      "beacon_attestation_0",
			expectedSubnet: 0,
			expectError:    false,
		},
		{
			name:           "parse subnet 63",
			topicName:      "beacon_attestation_63",
			expectedSubnet: 63,
			expectError:    false,
		},
		{
			name:        "wrong prefix",
			topicName:   "sync_committee_42",
			expectError: true,
			errorMsg:    "does not match pattern prefix",
		},
		{
			name:        "invalid subnet number",
			topicName:   "beacon_attestation_abc",
			expectError: true,
			errorMsg:    "failed to parse subnet ID",
		},
		{
			name:        "subnet exceeds max",
			topicName:   "beacon_attestation_100",
			expectError: true,
			errorMsg:    "exceeds maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet, err := subnetTopic.ParseSubnet(tt.topicName)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedSubnet, subnet)
			}
		})
	}
}

func TestSubnetTopicWithSuffix(t *testing.T) {
	encoder := &topicMockEncoder[string]{}
	// Pattern with suffix
	subnetTopic, err := v1.NewSubnetTopic("sync_contribution_%d_proof", 4, encoder)
	require.NoError(t, err)

	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	// Test topic creation
	topic, err := subnetTopic.TopicForSubnet(2, forkDigest)
	require.NoError(t, err)
	assert.Equal(t, "/eth2/01020304/sync_contribution_2_proof/ssz_snappy", topic.Name())

	// Test parsing
	tests := []struct {
		name           string
		topicName      string
		expectedSubnet uint64
		expectError    bool
	}{
		{
			name:           "parse with suffix from full topic",
			topicName:      "/eth2/01020304/sync_contribution_3_proof/ssz_snappy",
			expectedSubnet: 3,
			expectError:    false,
		},
		{
			name:           "parse with suffix from base name",
			topicName:      "sync_contribution_1_proof",
			expectedSubnet: 1,
			expectError:    false,
		},
		{
			name:        "missing suffix",
			topicName:   "sync_contribution_2",
			expectError: true,
		},
		{
			name:        "wrong suffix",
			topicName:   "sync_contribution_2_data",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet, err := subnetTopic.ParseSubnet(tt.topicName)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedSubnet, subnet)
			}
		})
	}
}
