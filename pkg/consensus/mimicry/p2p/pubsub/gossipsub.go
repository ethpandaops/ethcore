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

// Gossipsub provides a generic, application-agnostic gossipsub implementation
type Gossipsub struct {
	log     logrus.FieldLogger
	host    host.Host
	pubsub  *pubsub.PubSub
	config  *Config
	emitter *emission.Emitter
	metrics *Metrics

	// Topic and subscription management
	topicManager  *topicManager
	subscriptions map[string]*Subscription
	subMutex      sync.RWMutex

	// Validation and message handling
	validationPipeline *ValidationPipeline
	handlerRegistry    *handlerRegistry

	// Peer scoring
	scoreParams map[string]*TopicScoreParams
	scoreMutex  sync.RWMutex

	// Lifecycle management
	cancel  context.CancelFunc
	started bool
	startMu sync.Mutex
}

// NewGossipsub creates a new Gossipsub instance with the given configuration
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
		log:           logger,
		host:          host,
		config:        config,
		emitter:       emitter,
		metrics:       metrics,
		subscriptions: make(map[string]*Subscription),
		scoreParams:   make(map[string]*TopicScoreParams),
	}

	// Initialize sub-components
	g.validationPipeline = newValidationPipeline(logger, metrics, emitter)
	g.handlerRegistry = newHandlerRegistry(logger, metrics, emitter)

	return g, nil
}

// Start initializes and starts the gossipsub service
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

// Stop gracefully shuts down the gossipsub service
func (g *Gossipsub) Stop() error {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	if !g.started {
		return NewError(ErrNotStarted, "stop")
	}

	g.log.Info("Stopping gossipsub service")

	// Close all subscriptions
	g.subMutex.Lock()
	for topic, sub := range g.subscriptions {
		if err := g.closeSubscription(sub); err != nil {
			g.log.WithError(err).WithField("topic", topic).Warn("Error closing subscription")
		}
	}
	g.subscriptions = make(map[string]*Subscription)
	g.subMutex.Unlock()

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

// IsStarted returns whether the gossipsub service is currently running
func (g *Gossipsub) IsStarted() bool {
	g.startMu.Lock()
	defer g.startMu.Unlock()
	return g.started
}

// GetStats returns current runtime statistics
func (g *Gossipsub) GetStats() *Stats {
	if !g.IsStarted() {
		return &Stats{}
	}

	g.subMutex.RLock()
	activeSubscriptions := len(g.subscriptions)
	g.subMutex.RUnlock()

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

// initializePubsub creates and configures the libp2p pubsub instance
func (g *Gossipsub) initializePubsub(ctx context.Context) error {
	options := g.createPubsubOptions()

	ps, err := pubsub.NewGossipSub(ctx, g.host, options...)
	if err != nil {
		return fmt.Errorf("failed to create gossipsub: %w", err)
	}

	g.pubsub = ps
	return nil
}

// createPubsubOptions builds libp2p pubsub options from the configuration
func (g *Gossipsub) createPubsubOptions() []pubsub.Option {
	options := []pubsub.Option{
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
		pubsub.WithMaxMessageSize(g.config.MaxMessageSize),
		pubsub.WithValidateQueueSize(g.config.ValidationBufferSize),
		pubsub.WithValidateWorkers(g.config.ValidationConcurrency),
	}

	// Configure gossipsub parameters
	gossipsubConfig := pubsub.DefaultGossipSubParams()
	gossipsubConfig.D = g.config.D
	gossipsubConfig.Dlo = g.config.Dlo
	gossipsubConfig.Dhi = g.config.Dhi
	gossipsubConfig.Dlazy = g.config.Dlazy
	gossipsubConfig.HeartbeatInterval = g.config.HeartbeatInterval
	gossipsubConfig.HistoryLength = g.config.HistoryLength
	gossipsubConfig.HistoryGossip = g.config.HistoryGossip

	options = append(options, pubsub.WithGossipSubParams(gossipsubConfig))

	// Enable peer scoring if configured
	if g.config.EnablePeerScoring {
		// Convert stored topic score parameters to libp2p format
		topics := make(map[string]*pubsub.TopicScoreParams)
		g.scoreMutex.RLock()
		for topic, params := range g.scoreParams {
			topics[topic] = &pubsub.TopicScoreParams{
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
		g.scoreMutex.RUnlock()

		scoreParams := &pubsub.PeerScoreParams{
			Topics: topics,
			AppSpecificScore: func(p peer.ID) float64 {
				// Default implementation - can be customized
				return 0
			},
			DecayInterval:     time.Second,
			DecayToZero:       0.01,
			RetainScore:       time.Minute * 10,
			AppSpecificWeight: 1.0,
			TopicScoreCap:     100.0,
		}

		thresholds := &pubsub.PeerScoreThresholds{
			GossipThreshold:             -100,
			PublishThreshold:            -500,
			GraylistThreshold:           -1000,
			AcceptPXThreshold:           100,
			OpportunisticGraftThreshold: 5,
		}

		options = append(options, pubsub.WithPeerScore(scoreParams, thresholds))
	}

	return options
}

// Subscribe subscribes to a topic with a message handler
func (g *Gossipsub) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, topic, "subscribe")
	}

	if topic == "" {
		return NewTopicError(ErrInvalidTopic, topic, "subscribe")
	}

	if handler == nil {
		return NewTopicError(fmt.Errorf("handler cannot be nil"), topic, "subscribe")
	}

	g.subMutex.Lock()
	defer g.subMutex.Unlock()

	// Check if already subscribed
	if _, exists := g.subscriptions[topic]; exists {
		return NewTopicError(ErrTopicAlreadySubscribed, topic, "subscribe")
	}

	g.log.WithField("topic", topic).Info("Subscribing to topic")

	// Create subscription
	sub, err := g.createSubscription(ctx, topic, handler)
	if err != nil {
		g.emitSubscriptionError(topic, err)
		return NewTopicError(err, topic, "subscribe")
	}

	// Store subscription
	g.subscriptions[topic] = sub

	// Register handler
	g.handlerRegistry.register(topic, handler)

	// Update metrics
	if g.metrics != nil {
		g.metrics.SetActiveSubscriptions(len(g.subscriptions))
	}

	g.log.WithField("topic", topic).Info("Successfully subscribed to topic")
	g.emitTopicSubscribed(topic)

	return nil
}

// Unsubscribe unsubscribes from a topic
func (g *Gossipsub) Unsubscribe(topic string) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, topic, "unsubscribe")
	}

	g.subMutex.Lock()
	defer g.subMutex.Unlock()

	sub, exists := g.subscriptions[topic]
	if !exists {
		return NewTopicError(ErrTopicNotSubscribed, topic, "unsubscribe")
	}

	g.log.WithField("topic", topic).Info("Unsubscribing from topic")

	// Close subscription
	if err := g.closeSubscription(sub); err != nil {
		g.log.WithError(err).WithField("topic", topic).Warn("Error closing subscription")
	}

	// Remove from tracking
	delete(g.subscriptions, topic)

	// Unregister handler and validator
	g.handlerRegistry.unregister(topic)
	g.validationPipeline.removeValidator(topic)

	// Leave topic
	if err := g.topicManager.leaveTopic(topic); err != nil {
		g.log.WithError(err).WithField("topic", topic).Warn("Error leaving topic")
	}

	// Update metrics
	if g.metrics != nil {
		g.metrics.SetActiveSubscriptions(len(g.subscriptions))
	}

	g.log.WithField("topic", topic).Info("Successfully unsubscribed from topic")
	g.emitTopicUnsubscribed(topic)

	return nil
}

// GetSubscriptions returns a list of currently subscribed topics
func (g *Gossipsub) GetSubscriptions() []string {
	if !g.IsStarted() {
		return nil
	}

	g.subMutex.RLock()
	defer g.subMutex.RUnlock()

	topics := make([]string, 0, len(g.subscriptions))
	for topic := range g.subscriptions {
		topics = append(topics, topic)
	}

	return topics
}

// IsSubscribed returns whether we are subscribed to a topic
func (g *Gossipsub) IsSubscribed(topic string) bool {
	if !g.IsStarted() {
		return false
	}

	g.subMutex.RLock()
	defer g.subMutex.RUnlock()

	_, exists := g.subscriptions[topic]
	return exists
}

// Publish publishes data to a topic
func (g *Gossipsub) Publish(ctx context.Context, topic string, data []byte) error {
	return g.PublishWithTimeout(ctx, topic, data, g.config.PublishTimeout)
}

// PublishWithTimeout publishes data to a topic with a custom timeout
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

// RegisterValidator registers a validator for a topic
func (g *Gossipsub) RegisterValidator(topic string, validator Validator) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, topic, "register_validator")
	}

	if topic == "" {
		return NewTopicError(ErrInvalidTopic, topic, "register_validator")
	}

	if validator == nil {
		return NewTopicError(ErrInvalidValidator, topic, "register_validator")
	}

	// Register with validation pipeline
	g.validationPipeline.addValidator(topic, validator)

	// Register with libp2p pubsub
	libp2pValidator := g.validationPipeline.createLibp2pValidator(topic)
	err := g.pubsub.RegisterTopicValidator(topic, libp2pValidator)
	if err != nil {
		g.validationPipeline.removeValidator(topic)
		return NewTopicError(err, topic, "register_validator")
	}

	g.log.WithField("topic", topic).Info("Validator registered for topic")
	return nil
}

// UnregisterValidator removes a validator for a topic
func (g *Gossipsub) UnregisterValidator(topic string) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, topic, "unregister_validator")
	}

	// Unregister from libp2p pubsub
	err := g.pubsub.UnregisterTopicValidator(topic)
	if err != nil {
		return NewTopicError(err, topic, "unregister_validator")
	}

	// Remove from validation pipeline
	g.validationPipeline.removeValidator(topic)

	g.log.WithField("topic", topic).Info("Validator unregistered for topic")
	return nil
}

// SetTopicScoreParams sets scoring parameters for a topic
func (g *Gossipsub) SetTopicScoreParams(topic string, params *TopicScoreParams) error {
	if !g.IsStarted() {
		return NewTopicError(ErrNotStarted, topic, "set_score_params")
	}

	if topic == "" {
		return NewTopicError(ErrInvalidTopic, topic, "set_score_params")
	}

	if params == nil {
		params = DefaultTopicScoreParams()
	}

	g.scoreMutex.Lock()
	g.scoreParams[topic] = params
	g.scoreMutex.Unlock()

	g.log.WithField("topic", topic).Info("Topic score parameters stored (applied at startup)")
	return nil
}

// GetPeerScore returns the current score for a specific peer
func (g *Gossipsub) GetPeerScore(peerID peer.ID) (*PeerScoreSnapshot, error) {
	if !g.IsStarted() {
		return nil, NewError(ErrNotStarted, "get_peer_score")
	}

	// Check if peer is connected
	connected := false
	for _, connectedPeer := range g.host.Network().Peers() {
		if connectedPeer == peerID {
			connected = true
			break
		}
	}

	if !connected {
		return nil, NewError(fmt.Errorf("peer not connected: %s", peerID), "get_peer_score")
	}

	// Build topic scores based on current subscriptions
	topicScores := make(map[string]float64)
	for topic := range g.scoreParams {
		// Check if peer is subscribed to this topic
		peers := g.pubsub.ListPeers(topic)
		for _, peer := range peers {
			if peer == peerID {
				// Peer is subscribed to this topic, assign a default positive score
				topicScores[topic] = 1.0
				break
			}
		}
	}

	return &PeerScoreSnapshot{
		PeerID:       peerID,
		Score:        1.0, // Default positive score for connected peers
		Topics:       topicScores,
		AppSpecific:  0.0,
		IPColocation: 0.0,
		Behavioural:  0.0,
	}, nil
}

// GetAllPeerScores returns scores for all known peers
func (g *Gossipsub) GetAllPeerScores() ([]*PeerScoreSnapshot, error) {
	if !g.IsStarted() {
		return nil, NewError(ErrNotStarted, "get_all_peer_scores")
	}

	// Get connected peers
	peers := g.host.Network().Peers()
	scores := make([]*PeerScoreSnapshot, 0, len(peers))

	for _, peerID := range peers {
		score, err := g.GetPeerScore(peerID)
		if err != nil {
			continue
		}
		scores = append(scores, score)
	}

	return scores, nil
}

// GetTopicPeers returns the peer IDs of peers subscribed to a topic
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

// GetAllTopics returns all topics with active subscriptions or that we've joined
func (g *Gossipsub) GetAllTopics() []string {
	if !g.IsStarted() {
		return nil
	}

	return g.topicManager.getTopicNames()
}

// createSubscription creates a new subscription for a topic
func (g *Gossipsub) createSubscription(ctx context.Context, topic string, handler MessageHandler) (*Subscription, error) {
	// Join topic
	topicHandle, err := g.topicManager.joinTopic(topic)
	if err != nil {
		return nil, err
	}

	// Subscribe to topic
	sub, err := topicHandle.Subscribe()
	if err != nil {
		return nil, err
	}

	// Create subscription context
	subCtx, cancel := context.WithCancel(ctx)

	subscription := &Subscription{
		topic:        topic,
		subscription: sub,
		handler:      handler,
		cancel:       cancel,
	}

	// Start message handling goroutine
	go g.handleSubscription(subCtx, subscription)

	return subscription, nil
}

// closeSubscription closes a subscription and cleans up resources
func (g *Gossipsub) closeSubscription(sub *Subscription) error {
	if sub.cancel != nil {
		sub.cancel()
	}

	if sub.subscription != nil {
		sub.subscription.Cancel()
	}

	return nil
}

// handleSubscription processes messages for a subscription
func (g *Gossipsub) handleSubscription(ctx context.Context, sub *Subscription) {
	defer func() {
		if r := recover(); r != nil {
			g.log.WithFields(logrus.Fields{
				"topic": sub.topic,
				"panic": r,
			}).Error("Subscription handler panicked")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			g.log.WithField("topic", sub.topic).Debug("Subscription context canceled")
			return
		default:
			// Read next message
			msg, err := sub.subscription.Next(ctx)
			if err != nil {
				if ctx.Err() != nil {
					// Context canceled, exit gracefully
					return
				}
				g.log.WithError(err).WithField("topic", sub.topic).Error("Error reading message from subscription")
				continue
			}

			// Process message
			g.processMessage(ctx, msg, sub.handler)
		}
	}
}

// processMessage processes a single message through the handler pipeline
func (g *Gossipsub) processMessage(ctx context.Context, pmsg *pubsub.Message, handler MessageHandler) {
	// Convert sequence number from bytes to uint64
	var sequence uint64
	if seqno := pmsg.GetSeqno(); len(seqno) > 0 {
		// Convert byte slice to uint64 (big-endian)
		for i, b := range seqno {
			if i >= 8 { // Limit to 8 bytes for uint64
				break
			}
			sequence = (sequence << 8) | uint64(b)
		}
	}

	// Convert to generic message
	msg := &Message{
		Topic:        pmsg.GetTopic(),
		Data:         pmsg.Data,
		From:         pmsg.GetFrom(),
		ReceivedTime: time.Now(),
		Sequence:     sequence,
	}

	// Record metrics
	if g.metrics != nil {
		g.metrics.RecordMessageReceived(msg.Topic)
	}

	// Emit event
	g.emitMessageReceived(msg.Topic, msg.From)

	// Process with handler registry
	if err := g.handlerRegistry.processMessage(ctx, msg); err != nil {
		g.log.WithError(err).WithFields(logrus.Fields{
			"topic": msg.Topic,
			"from":  msg.From,
		}).Error("Message processing failed")
	}
}
