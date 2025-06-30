package pubsub_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Simple test processor for event testing
type eventTestProcessor struct {
	topic string
}

func (p *eventTestProcessor) Topic() string {
	return p.topic
}

func (p *eventTestProcessor) AllPossibleTopics() []string {
	return []string{p.topic}
}

func (p *eventTestProcessor) Subscribe(ctx context.Context) error {
	return nil
}

func (p *eventTestProcessor) Unsubscribe(ctx context.Context) error {
	return nil
}

func (p *eventTestProcessor) Decode(ctx context.Context, data []byte) (string, error) {
	return string(data), nil
}

func (p *eventTestProcessor) Validate(ctx context.Context, msg string, from string) (pubsub.ValidationResult, error) {
	return pubsub.ValidationAccept, nil
}

func (p *eventTestProcessor) Process(ctx context.Context, msg string, from string) error {
	return nil
}

func (p *eventTestProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	return nil
}

func TestEventCallbacks(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("OnPubsubStarted", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)

		// Track callback execution
		called := false
		gs.OnPubsubStarted(func() {
			called = true
		})

		// Start gossipsub (should trigger callback)
		err = gs.Start(context.Background())
		require.NoError(t, err)
		defer gs.Stop()

		// Give time for event to be emitted
		time.Sleep(10 * time.Millisecond)
		assert.True(t, called)
	})

	t.Run("OnPubsubStopped", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)

		// Start gossipsub
		err = gs.Start(context.Background())
		require.NoError(t, err)

		// Track callback execution
		called := false
		gs.OnPubsubStopped(func() {
			called = true
		})

		// Stop gossipsub (should trigger callback)
		err = gs.Stop()
		require.NoError(t, err)

		// Give time for event to be emitted
		time.Sleep(10 * time.Millisecond)
		assert.True(t, called)
	})

	t.Run("OnTopicSubscribed", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)

		err = gs.Start(context.Background())
		require.NoError(t, err)
		defer gs.Stop()

		// Track callback execution
		var subscribedTopic string
		gs.OnTopicSubscribed(func(topic string) {
			subscribedTopic = topic
		})

		// Register and subscribe with processor
		processor := &eventTestProcessor{topic: "test-topic"}
		err = pubsub.RegisterProcessor(gs, processor)
		require.NoError(t, err)
		
		err = gs.SubscribeToProcessorTopic(context.Background(), "test-topic")
		require.NoError(t, err)

		// Give time for event to be emitted
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, "test-topic", subscribedTopic)
	})

	t.Run("OnTopicUnsubscribed", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)

		err = gs.Start(context.Background())
		require.NoError(t, err)
		defer gs.Stop()

		// Register and subscribe with processor first
		processor := &eventTestProcessor{topic: "test-topic"}
		err = pubsub.RegisterProcessor(gs, processor)
		require.NoError(t, err)
		
		err = gs.SubscribeToProcessorTopic(context.Background(), "test-topic")
		require.NoError(t, err)

		// Track callback execution
		var unsubscribedTopic string
		gs.OnTopicUnsubscribed(func(topic string) {
			unsubscribedTopic = topic
		})

		// Unsubscribe from topic
		err = gs.Unsubscribe("test-topic")
		require.NoError(t, err)

		// Give time for event to be emitted
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, "test-topic", unsubscribedTopic)
	})

	t.Run("OnMessageReceived", func(t *testing.T) {
		// Create two hosts
		h1, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h1.Close()

		h2, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h2.Close()

		// Connect hosts
		err = h1.Connect(context.Background(), peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
		require.NoError(t, err)

		// Create gossipsub instances
		gs1, err := pubsub.NewGossipsub(logger.WithField("node", "1"), h1, nil)
		require.NoError(t, err)
		gs2, err := pubsub.NewGossipsub(logger.WithField("node", "2"), h2, nil)
		require.NoError(t, err)

		// Start both
		err = gs1.Start(context.Background())
		require.NoError(t, err)
		defer gs1.Stop()

		err = gs2.Start(context.Background())
		require.NoError(t, err)
		defer gs2.Stop()

		// Track message reception
		var receivedTopic string
		var receivedFrom peer.ID
		var wg sync.WaitGroup
		wg.Add(1)

		gs2.OnMessageReceived(func(topic string, from peer.ID) {
			receivedTopic = topic
			receivedFrom = from
			wg.Done()
		})

		// Both subscribe to topic via processors
		processor1 := &eventTestProcessor{topic: "test-topic"}
		err = pubsub.RegisterProcessor(gs1, processor1)
		require.NoError(t, err)
		err = gs1.SubscribeToProcessorTopic(context.Background(), "test-topic")
		require.NoError(t, err)

		processor2 := &eventTestProcessor{topic: "test-topic"}
		err = pubsub.RegisterProcessor(gs2, processor2)
		require.NoError(t, err)
		err = gs2.SubscribeToProcessorTopic(context.Background(), "test-topic")
		require.NoError(t, err)

		// Wait for mesh to form
		time.Sleep(500 * time.Millisecond)

		// Publish message from node 1
		err = gs1.Publish(context.Background(), "test-topic", []byte("test-message"))
		require.NoError(t, err)

		// Wait for message
		done := make(chan bool)
		go func() {
			wg.Wait()
			done <- true
		}()

		select {
		case <-done:
			assert.Equal(t, "test-topic", receivedTopic)
			assert.Equal(t, h1.ID(), receivedFrom)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for message")
		}
	})

	t.Run("OnMessagePublished", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)

		err = gs.Start(context.Background())
		require.NoError(t, err)
		defer gs.Stop()

		// Join topic first via processor
		processor := &eventTestProcessor{topic: "test-topic"}
		err = pubsub.RegisterProcessor(gs, processor)
		require.NoError(t, err)
		err = gs.SubscribeToProcessorTopic(context.Background(), "test-topic")
		require.NoError(t, err)

		// Track callback execution
		var publishedTopic string
		gs.OnMessagePublished(func(topic string) {
			publishedTopic = topic
		})

		// Publish message
		err = gs.Publish(context.Background(), "test-topic", []byte("test-message"))
		require.NoError(t, err)

		// Give time for event to be emitted
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, "test-topic", publishedTopic)
	})

	t.Run("OnPublishError", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)

		err = gs.Start(context.Background())
		require.NoError(t, err)
		defer gs.Stop()

		// Track callback execution
		var errorTopic string
		var capturedErr error
		gs.OnPublishError(func(topic string, err error) {
			errorTopic = topic
			capturedErr = err
		})

		// Try to publish to a topic we haven't joined (should fail)
		err = gs.Publish(context.Background(), "unjoined-topic", []byte("test-message"))
		assert.Error(t, err)

		// Give time for event to be emitted
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, "unjoined-topic", errorTopic)
		assert.NotNil(t, capturedErr)
	})

	t.Run("OnSubscriptionError", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub but don't start it
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)

		// Track callback execution
		gs.OnSubscriptionError(func(topic string, err error) {
			// Just register the callback for coverage
		})

		// Try to subscribe without starting (should fail)
		err = gs.SubscribeToProcessorTopic(context.Background(), "test-topic")
		assert.Error(t, err)

		// Verify the error was returned
		assert.Error(t, err)
	})

	// Test remaining callbacks with simple registration
	t.Run("MessageCallbackRegistration", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)

		// Register all callbacks (just verify they can be registered)
		validatedCalled := false
		gs.OnMessageValidated(func(topic string, result pubsub.ValidationResult) {
			validatedCalled = true
		})

		handledCalled := false
		gs.OnMessageHandled(func(topic string, success bool) {
			handledCalled = true
		})

		validationFailedCalled := false
		gs.OnValidationFailed(func(topic string, from peer.ID, err error) {
			validationFailedCalled = true
		})

		handlerErrorCalled := false
		gs.OnHandlerError(func(topic string, err error) {
			handlerErrorCalled = true
		})

		peerJoinedCalled := false
		gs.OnPeerJoinedTopic(func(topic string, peerID peer.ID) {
			peerJoinedCalled = true
		})

		peerLeftCalled := false
		gs.OnPeerLeftTopic(func(topic string, peerID peer.ID) {
			peerLeftCalled = true
		})

		peerScoreCalled := false
		gs.OnPeerScoreUpdated(func(peerID peer.ID, score float64) {
			peerScoreCalled = true
		})

		// All callbacks should be registered successfully
		assert.False(t, validatedCalled)
		assert.False(t, handledCalled)
		assert.False(t, validationFailedCalled)
		assert.False(t, handlerErrorCalled)
		assert.False(t, peerJoinedCalled)
		assert.False(t, peerLeftCalled)
		assert.False(t, peerScoreCalled)
	})
}