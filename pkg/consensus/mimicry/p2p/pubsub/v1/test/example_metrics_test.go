package v1_test

import (
	"context"
	"fmt"
	"log"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// Example demonstrates how to use metrics with gossipsub v1.
func Example_metrics() {
	// Create a prometheus registry
	registry := prometheus.NewRegistry()

	// Create metrics instance
	metrics := v1.NewMetrics("gossipsub")

	// Register metrics with prometheus
	if err := metrics.Register(registry); err != nil {
		log.Fatalf("Failed to register metrics: %v", err)
	}

	// Create libp2p host
	host, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		log.Fatalf("Failed to create host: %v", err)
	}
	defer host.Close()

	// Create gossipsub with metrics
	ctx := context.Background()
	gs, err := v1.New(ctx, host,
		v1.WithLogger(logrus.StandardLogger()),
		v1.WithMetrics(metrics), // Enable metrics collection
	)
	if err != nil {
		log.Fatalf("Failed to create gossipsub: %v", err)
	}
	defer gs.Stop()

	// Create a topic with an encoder
	encoder := &MetricsExampleEncoder{}
	topic, err := v1.NewTopic[MetricsExampleMessage]("example-topic", encoder)
	if err != nil {
		log.Fatalf("Failed to create topic: %v", err)
	}

	// Create handler with validator and processor
	handler := v1.NewHandlerConfig[MetricsExampleMessage](
		v1.WithValidator(func(ctx context.Context, msg MetricsExampleMessage, from peer.ID) v1.ValidationResult {
			// Validate message
			if msg.Content == "" {
				return v1.ValidationReject
			}
			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, msg MetricsExampleMessage, from peer.ID) error {
			// Process message
			fmt.Printf("Received message: %s\n", msg.Content)
			return nil
		}),
	)

	// Register handler using the registry
	if err := v1.Register(gs.Registry(), topic, handler); err != nil {
		log.Fatalf("Failed to register handler: %v", err)
	}

	// Subscribe to topic - metrics will track active subscriptions
	sub, err := v1.Subscribe(gs, topic)
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Cancel()

	// Publish a message - metrics will track publish duration and success
	msg := MetricsExampleMessage{Content: "Hello, Gossipsub with Metrics!"}
	if err := v1.Publish(gs, topic, msg); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}

	// Metrics are now being collected:
	// - Messages received/published/validated/handled counts
	// - Validation and handler durations
	// - Active subscriptions count
	// - Error counts

	// You can expose these metrics via HTTP for Prometheus scraping
	fmt.Println("Metrics are being collected!")
}

// MetricsExampleMessage is a simple message type for the metrics example.
type MetricsExampleMessage struct {
	Content string
}

// MetricsExampleEncoder implements the Encoder interface for MetricsExampleMessage.
type MetricsExampleEncoder struct{}

func (e *MetricsExampleEncoder) Encode(msg MetricsExampleMessage) ([]byte, error) {
	return []byte(msg.Content), nil
}

func (e *MetricsExampleEncoder) Decode(data []byte) (MetricsExampleMessage, error) {
	return MetricsExampleMessage{Content: string(data)}, nil
}
