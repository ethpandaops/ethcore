package v1

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Registry stores and manages topic handlers for the gossipsub system.
// It provides thread-safe registration and lookup of handlers for both
// regular topics and subnet-based topics.
type Registry struct {
	// mu protects concurrent access to the handlers maps
	mu sync.RWMutex

	// handlers maps topic names to their handler configurations
	handlers map[string]*HandlerConfig[any]

	// subnetHandlers maps subnet topic patterns to their handler configurations
	// The key is the base pattern (e.g., "beacon_attestation_%d")
	subnetHandlers map[string]*HandlerConfig[any]
}

// NewRegistry creates a new handler registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers:       make(map[string]*HandlerConfig[any]),
		subnetHandlers: make(map[string]*HandlerConfig[any]),
	}
}

// Register registers a handler for a specific topic.
// The handler will be used to decode, validate, and process messages on this topic.
// Returns an error if the topic is already registered or the handler is invalid.
func Register[T any](r *Registry, topic *Topic[T], handler *HandlerConfig[T]) error {
	if r == nil {
		return fmt.Errorf("registry is nil")
	}
	if topic == nil {
		return fmt.Errorf("topic is nil")
	}
	if handler == nil {
		return fmt.Errorf("handler is nil")
	}

	// Validate the handler configuration
	if err := handler.Validate(); err != nil {
		return fmt.Errorf("invalid handler configuration: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	topicName := topic.Name()
	if _, exists := r.handlers[topicName]; exists {
		return fmt.Errorf("topic '%s' is already registered", topicName)
	}

	// Store the handler as HandlerConfig[any] for type erasure
	// We need to create a new HandlerConfig[any] that wraps the typed handler
	r.handlers[topicName] = &HandlerConfig[any]{
		encoder:     wrapEncoder(handler.encoder),
		Compressor:  handler.Compressor,
		decoder:     wrapDecoder(handler.decoder, handler.encoder),
		validator:   wrapValidator(handler.validator),
		processor:   wrapProcessor(handler.processor),
		scoreParams: handler.scoreParams,
		events:      handler.events,
	}
	return nil
}

// RegisterSubnet registers a handler for a subnet topic pattern.
// The handler will be used for all subnets matching the pattern.
// Returns an error if the pattern is already registered or the handler is invalid.
func RegisterSubnet[T any](r *Registry, subnetTopic *SubnetTopic[T], handler *HandlerConfig[T]) error {
	if r == nil {
		return fmt.Errorf("registry is nil")
	}
	if subnetTopic == nil {
		return fmt.Errorf("subnet topic is nil")
	}
	if handler == nil {
		return fmt.Errorf("handler is nil")
	}

	// Validate the handler configuration
	if err := handler.Validate(); err != nil {
		return fmt.Errorf("invalid handler configuration: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Extract the base pattern from the subnet topic
	pattern := subnetTopic.pattern
	if _, exists := r.subnetHandlers[pattern]; exists {
		return fmt.Errorf("subnet pattern '%s' is already registered", pattern)
	}

	// Store the handler as HandlerConfig[any] for type erasure
	// We need to create a new HandlerConfig[any] that wraps the typed handler
	r.subnetHandlers[pattern] = &HandlerConfig[any]{
		encoder:     wrapEncoder(handler.encoder),
		Compressor:  handler.Compressor,
		decoder:     wrapDecoder(handler.decoder, handler.encoder),
		validator:   wrapValidator(handler.validator),
		processor:   wrapProcessor(handler.processor),
		scoreParams: handler.scoreParams,
		events:      handler.events,
	}
	return nil
}

// GetHandler retrieves a handler for the given topic name.
// It first checks exact topic matches, then checks subnet patterns.
// Returns nil if no handler is found.
func (r *Registry) GetHandler(topicName string) *HandlerConfig[any] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// First check for exact topic match
	if handler, ok := r.handlers[topicName]; ok {
		return handler
	}

	// Check subnet handlers by trying to parse the topic
	// This is less efficient but allows flexible subnet matching
	for pattern, handler := range r.subnetHandlers {
		// Create a temporary subnet topic to use its parsing logic
		// We use a dummy encoder here since we only need the parsing functionality
		tempSubnet := &SubnetTopic[any]{
			pattern:    pattern,
			maxSubnets: ^uint64(0), // Max uint64 for parsing purposes
		}

		// Try to parse the subnet from the topic name
		if _, err := tempSubnet.ParseSubnet(topicName); err == nil {
			return handler
		}
	}

	return nil
}

// HasHandler checks if a handler exists for the given topic name.
// Returns true if a handler is registered for the topic or if it matches a subnet pattern.
func (r *Registry) HasHandler(topicName string) bool {
	return r.GetHandler(topicName) != nil
}

// Clear removes all registered handlers from the registry.
// This is primarily useful for testing and cleanup scenarios.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear all handlers
	clear(r.handlers)
	clear(r.subnetHandlers)
}

// TopicCount returns the number of registered regular topics.
func (r *Registry) TopicCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}

// SubnetPatternCount returns the number of registered subnet patterns.
func (r *Registry) SubnetPatternCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.subnetHandlers)
}

// wrapDecoder wraps a typed decoder to work with any type.
// If the decoder is nil, it falls back to the encoder's Decode method.
func wrapDecoder[T any](decoder Decoder[T], encoder Encoder[T]) Decoder[any] {
	if decoder == nil && encoder != nil {
		// Use the encoder's decode method if no custom decoder is provided
		return func(data []byte) (any, error) {
			return encoder.Decode(data)
		}
	}
	if decoder == nil {
		return nil
	}
	return func(data []byte) (any, error) {
		return decoder(data)
	}
}

// wrapValidator wraps a typed validator to work with any type.
func wrapValidator[T any](validator Validator[T]) Validator[any] {
	if validator == nil {
		return nil
	}
	return func(ctx context.Context, msg any, from peer.ID) ValidationResult {
		// Type assert the message to the expected type
		typedMsg, ok := msg.(T)
		if !ok {
			// If type assertion fails, reject the message
			return ValidationReject
		}
		return validator(ctx, typedMsg, from)
	}
}

// wrapProcessor wraps a typed processor to work with any type.
func wrapProcessor[T any](processor Processor[T]) Processor[any] {
	if processor == nil {
		return nil
	}
	return func(ctx context.Context, msg any, from peer.ID) error {
		// Type assert the message to the expected type
		typedMsg, ok := msg.(T)
		if !ok {
			return fmt.Errorf("processor type assertion failed: expected %T, got %T", *new(T), msg)
		}
		return processor(ctx, typedMsg, from)
	}
}
