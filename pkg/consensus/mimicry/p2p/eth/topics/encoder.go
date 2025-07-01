package topics

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/golang/snappy"
	fastssz "github.com/prysmaticlabs/fastssz"
)

// SSZMarshaler is a type constraint that combines fastssz marshaling and unmarshalling.
type SSZMarshaler interface {
	fastssz.Marshaler
	fastssz.Unmarshaler
}

// SSZSnappyEncoder implements the v1.Encoder interface for SSZ+Snappy encoding.
// This encoder uses SSZ serialization followed by Snappy compression,
// which is the standard encoding for Ethereum consensus layer gossipsub messages.
type SSZSnappyEncoder[T SSZMarshaler] struct {
	// sszEncoder is the underlying Prysm SSZ network encoder
	sszEncoder *encoder.SszNetworkEncoder
}

// NewSSZSnappyEncoder creates a new SSZ+Snappy encoder.
func NewSSZSnappyEncoder[T SSZMarshaler]() *SSZSnappyEncoder[T] {
	return &SSZSnappyEncoder[T]{
		sszEncoder: &encoder.SszNetworkEncoder{},
	}
}

// Encode encodes the message using SSZ serialization followed by Snappy compression.
func (e *SSZSnappyEncoder[T]) Encode(msg T) ([]byte, error) {
	// First, serialize to SSZ
	sszData, err := msg.MarshalSSZ()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SSZ: %w", err)
	}

	// Then compress with Snappy
	compressed := snappy.Encode(nil, sszData)
	return compressed, nil
}

// Decode decodes the message by first decompressing with Snappy, then deserializing from SSZ.
func (e *SSZSnappyEncoder[T]) Decode(data []byte) (T, error) {
	var zero T

	// First, decompress with Snappy
	decompressed, err := snappy.Decode(nil, data)
	if err != nil {
		return zero, fmt.Errorf("failed to decompress snappy: %w", err)
	}

	// Use reflection to create a proper instance
	reflectType := reflect.TypeOf(zero)

	// If T is a pointer type (e.g., *eth.Attestation)
	if reflectType.Kind() == reflect.Ptr {
		// Create a new instance of the element type
		elem := reflect.New(reflectType.Elem())

		// Get the unmarshaler interface
		unmarshaler := elem.Interface().(fastssz.Unmarshaler)

		// Unmarshal the data
		if err := unmarshaler.UnmarshalSSZ(decompressed); err != nil {
			return zero, fmt.Errorf("failed to unmarshal SSZ: %w", err)
		}

		// Return the pointer as type T
		return elem.Interface().(T), nil
	}

	// For non-pointer types (though this shouldn't happen with our constraint)
	return zero, fmt.Errorf("expected pointer type, got %v", reflectType)
}

// SSZSnappyEncoderWithMaxLen implements the v1.Encoder interface with a maximum message length.
// This is useful for preventing DoS attacks via oversized messages.
type SSZSnappyEncoderWithMaxLen[T SSZMarshaler] struct {
	*SSZSnappyEncoder[T]
	maxLen uint64
}

// NewSSZSnappyEncoderWithMaxLen creates a new SSZ+Snappy encoder with a maximum message length.
func NewSSZSnappyEncoderWithMaxLen[T SSZMarshaler](maxLen uint64) *SSZSnappyEncoderWithMaxLen[T] {
	return &SSZSnappyEncoderWithMaxLen[T]{
		SSZSnappyEncoder: NewSSZSnappyEncoder[T](),
		maxLen:           maxLen,
	}
}

// Encode encodes the message with length validation.
func (e *SSZSnappyEncoderWithMaxLen[T]) Encode(msg T) ([]byte, error) {
	data, err := e.SSZSnappyEncoder.Encode(msg)
	if err != nil {
		return nil, err
	}

	if uint64(len(data)) > e.maxLen {
		return nil, fmt.Errorf("encoded message exceeds maximum length: %d > %d", len(data), e.maxLen)
	}

	return data, nil
}

// Decode decodes the message with length validation.
func (e *SSZSnappyEncoderWithMaxLen[T]) Decode(data []byte) (T, error) {
	if uint64(len(data)) > e.maxLen {
		var zero T
		return zero, fmt.Errorf("encoded message exceeds maximum length: %d > %d", len(data), e.maxLen)
	}

	return e.SSZSnappyEncoder.Decode(data)
}

// CreateEncoderForTopic creates an appropriate encoder for a given topic type.
// This function provides sensible defaults for maximum message sizes based on
// Ethereum consensus layer specifications.
func CreateEncoderForTopic[T SSZMarshaler](topicName string) v1.Encoder[T] {
	// Define maximum sizes for different message types
	// These are based on Ethereum consensus layer specifications
	maxSizes := map[string]uint64{
		BeaconBlockTopicName:          10 * 1024 * 1024, // 10 MB for beacon blocks
		BeaconAggregateAndProofName:   1 * 1024 * 1024,  // 1 MB for aggregates
		VoluntaryExitTopicName:        1 * 1024 * 1024,  // 1 MB for voluntary exits
		ProposerSlashingTopicName:     1 * 1024 * 1024,  // 1 MB for proposer slashings
		AttesterSlashingTopicName:     1 * 1024 * 1024,  // 1 MB for attester slashings
		BlsToExecutionChangeTopicName: 1 * 1024 * 1024,  // 1 MB for BLS changes
		// For patterns, we use the base name
		"beacon_attestation":                    1 * 1024 * 1024, // 1 MB for attestations
		"sync_committee":                        1 * 1024 * 1024, // 1 MB for sync committee messages
		"sync_committee_contribution_and_proof": 1 * 1024 * 1024, // 1 MB for sync contributions
	}

	// Check if we have a specific max size for this topic
	if maxSize, ok := maxSizes[topicName]; ok {
		return NewSSZSnappyEncoderWithMaxLen[T](maxSize)
	}

	// Default to 1 MB max size if not specified
	return NewSSZSnappyEncoderWithMaxLen[T](1 * 1024 * 1024)
}

// PrysmSSZSnappyEncoder wraps Prysm's SszNetworkEncoder to implement v1.Encoder.
// This provides compatibility with existing Prysm-based code.
// Note: T must implement fastssz.Marshaler for this encoder to work.
type PrysmSSZSnappyEncoder[T SSZMarshaler] struct {
	encoder *encoder.SszNetworkEncoder
}

// NewPrysmSSZSnappyEncoder creates a new encoder using Prysm's implementation.
func NewPrysmSSZSnappyEncoder[T SSZMarshaler]() *PrysmSSZSnappyEncoder[T] {
	return &PrysmSSZSnappyEncoder[T]{
		encoder: &encoder.SszNetworkEncoder{},
	}
}

// Encode encodes the message using Prysm's SSZ network encoder.
func (e *PrysmSSZSnappyEncoder[T]) Encode(msg T) ([]byte, error) {
	var buf bytes.Buffer
	_, err := e.encoder.EncodeWithMaxLength(&buf, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode with Prysm encoder: %w", err)
	}
	return buf.Bytes(), nil
}

// Decode decodes the message using Prysm's SSZ network encoder.
func (e *PrysmSSZSnappyEncoder[T]) Decode(data []byte) (T, error) {
	var zero T

	// Use reflection to create a proper instance
	reflectType := reflect.TypeOf(zero)

	// If T is a pointer type (e.g., *eth.Attestation)
	if reflectType.Kind() == reflect.Ptr {
		// Create a new instance of the element type
		elem := reflect.New(reflectType.Elem())

		// Decode into the pointer
		err := e.encoder.DecodeWithMaxLength(bytes.NewReader(data), elem.Interface().(fastssz.Unmarshaler))
		if err != nil {
			return zero, fmt.Errorf("failed to decode with Prysm encoder: %w", err)
		}

		// Return the pointer as type T
		return elem.Interface().(T), nil
	}

	// For non-pointer types
	return zero, fmt.Errorf("expected pointer type, got %v", reflectType)
}
