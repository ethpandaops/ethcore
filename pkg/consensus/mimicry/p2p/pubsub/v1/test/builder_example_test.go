package v1_test

import (
	"context"
	"fmt"
	"log"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// BeaconBlock represents a simplified beacon block for examples.
// In production, this would be a proper Ethereum beacon block type.
type BeaconBlock struct {
	Slot      uint64
	StateRoot [32]byte
	Body      []byte
}

// Implement SSZMarshaler interface (simplified for example).
func (b *BeaconBlock) MarshalSSZ() ([]byte, error) {
	// In production, use proper SSZ encoding
	return b.Body, nil
}

func (b *BeaconBlock) MarshalSSZTo(dst []byte) ([]byte, error) {
	data, err := b.MarshalSSZ()
	if err != nil {
		return nil, err
	}
	return append(dst, data...), nil
}

func (b *BeaconBlock) UnmarshalSSZ(data []byte) error {
	// In production, use proper SSZ decoding
	b.Body = make([]byte, len(data))
	copy(b.Body, data)
	return nil
}

func (b *BeaconBlock) SizeSSZ() int {
	return len(b.Body)
}

func (b *BeaconBlock) HashTreeRoot() ([32]byte, error) {
	return b.StateRoot, nil
}

func (b *BeaconBlock) HashTreeRootWith(hh interface{}) error {
	return nil
}

// Example_quickSetup demonstrates the simplest way to create a handler.
func Example_quickSetup() {
	// Simple processor that just logs received blocks
	processor := func(ctx context.Context, block *BeaconBlock, from peer.ID) error {
		fmt.Printf("Received block at slot %d from peer %s\n", block.Slot, from)
		return nil
	}

	// Create handler with SSZ decoding and processing
	handler := v1.NewSSZHandler(processor)

	// Register with a gossipsub instance
	registry := v1.NewRegistry()
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	topic, _ := v1.NewEthereumTopic[*BeaconBlock]("beacon_block", forkDigest)
	_ = v1.Register(registry, topic, handler)

	fmt.Println("Handler registered for beacon blocks")
}

// Example_validatedHandler demonstrates setting up a handler with validation.
func Example_validatedHandler() {
	// Validator ensures blocks are from valid slots
	validator := func(ctx context.Context, block *BeaconBlock, from peer.ID) v1.ValidationResult {
		if block.Slot == 0 {
			return v1.ValidationReject
		}
		return v1.ValidationAccept
	}

	// Processor handles validated blocks
	processor := func(ctx context.Context, block *BeaconBlock, from peer.ID) error {
		// Process the validated block
		return nil
	}

	// Create handler with validation and processing
	handler := v1.NewSSZValidatedHandler(validator, processor)

	// Can also add score parameters for peer scoring
	scoreParams := v1.DefaultEthereumScoreParams()
	handlerWithScoring := v1.NewSSZValidatedHandler(
		validator,
		processor,
		v1.WithScoreParams[*BeaconBlock](scoreParams),
	)

	_ = handler
	_ = handlerWithScoring
	fmt.Println("Validated handlers created")
}

// Example_strictValidation demonstrates using strict validation preset.
func Example_strictValidation() {
	// Define strict validation logic
	strictValidator := func(ctx context.Context, block *BeaconBlock, from peer.ID) error {
		if block.Slot == 0 {
			return fmt.Errorf("invalid slot: cannot be 0")
		}
		if len(block.Body) < 100 {
			return fmt.Errorf("block body too small")
		}
		return nil
	}

	// Processor for valid blocks
	processor := func(ctx context.Context, block *BeaconBlock, from peer.ID) error {
		log.Printf("Processing valid block at slot %d", block.Slot)
		return nil
	}

	// Create handler with strict validation using custom decoder
	sszDecoder := func(data []byte) (*BeaconBlock, error) {
		block := &BeaconBlock{}
		if err := block.UnmarshalSSZ(data); err != nil {
			return nil, err
		}
		return block, nil
	}

	handler := v1.NewHandlerConfig(
		v1.WithDecoder(sszDecoder),
		v1.WithStrictValidation[*BeaconBlock](strictValidator),
		v1.WithProcessor(processor),
	)

	_ = handler
	fmt.Println("Strict validation handler created")
}

// Example_secureHandling demonstrates using the secure handling preset.
func Example_secureHandling() {
	// Validator for security checks
	validator := func(ctx context.Context, block *BeaconBlock, from peer.ID) v1.ValidationResult {
		// Perform security validations
		if block.Slot > 1000000 {
			return v1.ValidationReject // Far future slot
		}
		return v1.ValidationAccept
	}

	// Processor for secure handling
	processor := func(ctx context.Context, block *BeaconBlock, from peer.ID) error {
		// Process securely validated blocks
		return nil
	}

	// Score parameters for peer scoring
	scoreParams := &pubsub.TopicScoreParams{
		TopicWeight:                    1.0,
		TimeInMeshWeight:               0.1,
		FirstMessageDeliveriesWeight:   1.0,
		InvalidMessageDeliveriesWeight: -1.0,
	}

	// Create handler with secure handling preset
	options := v1.WithSecureHandling(validator, processor, scoreParams)

	// Create custom SSZ decoder
	sszDecoder := func(data []byte) (*BeaconBlock, error) {
		block := &BeaconBlock{}
		if err := block.UnmarshalSSZ(data); err != nil {
			return nil, err
		}
		return block, nil
	}

	handler := v1.NewHandlerConfig(
		append([]v1.HandlerOption[*BeaconBlock]{
			v1.WithDecoder(sszDecoder),
		}, options...)...,
	)

	_ = handler
	fmt.Println("Secure handling configured")
}

// Example_ethereumSubnets demonstrates working with Ethereum subnet topics.
func Example_ethereumSubnets() {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	maxSubnets := uint64(64) // Ethereum attestation subnets

	// Create subnet topic for attestations
	attestationSubnet, err := v1.NewEthereumSubnetTopic[*BeaconBlock](
		"beacon_attestation_%d",
		maxSubnets,
		forkDigest,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create handlers for specific subnets
	for subnet := uint64(0); subnet < 5; subnet++ {
		topic, _ := attestationSubnet.TopicForSubnet(subnet, forkDigest)

		// Create subnet-specific processor
		processor := func(s uint64) v1.Processor[*BeaconBlock] {
			return func(ctx context.Context, block *BeaconBlock, from peer.ID) error {
				fmt.Printf("Received attestation on subnet %d\n", s)
				return nil
			}
		}(subnet)

		handler := v1.NewSSZHandler(processor)

		// Would register this handler for the specific subnet topic
		_ = topic
		_ = handler
	}

	fmt.Println("Subnet handlers configured")
}
