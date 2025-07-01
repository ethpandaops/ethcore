package v1_test

import (
	"context"
	"fmt"
	"testing"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	fastssz "github.com/prysmaticlabs/fastssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSSZMessage is a test type that implements SSZMarshaler.
type mockSSZMessage struct {
	Data  []byte
	Value uint64
}

func (m *mockSSZMessage) MarshalSSZ() ([]byte, error) {
	// Simple mock implementation
	if m.Data == nil {
		return nil, fmt.Errorf("data is nil")
	}
	return m.Data, nil
}

func (m *mockSSZMessage) MarshalSSZTo(dst []byte) ([]byte, error) {
	marshaled, err := m.MarshalSSZ()
	if err != nil {
		return nil, err
	}
	return append(dst, marshaled...), nil
}

func (m *mockSSZMessage) UnmarshalSSZ(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}
	m.Data = make([]byte, len(data))
	copy(m.Data, data)
	return nil
}

func (m *mockSSZMessage) SizeSSZ() int {
	return len(m.Data)
}

func (m *mockSSZMessage) HashTreeRoot() ([32]byte, error) {
	return [32]byte{}, nil
}

func (m *mockSSZMessage) HashTreeRootWith(hh *fastssz.Hasher) error {
	return nil
}

func TestNewSSZEncoder(t *testing.T) {
	encoder := v1.NewSSZEncoder[*mockSSZMessage]()
	require.NotNil(t, encoder)

	// Test encoding
	msg := &mockSSZMessage{Data: []byte("test data")}
	encoded, err := encoder.Encode(msg)
	require.NoError(t, err)
	assert.Equal(t, []byte("test data"), encoded)

	// Test decoding
	decoded, err := encoder.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, msg.Data, decoded.Data)
}

func TestNewSSZHandler(t *testing.T) {
	processedCount := 0
	processor := func(ctx context.Context, msg *mockSSZMessage, from peer.ID) error {
		processedCount++
		return nil
	}

	handler := v1.NewSSZHandler(processor)
	require.NotNil(t, handler)
	// Cannot test private fields from external test package
}

func TestNewSSZValidatedHandler(t *testing.T) {
	validatedCount := 0
	processedCount := 0

	validator := func(ctx context.Context, msg *mockSSZMessage, from peer.ID) v1.ValidationResult {
		validatedCount++
		if len(msg.Data) == 0 {
			return v1.ValidationReject
		}
		return v1.ValidationAccept
	}

	processor := func(ctx context.Context, msg *mockSSZMessage, from peer.ID) error {
		processedCount++
		return nil
	}

	handler := v1.NewSSZValidatedHandler(validator, processor)
	require.NotNil(t, handler)
	// Cannot test private fields from external test package
}

func TestWithStrictValidation(t *testing.T) {
	validationFunc := func(ctx context.Context, msg *mockSSZMessage, from peer.ID) error {
		if len(msg.Data) < 5 {
			return fmt.Errorf("data too short")
		}
		return nil
	}

	handler := v1.NewHandlerConfig(v1.WithStrictValidation[*mockSSZMessage](validationFunc))
	require.NotNil(t, handler)
	// Cannot test handler's internal validation behavior from external test package
}

func TestWithProcessingOnly(t *testing.T) {
	processedCount := 0
	processor := func(ctx context.Context, msg *mockSSZMessage, from peer.ID) error {
		processedCount++
		return nil
	}

	handler := v1.NewHandlerConfig(v1.WithProcessingOnly[*mockSSZMessage](processor))
	require.NotNil(t, handler)
	// Cannot test handler's internal validation behavior from external test package
}

func TestWithSecureHandling(t *testing.T) {
	validator := func(ctx context.Context, msg *mockSSZMessage, from peer.ID) v1.ValidationResult {
		return v1.ValidationAccept
	}

	processor := func(ctx context.Context, msg *mockSSZMessage, from peer.ID) error {
		return nil
	}

	scoreParams := &pubsub.TopicScoreParams{
		TopicWeight: 1.0,
	}

	options := v1.WithSecureHandling(validator, processor, scoreParams)
	handler := v1.NewHandlerConfig(options...)

	require.NotNil(t, handler)
	// Cannot test private fields from external test package
}

func TestNewEthereumTopic(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	topicName := "beacon_block"

	topic, err := v1.NewEthereumTopic[*mockSSZMessage](topicName, forkDigest)
	require.NoError(t, err)
	require.NotNil(t, topic)

	expectedName := "/eth2/01020304/beacon_block/ssz_snappy"
	assert.Equal(t, expectedName, topic.Name())
	assert.NotNil(t, topic.Encoder())

	// Test with empty topic name
	_, err = v1.NewEthereumTopic[*mockSSZMessage]("", forkDigest)
	assert.Error(t, err)
}

func TestNewEthereumSubnetTopic(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	pattern := "beacon_attestation_%d"
	maxSubnets := uint64(64)

	subnetTopic, err := v1.NewEthereumSubnetTopic[*mockSSZMessage](pattern, maxSubnets, forkDigest)
	require.NoError(t, err)
	require.NotNil(t, subnetTopic)

	// Test getting a topic for a specific subnet
	topic, err := subnetTopic.TopicForSubnet(10, forkDigest)
	require.NoError(t, err)
	require.NotNil(t, topic)

	expectedName := "/eth2/01020304/beacon_attestation_10/ssz_snappy"
	assert.Equal(t, expectedName, topic.Name())

	// Test with invalid subnet
	_, err = subnetTopic.TopicForSubnet(100, forkDigest)
	assert.Error(t, err)

	// Test with empty pattern
	_, err = v1.NewEthereumSubnetTopic[*mockSSZMessage]("", maxSubnets, forkDigest)
	assert.Error(t, err)
}

func TestDefaultEthereumScoreParams(t *testing.T) {
	params := v1.DefaultEthereumScoreParams()
	require.NotNil(t, params)

	// Verify some key parameters
	assert.Equal(t, float64(1.0), params.TopicWeight)
	assert.Equal(t, float64(0.03333333333333333), params.TimeInMeshWeight)
	assert.Equal(t, float64(1.0), params.FirstMessageDeliveriesWeight)
	assert.Equal(t, float64(-1.0), params.MeshMessageDeliveriesWeight)
	assert.Equal(t, float64(-1.0), params.InvalidMessageDeliveriesWeight)
}
