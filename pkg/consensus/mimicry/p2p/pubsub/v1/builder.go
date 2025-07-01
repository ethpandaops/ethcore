// Package v1 provides builder pattern helpers for creating gossipsub handlers.
// This file contains convenience functions and presets that simplify common
// use cases for Ethereum consensus layer gossipsub topics.
//
// Quick setup functions:
//   - NewSSZHandler: Creates a simple handler with SSZ decoding and processing
//   - NewSSZValidatedHandler: Creates a handler with validation and processing
//
// Preset configurations:
//   - WithStrictValidation: Configures strict message validation
//   - WithProcessingOnly: Configures processing without validation
//   - WithSecureHandling: Configures secure handling with scoring
//
// Ethereum-specific helpers:
//   - NewEthereumTopic: Creates topics formatted for Ethereum gossipsub
//   - NewEthereumSubnetTopic: Creates subnet topics for Ethereum
//   - DefaultEthereumScoreParams: Provides default scoring parameters
package v1

import (
	"context"
	"fmt"
	"reflect"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	fastssz "github.com/prysmaticlabs/fastssz"
)

// SSZMarshaler represents types that can be marshaled/unmarshaled using SSZ.
type SSZMarshaler interface {
	fastssz.Marshaler
	fastssz.Unmarshaler
}

// sszEncoder provides SSZ encoding/decoding functionality.
type sszEncoder[T SSZMarshaler] struct{}

// Encode encodes the message using SSZ.
func (e *sszEncoder[T]) Encode(msg T) ([]byte, error) {
	return msg.MarshalSSZ()
}

// Decode decodes the message using SSZ.
func (e *sszEncoder[T]) Decode(data []byte) (T, error) {
	var msg T
	// Check if T is already a pointer type
	if reflect.TypeOf(msg).Kind() == reflect.Ptr {
		// T is already a pointer, create new instance directly
		msgValue := reflect.New(reflect.TypeOf(msg).Elem())
		msgPtr := msgValue.Interface().(T)
		if err := msgPtr.UnmarshalSSZ(data); err != nil {
			return msg, fmt.Errorf("failed to unmarshal SSZ: %w", err)
		}
		return msgPtr, nil
	}
	// T is not a pointer, use new(T)
	msgPtr := new(T)
	if err := (*msgPtr).UnmarshalSSZ(data); err != nil {
		return msg, fmt.Errorf("failed to unmarshal SSZ: %w", err)
	}
	return *msgPtr, nil
}

// NewSSZEncoder creates a new SSZ encoder for types that implement SSZMarshaler.
func NewSSZEncoder[T SSZMarshaler]() Encoder[T] {
	return &sszEncoder[T]{}
}

// Quick Setup Functions

// NewSSZHandler creates a handler configuration for SSZ-encoded messages with minimal setup.
// This provides a simple handler that only processes messages without validation.
func NewSSZHandler[T SSZMarshaler](processor Processor[T], opts ...HandlerOption[T]) *HandlerConfig[T] {
	// Create a custom SSZ decoder for the SSZMarshaler type
	sszDecoder := func(data []byte) (T, error) {
		var msg T
		msgPtr := new(T)
		if err := (*msgPtr).UnmarshalSSZ(data); err != nil {
			return msg, fmt.Errorf("failed to unmarshal SSZ: %w", err)
		}
		return *msgPtr, nil
	}

	// Start with SSZ decoding
	options := []HandlerOption[T]{
		WithDecoder(sszDecoder),
		WithProcessor(processor),
	}
	// Append any additional options
	options = append(options, opts...)

	return NewHandlerConfig(options...)
}

// NewSSZValidatedHandler creates a handler configuration with both validation and processing.
// This is the recommended setup for production use cases.
func NewSSZValidatedHandler[T SSZMarshaler](
	validator Validator[T],
	processor Processor[T],
	opts ...HandlerOption[T],
) *HandlerConfig[T] {
	// Create a custom SSZ decoder for the SSZMarshaler type
	sszDecoder := func(data []byte) (T, error) {
		var msg T
		msgPtr := new(T)
		if err := (*msgPtr).UnmarshalSSZ(data); err != nil {
			return msg, fmt.Errorf("failed to unmarshal SSZ: %w", err)
		}
		return *msgPtr, nil
	}

	// Start with SSZ decoding and the provided validator/processor
	options := []HandlerOption[T]{
		WithDecoder(sszDecoder),
		WithValidator(validator),
		WithProcessor(processor),
	}
	// Append any additional options
	options = append(options, opts...)

	return NewHandlerConfig(options...)
}

// Preset Configurations

// WithStrictValidation returns options for strict message validation.
// This configuration rejects any message that fails validation.
func WithStrictValidation[T any](validationFunc func(ctx context.Context, msg T, from peer.ID) error) HandlerOption[T] {
	return WithValidator(func(ctx context.Context, msg T, from peer.ID) ValidationResult {
		if err := validationFunc(ctx, msg, from); err != nil {
			return ValidationReject
		}
		return ValidationAccept
	})
}

// WithProcessingOnly returns options for processing without validation.
// This configuration accepts all messages and only focuses on processing.
// Use with caution in production environments.
func WithProcessingOnly[T any](processor Processor[T]) HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		// Set a permissive validator that accepts everything
		h.validator = func(ctx context.Context, msg T, from peer.ID) ValidationResult {
			return ValidationAccept
		}
		h.processor = processor
	}
}

// WithSecureHandling returns options for secure message handling with scoring.
// This configuration includes strict validation and topic scoring parameters.
func WithSecureHandling[T any](
	validator Validator[T],
	processor Processor[T],
	scoreParams *pubsub.TopicScoreParams,
) []HandlerOption[T] {
	return []HandlerOption[T]{
		WithValidator(validator),
		WithProcessor(processor),
		WithScoreParams[T](scoreParams),
	}
}

// Ethereum-specific Helpers

// ethereumTopicConfig holds configuration for Ethereum topics.
type ethereumTopicConfig struct {
	forkDigest [4]byte
	topicName  string
}

// NewEthereumTopic creates a topic configured for Ethereum gossipsub.
// The topic name will be formatted as: /eth2/<fork_digest>/<topic_name>/ssz_snappy
func NewEthereumTopic[T SSZMarshaler](topicName string, forkDigest [4]byte) (*Topic[T], error) {
	if topicName == "" {
		return nil, fmt.Errorf("topic name cannot be empty")
	}

	// Format according to Ethereum gossipsub specification
	fullTopicName := fmt.Sprintf("/eth2/%x/%s/ssz_snappy", forkDigest, topicName)

	// Create SSZ encoder for the type
	encoder := NewSSZEncoder[T]()

	return NewTopic[T](fullTopicName, encoder)
}

// NewEthereumSubnetTopic creates a subnet topic configured for Ethereum gossipsub.
// The pattern should include %d for the subnet ID (e.g., "beacon_attestation_%d").
// Topics will be formatted as: /eth2/<fork_digest>/<pattern>/ssz_snappy
func NewEthereumSubnetTopic[T SSZMarshaler](
	pattern string,
	maxSubnets uint64,
	forkDigest [4]byte,
) (*SubnetTopic[T], error) {
	if pattern == "" {
		return nil, fmt.Errorf("subnet pattern cannot be empty")
	}

	// Create SSZ encoder for the type
	encoder := NewSSZEncoder[T]()

	// Create the subnet topic with the pattern
	// The WithForkDigest will be applied when TopicForSubnet is called
	return NewSubnetTopic[T](pattern, maxSubnets, encoder)
}

// DefaultEthereumScoreParams returns default topic score parameters for Ethereum topics.
// These parameters are based on Ethereum 2.0 specifications and can be customized.
func DefaultEthereumScoreParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		// Topic weight
		TopicWeight: 1.0,

		// Time in mesh parameters
		TimeInMeshWeight:  0.03333333333333333,
		TimeInMeshQuantum: 12 * 1000 * 1000 * 1000, // 12 seconds in nanoseconds
		TimeInMeshCap:     300,

		// First message deliveries
		FirstMessageDeliveriesWeight: 1.0,
		FirstMessageDeliveriesDecay:  0.99283,
		FirstMessageDeliveriesCap:    34.86,

		// Mesh message deliveries
		MeshMessageDeliveriesWeight:     -1.0,
		MeshMessageDeliveriesDecay:      0.99283,
		MeshMessageDeliveriesCap:        34.86,
		MeshMessageDeliveriesThreshold:  0.689,
		MeshMessageDeliveriesActivation: 384 * 1000 * 1000 * 1000, // 384 seconds in nanoseconds
		MeshMessageDeliveriesWindow:     2 * 1000 * 1000 * 1000,   // 2 seconds in nanoseconds

		// Mesh failure penalty
		MeshFailurePenaltyWeight: -1.0,
		MeshFailurePenaltyDecay:  0.99283,

		// Invalid message deliveries
		InvalidMessageDeliveriesWeight: -1.0,
		InvalidMessageDeliveriesDecay:  0.99283,
	}
}
