package v1

import (
	"context"
	"fmt"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/compression"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	fastssz "github.com/prysmaticlabs/fastssz"
)

// Decoder is a function type that decodes bytes into a message of type T.
// It returns the decoded message and any error that occurred during decoding.
type Decoder[T any] func([]byte) (T, error)

// Validator is a function type that validates a decoded message.
// It receives the message, the sender's peer ID, and validation context.
// It returns a ValidationResult indicating whether to accept, reject, or ignore the message.
type Validator[T any] func(ctx context.Context, msg T, from peer.ID) ValidationResult

// Processor is a function type that processes a validated message.
// It performs the actual business logic after a message has been validated.
// Any errors returned will be logged but won't affect message propagation.
type Processor[T any] func(ctx context.Context, msg T, from peer.ID) error

// SubnetValidator is a function type that validates messages for specific subnets.
// It includes the subnet ID as an additional parameter for subnet-specific validation logic.
type SubnetValidator[T any] func(ctx context.Context, msg T, from peer.ID, subnet uint64) ValidationResult

// SubnetProcessor is a function type that processes messages for specific subnets.
// It includes the subnet ID as an additional parameter for subnet-specific processing logic.
type SubnetProcessor[T any] func(ctx context.Context, msg T, from peer.ID, subnet uint64) error

// InvalidPayloadHandler is a function type that handles invalid payload decoding errors.
// It receives the raw data that failed to decode, the decoding error, and the sender's peer ID.
// This allows for custom handling of malformed messages, such as logging, metrics, or peer scoring.
type InvalidPayloadHandler[T any] func(ctx context.Context, data []byte, err error, from peer.ID)

// HandlerConfig contains the configuration for message handling.
// It defines how messages of type T should be decoded, validated, and processed.
type HandlerConfig[T any] struct {
	// encoder is the encoder used for encoding/decoding messages.
	// If nil and decoder is also nil, an error will occur during registration.
	encoder Encoder[T]

	// Compressor is the optional compressor used for message compression/decompression.
	// If nil, no compression is applied.
	Compressor compression.Compressor

	// decoder is the function used to decode raw bytes into messages of type T.
	// If nil, the encoder.Decode method will be used.
	decoder Decoder[T]

	// validator is the function used to validate decoded messages.
	// It determines whether messages should be accepted, rejected, or ignored.
	validator Validator[T]

	// processor is the function used to process validated messages.
	// This is where the actual business logic for handling messages is implemented.
	processor Processor[T]

	// invalidPayloadHandler is called when decoding fails for a message.
	// This allows for custom handling of malformed payloads (logging, metrics, etc.).
	invalidPayloadHandler InvalidPayloadHandler[T]

	// scoreParams defines the topic scoring parameters for gossipsub.
	// These parameters affect how peers are scored based on their behavior on this topic.
	scoreParams *pubsub.TopicScoreParams

	// events is an optional event channel for publishing handler events.
	// If provided, significant events (validations, processing, errors) will be published here.
	events chan<- Event
}

// Event represents an event that occurs during message handling.
// This is a placeholder type that should be defined based on the actual event system.
type Event interface {
	// Type returns the event type
	Type() string
}

// ValidationResult represents the outcome of message validation.
type ValidationResult int

const (
	// ValidationAccept indicates the message should be accepted and propagated.
	ValidationAccept ValidationResult = iota
	// ValidationReject indicates the message should be rejected and not propagated.
	ValidationReject
	// ValidationIgnore indicates the message should be ignored (not propagated, but not rejected).
	ValidationIgnore
)

// HandlerOption is a functional option for configuring a HandlerConfig.
type HandlerOption[T any] func(*HandlerConfig[T])

// WithDecoder sets a custom decoder function for the handler.
// If not set, the topic's encoder.Decode method will be used.
func WithDecoder[T any](decoder Decoder[T]) HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		h.decoder = decoder
	}
}

// WithValidator sets the validator function for the handler.
// The validator determines whether messages should be accepted, rejected, or ignored.
func WithValidator[T any](validator Validator[T]) HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		h.validator = validator
	}
}

// WithProcessor sets the processor function for the handler.
// The processor contains the business logic for handling validated messages.
func WithProcessor[T any](processor Processor[T]) HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		h.processor = processor
	}
}

// WithSSZDecoding configures the handler to use SSZ decoding for messages of type T.
// This requires T to implement the fastssz.Unmarshaler interface.
func WithSSZDecoding[T any]() HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		h.decoder = func(data []byte) (T, error) {
			var msg T
			// Try to create a new instance and check if it implements Unmarshaler
			msgPtr := new(T)
			if unmarshaler, ok := any(msgPtr).(fastssz.Unmarshaler); ok {
				if err := unmarshaler.UnmarshalSSZ(data); err != nil {
					return msg, fmt.Errorf("failed to unmarshal SSZ: %w", err)
				}
				return *msgPtr, nil
			}
			return msg, fmt.Errorf("type %T does not implement fastssz.Unmarshaler", msg)
		}
	}
}

// WithScoreParams sets the topic scoring parameters for gossipsub.
// These parameters affect how peers are scored based on their behavior on this topic.
func WithScoreParams[T any](params *pubsub.TopicScoreParams) HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		h.scoreParams = params
	}
}

// WithInvalidPayloadHandler sets the invalid payload handler for the handler.
// This handler will be called when decoding fails, allowing for custom error handling.
func WithInvalidPayloadHandler[T any](handler InvalidPayloadHandler[T]) HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		h.invalidPayloadHandler = handler
	}
}

// WithEvents sets the event channel for publishing handler events.
// Events will be published for significant occurrences during message handling.
func WithEvents[T any](events chan<- Event) HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		h.events = events
	}
}

// WithCompressor sets the compressor for the handler.
// If set, messages will be decompressed before decoding and compressed after encoding.
func WithCompressor[T any](compressor compression.Compressor) HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		h.Compressor = compressor
	}
}

// WithEncoder sets the encoder for the handler.
// The encoder is used for both encoding and decoding messages unless a custom decoder is provided.
func WithEncoder[T any](encoder Encoder[T]) HandlerOption[T] {
	return func(h *HandlerConfig[T]) {
		h.encoder = encoder
	}
}

// NewHandlerConfig creates a new HandlerConfig with the provided options.
func NewHandlerConfig[T any](opts ...HandlerOption[T]) *HandlerConfig[T] {
	h := &HandlerConfig[T]{}
	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Validate checks if the handler configuration is valid.
func (h *HandlerConfig[T]) Validate() error {
	// At minimum, we need either a validator or processor to do something useful
	if h.validator == nil && h.processor == nil {
		return ErrNoHandler
	}

	// We need either an encoder or decoder to decode messages
	if h.encoder == nil && h.decoder == nil {
		return fmt.Errorf("either encoder or decoder must be configured")
	}

	return nil
}

// ErrNoHandler is returned when no validator or processor is configured.
var ErrNoHandler = &handlerError{msg: "no validator or processor configured"}

// handlerError represents an error in handler configuration or operation.
type handlerError struct {
	msg string
}

func (e *handlerError) Error() string {
	return e.msg
}
