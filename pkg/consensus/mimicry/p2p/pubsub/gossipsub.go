package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chuckpreslar/emission"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// multiProcessorSubscription tracks the subscription state for a MultiProcessor
type multiProcessorSubscription struct {
	multiProcessor any // the MultiProcessor instance
	activeTopics   map[string]bool // currently subscribed topics
	mutex          sync.RWMutex
}

// Gossipsub provides a processor-based gossipsub implementation.
type Gossipsub struct {
	log     logrus.FieldLogger
	host    host.Host
	pubsub  *pubsub.PubSub
	config  *Config
	emitter *emission.Emitter
	metrics *Metrics

	// Topic and subscription management
	topicManager *topicManager

	// Processor subscriptions
	processorSubs map[string]any // stores ProcessorSubscription[T]
	procMutex     sync.RWMutex

	// MultiProcessor subscriptions tracking
	multiProcSubs map[any]*multiProcessorSubscription // maps multiProcessor to its subscription state
	multiProcMutex sync.RWMutex

	// No processor registry - processors are provided at subscribe time

	// Compatibility tracking for old API (for tests)
	registeredProcessors     map[string]any // topic -> processor (for RegisterProcessor)
	registeredMultiProcessors map[string]any // name -> multiProcessor (for RegisterMultiProcessor)
	registryMutex            sync.RWMutex

	// Lifecycle management
	cancel  context.CancelFunc
	started bool
	startMu sync.Mutex
}

// NewGossipsub creates a new Gossipsub instance with the given configuration.
func NewGossipsub(log logrus.FieldLogger, host host.Host, config *Config) (*Gossipsub, error) {
	if log == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if host == nil {
		return nil, fmt.Errorf("host cannot be nil")
	}

	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	logger := log.WithField("component", "gossipsub")
	emitter := emission.NewEmitter()
	metrics := NewMetrics("gossipsub")

	g := &Gossipsub{
		log:                       logger,
		host:                      host,
		config:                    config,
		emitter:                   emitter,
		metrics:                   metrics,
		processorSubs:             make(map[string]any),
		multiProcSubs:             make(map[any]*multiProcessorSubscription),
		registeredProcessors:      make(map[string]any),
		registeredMultiProcessors: make(map[string]any),
	}

	return g, nil
}

// Start initializes and starts the gossipsub service.
func (g *Gossipsub) Start(ctx context.Context) error {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	if g.started {
		return NewError(ErrAlreadyStarted, "start")
	}

	g.log.Info("Starting gossipsub service")

	// Create cancel context for the gossipsub instance
	var pubsubCtx context.Context
	pubsubCtx, g.cancel = context.WithCancel(ctx)

	// Initialize libp2p pubsub
	if err := g.initializePubsub(pubsubCtx); err != nil {
		g.cancel()

		return NewError(err, "pubsub initialization")
	}

	// Initialize topic manager
	g.topicManager = newTopicManager(g.log, g.pubsub)

	g.started = true
	g.log.Info("Gossipsub service started successfully")
	g.emitPubsubStarted()

	return nil
}

// Stop gracefully shuts down the gossipsub service.
func (g *Gossipsub) Stop() error {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	if !g.started {
		return NewError(ErrNotStarted, "stop")
	}

	g.log.Info("Stopping gossipsub service")

	// Close all processor subscriptions
	g.procMutex.Lock()
	for topic, procSubInterface := range g.processorSubs {
		// Use type assertion to call Stop() method
		if stopper, ok := procSubInterface.(interface{ Stop() error }); ok {
			if err := stopper.Stop(); err != nil {
				g.log.WithError(err).WithField("topic", topic).Warn("Error stopping processor subscription")
			}
		}
	}

	g.processorSubs = make(map[string]any)
	g.procMutex.Unlock()

	// Clear all MultiProcessor subscriptions
	g.multiProcMutex.Lock()
	g.multiProcSubs = make(map[any]*multiProcessorSubscription)
	g.multiProcMutex.Unlock()

	// Close topic manager
	if g.topicManager != nil {
		if err := g.topicManager.close(); err != nil {
			g.log.WithError(err).Warn("Error closing topic manager")
		}
	}

	// Cancel context and wait for goroutines
	if g.cancel != nil {
		g.cancel()
	}

	g.started = false
	g.log.Info("Gossipsub service stopped")
	g.emitPubsubStopped()

	return nil
}

// IsStarted returns whether the gossipsub service is currently running.
func (g *Gossipsub) IsStarted() bool {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	return g.started
}

// GetStats returns current runtime statistics.
func (g *Gossipsub) GetStats() *Stats {
	if !g.IsStarted() {
		return &Stats{}
	}

	g.procMutex.RLock()
	activeSubscriptions := len(g.processorSubs)
	g.procMutex.RUnlock()

	var connectedPeers int
	if g.host != nil {
		connectedPeers = len(g.host.Network().Peers())
	}

	topicCount := 0
	if g.topicManager != nil {
		topicCount = len(g.topicManager.getTopicNames())
	}

	return &Stats{
		ActiveSubscriptions: activeSubscriptions,
		ConnectedPeers:      connectedPeers,
		TopicCount:          topicCount,
	}
}

// initializePubsub creates and configures the libp2p pubsub instance.
func (g *Gossipsub) initializePubsub(ctx context.Context) error {
	options := g.createPubsubOptions()

	ps, err := pubsub.NewGossipSub(ctx, g.host, options...)
	if err != nil {
		return fmt.Errorf("failed to create gossipsub: %w", err)
	}

	g.pubsub = ps

	return nil
}

// createPubsubOptions builds libp2p pubsub options from the configuration.
func (g *Gossipsub) createPubsubOptions() []pubsub.Option {
	options := []pubsub.Option{
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
		pubsub.WithMaxMessageSize(g.config.MaxMessageSize),
		pubsub.WithValidateQueueSize(g.config.ValidationBufferSize),
		pubsub.WithValidateWorkers(g.config.ValidationConcurrency),
	}

	// Apply custom gossipsub parameters if specified, otherwise use libp2p defaults
	if g.config.GossipSubParams != nil {
		options = append(options, pubsub.WithGossipSubParams(*g.config.GossipSubParams))
	}

	// Enable peer scoring with default parameters
	// Topic-specific scoring is configured from stored processor parameters
	topicScores := g.buildTopicScoreMap()
	options = append(options, pubsub.WithPeerScore(
		&pubsub.PeerScoreParams{
			AppSpecificScore: func(p peer.ID) float64 {
				// Default to 0, can be overridden by application
				return 0
			},
			AppSpecificWeight:           1.0,
			IPColocationFactorWeight:    -1.0,
			IPColocationFactorThreshold: 5.0,
			BehaviourPenaltyWeight:      -1.0,
			BehaviourPenaltyThreshold:   6.0,
			BehaviourPenaltyDecay:       0.99,
			DecayInterval:               time.Second,
			DecayToZero:                 0.01,
			RetainScore:                 time.Minute * 10,
			Topics:                      topicScores,
		},
		&pubsub.PeerScoreThresholds{
			GossipThreshold:             -500,
			PublishThreshold:            -1000,
			GraylistThreshold:           -2500,
			AcceptPXThreshold:           0,
			OpportunisticGraftThreshold: 2,
		},
	))

	return options
}

// buildTopicScoreMap returns empty map - topic scoring is configured per subscription.
func (g *Gossipsub) buildTopicScoreMap() map[string]*pubsub.TopicScoreParams {
	return make(map[string]*pubsub.TopicScoreParams)
}


// SubscribeWithProcessor subscribes to a topic using a typed processor.
func SubscribeWithProcessor[T any](g *Gossipsub, ctx context.Context, processor Processor[T]) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, processor.Topic(), "subscribe")
	}

	if processor == nil {
		return NewTopicError(fmt.Errorf("processor cannot be nil"), "", "subscribe")
	}

	topic := processor.Topic()
	if topic == "" {
		return NewTopicError(ErrInvalidTopic, topic, "subscribe")
	}

	g.procMutex.Lock()
	defer g.procMutex.Unlock()

	// Check if already subscribed
	if _, exists := g.processorSubs[topic]; exists {
		return NewTopicError(ErrTopicAlreadySubscribed, topic, "subscribe")
	}

	g.log.WithField("topic", topic).Info("Subscribing to topic with processor")

	// Join topic
	topicHandle, err := g.topicManager.joinTopic(topic)
	if err != nil {
		g.emitSubscriptionError(topic, err)
		return NewTopicError(err, topic, "subscribe")
	}

	// Subscribe to topic
	sub, err := topicHandle.Subscribe()
	if err != nil {
		g.emitSubscriptionError(topic, err)
		return NewTopicError(err, topic, "subscribe")
	}

	// Create processor subscription
	metrics := NewProcessorMetrics(g.log)
	procSub := newProcessorSubscription(processor, sub, metrics, g.emitter, g.log)

	// Register validator
	validator := procSub.createValidator()
	if err := g.pubsub.RegisterTopicValidator(topic, validator); err != nil {
		sub.Cancel()
		g.emitSubscriptionError(topic, err)
		return NewTopicError(err, topic, "subscribe")
	}

	// Start processor subscription
	if err := procSub.Start(); err != nil {
		if unregErr := g.pubsub.UnregisterTopicValidator(topic); unregErr != nil {
			g.log.WithError(unregErr).WithField("topic", topic).Warn("Failed to unregister validator during cleanup")
		}
		sub.Cancel()
		g.emitSubscriptionError(topic, err)
		return NewTopicError(err, topic, "subscribe")
	}

	// Store processor subscription
	g.processorSubs[topic] = procSub

	// Update metrics
	if g.metrics != nil {
		g.metrics.SetActiveSubscriptions(len(g.processorSubs))
	}

	g.log.WithField("topic", topic).Info("Successfully subscribed to topic with processor")
	g.emitTopicSubscribed(topic)

	return nil
}

// SubscribeMultiWithProcessor subscribes to multiple topics using a typed multi-processor.
func SubscribeMultiWithProcessor[T any](g *Gossipsub, ctx context.Context, multiProcessor MultiProcessor[T]) error {
	if !g.IsStarted() {
		return NewError(ErrNotStarted, "subscribe_multi")
	}

	if multiProcessor == nil {
		return NewError(fmt.Errorf("multiProcessor cannot be nil"), "subscribe_multi")
	}

	// Get only active topics from the processor (not all possible topics)
	activeTopics := multiProcessor.ActiveTopics()
	if len(activeTopics) == 0 {
		g.log.Info("MultiProcessor has no active topics, creating subscription tracker without subscriptions")
	}

	g.log.WithField("topics", len(activeTopics)).Info("Subscribing to active topics with multi-processor")

	// Check if already subscribed to this multiprocessor
	g.multiProcMutex.Lock()
	if _, exists := g.multiProcSubs[multiProcessor]; exists {
		g.multiProcMutex.Unlock()
		return NewError(fmt.Errorf("multiProcessor already subscribed"), "subscribe_multi")
	}

	// Create subscription tracker
	subscription := &multiProcessorSubscription{
		multiProcessor: multiProcessor,
		activeTopics:   make(map[string]bool),
	}
	g.multiProcSubs[multiProcessor] = subscription
	g.multiProcMutex.Unlock()

	// Track subscriptions for rollback on error
	subscribedTopics := make([]string, 0, len(activeTopics))

	rollback := func() {
		for _, topic := range subscribedTopics {
			if err := g.Unsubscribe(topic); err != nil {
				g.log.WithError(err).WithField("topic", topic).Warn("Failed to rollback subscription")
			}
		}
		// Remove from multiProcSubs
		g.multiProcMutex.Lock()
		delete(g.multiProcSubs, multiProcessor)
		g.multiProcMutex.Unlock()
	}

	// Subscribe to each active topic
	for _, topic := range activeTopics {
		// Create a topic-specific processor wrapper that delegates to the multi-processor
		wrapper := &multiProcessorWrapper[T]{
			multiProcessor: multiProcessor,
			topic:          topic,
		}

		// Use the existing single-topic subscription logic
		if err := SubscribeWithProcessor(g, ctx, wrapper); err != nil {
			rollback()
			return NewError(fmt.Errorf("failed to subscribe to topic %s: %w", topic, err), "subscribe_multi")
		}

		subscribedTopics = append(subscribedTopics, topic)
		
		// Track in subscription state
		subscription.mutex.Lock()
		subscription.activeTopics[topic] = true
		subscription.mutex.Unlock()
	}

	g.log.WithField("topics", len(subscribedTopics)).Info("Successfully subscribed to active topics with multi-processor")

	return nil
}

// Unsubscribe unsubscribes from a topic.
func (g *Gossipsub) Unsubscribe(topic string) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, topic, "unsubscribe")
	}

	// Check if we have a processor subscription for this topic
	g.procMutex.RLock()
	_, hasProcessorSub := g.processorSubs[topic]
	g.procMutex.RUnlock()

	if !hasProcessorSub {
		return NewTopicError(ErrTopicNotSubscribed, topic, "unsubscribe")
	}

	g.log.WithField("topic", topic).Info("Unsubscribing from topic")

	// Handle processor subscription
	g.procMutex.Lock()
	if procSubInterface, exists := g.processorSubs[topic]; exists {
		if stopper, ok := procSubInterface.(interface{ Stop() error }); ok {
			if err := stopper.Stop(); err != nil {
				g.log.WithError(err).WithField("topic", topic).Warn("Error stopping processor subscription during unsubscribe")
			}
		}

		delete(g.processorSubs, topic)
	}
	g.procMutex.Unlock()

	// Leave topic
	if err := g.topicManager.leaveTopic(topic); err != nil {
		g.log.WithError(err).WithField("topic", topic).Warn("Error leaving topic")
	}

	// Update metrics
	if g.metrics != nil {
		g.procMutex.RLock()
		totalSubs := len(g.processorSubs)
		g.procMutex.RUnlock()
		g.metrics.SetActiveSubscriptions(totalSubs)
	}

	g.log.WithField("topic", topic).Info("Successfully unsubscribed from topic")
	g.emitTopicUnsubscribed(topic)

	return nil
}

// GetSubscriptions returns a list of currently subscribed topics.
func (g *Gossipsub) GetSubscriptions() []string {
	if !g.IsStarted() {
		return nil
	}

	// Collect processor subscriptions
	g.procMutex.RLock()
	topics := make([]string, 0, len(g.processorSubs))

	for topic := range g.processorSubs {
		topics = append(topics, topic)
	}
	g.procMutex.RUnlock()

	return topics
}

// IsSubscribed returns whether we are subscribed to a topic.
func (g *Gossipsub) IsSubscribed(topic string) bool {
	if !g.IsStarted() {
		return false
	}

	// Check processor subscriptions
	g.procMutex.RLock()
	_, exists := g.processorSubs[topic]
	g.procMutex.RUnlock()

	return exists
}

// Publish publishes data to a topic.
func (g *Gossipsub) Publish(ctx context.Context, topic string, data []byte) error {
	return g.PublishWithTimeout(ctx, topic, data, g.config.PublishTimeout)
}

// PublishWithTimeout publishes data to a topic with a custom timeout.
func (g *Gossipsub) PublishWithTimeout(ctx context.Context, topic string, data []byte, timeout time.Duration) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, topic, "publish")
	}

	if topic == "" {
		return NewTopicError(ErrInvalidTopic, topic, "publish")
	}

	if len(data) > g.config.MaxMessageSize {
		return NewTopicError(ErrMessageTooLarge, topic, "publish")
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if g.metrics != nil {
			g.metrics.RecordPublishDuration(topic, duration)
		}
	}()

	// Create timeout context
	publishCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Get or join topic
	topicHandle, err := g.topicManager.joinTopic(topic)
	if err != nil {
		if g.metrics != nil {
			g.metrics.RecordPublishError(topic)
		}

		g.emitPublishError(topic, err)

		return NewTopicError(err, topic, "publish")
	}

	// Publish message
	err = topicHandle.Publish(publishCtx, data)

	// Record metrics and emit events
	success := err == nil
	if g.metrics != nil {
		g.metrics.RecordMessagePublished(topic, success)
	}

	if err != nil {
		if g.metrics != nil {
			g.metrics.RecordPublishError(topic)
		}

		g.emitPublishError(topic, err)

		return NewTopicError(err, topic, "publish")
	}

	g.log.WithFields(logrus.Fields{
		"topic": topic,
		"size":  len(data),
	}).Debug("Message published successfully")

	g.emitMessagePublished(topic)

	return nil
}

// GetTopicPeers returns the peer IDs of peers subscribed to a topic.
func (g *Gossipsub) GetTopicPeers(topic string) []peer.ID {
	if !g.IsStarted() {
		return nil
	}

	// Get topic handle
	topicHandle, err := g.topicManager.getTopic(topic)
	if err != nil {
		return nil
	}

	return topicHandle.ListPeers()
}

// GetAllTopics returns all topics with active subscriptions or that we've joined.
func (g *Gossipsub) GetAllTopics() []string {
	if !g.IsStarted() {
		return nil
	}

	return g.topicManager.getTopicNames()
}

// UpdateMultiProcessorTopics dynamically updates the active topics for a MultiProcessor.
func UpdateMultiProcessorTopics[T any](g *Gossipsub, ctx context.Context, multiProcessor MultiProcessor[T], newActiveTopics []string) error {
	if !g.IsStarted() {
		return NewError(ErrNotStarted, "update_multi_topics")
	}

	if multiProcessor == nil {
		return NewError(fmt.Errorf("multiProcessor cannot be nil"), "update_multi_topics")
	}

	// Find the subscription tracker
	g.multiProcMutex.RLock()
	subscription, exists := g.multiProcSubs[multiProcessor]
	g.multiProcMutex.RUnlock()

	if !exists {
		return NewError(fmt.Errorf("multiProcessor not found in subscriptions"), "update_multi_topics")
	}

	// Use the MultiProcessor's UpdateActiveTopics method to get the changes
	toSubscribe, toUnsubscribe, err := multiProcessor.UpdateActiveTopics(newActiveTopics)
	if err != nil {
		return NewError(fmt.Errorf("failed to update active topics: %w", err), "update_multi_topics")
	}

	g.log.WithFields(logrus.Fields{
		"to_subscribe":   len(toSubscribe),
		"to_unsubscribe": len(toUnsubscribe),
	}).Info("Updating MultiProcessor topic subscriptions")

	// First, unsubscribe from topics we no longer need
	for _, topic := range toUnsubscribe {
		if err := g.Unsubscribe(topic); err != nil {
			g.log.WithError(err).WithField("topic", topic).Warn("Failed to unsubscribe from topic during update")
			// Continue with other unsubscriptions rather than failing completely
		} else {
			// Remove from tracking
			subscription.mutex.Lock()
			delete(subscription.activeTopics, topic)
			subscription.mutex.Unlock()
		}
	}

	// Then, subscribe to new topics
	subscribedNewTopics := make([]string, 0, len(toSubscribe))
	for _, topic := range toSubscribe {
		// Create a topic-specific processor wrapper
		wrapper := &multiProcessorWrapper[T]{
			multiProcessor: multiProcessor,
			topic:          topic,
		}

		// Subscribe to the new topic
		if err := SubscribeWithProcessor(g, ctx, wrapper); err != nil {
			g.log.WithError(err).WithField("topic", topic).Error("Failed to subscribe to new topic during update")
			
			// Rollback newly subscribed topics from this update
			for _, newTopic := range subscribedNewTopics {
				if rollbackErr := g.Unsubscribe(newTopic); rollbackErr != nil {
					g.log.WithError(rollbackErr).WithField("topic", newTopic).Warn("Failed to rollback new subscription")
				}
				subscription.mutex.Lock()
				delete(subscription.activeTopics, newTopic)
				subscription.mutex.Unlock()
			}
			
			return NewError(fmt.Errorf("failed to subscribe to topic %s: %w", topic, err), "update_multi_topics")
		}

		subscribedNewTopics = append(subscribedNewTopics, topic)
		
		// Add to tracking
		subscription.mutex.Lock()
		subscription.activeTopics[topic] = true
		subscription.mutex.Unlock()
	}

	g.log.WithFields(logrus.Fields{
		"unsubscribed": len(toUnsubscribe),
		"subscribed":   len(subscribedNewTopics),
	}).Info("Successfully updated MultiProcessor topic subscriptions")

	return nil
}

// UnsubscribeMultiProcessor unsubscribes from all topics associated with a MultiProcessor.
func UnsubscribeMultiProcessor[T any](g *Gossipsub, multiProcessor MultiProcessor[T]) error {
	if !g.IsStarted() {
		return NewError(ErrNotStarted, "unsubscribe_multi")
	}

	if multiProcessor == nil {
		return NewError(fmt.Errorf("multiProcessor cannot be nil"), "unsubscribe_multi")
	}

	// Find and remove the subscription tracker
	g.multiProcMutex.Lock()
	subscription, exists := g.multiProcSubs[multiProcessor]
	if !exists {
		g.multiProcMutex.Unlock()
		return NewError(fmt.Errorf("multiProcessor not found in subscriptions"), "unsubscribe_multi")
	}
	delete(g.multiProcSubs, multiProcessor)
	g.multiProcMutex.Unlock()

	// Get all currently active topics
	subscription.mutex.RLock()
	activeTopics := make([]string, 0, len(subscription.activeTopics))
	for topic := range subscription.activeTopics {
		activeTopics = append(activeTopics, topic)
	}
	subscription.mutex.RUnlock()

	g.log.WithField("topics", len(activeTopics)).Info("Unsubscribing MultiProcessor from all active topics")

	// Unsubscribe from all active topics
	var lastError error
	for _, topic := range activeTopics {
		if err := g.Unsubscribe(topic); err != nil {
			g.log.WithError(err).WithField("topic", topic).Warn("Failed to unsubscribe from topic during MultiProcessor cleanup")
			lastError = err // Keep track of the last error
		}
	}

	if lastError != nil {
		return NewError(fmt.Errorf("some topics failed to unsubscribe: %w", lastError), "unsubscribe_multi")
	}

	g.log.WithField("topics", len(activeTopics)).Info("Successfully unsubscribed MultiProcessor from all topics")

	return nil
}

// RegisterProcessor is a compatibility function for registering processors.
// This maintains backward compatibility with the old API used in tests.
func RegisterProcessor[T any](g *Gossipsub, processor Processor[T]) error {
	// In the new API, processors are registered implicitly during subscription
	// This function is kept for compatibility and does nothing except validate
	if processor == nil {
		return NewError(fmt.Errorf("processor cannot be nil"), "register")
	}
	
	topic := processor.Topic()
	if topic == "" {
		return NewTopicError(ErrInvalidTopic, "", "register")
	}
	
	// Track registration for compatibility
	g.registryMutex.Lock()
	defer g.registryMutex.Unlock()
	
	if _, exists := g.registeredProcessors[topic]; exists {
		return NewTopicError(ErrAlreadyRegistered, topic, "register")
	}
	
	g.registeredProcessors[topic] = processor
	return nil
}

// SubscribeToProcessorTopic is a compatibility method for the old API.
// It subscribes using the first registered processor for the given topic.
func (g *Gossipsub) SubscribeToProcessorTopic(ctx context.Context, topic string) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, topic, "subscribe_processor_topic")
	}

	if topic == "" {
		return NewTopicError(ErrInvalidTopic, topic, "subscribe_processor_topic")
	}

	// Check if already subscribed
	if g.IsSubscribed(topic) {
		return NewTopicError(ErrTopicAlreadySubscribed, topic, "subscribe_processor_topic")
	}

	// Check if we have a processor registered for this topic
	g.registryMutex.RLock()
	processor, hasProcessor := g.registeredProcessors[topic]
	g.registryMutex.RUnlock()

	if hasProcessor {
		// Use the registered processor to subscribe
		if typedProcessor, ok := processor.(Processor[string]); ok {
			return SubscribeWithProcessor(g, ctx, typedProcessor)
		}
		// If we can't cast to the expected type, fall through to error
	}

	// If no processor is registered, fail
	return NewTopicError(fmt.Errorf("no processor registered"), topic, "subscribe_processor_topic")
}


// RegisterMultiProcessor is a compatibility function for registering multi-processors.
// This maintains backward compatibility with the old API used in tests.
func RegisterMultiProcessor[T any](g *Gossipsub, name string, multiProcessor MultiProcessor[T]) error {
	// In the new API, processors are registered implicitly during subscription
	// This function is kept for compatibility and does nothing except validate
	if multiProcessor == nil {
		return NewError(fmt.Errorf("multiProcessor cannot be nil"), "register_multi")
	}
	
	if name == "" {
		return NewError(fmt.Errorf("name cannot be empty"), "register_multi")
	}
	
	// Track registration for compatibility
	g.registryMutex.Lock()
	defer g.registryMutex.Unlock()
	
	if _, exists := g.registeredMultiProcessors[name]; exists {
		return NewError(ErrAlreadyRegistered, "register_multi")
	}
	
	// Check if we can get active topics (validates the interface)
	activeTopics := multiProcessor.ActiveTopics()
	if len(activeTopics) == 0 {
		// This is OK - the processor might not have any active topics yet
	}
	
	g.registeredMultiProcessors[name] = multiProcessor
	return nil
}

// RegisterWithProcessor is a compatibility function for registering and subscribing with a processor.
// This maintains backward compatibility with the old API used in tests.
func RegisterWithProcessor[T any](g *Gossipsub, ctx context.Context, processor Processor[T]) error {
	// This combines registration and subscription in one call
	if processor == nil {
		return NewError(fmt.Errorf("processor cannot be nil"), "register_with_processor")
	}
	
	// Use the new API to subscribe with the processor
	return SubscribeWithProcessor(g, ctx, processor)
}
