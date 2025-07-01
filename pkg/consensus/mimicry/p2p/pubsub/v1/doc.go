// Package v1 provides a type-safe, production-ready wrapper around libp2p's gossipsub
// implementation, specifically designed for Ethereum consensus layer communication.
//
// # Overview
//
// This package offers a modern, generic-based API for handling gossipsub topics with
// built-in support for validation, processing, metrics, and events. It's designed to
// simplify the complexity of gossipsub while providing the flexibility needed for
// production Ethereum consensus clients.
//
// # Key Features
//
//   - Type-safe topic handling with Go generics
//   - Built-in message validation and processing pipelines
//   - Comprehensive metrics collection
//   - Event system for monitoring and debugging
//   - Subnet subscription management
//   - Production-ready error handling and timeouts
//   - Memory-efficient message handling
//   - Configurable scoring and peer management
//
// # Quick Start
//
// Here's a minimal example of setting up a gossipsub instance:
//
//	import (
//		"context"
//		"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
//		"github.com/libp2p/go-libp2p"
//		"github.com/sirupsen/logrus"
//	)
//
//	// Create a libp2p host
//	host, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
//	if err != nil {
//		panic(err)
//	}
//	defer host.Close()
//
//	// Create gossipsub instance
//	gs, err := v1.New(context.Background(), host,
//		v1.WithLogger(logrus.StandardLogger()),
//		v1.WithValidationConcurrency(100),
//	)
//	if err != nil {
//		panic(err)
//	}
//	defer gs.Stop()
//
// # Core Components
//
// ## Gossipsub
//
// The main Gossipsub struct manages the overall gossipsub instance and provides
// methods for lifecycle management:
//
//	type Gossipsub struct {
//		// Contains libp2p pubsub instance and configuration
//	}
//
// Key methods:
//   - New(): Create a new gossipsub instance
//   - Stop(): Gracefully shutdown the instance
//   - Registry(): Get the topic registry for handler registration
//   - TopicCount(): Get number of active topics
//   - ActiveTopics(): List active topic names
//
// ## Topics
//
// Topics are type-safe wrappers around gossipsub topic names with associated encoders:
//
//	type Topic[T any] struct {
//		name    string
//		encoder Encoder[T]
//	}
//
// Create topics using:
//
//	topic, err := v1.NewTopic[MyMessageType]("my-topic", encoder)
//
// ## Encoders
//
// Encoders handle serialization/deserialization of messages:
//
//	type Encoder[T any] interface {
//		Encode(T) ([]byte, error)
//		Decode([]byte) (T, error)
//	}
//
// The package provides SSZ encoders for Ethereum types:
//
//	encoder := topics.NewSSZSnappyEncoder[*eth.BeaconBlock]()
//
// ## Handlers
//
// Handlers define how messages are validated and processed:
//
//	type HandlerConfig[T any] struct {
//		validator Validator[T]
//		processor Processor[T]
//		// ... other configuration
//	}
//
// Create handlers using:
//
//	handler := v1.NewHandlerConfig[MyMessageType](
//		v1.WithValidator(myValidator),
//		v1.WithProcessor(myProcessor),
//	)
//
// ## Validation
//
// Validators determine if incoming messages should be accepted, rejected, or ignored:
//
//	type Validator[T any] func(ctx context.Context, msg T, from peer.ID) ValidationResult
//
//	func myValidator(ctx context.Context, msg MyMessageType, from peer.ID) v1.ValidationResult {
//		// Validate message
//		if isValid(msg) {
//			return v1.ValidationAccept
//		}
//		return v1.ValidationReject
//	}
//
// Validation results:
//   - ValidationAccept: Accept and propagate the message
//   - ValidationReject: Reject and penalize the sender
//   - ValidationIgnore: Ignore without penalty
//
// ## Processing
//
// Processors handle accepted messages:
//
//	type Processor[T any] func(ctx context.Context, msg T, from peer.ID) error
//
//	func myProcessor(ctx context.Context, msg MyMessageType, from peer.ID) error {
//		// Process the message
//		return saveToDatabase(msg)
//	}
//
// # Complete Example
//
// Here's a complete example showing all components:
//
//	package main
//
//	import (
//		"context"
//		"fmt"
//		"log"
//
//		v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
//		"github.com/libp2p/go-libp2p"
//		"github.com/libp2p/go-libp2p/core/peer"
//		"github.com/sirupsen/logrus"
//	)
//
//	// Define your message type
//	type MyMessage struct {
//		ID      string
//		Content string
//	}
//
//	// Define an encoder
//	type MyEncoder struct{}
//
//	func (e *MyEncoder) Encode(msg MyMessage) ([]byte, error) {
//		return []byte(fmt.Sprintf("%s:%s", msg.ID, msg.Content)), nil
//	}
//
//	func (e *MyEncoder) Decode(data []byte) (MyMessage, error) {
//		// Simplified decoding
//		return MyMessage{ID: "decoded", Content: string(data)}, nil
//	}
//
//	func main() {
//		// Create host
//		host, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer host.Close()
//
//		// Create gossipsub
//		gs, err := v1.New(context.Background(), host,
//			v1.WithLogger(logrus.StandardLogger()),
//		)
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer gs.Stop()
//
//		// Create topic
//		encoder := &MyEncoder{}
//		topic, err := v1.NewTopic[MyMessage]("my-topic", encoder)
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		// Create handler
//		handler := v1.NewHandlerConfig[MyMessage](
//			v1.WithValidator(func(ctx context.Context, msg MyMessage, from peer.ID) v1.ValidationResult {
//				if msg.Content != "" {
//					return v1.ValidationAccept
//				}
//				return v1.ValidationReject
//			}),
//			v1.WithProcessor(func(ctx context.Context, msg MyMessage, from peer.ID) error {
//				fmt.Printf("Received: %s from %s\n", msg.Content, from)
//				return nil
//			}),
//		)
//
//		// Register handler
//		if err := v1.Register(gs.Registry(), topic, handler); err != nil {
//			log.Fatal(err)
//		}
//
//		// Subscribe
//		sub, err := v1.Subscribe(context.Background(), gs, topic)
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer sub.Cancel()
//
//		// Publish a message
//		msg := MyMessage{ID: "1", Content: "Hello, Gossipsub!"}
//		if err := v1.Publish(gs, topic, msg); err != nil {
//			log.Fatal(err)
//		}
//
//		// Keep running
//		select {}
//	}
//
// # Subnet Management
//
// For Ethereum attestation subnets, use SubnetTopic and SubnetSubscription:
//
//	// Create subnet topic template
//	subnetTopic, err := v1.NewSubnetTopic[*eth.Attestation]("beacon_attestation_%d", 64, encoder)
//	if err != nil {
//		return err
//	}
//
//	// Create subnet subscription manager
//	subnetSub, err := v1.NewSubnetSubscription(subnetTopic)
//	if err != nil {
//		return err
//	}
//
//	// Subscribe to specific subnets
//	for _, subnet := range mySubnets {
//		topic, err := subnetTopic.TopicForSubnet(subnet, forkDigest)
//		if err != nil {
//			continue
//		}
//		sub, err := v1.Subscribe(ctx, gs, topic)
//		if err != nil {
//			continue
//		}
//		subnetSub.Add(subnet, sub)
//	}
//
// # Metrics
//
// Enable metrics collection for monitoring:
//
//	// Create metrics
//	metrics := v1.NewMetrics("my_gossipsub")
//
//	// Register with Prometheus
//	registry := prometheus.NewRegistry()
//	metrics.Register(registry)
//
//	// Pass to gossipsub
//	gs, err := v1.New(ctx, host,
//		v1.WithMetrics(metrics),
//	)
//
// Available metrics:
//   - Messages received/published/validated/handled counts
//   - Validation and processing durations
//   - Active subscriptions count
//   - Error counts by type
//
// # Events
//
// Monitor gossipsub activity with the event system:
//
//	events := make(chan v1.Event, 1000)
//
//	handler := v1.NewHandlerConfig[MyMessage](
//		v1.WithValidator(myValidator),
//		v1.WithProcessor(myProcessor),
//		v1.WithEvents(events),
//	)
//
//	// Monitor events
//	go func() {
//		for event := range events {
//			switch e := event.(type) {
//			case *v1.GossipsubEvent:
//				fmt.Printf("Event: %s on topic %s\n", e.EventType, e.Topic)
//			}
//		}
//	}()
//
// # Configuration Options
//
// Gossipsub supports various configuration options:
//
//	gs, err := v1.New(ctx, host,
//		v1.WithLogger(logger),                    // Custom logger
//		v1.WithMetrics(metrics),                  // Metrics collection
//		v1.WithMaxMessageSize(10*1024*1024),      // Max message size
//		v1.WithValidationConcurrency(200),        // Validation workers
//		v1.WithPublishTimeout(5*time.Second),     // Publish timeout
//		v1.WithGossipSubParams(customParams),     // Custom gossipsub params
//	)
//
// Handler configuration options:
//
//	handler := v1.NewHandlerConfig[T](
//		v1.WithValidator(validator),              // Message validator
//		v1.WithProcessor(processor),              // Message processor
//		v1.WithScoreParams(scoreParams),          // Topic scoring
//		v1.WithEvents(events),                    // Event channel
//	)
//
// # Error Handling
//
// The package uses structured error handling with proper context:
//
//   - Validation errors are logged and metrics are updated
//   - Processing errors are returned and logged
//   - Network errors are handled gracefully with retries
//   - Timeouts prevent hanging operations
//
// # Best Practices
//
// 1. Always use timeouts in validators and processors
// 2. Handle errors gracefully without panicking
// 3. Use appropriate log levels (Debug/Info/Warn/Error)
// 4. Monitor metrics for performance insights
// 5. Implement proper scoring for spam protection
// 6. Use appropriate buffer sizes for event channels
// 7. Test with the race detector enabled
// 8. Validate message structure before processing
// 9. Use proper fork digests for network isolation
// 10. Implement graceful shutdown procedures
//
// # Thread Safety
//
// All public APIs are thread-safe and can be called concurrently from multiple
// goroutines. Internal synchronization is handled automatically.
//
// # Performance Considerations
//
//   - Use buffered channels for events to prevent blocking
//   - Configure appropriate validation concurrency
//   - Monitor memory usage with large message volumes
//   - Use efficient encoders (SSZ + Snappy for Ethereum)
//   - Implement proper message deduplication
//   - Configure topic scoring to prevent spam
//
// # Ethereum Integration
//
// This package is specifically designed for Ethereum consensus layer protocols.
// See the examples in the eth package for production-ready implementations of:
//
//   - Beacon block handling
//   - Attestation subnet management
//   - Sync committee message processing
//   - Voluntary exit handling
//   - BLS to execution change processing
//
// # Testing
//
// The package includes comprehensive test coverage. Run tests with:
//
//	go test -race ./...
//
// For benchmarks:
//
//	go test -bench=. ./...
//
// # Examples
//
// See the examples directory for complete, production-ready examples of:
//   - Basic gossipsub setup
//   - Beacon block handlers
//   - Attestation subnet management
//   - Metrics integration
//   - Event monitoring
package v1