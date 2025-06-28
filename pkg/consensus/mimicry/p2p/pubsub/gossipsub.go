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
	processorSubs map[string]interface{} // stores ProcessorSubscription[T]
	procMutex     sync.RWMutex

	// Processor registration
	registeredProcessors      map[string]interface{} // stores Processor[T] or MultiProcessor[T]
	registeredMultiProcessors map[string]interface{} // stores MultiProcessor[T]
	regMutex                  sync.RWMutex

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
		processorSubs:             make(map[string]interface{}),
		registeredProcessors:      make(map[string]interface{}),
		registeredMultiProcessors: make(map[string]interface{}),
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

	g.processorSubs = make(map[string]interface{})
	g.procMutex.Unlock()

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

// buildTopicScoreMap converts registered processor scoring to libp2p format.
func (g *Gossipsub) buildTopicScoreMap() map[string]*pubsub.TopicScoreParams {
	g.regMutex.RLock()
	defer g.regMutex.RUnlock()

	topicScores := make(map[string]*pubsub.TopicScoreParams)

	// Process single-topic processors
	for _, procInterface := range g.registeredProcessors {
		if singleProc, ok := procInterface.(interface {
			AllPossibleTopics() []string
			GetTopicScoreParams() *TopicScoreParams
		}); ok {
			for _, topic := range singleProc.AllPossibleTopics() {
				if params := singleProc.GetTopicScoreParams(); params != nil {
					topicScores[topic] = &pubsub.TopicScoreParams{
						TopicWeight:                    params.TopicWeight,
						TimeInMeshWeight:               params.TimeInMeshWeight,
						TimeInMeshQuantum:              params.TimeInMeshQuantum,
						TimeInMeshCap:                  params.TimeInMeshCap,
						FirstMessageDeliveriesWeight:   params.FirstMessageDeliveriesWeight,
						FirstMessageDeliveriesDecay:    params.FirstMessageDeliveriesDecay,
						FirstMessageDeliveriesCap:      params.FirstMessageDeliveriesCap,
						InvalidMessageDeliveriesWeight: params.InvalidMessageDeliveriesWeight,
						InvalidMessageDeliveriesDecay:  params.InvalidMessageDeliveriesDecay,
					}
				}
			}
		}
	}

	// Process multi-topic processors
	for _, procInterface := range g.registeredMultiProcessors {
		if multiProc, ok := procInterface.(interface {
			AllPossibleTopics() []string
			GetTopicScoreParams(topic string) *TopicScoreParams
		}); ok {
			for _, topic := range multiProc.AllPossibleTopics() {
				if params := multiProc.GetTopicScoreParams(topic); params != nil {
					topicScores[topic] = &pubsub.TopicScoreParams{
						TopicWeight:                    params.TopicWeight,
						TimeInMeshWeight:               params.TimeInMeshWeight,
						TimeInMeshQuantum:              params.TimeInMeshQuantum,
						TimeInMeshCap:                  params.TimeInMeshCap,
						FirstMessageDeliveriesWeight:   params.FirstMessageDeliveriesWeight,
						FirstMessageDeliveriesDecay:    params.FirstMessageDeliveriesDecay,
						FirstMessageDeliveriesCap:      params.FirstMessageDeliveriesCap,
						InvalidMessageDeliveriesWeight: params.InvalidMessageDeliveriesWeight,
						InvalidMessageDeliveriesDecay:  params.InvalidMessageDeliveriesDecay,
					}
				}
			}
		}
	}

	return topicScores
}

// RegisterProcessor registers a single-topic processor for later subscription.
// Must be called before Start().
func RegisterProcessor[T any](g *Gossipsub, processor Processor[T]) error {
	if g.IsStarted() {
		return NewError(fmt.Errorf("cannot register processors after gossipsub has started"), "register_processor")
	}

	if processor == nil {
		return NewError(fmt.Errorf("processor cannot be nil"), "register_processor")
	}

	topic := processor.Topic()
	if topic == "" {
		return NewError(fmt.Errorf("processor topic cannot be empty"), "register_processor")
	}

	g.regMutex.Lock()
	defer g.regMutex.Unlock()

	if _, exists := g.registeredProcessors[topic]; exists {
		return NewError(fmt.Errorf("processor already registered for topic %s", topic), "register_processor")
	}

	g.registeredProcessors[topic] = processor
	g.log.WithField("topic", topic).Debug("Registered single-topic processor")

	return nil
}

// RegisterMultiProcessor registers a multi-topic processor for later subscription.
// Must be called before Start().
func RegisterMultiProcessor[T any](g *Gossipsub, name string, processor MultiProcessor[T]) error {
	if g.IsStarted() {
		return NewError(fmt.Errorf("cannot register processors after gossipsub has started"), "register_multi_processor")
	}

	if processor == nil {
		return NewError(fmt.Errorf("processor cannot be nil"), "register_multi_processor")
	}

	if name == "" {
		return NewError(fmt.Errorf("processor name cannot be empty"), "register_multi_processor")
	}

	g.regMutex.Lock()
	defer g.regMutex.Unlock()

	if _, exists := g.registeredMultiProcessors[name]; exists {
		return NewError(fmt.Errorf("multi-processor already registered with name %s", name), "register_multi_processor")
	}

	g.registeredMultiProcessors[name] = processor
	g.log.WithFields(logrus.Fields{
		"name":            name,
		"possible_topics": len(processor.AllPossibleTopics()),
	}).Debug("Registered multi-topic processor")

	return nil
}

// SubscribeWithProcessor subscribes to a topic using a processor for type-safe message handling.
func SubscribeWithProcessor[T any](g *Gossipsub, ctx context.Context, processor Processor[T]) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, processor.Topic(), "subscribe_with_processor")
	}

	if processor == nil {
		return NewTopicError(fmt.Errorf("processor cannot be nil"), "", "subscribe_with_processor")
	}

	topic := processor.Topic()
	if topic == "" {
		return NewTopicError(ErrInvalidTopic, topic, "subscribe_with_processor")
	}

	g.procMutex.Lock()
	defer g.procMutex.Unlock()

	// Check if already subscribed
	if _, exists := g.processorSubs[topic]; exists {
		return NewTopicError(ErrTopicAlreadySubscribed, topic, "subscribe_with_processor")
	}

	g.log.WithField("topic", topic).Info("Subscribing to topic with processor")

	// Join topic
	topicHandle, err := g.topicManager.joinTopic(topic)
	if err != nil {
		g.emitSubscriptionError(topic, err)

		return NewTopicError(err, topic, "subscribe_with_processor")
	}

	// Subscribe to topic
	sub, err := topicHandle.Subscribe()
	if err != nil {
		g.emitSubscriptionError(topic, err)

		return NewTopicError(err, topic, "subscribe_with_processor")
	}

	// Create processor subscription
	metrics := NewProcessorMetrics(g.log)
	procSub := newProcessorSubscription(processor, sub, metrics, g.emitter, g.log)

	// Register validator
	validator := procSub.createValidator()
	if err := g.pubsub.RegisterTopicValidator(topic, validator); err != nil {
		sub.Cancel()
		g.emitSubscriptionError(topic, err)

		return NewTopicError(err, topic, "subscribe_with_processor")
	}

	// Topic scoring is handled during pubsub initialization from registered processors

	// Start processor subscription
	if err := procSub.Start(); err != nil {
		if unregErr := g.pubsub.UnregisterTopicValidator(topic); unregErr != nil {
			g.log.WithError(unregErr).WithField("topic", topic).Warn("Failed to unregister validator during cleanup")
		}

		sub.Cancel()
		g.emitSubscriptionError(topic, err)

		return NewTopicError(err, topic, "subscribe_with_processor")
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

// SubscribeMultiWithProcessor subscribes to multiple topics using a MultiProcessor.
func SubscribeMultiWithProcessor[T any](g *Gossipsub, ctx context.Context, multiProcessor MultiProcessor[T]) error {
	if !g.IsStarted() {
		return NewError(ErrNotStarted, "subscribe_multi_with_processor")
	}

	if multiProcessor == nil {
		return NewError(fmt.Errorf("multiProcessor cannot be nil"), "subscribe_multi_with_processor")
	}

	// Get all possible topics from the processor
	allTopics := multiProcessor.AllPossibleTopics()
	if len(allTopics) == 0 {
		return NewError(fmt.Errorf("multiProcessor returned no topics"), "subscribe_multi_with_processor")
	}

	g.log.WithField("topics", len(allTopics)).Info("Subscribing to multiple topics with multi-processor")

	// Track subscriptions for rollback on error
	subscribedTopics := make([]string, 0, len(allTopics))

	rollback := func() {
		for _, topic := range subscribedTopics {
			if err := g.Unsubscribe(topic); err != nil {
				g.log.WithError(err).WithField("topic", topic).Warn("Failed to rollback subscription")
			}
		}
	}

	// Subscribe to each topic
	for _, topic := range allTopics {
		// Create a topic-specific processor wrapper that delegates to the multi-processor
		wrapper := &multiProcessorWrapper[T]{
			multiProcessor: multiProcessor,
			topic:          topic,
		}

		// Use the existing single-topic subscription logic
		if err := SubscribeWithProcessor(g, ctx, wrapper); err != nil {
			rollback()

			return NewError(fmt.Errorf("failed to subscribe to topic %s: %w", topic, err), "subscribe_multi_with_processor")
		}

		subscribedTopics = append(subscribedTopics, topic)
	}

	g.log.WithField("topics", len(subscribedTopics)).Info("Successfully subscribed to all topics with multi-processor")

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

// GetRegisteredProcessor retrieves a registered single-topic processor by topic.
func (g *Gossipsub) GetRegisteredProcessor(topic string) (interface{}, bool) {
	g.regMutex.RLock()
	defer g.regMutex.RUnlock()

	processor, exists := g.registeredProcessors[topic]

	return processor, exists
}

// GetRegisteredMultiProcessor retrieves a registered multi-topic processor by name.
func (g *Gossipsub) GetRegisteredMultiProcessor(name string) (interface{}, bool) {
	g.regMutex.RLock()
	defer g.regMutex.RUnlock()

	processor, exists := g.registeredMultiProcessors[name]

	return processor, exists
}

// This is called by processors during their Subscribe() method.
func (g *Gossipsub) SubscribeToProcessorTopic(ctx context.Context, topic string) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, topic, "subscribe_processor_topic")
	}

	// Check if we have a registered processor for this topic
	g.regMutex.RLock()
	_, exists := g.registeredProcessors[topic]
	g.regMutex.RUnlock()

	if !exists {
		return NewTopicError(fmt.Errorf("no processor registered for topic %s", topic), topic, "subscribe_processor_topic")
	}

	// Direct subscription through processor interface is not supported due to Go's type system limitations
	// Processors must be subscribed using the typed SubscribeWithProcessor[T] method
	return NewTopicError(
		fmt.Errorf("direct processor subscription not supported - use SubscribeWithProcessor[T] with the typed processor"),
		topic,
		"subscribe_processor_topic",
	)
}

// This is called by multi-processors during their Subscribe() method.
func (g *Gossipsub) SubscribeToMultiProcessorTopics(ctx context.Context, name string, subnets []uint64) error {
	if !g.IsStarted() {
		return NewError(ErrNotStarted, "subscribe_multi_processor_topics")
	}

	// Check if we have a registered multi-processor with this name
	g.regMutex.RLock()
	_, exists := g.registeredMultiProcessors[name]
	g.regMutex.RUnlock()

	if !exists {
		return NewError(fmt.Errorf("no multi-processor registered with name %s", name), "subscribe_multi_processor_topics")
	}

	// Direct subscription through multi-processor interface is not supported due to Go's type system limitations
	// Multi-processors must be subscribed using the typed SubscribeMultiWithProcessor[T] method
	return NewError(
		fmt.Errorf("direct multi-processor subscription not supported - use SubscribeMultiWithProcessor[T] with the typed processor"),
		"subscribe_multi_processor_topics",
	)
}
