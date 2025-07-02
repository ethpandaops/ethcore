// Package v1 provides a type-safe gossipsub implementation for Ethereum consensus layer p2p communication.
//
// This file contains the core Gossipsub struct and its lifecycle management methods.
// The Gossipsub struct wraps libp2p's pubsub implementation with additional features
// like metrics, validation concurrency control, and type-safe topic handling.
package v1

import (
	"context"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// Option is a functional option for configuring Gossipsub.
type Option func(*Gossipsub) error

// WithLogger sets a custom logger for the Gossipsub instance.
func WithLogger(log logrus.FieldLogger) Option {
	return func(g *Gossipsub) error {
		if log == nil {
			return fmt.Errorf("logger cannot be nil")
		}

		g.log = log.WithField("component", "ethcore/gossipsub-v1")

		return nil
	}
}

// WithPublishTimeout sets the timeout for publishing messages.
func WithPublishTimeout(timeout time.Duration) Option {
	return func(g *Gossipsub) error {
		if timeout <= 0 {
			return fmt.Errorf("publish timeout must be positive, got %v", timeout)
		}
		g.publishTimeout = timeout
		return nil
	}
}

// WithGossipSubParams sets custom gossipsub protocol parameters.
func WithGossipSubParams(params pubsub.GossipSubParams) Option {
	return func(g *Gossipsub) error {
		g.gossipSubParams = params
		return nil
	}
}

// WithMetrics sets the metrics instance for the Gossipsub.
func WithMetrics(metrics *Metrics) Option {
	return func(g *Gossipsub) error {
		if metrics == nil {
			return fmt.Errorf("metrics cannot be nil")
		}

		g.metrics = metrics

		return nil
	}
}

// WithGlobalInvalidPayloadHandler sets a global handler for invalid payload errors across all topics.
// This handler will be called for any decoding failures that occur, in addition to any topic-specific handlers.
func WithGlobalInvalidPayloadHandler(handler func(ctx context.Context, data []byte, err error, from peer.ID, topic string)) Option {
	return func(g *Gossipsub) error {
		g.globalInvalidPayloadHandler = handler

		return nil
	}
}

// WithPubsubOptions sets additional libp2p pubsub options.
// These options will be appended to the default options when creating the pubsub instance.
func WithPubsubOptions(opts ...pubsub.Option) Option {
	return func(g *Gossipsub) error {
		g.pubsubOpts = append(g.pubsubOpts, opts...)
		return nil
	}
}

// Gossipsub provides a type-safe gossipsub implementation with support for
// regular topics and subnet-based topics.
type Gossipsub struct {
	// Dependencies
	log    logrus.FieldLogger
	host   host.Host
	pubsub *pubsub.PubSub
	cancel context.CancelFunc

	// Configuration
	publishTimeout  time.Duration
	gossipSubParams pubsub.GossipSubParams

	// Pubsub options
	pubsubOpts []pubsub.Option

	// Handler registry
	registry *Registry

	// Active subscriptions - maps topic name to subscription
	subscriptions map[string]*Subscription
	subMutex      sync.RWMutex

	// Active processors - maps topic name to processor
	processors map[string]*processor[any]
	procMutex  sync.RWMutex

	// Metrics
	metrics *Metrics

	// Global invalid payload handler
	globalInvalidPayloadHandler func(ctx context.Context, data []byte, err error, from peer.ID, topic string)

	// Lifecycle
	started bool
	startMu sync.Mutex
	wg      sync.WaitGroup
}

// New creates a new Gossipsub instance with the given host and options.
func New(ctx context.Context, host host.Host, opts ...Option) (*Gossipsub, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if host == nil {
		return nil, fmt.Errorf("host cannot be nil")
	}

	// Create context for this gossipsub instance
	gossipCtx, cancel := context.WithCancel(ctx)

	g := &Gossipsub{
		log:            logrus.StandardLogger().WithField("component", "gossipsub-v1"),
		host:           host,
		cancel:         cancel,
		publishTimeout: 5 * time.Second,
		registry:       NewRegistry(),
		subscriptions:  make(map[string]*Subscription),
		processors:     make(map[string]*processor[any]),
		pubsubOpts:     []pubsub.Option{}, // Start with empty options
	}

	// Apply options (which may add more pubsub options)
	for _, opt := range opts {
		if err := opt(g); err != nil {
			cancel()
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Initialize libp2p pubsub
	if err := g.initializePubSub(gossipCtx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize pubsub: %w", err)
	}

	g.started = true

	g.log.Info("Gossipsub started")

	return g, nil
}

// initializePubSub creates the underlying libp2p pubsub instance.
func (g *Gossipsub) initializePubSub(ctx context.Context) error {
	// Add custom gossipsub params if provided
	// Note: We always have gossipSubParams initialized, check if it's not the zero value
	if g.gossipSubParams.D > 0 {
		g.pubsubOpts = append(g.pubsubOpts, pubsub.WithGossipSubParams(g.gossipSubParams))
	}

	g.log.Info("Creating pubsub")

	ps, err := pubsub.NewGossipSub(ctx, g.host, g.pubsubOpts...)
	if err != nil {
		return fmt.Errorf("failed to create gossipsub: %w", err)
	}

	g.pubsub = ps
	return nil
}

// Register registers a handler for a specific topic.
func (g *Gossipsub) Register(topic *Topic[any], handler *HandlerConfig[any]) error {
	if !g.started {
		return fmt.Errorf("gossipsub not started")
	}

	g.log.WithField("topic", topic.Name()).Info("Registering topic")

	return Register(g.registry, topic, handler)
}

// RegisterSubnet registers a handler for a subnet topic pattern.
func (g *Gossipsub) RegisterSubnet(subnetTopic *SubnetTopic[any], handler *HandlerConfig[any]) error {
	if !g.started {
		return fmt.Errorf("gossipsub not started")
	}

	g.log.WithField("topics", subnetTopic.pattern).Info("Registering subnet topic")

	return RegisterSubnet(g.registry, subnetTopic, handler)
}

// Subscribe subscribes to a specific topic and returns a subscription handle.
func Subscribe[T any](ctx context.Context, g *Gossipsub, topic *Topic[T]) (*Subscription, error) {
	if !g.started {
		return nil, fmt.Errorf("gossipsub not started")
	}

	g.log.WithField("topic", topic.Name()).Info("Subscribing to topic")

	topicName := topic.Name()

	// Check if handler is registered
	handler := g.registry.getHandler(topicName)
	if handler == nil {
		return nil, fmt.Errorf("no handler registered for topic %s", topicName)
	}

	g.subMutex.Lock()
	defer g.subMutex.Unlock()

	// Check if already subscribed
	if sub, exists := g.subscriptions[topicName]; exists && !sub.IsCancelled() {
		return nil, fmt.Errorf("already subscribed to topic %s", topicName)
	}

	// Create anyTopic early for validator registration
	anyTopic := &Topic[any]{
		name:    topic.name,
		encoder: wrapEncoder(topic.encoder),
	}

	// Register validator with libp2p if handler has one
	if handler.validator != nil {
		// Create a wrapper that converts our validator to libp2p's validator format
		libp2pValidator := g.createLibp2pValidator(anyTopic, handler)
		if err := g.pubsub.RegisterTopicValidator(topicName, libp2pValidator); err != nil {
			return nil, fmt.Errorf("failed to register validator for topic %s: %w", topicName, err)
		}
	}

	// Join the topic first
	topicHandle, err := g.pubsub.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic %s: %w", topicName, err)
	}

	// Apply score parameters if configured
	if handler.scoreParams != nil {
		if err := topicHandle.SetScoreParams(handler.scoreParams); err != nil {
			return nil, fmt.Errorf("failed to set score params for topic %s: %w", topicName, err)
		}
	}

	// Subscribe to the topic
	libp2pSub, err := topicHandle.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to topic %s: %w", topicName, err)
	}

	// Create processor
	proc, procCtx, err := g.createProcessor(ctx, anyTopic, handler, libp2pSub)
	if err != nil {
		libp2pSub.Cancel()
		return nil, fmt.Errorf("failed to create processor: %w", err)
	}

	// Create subscription
	subCtx, cancel := context.WithCancel(ctx)

	sub := &Subscription{
		topic:  topicName,
		cancel: cancel,
	}

	// Store subscription and processor
	g.subscriptions[topicName] = sub
	g.processors[topicName] = proc

	// Update metrics
	if g.metrics != nil {
		g.metrics.SetActiveSubscriptions(len(g.subscriptions))
	}

	// Start processor in background
	g.wg.Add(1)

	go func() {
		defer g.wg.Done()
		<-subCtx.Done()
		g.removeProcessor(topicName)
	}()

	// Start the processor
	proc.start(procCtx)

	g.log.WithFields(logrus.Fields{
		"topic":         topicName,
		"has_validator": handler.validator != nil,
	}).Info("Subscribed to topic")

	return sub, nil
}

// SubscribeSubnet subscribes to a specific subnet of a subnet topic.
func SubscribeSubnet[T any](ctx context.Context, g *Gossipsub, subnetTopic *SubnetTopic[T], subnet uint64, forkDigest [4]byte) (*Subscription, error) {
	if !g.started {
		return nil, fmt.Errorf("gossipsub not started")
	}

	// Get the specific topic for this subnet
	topic, err := subnetTopic.TopicForSubnet(subnet, forkDigest)
	if err != nil {
		return nil, fmt.Errorf("failed to create subnet topic: %w", err)
	}

	// Subscribe using the regular Subscribe function
	return Subscribe(ctx, g, topic)
}

// CreateSubnetSubscription creates a new subnet subscription manager.
func CreateSubnetSubscription[T any](g *Gossipsub, subnetTopic *SubnetTopic[T]) (*SubnetSubscription[T], error) {
	if !g.started {
		return nil, fmt.Errorf("gossipsub not started")
	}

	return NewSubnetSubscription(subnetTopic)
}

// Publish publishes a message to a topic.
func Publish[T any](g *Gossipsub, topic *Topic[T], msg T) error {
	if !g.started {
		return fmt.Errorf("gossipsub not started")
	}

	topicName := topic.Name()
	startTime := time.Now()

	// Encode the message
	data, err := topic.encoder.Encode(msg)
	if err != nil {
		if g.metrics != nil {
			g.metrics.RecordPublishError(topicName)
		}
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// Publish to the topic
	if err := g.pubsub.Publish(topicName, data); err != nil {
		if g.metrics != nil {
			g.metrics.RecordPublishError(topicName)
			g.metrics.RecordMessagePublished(topicName, false)
		}
		return fmt.Errorf("failed to publish to topic %s: %w", topicName, err)
	}

	// Record metrics
	if g.metrics != nil {
		g.metrics.RecordPublishDuration(topicName, time.Since(startTime))
		g.metrics.RecordMessagePublished(topicName, true)
	}

	g.log.WithFields(logrus.Fields{
		"topic": topicName,
		"size":  len(data),
	}).Debug("Published message")

	return nil
}

// createProcessor creates a new processor for a topic.
func (g *Gossipsub) createProcessor(ctx context.Context, topic *Topic[any], handler *HandlerConfig[any], sub *pubsub.Subscription) (*processor[any], context.Context, error) {
	g.log.WithField("topic", topic.Name()).Debug("Creating processor")

	proc, procCtx, err := newProcessor(ctx, topic, handler, sub, g.metrics, g.globalInvalidPayloadHandler)
	if err != nil {
		return nil, nil, err
	}

	g.procMutex.Lock()
	g.processors[topic.Name()] = proc
	g.procMutex.Unlock()

	return proc, procCtx, nil
}

// removeProcessor removes a processor for a topic.
func (g *Gossipsub) removeProcessor(topicName string) {
	g.procMutex.Lock()
	proc, exists := g.processors[topicName]
	if exists {
		delete(g.processors, topicName)
	}
	g.procMutex.Unlock()

	if exists && proc != nil {
		proc.stop()
	}

	// Unregister the topic validator
	if err := g.pubsub.UnregisterTopicValidator(topicName); err != nil {
		g.log.WithError(err).WithField("topic", topicName).Warn("Failed to unregister topic validator")
	}

	// Remove subscription
	g.subMutex.Lock()
	delete(g.subscriptions, topicName)
	subCount := len(g.subscriptions)
	g.subMutex.Unlock()

	// Update metrics
	if g.metrics != nil {
		g.metrics.SetActiveSubscriptions(subCount)
	}

	g.log.WithField("topic", topicName).Debug("Removed processor and subscription")
}

// Registry returns the handler registry for this gossipsub instance.
func (g *Gossipsub) Registry() *Registry {
	return g.registry
}

// Stop gracefully shuts down the gossipsub instance.
func (g *Gossipsub) Stop() error {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	if !g.started {
		return fmt.Errorf("gossipsub not started")
	}

	g.log.Info("Stopping gossipsub")

	// Cancel context to signal shutdown
	g.cancel()

	// Cancel all subscriptions
	g.subMutex.Lock()
	for _, sub := range g.subscriptions {
		sub.Cancel()
	}
	g.subMutex.Unlock()

	// Stop all processors
	g.procMutex.Lock()
	for _, proc := range g.processors {
		proc.stop()
	}
	g.procMutex.Unlock()

	// Wait for all goroutines to finish
	g.wg.Wait()

	// Clear maps
	g.subscriptions = make(map[string]*Subscription)
	g.processors = make(map[string]*processor[any])

	g.started = false
	g.log.Info("Gossipsub stopped")

	return nil
}

// createLibp2pValidator creates a libp2p validator function from our handler's validator.
// This allows our validation logic to run at the libp2p level before messages are propagated.
func (g *Gossipsub) createLibp2pValidator(topic *Topic[any], handler *HandlerConfig[any]) pubsub.ValidatorEx {
	g.log.WithField("topic", topic.Name()).Info("Creating libp2p gossipsub validator")

	return func(ctx context.Context, pid peer.ID, msg *pubsub.Message) (result pubsub.ValidationResult) {
		defer func() {
			g.log.WithFields(logrus.Fields{
				"topic":  topic.Name(),
				"pid":    pid,
				"result": result,
			}).Info("Validated message")
		}()
		// Record validation start
		if g.metrics != nil {
			g.metrics.RecordMessageReceived(topic.Name())
		}

		// Decode the message
		decoder := handler.decoder
		if decoder == nil {
			decoder = topic.encoder.Decode
		}

		decoded, err := decoder(msg.Data)
		if err != nil {
			// Call invalid payload handler if configured
			if handler.invalidPayloadHandler != nil {
				handler.invalidPayloadHandler(ctx, msg.Data, err, pid)
			}

			// Call global invalid payload handler if configured
			if g.globalInvalidPayloadHandler != nil {
				g.globalInvalidPayloadHandler(ctx, msg.Data, err, pid, topic.Name())
			}

			// Reject messages that can't be decoded
			if g.metrics != nil {
				g.metrics.RecordMessageValidated(topic.Name(), ValidationReject)
			}

			return pubsub.ValidationReject
		}

		// Run the validator
		startTime := time.Now()
		res := handler.validator(ctx, decoded, pid)

		// Record validation metrics
		if g.metrics != nil {
			g.metrics.RecordValidationDuration(topic.Name(), time.Since(startTime))
			g.metrics.RecordMessageValidated(topic.Name(), res)
		}

		// Convert our validation result to libp2p's format
		switch res {
		case ValidationAccept:
			return pubsub.ValidationAccept
		case ValidationReject:
			return pubsub.ValidationReject
		case ValidationIgnore:
			return pubsub.ValidationIgnore
		default:
			// Default to reject for unknown results
			return pubsub.ValidationReject
		}
	}
}

// wrapEncoder wraps a typed encoder to work with any type.
func wrapEncoder[T any](encoder Encoder[T]) Encoder[any] {
	return &anyEncoder[T]{typed: encoder}
}

// anyEncoder wraps a typed encoder to work with any type.
type anyEncoder[T any] struct {
	typed Encoder[T]
}

func (e *anyEncoder[T]) Encode(msg any) ([]byte, error) {
	typedMsg, ok := msg.(T)
	if !ok {
		return nil, fmt.Errorf("encoder type assertion failed: expected %T, got %T", *new(T), msg)
	}
	return e.typed.Encode(typedMsg)
}

func (e *anyEncoder[T]) Decode(data []byte) (any, error) {
	return e.typed.Decode(data)
}

// GetHost returns the underlying libp2p host.
func (g *Gossipsub) GetHost() host.Host {
	return g.host
}

// GetPubSub returns the underlying libp2p pubsub instance.
// This is useful for advanced use cases that need direct access.
func (g *Gossipsub) GetPubSub() *pubsub.PubSub {
	return g.pubsub
}

// PeerID returns the peer ID of the host.
func (g *Gossipsub) PeerID() peer.ID {
	return g.host.ID()
}

// IsStarted returns whether the gossipsub instance is started.
func (g *Gossipsub) IsStarted() bool {
	g.startMu.Lock()
	defer g.startMu.Unlock()
	return g.started
}

// TopicCount returns the number of active topic subscriptions.
func (g *Gossipsub) TopicCount() int {
	g.subMutex.RLock()
	defer g.subMutex.RUnlock()
	return len(g.subscriptions)
}

// ActiveTopics returns a list of currently subscribed topics.
func (g *Gossipsub) ActiveTopics() []string {
	g.subMutex.RLock()
	defer g.subMutex.RUnlock()

	topics := make([]string, 0, len(g.subscriptions))
	for topic := range g.subscriptions {
		topics = append(topics, topic)
	}
	return topics
}
