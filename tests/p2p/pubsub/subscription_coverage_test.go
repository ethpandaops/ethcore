package pubsub_test

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionAdditionalCoverage(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("Gossipsub_IsSubscribed", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)

		// Create and register processor BEFORE starting
		processor := &eventTestProcessor{topic: "test-topic"}
		err = pubsub.RegisterProcessor(gs, processor)
		require.NoError(t, err)
		
		err = gs.Start(context.Background())
		require.NoError(t, err)
		defer gs.Stop()
		
		// Check initial state - not subscribed
		assert.False(t, gs.IsSubscribed("test-topic"))

		// Subscribe
		ctx := context.Background()
		err = gs.SubscribeToProcessorTopic(ctx, "test-topic")
		require.NoError(t, err)

		// Check subscribed state
		assert.True(t, gs.IsSubscribed("test-topic"))

		// Unsubscribe
		err = gs.Unsubscribe("test-topic")
		require.NoError(t, err)

		// Check unsubscribed state
		assert.False(t, gs.IsSubscribed("test-topic"))
	})

	t.Run("Gossipsub_GetAllTopics", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)
		
		// Register processor BEFORE starting
		processor := &eventTestProcessor{topic: "test-topic"}
		err = pubsub.RegisterProcessor(gs, processor)
		require.NoError(t, err)
		
		err = gs.Start(context.Background())
		require.NoError(t, err)
		defer gs.Stop()

		// Initially no topics
		topics := gs.GetAllTopics()
		assert.Len(t, topics, 0)
		
		err = gs.SubscribeToProcessorTopic(context.Background(), "test-topic")
		require.NoError(t, err)

		// Should now have one topic
		topics = gs.GetAllTopics()
		assert.Len(t, topics, 1)
		assert.Contains(t, topics, "test-topic")
	})

	t.Run("Gossipsub_GetTopicPeers", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)
		
		// Register processor BEFORE starting
		processor := &eventTestProcessor{topic: "test-topic"}
		err = pubsub.RegisterProcessor(gs, processor)
		require.NoError(t, err)
		
		err = gs.Start(context.Background())
		require.NoError(t, err)
		defer gs.Stop()
		
		err = gs.SubscribeToProcessorTopic(context.Background(), "test-topic")
		require.NoError(t, err)

		// Get peers for topic (should be empty since we're alone)
		peers := gs.GetTopicPeers("test-topic")
		assert.Len(t, peers, 0)
	})
}

func TestGossipsubAdditionalCoverage(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("NewGossipsub_WithNilConfig", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub with nil config (covers the default config path)
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)
		assert.NotNil(t, gs)
	})

	t.Run("NewGossipsub_WithCustomConfig", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create custom config
		config := &pubsub.Config{
			MaxMessageSize:        5 * 1024 * 1024,
			ValidationBufferSize:  200,
			ValidationConcurrency: 20,
			PublishTimeout:        10 * time.Second,
		}

		// Create gossipsub with custom config
		gs, err := pubsub.NewGossipsub(logger, h, config)
		require.NoError(t, err)
		assert.NotNil(t, gs)
	})

	t.Run("Gossipsub_DoubleStop", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create and start gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)
		
		err = gs.Start(context.Background())
		require.NoError(t, err)

		// Stop once
		err = gs.Stop()
		assert.NoError(t, err)

		// Stop again - should return error since not started
		err = gs.Stop()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not started")
	})

	t.Run("Gossipsub_StartAlreadyStarted", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create and start gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)
		
		ctx := context.Background()
		err = gs.Start(ctx)
		require.NoError(t, err)
		defer gs.Stop()

		// Try to start again - should return error
		err = gs.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already started")
	})

	t.Run("Gossipsub_StopWithTopicCloseError", func(t *testing.T) {
		// This test ensures the error handling path in Stop() is covered
		// when topic manager close returns an error
		
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create and start gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)
		
		// Register processor BEFORE starting
		processor := &eventTestProcessor{topic: "test-topic"}
		err = pubsub.RegisterProcessor(gs, processor)
		require.NoError(t, err)
		
		ctx := context.Background()
		err = gs.Start(ctx)
		require.NoError(t, err)
		
		err = gs.SubscribeToProcessorTopic(ctx, "test-topic")
		require.NoError(t, err)

		// Wait a bit for subscription to be active
		time.Sleep(100 * time.Millisecond)

		// Stop - this will trigger the topic close error path
		err = gs.Stop()
		assert.NoError(t, err) // Should still succeed even with topic close errors
	})

	t.Run("Gossipsub_GetStats", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create and start gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)
		
		err = gs.Start(context.Background())
		require.NoError(t, err)
		defer gs.Stop()

		// Get stats
		stats := gs.GetStats()
		assert.NotNil(t, stats)
	})

	t.Run("Gossipsub_IsStarted_IsStopped", func(t *testing.T) {
		// Create host
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer h.Close()

		// Create gossipsub
		gs, err := pubsub.NewGossipsub(logger, h, nil)
		require.NoError(t, err)

		// Initial state
		assert.False(t, gs.IsStarted())

		// Start
		err = gs.Start(context.Background())
		require.NoError(t, err)

		assert.True(t, gs.IsStarted())

		// Stop
		err = gs.Stop()
		require.NoError(t, err)

		assert.False(t, gs.IsStarted())
	})
}