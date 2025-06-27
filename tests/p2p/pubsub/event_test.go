package pubsub_test

import (
	ethpubsub "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

func TestEventNames(t *testing.T) {
	// Test that all event names are defined and unique
	eventNames := []string{
		ethpubsub.PubsubStartedEvent,
		ethpubsub.PubsubStoppedEvent,
		ethpubsub.TopicSubscribedEvent,
		ethpubsub.TopicUnsubscribedEvent,
		ethpubsub.SubscriptionErrorEvent,
		ethpubsub.MessageReceivedEvent,
		ethpubsub.MessagePublishedEvent,
		ethpubsub.MessageValidatedEvent,
		ethpubsub.MessageHandledEvent,
		ethpubsub.ValidationFailedEvent,
		ethpubsub.HandlerErrorEvent,
		ethpubsub.PublishErrorEvent,
		ethpubsub.PeerJoinedTopicEvent,
		ethpubsub.PeerLeftTopicEvent,
		ethpubsub.PeerScoreUpdatedEvent,
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
		_ ethpubsub.PubsubStartedCallback     = func() {}
		_ ethpubsub.PubsubStoppedCallback     = func() {}
		_ ethpubsub.TopicSubscribedCallback   = func(topic string) {}
		_ ethpubsub.TopicUnsubscribedCallback = func(topic string) {}
		_ ethpubsub.SubscriptionErrorCallback = func(topic string, err error) {}
		_ ethpubsub.MessageReceivedCallback   = func(topic string, from peer.ID) {}
		_ ethpubsub.MessagePublishedCallback  = func(topic string) {}
		_ ethpubsub.MessageValidatedCallback  = func(topic string, result ethpubsub.ValidationResult) {}
		_ ethpubsub.MessageHandledCallback    = func(topic string, success bool) {}
		_ ethpubsub.ValidationFailedCallback  = func(topic string, from peer.ID, err error) {}
		_ ethpubsub.HandlerErrorCallback      = func(topic string, err error) {}
		_ ethpubsub.PublishErrorCallback      = func(topic string, err error) {}
		_ ethpubsub.PeerJoinedTopicCallback   = func(topic string, peerID peer.ID) {}
		_ ethpubsub.PeerLeftTopicCallback     = func(topic string, peerID peer.ID) {}
		_ ethpubsub.PeerScoreUpdatedCallback  = func(peerID peer.ID, score float64) {}
	)
}

func TestEventValues(t *testing.T) {
	// Test specific event string values
	assert.Equal(t, "pubsub:started", ethpubsub.PubsubStartedEvent)
	assert.Equal(t, "pubsub:stopped", ethpubsub.PubsubStoppedEvent)
	assert.Equal(t, "pubsub:topic:subscribed", ethpubsub.TopicSubscribedEvent)
	assert.Equal(t, "pubsub:topic:unsubscribed", ethpubsub.TopicUnsubscribedEvent)
	assert.Equal(t, "pubsub:subscription:error", ethpubsub.SubscriptionErrorEvent)
	assert.Equal(t, "pubsub:message:received", ethpubsub.MessageReceivedEvent)
	assert.Equal(t, "pubsub:message:published", ethpubsub.MessagePublishedEvent)
	assert.Equal(t, "pubsub:message:validated", ethpubsub.MessageValidatedEvent)
	assert.Equal(t, "pubsub:message:handled", ethpubsub.MessageHandledEvent)
	assert.Equal(t, "pubsub:validation:failed", ethpubsub.ValidationFailedEvent)
	assert.Equal(t, "pubsub:handler:error", ethpubsub.HandlerErrorEvent)
	assert.Equal(t, "pubsub:publish:error", ethpubsub.PublishErrorEvent)
	assert.Equal(t, "pubsub:peer:joined", ethpubsub.PeerJoinedTopicEvent)
	assert.Equal(t, "pubsub:peer:left", ethpubsub.PeerLeftTopicEvent)
	assert.Equal(t, "pubsub:peer:score:updated", ethpubsub.PeerScoreUpdatedEvent)
}

func TestEventCategories(t *testing.T) {
	// Test that events follow naming conventions
	lifecycleEvents := []string{ethpubsub.PubsubStartedEvent, ethpubsub.PubsubStoppedEvent}
	for _, event := range lifecycleEvents {
		assert.Contains(t, event, "pubsub:")
	}

	topicEvents := []string{ethpubsub.TopicSubscribedEvent, ethpubsub.TopicUnsubscribedEvent}
	for _, event := range topicEvents {
		assert.Contains(t, event, "pubsub:topic:")
	}

	messageEvents := []string{ethpubsub.MessageReceivedEvent, ethpubsub.MessagePublishedEvent, ethpubsub.MessageValidatedEvent, ethpubsub.MessageHandledEvent}
	for _, event := range messageEvents {
		assert.Contains(t, event, "pubsub:message:")
	}

	errorEvents := []string{ethpubsub.ValidationFailedEvent, ethpubsub.HandlerErrorEvent, ethpubsub.PublishErrorEvent}
	for _, event := range errorEvents {
		assert.Contains(t, event, "pubsub:")
		assert.True(t,
			contains(event, "error") || contains(event, "failed"),
			"Error event should contain 'error' or 'failed': %s", event,
		)
	}

	peerEvents := []string{ethpubsub.PeerJoinedTopicEvent, ethpubsub.PeerLeftTopicEvent, ethpubsub.PeerScoreUpdatedEvent}
	for _, event := range peerEvents {
		assert.Contains(t, event, "pubsub:peer:")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s[1:], substr) || len(substr) > 0 && s[0] == substr[0] && contains(s[1:], substr[1:]))
}
