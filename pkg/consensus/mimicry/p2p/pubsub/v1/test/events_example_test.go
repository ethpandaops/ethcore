package v1_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
)

// ExampleWithEventCallbacks demonstrates how to use the event callbacks system.
func ExampleWithEventCallbacks() {
	// Create event callbacks
	callbacks := &v1.EventCallbacks{
		OnSubscribed: func(topic string) {
			fmt.Printf("Subscribed to topic: %s\n", topic)
		},
		OnMessageReceived: func(topic string, from peer.ID, data *v1.MessageEventData) {
			fmt.Printf("Received message %s from %s on topic %s (size: %d bytes)\n",
				data.MessageID, from, topic, data.Size)
		},
		OnMessageValidated: func(topic string, from peer.ID, data *v1.MessageEventData) {
			fmt.Printf("Message %s validated with result: %v\n",
				data.MessageID, data.ValidationResult)
		},
		OnMessageProcessed: func(topic string, from peer.ID, data *v1.MessageEventData) {
			fmt.Printf("Message %s processed in %v\n",
				data.MessageID, data.ProcessingDuration)
		},
		OnError: func(topic string, err error, data *v1.ErrorEventData) {
			fmt.Printf("Error during %s: %v (retry count: %d)\n",
				data.Operation, err, data.RetryCount)
		},
	}

	// Create a handler with event callbacks
	handler := v1.NewHandlerConfig[string](
		v1.WithEventCallbacks[string](callbacks),
		v1.WithValidator[string](func(ctx context.Context, msg string, from peer.ID) v1.ValidationResult {
			// Simple validation logic
			if len(msg) > 0 {
				return v1.ValidationAccept
			}
			return v1.ValidationReject
		}),
		v1.WithProcessor[string](func(ctx context.Context, msg string, from peer.ID) error {
			// Process the message
			fmt.Printf("Processing message: %s\n", msg)
			return nil
		}),
	)

	// The handler is now configured with event callbacks
	_ = handler
}

// ExampleNewEvent demonstrates creating and manipulating events.
func ExampleNewEvent() {
	peerID := peer.ID("12D3KooWExample")

	// Create a message received event
	event := v1.NewEvent(v1.EventTypeMessageReceived, "test-topic", peerID).
		WithMessageData("msg-123", 1024)

	fmt.Printf("Event type: %s\n", event.Type())
	fmt.Printf("Topic: %s\n", event.Topic)
	fmt.Printf("PeerID: %s\n", event.PeerID)

	// Create an error event
	errorEvent := v1.NewEvent(v1.EventTypeError, "test-topic", peerID).
		WithError(fmt.Errorf("validation failed"), "message validation")

	fmt.Printf("Error event: %s - %v\n", errorEvent.Type(), errorEvent.Error)

	// Create a message processed event with duration
	processedEvent := v1.NewEvent(v1.EventTypeMessageProcessed, "test-topic", peerID).
		WithMessageData("msg-456", 2048).
		WithProcessingDuration(50 * time.Millisecond)

	if data, ok := processedEvent.Data.(*v1.MessageEventData); ok {
		fmt.Printf("Processed message %s in %v\n", data.MessageID, data.ProcessingDuration)
	}
}
