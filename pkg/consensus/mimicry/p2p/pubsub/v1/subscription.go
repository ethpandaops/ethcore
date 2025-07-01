package v1

import (
	"context"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Subscription represents a subscription to a gossipsub topic.
// It provides a way to manage the lifecycle of the subscription.
type Subscription struct {
	// topic is the name of the subscribed topic
	topic string

	// cancel is the function to call to cancel the subscription
	cancel context.CancelFunc

	// mu protects the cancelled state
	mu sync.RWMutex

	// cancelled tracks whether the subscription has been cancelled
	cancelled bool
}

// Topic returns the topic name for this subscription.
func (s *Subscription) Topic() string {
	if s == nil {
		return ""
	}

	return s.topic
}

// Cancel cancels the subscription.
// It is safe to call Cancel multiple times.
func (s *Subscription) Cancel() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cancelled {
		s.cancelled = true
		if s.cancel != nil {
			s.cancel()
		}
	}
}

// IsCancelled returns whether the subscription has been cancelled.
func (s *Subscription) IsCancelled() bool {
	if s == nil {
		return true
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.cancelled
}

// SubnetSubscription manages multiple subnet subscriptions of the same type.
// It provides methods to dynamically add, remove, and query active subnet subscriptions.
type SubnetSubscription[T any] struct {
	// mu protects concurrent access to the subscriptions map
	mu sync.RWMutex

	// subscriptions maps subnet IDs to their active subscriptions
	subscriptions map[uint64]*Subscription

	// subnetTopic is the subnet topic configuration
	subnetTopic *SubnetTopic[T]
}

// NewSubnetSubscription creates a new subnet subscription manager.
func NewSubnetSubscription[T any](subnetTopic *SubnetTopic[T]) (*SubnetSubscription[T], error) {
	if subnetTopic == nil {
		return nil, fmt.Errorf("subnet topic cannot be nil")
	}

	return &SubnetSubscription[T]{
		subscriptions: make(map[uint64]*Subscription),
		subnetTopic:   subnetTopic,
	}, nil
}

// Add adds a subscription for a specific subnet.
// If a subscription for this subnet already exists, it will be replaced
// and the old subscription will be cancelled.
func (ss *SubnetSubscription[T]) Add(subnet uint64, subscription *Subscription) error {
	if subscription == nil {
		return fmt.Errorf("subscription cannot be nil")
	}

	if subnet >= ss.subnetTopic.MaxSubnets() {
		return fmt.Errorf("subnet %d exceeds maximum %d", subnet, ss.subnetTopic.MaxSubnets()-1)
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Cancel existing subscription if present
	if existing, exists := ss.subscriptions[subnet]; exists {
		existing.Cancel()
	}

	ss.subscriptions[subnet] = subscription

	return nil
}

// Remove removes and cancels the subscription for a specific subnet.
// Returns true if a subscription was removed, false if no subscription existed.
func (ss *SubnetSubscription[T]) Remove(subnet uint64) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	subscription, exists := ss.subscriptions[subnet]
	if !exists {
		return false
	}

	subscription.Cancel()
	delete(ss.subscriptions, subnet)

	return true
}

// Set replaces all subnet subscriptions with the provided map.
// All existing subscriptions not in the new map will be cancelled.
// The provided map will be copied to prevent external modifications.
func (ss *SubnetSubscription[T]) Set(subscriptions map[uint64]*Subscription) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Validate all subnet IDs
	for subnet := range subscriptions {
		if subnet >= ss.subnetTopic.MaxSubnets() {
			return fmt.Errorf("subnet %d exceeds maximum %d", subnet, ss.subnetTopic.MaxSubnets()-1)
		}
	}

	// Cancel all existing subscriptions that are not in the new set
	for subnet, sub := range ss.subscriptions {
		if _, exists := subscriptions[subnet]; !exists {
			sub.Cancel()
		}
	}

	// Create new map to prevent external modifications
	newSubs := make(map[uint64]*Subscription, len(subscriptions))

	for subnet, sub := range subscriptions {
		if sub != nil {
			newSubs[subnet] = sub
		}
	}

	ss.subscriptions = newSubs

	return nil
}

// Active returns a slice of currently active subnet IDs.
// The returned slice is a snapshot and safe to modify.
func (ss *SubnetSubscription[T]) Active() []uint64 {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	active := make([]uint64, 0, len(ss.subscriptions))

	for subnet, sub := range ss.subscriptions {
		if sub != nil && !sub.IsCancelled() {
			active = append(active, subnet)
		}
	}

	return active
}

// Get returns the subscription for a specific subnet, if it exists.
// Returns nil if no subscription exists for the subnet.
func (ss *SubnetSubscription[T]) Get(subnet uint64) *Subscription {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	return ss.subscriptions[subnet]
}

// Clear cancels and removes all subnet subscriptions.
func (ss *SubnetSubscription[T]) Clear() {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	for _, sub := range ss.subscriptions {
		if sub != nil {
			sub.Cancel()
		}
	}

	// Create new empty map
	ss.subscriptions = make(map[uint64]*Subscription)
}

// Count returns the number of active subnet subscriptions.
func (ss *SubnetSubscription[T]) Count() int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	count := 0

	for _, sub := range ss.subscriptions {
		if sub != nil && !sub.IsCancelled() {
			count++
		}
	}

	return count
}

// processor is an internal type that wraps a HandlerConfig with a libp2p subscription.
// This type is not exposed to users and is used internally by the gossipsub implementation.
// It handles the lifecycle of message processing including decoding, validation, and processing.
type processor[T any] struct {
	// handler contains the configuration for processing messages
	handler *HandlerConfig[T]

	// sub is the underlying libp2p subscription
	sub *pubsub.Subscription

	// topic is the topic this processor is subscribed to
	topic *Topic[T]

	// globalInvalidPayloadHandler is the global handler for invalid payloads
	globalInvalidPayloadHandler func(ctx context.Context, data []byte, err error, from peer.ID, topic string)

	// cancel is the function to cancel this processor
	cancel context.CancelFunc

	// wg is used to wait for the processor goroutine to finish
	wg sync.WaitGroup

	// metrics is the metrics instance for recording events
	metrics *Metrics
}

// newProcessor creates a new processor for handling messages on a topic.
func newProcessor[T any](ctx context.Context, topic *Topic[T], handler *HandlerConfig[T], sub *pubsub.Subscription, metrics *Metrics, globalInvalidPayloadHandler func(ctx context.Context, data []byte, err error, from peer.ID, topic string)) (*processor[T], context.Context, error) {
	if topic == nil {
		return nil, nil, fmt.Errorf("topic cannot be nil")
	}

	if handler == nil {
		return nil, nil, fmt.Errorf("handler cannot be nil")
	}

	if sub == nil {
		return nil, nil, fmt.Errorf("subscription cannot be nil")
	}

	// Create a cancellable context for this processor
	procCtx, cancel := context.WithCancel(ctx)

	p := &processor[T]{
		handler:                     handler,
		sub:                         sub,
		topic:                       topic,
		globalInvalidPayloadHandler: globalInvalidPayloadHandler,
		cancel:                      cancel,
		metrics:                     metrics,
	}

	return p, procCtx, nil
}

// start begins processing messages from the subscription.
func (p *processor[T]) start(ctx context.Context) {
	p.wg.Add(1)
	go p.run(ctx)
}

// stop cancels the processor and waits for it to finish.
func (p *processor[T]) stop() {
	p.cancel()
	p.wg.Wait()
	p.sub.Cancel()
}

// run is the main message processing loop.
func (p *processor[T]) run(ctx context.Context) {
	defer p.wg.Done()

	for {
		msg, err := p.sub.Next(ctx)
		if err != nil {
			// Context cancelled, normal shutdown
			if ctx.Err() != nil {
				return
			}
			// Log error and continue
			// Note: In production, this would use the logger passed to the gossipsub instance
			continue
		}

		// Process message in a separate goroutine to avoid blocking
		go p.processMessage(ctx, msg)
	}
}

// processMessage handles a single message.
func (p *processor[T]) processMessage(ctx context.Context, msg *pubsub.Message) {
	// Skip messages from self
	if msg.ReceivedFrom == peer.ID("") {
		return
	}

	topicName := p.topic.Name()

	// Record message received
	if p.metrics != nil {
		p.metrics.RecordMessageReceived(topicName)
	}

	// Decode the message
	decoder := p.handler.decoder
	if decoder == nil {
		decoder = p.topic.encoder.Decode
	}

	decoded, err := decoder(msg.Data)
	if err != nil {
		// Call topic-specific invalid payload handler if configured
		if p.handler.invalidPayloadHandler != nil {
			p.handler.invalidPayloadHandler(ctx, msg.Data, err, msg.ReceivedFrom)
		}

		// Call global invalid payload handler if configured
		if p.globalInvalidPayloadHandler != nil {
			p.globalInvalidPayloadHandler(ctx, msg.Data, err, msg.ReceivedFrom, topicName)
		}

		// Decoding error - ignore message after calling handlers
		return
	}

	// Validate the message if validator is configured
	if p.handler.validator != nil {
		startTime := time.Now()
		result := p.handler.validator(ctx, decoded, msg.ReceivedFrom)

		if p.metrics != nil {
			p.metrics.RecordValidationDuration(topicName, time.Since(startTime))
			p.metrics.RecordMessageValidated(topicName, result)
		}

		switch result {
		case ValidationReject, ValidationIgnore:
			// Don't process rejected or ignored messages
			return
		}
	}

	// Process the message if processor is configured
	if p.handler.processor != nil {
		startTime := time.Now()
		err := p.handler.processor(ctx, decoded, msg.ReceivedFrom)

		if p.metrics != nil {
			success := err == nil
			p.metrics.RecordMessageHandled(topicName, success, time.Since(startTime))

			if !success {
				p.metrics.RecordHandlerError(topicName)
			}
		}
	}

	// Publish events if configured
	if p.handler.events != nil {
		// In production, this would publish appropriate events
		// based on the processing results
	}
}
