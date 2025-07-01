package v1_test

import (
	"context"
	"fmt"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
)

// RegistryExampleMessage represents a sample message type for registry examples
type RegistryExampleMessage struct {
	ID      string
	Content string
}

// RegistryExampleEncoder implements the Encoder interface for RegistryExampleMessage
type RegistryExampleEncoder struct{}

func (e *RegistryExampleEncoder) Encode(msg RegistryExampleMessage) ([]byte, error) {
	return []byte(fmt.Sprintf("%s:%s", msg.ID, msg.Content)), nil
}

func (e *RegistryExampleEncoder) Decode(data []byte) (RegistryExampleMessage, error) {
	// Simple decode logic for example
	return RegistryExampleMessage{ID: "decoded", Content: string(data)}, nil
}

// Example demonstrates how to use the Registry with regular topics
func Example_registry() {
	// Create a new registry
	registry := v1.NewRegistry()

	// Create a topic with an encoder
	topic, err := v1.NewTopic[RegistryExampleMessage]("example_topic", &RegistryExampleEncoder{})
	if err != nil {
		panic(err)
	}

	// Create a handler configuration
	handler := v1.NewHandlerConfig[RegistryExampleMessage](
		v1.WithValidator(func(ctx context.Context, msg RegistryExampleMessage, from peer.ID) v1.ValidationResult {
			// Validate the message
			if msg.ID == "" {
				return v1.ValidationReject
			}
			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg RegistryExampleMessage, from peer.ID) error {
			// Process the message
			fmt.Printf("Processing message %s from %s\n", msg.ID, from.String())
			return nil
		}),
	)

	// Register the handler
	if err := v1.Register(registry, topic, handler); err != nil {
		panic(err)
	}

	fmt.Println("Handler registered successfully")
	// Output: Handler registered successfully
}

// Example demonstrates how to use the Registry with subnet topics
func Example_registrySubnet() {
	// Create a new registry
	registry := v1.NewRegistry()

	// Create a subnet topic pattern for attestations
	// This will handle topics like "beacon_attestation_0", "beacon_attestation_1", etc.
	subnetTopic, err := v1.NewSubnetTopic[RegistryExampleMessage](
		"beacon_attestation_%d",
		64, // Maximum 64 subnets
		&RegistryExampleEncoder{},
	)
	if err != nil {
		panic(err)
	}

	// Create a handler configuration for subnet messages
	handler := v1.NewHandlerConfig[RegistryExampleMessage](
		v1.WithProcessor(func(ctx context.Context, msg RegistryExampleMessage, from peer.ID) error {
			// Process subnet messages
			fmt.Printf("Processing subnet message %s\n", msg.ID)
			return nil
		}),
	)

	// Register the subnet handler
	if err := v1.RegisterSubnet(registry, subnetTopic, handler); err != nil {
		panic(err)
	}

	fmt.Println("Subnet handler registered successfully")
	// Output: Subnet handler registered successfully
}
