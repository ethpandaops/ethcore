package pubsub

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// Event names for the gossipsub event system.
var (
	// Lifecycle events.
	PubsubStartedEvent = "pubsub:started"
	PubsubStoppedEvent = "pubsub:stopped"

	// Subscription events
	TopicSubscribedEvent   = "pubsub:topic:subscribed"
	TopicUnsubscribedEvent = "pubsub:topic:unsubscribed"
	SubscriptionErrorEvent = "pubsub:subscription:error"

	// Message events
	MessageReceivedEvent  = "pubsub:message:received"
	MessagePublishedEvent = "pubsub:message:published"
	MessageValidatedEvent = "pubsub:message:validated"
	MessageHandledEvent   = "pubsub:message:handled"

	// Error events
	ValidationFailedEvent = "pubsub:validation:failed"
	HandlerErrorEvent     = "pubsub:handler:error"
	PublishErrorEvent     = "pubsub:publish:error"

	// Peer events
	PeerJoinedTopicEvent  = "pubsub:peer:joined"
	PeerLeftTopicEvent    = "pubsub:peer:left"
	PeerScoreUpdatedEvent = "pubsub:peer:score:updated"
)

// Event callback function types
type PubsubStartedCallback func()
type PubsubStoppedCallback func()
type TopicSubscribedCallback func(topic string)
type TopicUnsubscribedCallback func(topic string)
type SubscriptionErrorCallback func(topic string, err error)
type MessageReceivedCallback func(topic string, from peer.ID)
type MessagePublishedCallback func(topic string)
type MessageValidatedCallback func(topic string, result ValidationResult)
type MessageHandledCallback func(topic string, success bool)
type ValidationFailedCallback func(topic string, from peer.ID, err error)
type HandlerErrorCallback func(topic string, err error)
type PublishErrorCallback func(topic string, err error)
type PeerJoinedTopicCallback func(topic string, peerID peer.ID)
type PeerLeftTopicCallback func(topic string, peerID peer.ID)
type PeerScoreUpdatedCallback func(peerID peer.ID, score float64)

// Event registration methods for Gossipsub
func (g *Gossipsub) OnPubsubStarted(callback PubsubStartedCallback) {
	g.emitter.On(PubsubStartedEvent, callback)
}

func (g *Gossipsub) OnPubsubStopped(callback PubsubStoppedCallback) {
	g.emitter.On(PubsubStoppedEvent, callback)
}

func (g *Gossipsub) OnTopicSubscribed(callback TopicSubscribedCallback) {
	g.emitter.On(TopicSubscribedEvent, callback)
}

func (g *Gossipsub) OnTopicUnsubscribed(callback TopicUnsubscribedCallback) {
	g.emitter.On(TopicUnsubscribedEvent, callback)
}

func (g *Gossipsub) OnSubscriptionError(callback SubscriptionErrorCallback) {
	g.emitter.On(SubscriptionErrorEvent, callback)
}

func (g *Gossipsub) OnMessageReceived(callback MessageReceivedCallback) {
	g.emitter.On(MessageReceivedEvent, callback)
}

func (g *Gossipsub) OnMessagePublished(callback MessagePublishedCallback) {
	g.emitter.On(MessagePublishedEvent, callback)
}

func (g *Gossipsub) OnMessageValidated(callback MessageValidatedCallback) {
	g.emitter.On(MessageValidatedEvent, callback)
}

func (g *Gossipsub) OnMessageHandled(callback MessageHandledCallback) {
	g.emitter.On(MessageHandledEvent, callback)
}

func (g *Gossipsub) OnValidationFailed(callback ValidationFailedCallback) {
	g.emitter.On(ValidationFailedEvent, callback)
}

func (g *Gossipsub) OnHandlerError(callback HandlerErrorCallback) {
	g.emitter.On(HandlerErrorEvent, callback)
}

func (g *Gossipsub) OnPublishError(callback PublishErrorCallback) {
	g.emitter.On(PublishErrorEvent, callback)
}

func (g *Gossipsub) OnPeerJoinedTopic(callback PeerJoinedTopicCallback) {
	g.emitter.On(PeerJoinedTopicEvent, callback)
}

func (g *Gossipsub) OnPeerLeftTopic(callback PeerLeftTopicCallback) {
	g.emitter.On(PeerLeftTopicEvent, callback)
}

func (g *Gossipsub) OnPeerScoreUpdated(callback PeerScoreUpdatedCallback) {
	g.emitter.On(PeerScoreUpdatedEvent, callback)
}

// Internal emission methods
func (g *Gossipsub) emitPubsubStarted() {
	g.emitter.Emit(PubsubStartedEvent)
}

func (g *Gossipsub) emitPubsubStopped() {
	g.emitter.Emit(PubsubStoppedEvent)
}

func (g *Gossipsub) emitTopicSubscribed(topic string) {
	g.emitter.Emit(TopicSubscribedEvent, topic)
}

func (g *Gossipsub) emitTopicUnsubscribed(topic string) {
	g.emitter.Emit(TopicUnsubscribedEvent, topic)
}

func (g *Gossipsub) emitSubscriptionError(topic string, err error) {
	g.emitter.Emit(SubscriptionErrorEvent, topic, err)
}

func (g *Gossipsub) emitMessageReceived(topic string, from peer.ID) {
	g.emitter.Emit(MessageReceivedEvent, topic, from)
}

func (g *Gossipsub) emitMessagePublished(topic string) {
	g.emitter.Emit(MessagePublishedEvent, topic)
}

func (g *Gossipsub) emitPublishError(topic string, err error) {
	g.emitter.Emit(PublishErrorEvent, topic, err)
}
