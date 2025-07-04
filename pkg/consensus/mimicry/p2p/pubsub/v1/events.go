package v1

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// EventType represents the type of event that occurred in the gossipsub system.
type EventType int

const (
	// EventTypeSubscribed indicates a successful subscription to a topic.
	EventTypeSubscribed EventType = iota
	// EventTypeUnsubscribed indicates an unsubscription from a topic.
	EventTypeUnsubscribed
	// EventTypeMessageReceived indicates a message was received on a topic.
	EventTypeMessageReceived
	// EventTypeMessageValidated indicates a message passed validation.
	EventTypeMessageValidated
	// EventTypeMessageRejected indicates a message failed validation.
	EventTypeMessageRejected
	// EventTypeMessageProcessed indicates a message was successfully processed.
	EventTypeMessageProcessed
	// EventTypeMessagePublished indicates a message was published to a topic.
	EventTypeMessagePublished
	// EventTypeError indicates an error occurred during operation.
	EventTypeError
	// EventTypePeerConnected indicates a peer connected to a topic.
	EventTypePeerConnected
	// EventTypePeerDisconnected indicates a peer disconnected from a topic.
	EventTypePeerDisconnected
)

// String returns the string representation of an EventType.
func (t EventType) String() string {
	switch t {
	case EventTypeSubscribed:
		return "subscribed"
	case EventTypeUnsubscribed:
		return "unsubscribed"
	case EventTypeMessageReceived:
		return "message_received"
	case EventTypeMessageValidated:
		return "message_validated"
	case EventTypeMessageRejected:
		return "message_rejected"
	case EventTypeMessageProcessed:
		return "message_processed"
	case EventTypeMessagePublished:
		return "message_published"
	case EventTypeError:
		return "error"
	case EventTypePeerConnected:
		return "peer_connected"
	case EventTypePeerDisconnected:
		return "peer_disconnected"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// GossipsubEvent represents an event that occurs during gossipsub operations.
// It implements the Event interface defined in handler.go.
type GossipsubEvent struct {
	// EventType is the type of event that occurred.
	EventType EventType

	// Topic is the topic name associated with the event.
	Topic string

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// Data contains event-specific data. The type depends on the EventType:
	// - EventTypeMessageReceived/Validated/Rejected/Processed: MessageEventData
	// - EventTypeError: ErrorEventData
	// - EventTypePeerConnected/Disconnected: PeerEventData
	// - Others: may be nil
	Data any

	// Error contains the error if EventType is EventTypeError.
	Error error

	// PeerID is the peer associated with the event, if applicable.
	PeerID peer.ID
}

// Type returns the event type as a string.
// This implements the Event interface from handler.go.
func (e GossipsubEvent) Type() string {
	return e.EventType.String()
}

// MessageEventData contains data specific to message-related events.
type MessageEventData struct {
	// MessageID is the unique identifier of the message.
	MessageID string

	// Size is the size of the message in bytes.
	Size int

	// ValidationResult is the result of message validation (for validated/rejected events).
	ValidationResult ValidationResult

	// ProcessingDuration is how long it took to process the message (for processed events).
	ProcessingDuration time.Duration
}

// ErrorEventData contains data specific to error events.
type ErrorEventData struct {
	// Operation describes what operation was being performed when the error occurred.
	Operation string

	// MessageID is the message ID if the error is related to a specific message.
	MessageID string

	// RetryCount is the number of times this operation has been retried.
	RetryCount int
}

// PeerEventData contains data specific to peer connection events.
type PeerEventData struct {
	// Protocols is the list of protocols supported by the peer.
	Protocols []string

	// SubscribedTopics is the list of topics the peer is subscribed to.
	SubscribedTopics []string
}

// EventCallbacks contains callback functions for different event types.
// This provides a convenient way to handle events without implementing a full event loop.
type EventCallbacks struct {
	// OnSubscribed is called when successfully subscribed to a topic.
	OnSubscribed func(topic string)

	// OnUnsubscribed is called when unsubscribed from a topic.
	OnUnsubscribed func(topic string)

	// OnMessageReceived is called when a message is received.
	OnMessageReceived func(topic string, from peer.ID, data *MessageEventData)

	// OnMessageValidated is called when a message passes validation.
	OnMessageValidated func(topic string, from peer.ID, data *MessageEventData)

	// OnMessageRejected is called when a message fails validation.
	OnMessageRejected func(topic string, from peer.ID, data *MessageEventData)

	// OnMessageProcessed is called when a message is successfully processed.
	OnMessageProcessed func(topic string, from peer.ID, data *MessageEventData)

	// OnMessagePublished is called when a message is published.
	OnMessagePublished func(topic string, data *MessageEventData)

	// OnError is called when an error occurs.
	OnError func(topic string, err error, data *ErrorEventData)

	// OnPeerConnected is called when a peer connects to a topic.
	OnPeerConnected func(topic string, peerID peer.ID, data *PeerEventData)

	// OnPeerDisconnected is called when a peer disconnects from a topic.
	OnPeerDisconnected func(topic string, peerID peer.ID, data *PeerEventData)
}

// WithEventCallbacks creates a handler option that sets up event callbacks.
// This provides a convenient way to handle events without managing channels directly.
func WithEventCallbacks[T any](callbacks *EventCallbacks) HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		if callbacks == nil {
			return
		}

		// Create a channel for events if not already set
		eventChan := make(chan Event, 100)
		h.events = eventChan

		// Start a goroutine to process events and call appropriate callbacks
		go func() {
			for event := range eventChan {
				if gossipEvent, ok := event.(*GossipsubEvent); ok {
					processEventCallback(callbacks, gossipEvent)
				}
			}
		}()
	}
}

// processEventCallback handles the event callback routing based on event type.
func processEventCallback(callbacks *EventCallbacks, gossipEvent *GossipsubEvent) {
	switch gossipEvent.EventType {
	case EventTypeSubscribed:
		if callbacks.OnSubscribed != nil {
			callbacks.OnSubscribed(gossipEvent.Topic)
		}
	case EventTypeUnsubscribed:
		if callbacks.OnUnsubscribed != nil {
			callbacks.OnUnsubscribed(gossipEvent.Topic)
		}
	case EventTypeMessageReceived:
		processMessageCallback(callbacks.OnMessageReceived, gossipEvent)
	case EventTypeMessageValidated:
		processMessageCallback(callbacks.OnMessageValidated, gossipEvent)
	case EventTypeMessageRejected:
		processMessageCallback(callbacks.OnMessageRejected, gossipEvent)
	case EventTypeMessageProcessed:
		processMessageCallback(callbacks.OnMessageProcessed, gossipEvent)
	case EventTypeMessagePublished:
		processPublishCallback(callbacks.OnMessagePublished, gossipEvent)
	case EventTypeError:
		processErrorCallback(callbacks.OnError, gossipEvent)
	case EventTypePeerConnected:
		processPeerCallback(callbacks.OnPeerConnected, gossipEvent)
	case EventTypePeerDisconnected:
		processPeerCallback(callbacks.OnPeerDisconnected, gossipEvent)
	}
}

// processMessageCallback handles message-related callbacks.
func processMessageCallback(callback func(string, peer.ID, *MessageEventData), gossipEvent *GossipsubEvent) {
	if callback != nil {
		if data, ok := gossipEvent.Data.(*MessageEventData); ok {
			callback(gossipEvent.Topic, gossipEvent.PeerID, data)
		}
	}
}

// processPublishCallback handles message published callbacks.
func processPublishCallback(callback func(string, *MessageEventData), gossipEvent *GossipsubEvent) {
	if callback != nil {
		if data, ok := gossipEvent.Data.(*MessageEventData); ok {
			callback(gossipEvent.Topic, data)
		}
	}
}

// processErrorCallback handles error callbacks.
func processErrorCallback(callback func(string, error, *ErrorEventData), gossipEvent *GossipsubEvent) {
	if callback != nil {
		if data, ok := gossipEvent.Data.(*ErrorEventData); ok {
			callback(gossipEvent.Topic, gossipEvent.Error, data)
		}
	}
}

// processPeerCallback handles peer-related callbacks.
func processPeerCallback(callback func(string, peer.ID, *PeerEventData), gossipEvent *GossipsubEvent) {
	if callback != nil {
		if data, ok := gossipEvent.Data.(*PeerEventData); ok {
			callback(gossipEvent.Topic, gossipEvent.PeerID, data)
		}
	}
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType EventType, topic string, peerID peer.ID) *GossipsubEvent {
	return &GossipsubEvent{
		EventType: eventType,
		Topic:     topic,
		Timestamp: time.Now(),
		PeerID:    peerID,
	}
}

// WithError adds an error to the event and sets the type to EventTypeError.
func (e *GossipsubEvent) WithError(err error, operation string) *GossipsubEvent {
	e.EventType = EventTypeError
	e.Error = err
	e.Data = &ErrorEventData{
		Operation: operation,
	}

	return e
}

// WithMessageData adds message-specific data to the event.
func (e *GossipsubEvent) WithMessageData(messageID string, size int) *GossipsubEvent {
	if e.Data == nil {
		e.Data = &MessageEventData{}
	}

	if data, ok := e.Data.(*MessageEventData); ok {
		data.MessageID = messageID
		data.Size = size
	}

	return e
}

// WithValidationResult adds validation result to message event data.
func (e *GossipsubEvent) WithValidationResult(result ValidationResult) *GossipsubEvent {
	if e.Data == nil {
		e.Data = &MessageEventData{}
	}

	if data, ok := e.Data.(*MessageEventData); ok {
		data.ValidationResult = result
	}

	return e
}

// WithProcessingDuration adds processing duration to message event data.
func (e *GossipsubEvent) WithProcessingDuration(duration time.Duration) *GossipsubEvent {
	if e.Data == nil {
		e.Data = &MessageEventData{}
	}

	if data, ok := e.Data.(*MessageEventData); ok {
		data.ProcessingDuration = duration
	}

	return e
}
