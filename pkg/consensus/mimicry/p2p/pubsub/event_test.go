package pubsub

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

func TestEventNames(t *testing.T) {
	// Test that all event names are defined and unique
	eventNames := []string{
		PubsubStartedEvent,
		PubsubStoppedEvent,
		TopicSubscribedEvent,
		TopicUnsubscribedEvent,
		SubscriptionErrorEvent,
		MessageReceivedEvent,
		MessagePublishedEvent,
		MessageValidatedEvent,
		MessageHandledEvent,
		ValidationFailedEvent,
		HandlerErrorEvent,
		PublishErrorEvent,
		PeerJoinedTopicEvent,
		PeerLeftTopicEvent,
		PeerScoreUpdatedEvent,
	}

	seen := make(map[string]bool)
	for _, name := range eventNames {
		assert.NotEmpty(t, name, "Event name should not be empty")
		assert.False(t, seen[name], "Event name %s is duplicated", name)
		seen[name] = true
	}
}

func TestCallbackTypes(t *testing.T) {
	// This test just ensures callback types compile correctly
	var (
		_ PubsubStartedCallback     = func() {}
		_ PubsubStoppedCallback     = func() {}
		_ TopicSubscribedCallback   = func(topic string) {}
		_ TopicUnsubscribedCallback = func(topic string) {}
		_ SubscriptionErrorCallback = func(topic string, err error) {}
		_ MessageReceivedCallback   = func(topic string, from peer.ID) {}
		_ MessagePublishedCallback  = func(topic string) {}
		_ MessageValidatedCallback  = func(topic string, result ValidationResult) {}
		_ MessageHandledCallback    = func(topic string, success bool) {}
		_ ValidationFailedCallback  = func(topic string, from peer.ID, err error) {}
		_ HandlerErrorCallback      = func(topic string, err error) {}
		_ PublishErrorCallback      = func(topic string, err error) {}
		_ PeerJoinedTopicCallback   = func(topic string, peerID peer.ID) {}
		_ PeerLeftTopicCallback     = func(topic string, peerID peer.ID) {}
		_ PeerScoreUpdatedCallback  = func(peerID peer.ID, score float64) {}
	)
}

func TestEventValues(t *testing.T) {
	// Test specific event string values
	assert.Equal(t, "pubsub:started", PubsubStartedEvent)
	assert.Equal(t, "pubsub:stopped", PubsubStoppedEvent)
	assert.Equal(t, "pubsub:topic:subscribed", TopicSubscribedEvent)
	assert.Equal(t, "pubsub:topic:unsubscribed", TopicUnsubscribedEvent)
	assert.Equal(t, "pubsub:subscription:error", SubscriptionErrorEvent)
	assert.Equal(t, "pubsub:message:received", MessageReceivedEvent)
	assert.Equal(t, "pubsub:message:published", MessagePublishedEvent)
	assert.Equal(t, "pubsub:message:validated", MessageValidatedEvent)
	assert.Equal(t, "pubsub:message:handled", MessageHandledEvent)
	assert.Equal(t, "pubsub:validation:failed", ValidationFailedEvent)
	assert.Equal(t, "pubsub:handler:error", HandlerErrorEvent)
	assert.Equal(t, "pubsub:publish:error", PublishErrorEvent)
	assert.Equal(t, "pubsub:peer:joined", PeerJoinedTopicEvent)
	assert.Equal(t, "pubsub:peer:left", PeerLeftTopicEvent)
	assert.Equal(t, "pubsub:peer:score:updated", PeerScoreUpdatedEvent)
}

func TestEventCategories(t *testing.T) {
	// Test that events follow naming conventions
	lifecycleEvents := []string{PubsubStartedEvent, PubsubStoppedEvent}
	for _, event := range lifecycleEvents {
		assert.Contains(t, event, "pubsub:")
	}

	topicEvents := []string{TopicSubscribedEvent, TopicUnsubscribedEvent}
	for _, event := range topicEvents {
		assert.Contains(t, event, "pubsub:topic:")
	}

	messageEvents := []string{MessageReceivedEvent, MessagePublishedEvent, MessageValidatedEvent, MessageHandledEvent}
	for _, event := range messageEvents {
		assert.Contains(t, event, "pubsub:message:")
	}

	errorEvents := []string{ValidationFailedEvent, HandlerErrorEvent, PublishErrorEvent}
	for _, event := range errorEvents {
		assert.Contains(t, event, "pubsub:")
		assert.True(t,
			contains(event, "error") || contains(event, "failed"),
			"Error event should contain 'error' or 'failed': %s", event,
		)
	}

	peerEvents := []string{PeerJoinedTopicEvent, PeerLeftTopicEvent, PeerScoreUpdatedEvent}
	for _, event := range peerEvents {
		assert.Contains(t, event, "pubsub:peer:")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s[1:], substr) || len(substr) > 0 && s[0] == substr[0] && contains(s[1:], substr[1:]))
}
