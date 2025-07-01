package v1_test

import (
	"context"
	"fmt"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
)

// ExampleMessage represents a simple message type.
type ExampleMessage struct {
	Data string
}

// exampleEncoder implements the Encoder interface for ExampleMessage.
type exampleEncoder struct{}

func (e *exampleEncoder) Encode(msg ExampleMessage) ([]byte, error) {
	return []byte(msg.Data), nil
}

func (e *exampleEncoder) Decode(data []byte) (ExampleMessage, error) {
	return ExampleMessage{Data: string(data)}, nil
}

// Example demonstrates basic usage of the v1 gossipsub package.
func Example() {
	// Create a registry to manage topics and handlers
	registry := v1.NewRegistry()

	// Create a topic with an encoder
	encoder := &exampleEncoder{}
	topic, err := v1.NewTopic[ExampleMessage]("example-topic", encoder)
	if err != nil {
		panic(err)
	}

	// Create a handler for the topic
	handler := v1.NewHandlerConfig[ExampleMessage](
		v1.WithProcessor[ExampleMessage](func(ctx context.Context, msg ExampleMessage, from peer.ID) error {
			fmt.Printf("Received message: %s from %s\n", msg.Data, from)
			return nil
		}),
		v1.WithValidator[ExampleMessage](func(ctx context.Context, msg ExampleMessage, from peer.ID) v1.ValidationResult {
			// Validate message
			if len(msg.Data) > 100 {
				return v1.ValidationReject
			}
			return v1.ValidationAccept
		}),
	)

	// Register the handler
	if err := v1.Register(registry, topic, handler); err != nil {
		panic(err)
	}

	fmt.Println("Handler registered successfully")
	// Output: Handler registered successfully
}

// Example_subnetTopics demonstrates working with subnet topics.
func Example_subnetTopics() {
	// Create a registry
	registry := v1.NewRegistry()

	// Create a subnet topic for attestations
	maxSubnets := uint64(64)
	subnetTopic, err := v1.NewSubnetTopic[ExampleMessage](
		"attestation_%d",
		maxSubnets,
		&exampleEncoder{},
	)
	if err != nil {
		panic(err)
	}

	// Create a handler that will be used for all subnets
	handler := v1.NewHandlerConfig[ExampleMessage](
		v1.WithProcessor[ExampleMessage](func(ctx context.Context, msg ExampleMessage, from peer.ID) error {
			fmt.Printf("Processing subnet message: %s\n", msg.Data)
			return nil
		}),
	)

	// Register the subnet handler
	if err := v1.RegisterSubnet(registry, subnetTopic, handler); err != nil {
		panic(err)
	}

	// Get specific subnet topics
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	subnet5, err := subnetTopic.TopicForSubnet(5, forkDigest)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Subnet 5 topic name: %s\n", subnet5.Name())

	// Output: Subnet 5 topic name: /eth2/01020304/attestation_5/ssz_snappy
}
