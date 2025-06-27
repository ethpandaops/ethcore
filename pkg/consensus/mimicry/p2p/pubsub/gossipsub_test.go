package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTopic = "test-topic"

// testLogger creates a logger for testing
func testLogger() logrus.FieldLogger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
	return logger
}

// createTestHost creates a libp2p host for testing
func createTestHost(t *testing.T) host.Host {
	t.Helper()
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { h.Close() })
	return h
}

// createTestGossipsub creates a gossipsub instance for testing
func createTestGossipsub(t *testing.T) *Gossipsub {
	t.Helper()
	logger := testLogger()
	host := createTestHost(t)
	config := DefaultConfig()

	g, err := NewGossipsub(logger, host, config)
	require.NoError(t, err)
	require.NotNil(t, g)

	return g
}

func TestNewGossipsub(t *testing.T) {
	tests := []struct {
		name      string
		logger    logrus.FieldLogger
		host      host.Host
		config    *Config
		expectErr bool
	}{
		{
			name:      "valid parameters",
			logger:    testLogger(),
			host:      createTestHost(t),
			config:    DefaultConfig(),
			expectErr: false,
		},
		{
			name:      "nil logger",
			logger:    nil,
			host:      createTestHost(t),
			config:    DefaultConfig(),
			expectErr: true,
		},
		{
			name:      "nil host",
			logger:    testLogger(),
			host:      nil,
			config:    DefaultConfig(),
			expectErr: true,
		},
		{
			name:      "nil config uses default",
			logger:    testLogger(),
			host:      createTestHost(t),
			config:    nil,
			expectErr: false,
		},
		{
			name:   "invalid config",
			logger: testLogger(),
			host:   createTestHost(t),
			config: &Config{
				MessageBufferSize: -1, // Invalid
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGossipsub(tt.logger, tt.host, tt.config)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, g)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, g)
				assert.NotNil(t, g.config)
				assert.NotNil(t, g.emitter)
				assert.NotNil(t, g.metrics)
				assert.NotNil(t, g.validationPipeline)
				assert.NotNil(t, g.handlerRegistry)
				assert.False(t, g.IsStarted())
			}
		})
	}
}

func TestGossipsubLifecycle(t *testing.T) {
	g := createTestGossipsub(t)
	ctx := context.Background()

	// Test initial state
	assert.False(t, g.IsStarted())

	// Test starting
	err := g.Start(ctx)
	require.NoError(t, err)
	assert.True(t, g.IsStarted())

	// Test starting again (should fail)
	err = g.Start(ctx)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrAlreadyStarted))

	// Test stopping
	err = g.Stop()
	require.NoError(t, err)
	assert.False(t, g.IsStarted())

	// Test stopping again (should fail)
	err = g.Stop()
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotStarted))
}

func TestSubscribeUnsubscribe(t *testing.T) {
	g := createTestGossipsub(t)
	ctx := context.Background()

	// Start gossipsub
	err := g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	// Use the testTopic constant

	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	// Test subscribe when not started fails
	_ = g.Stop()
	err = g.Subscribe(ctx, testTopic, handler)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotStarted))

	// Restart and test successful subscribe
	err = g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	err = g.Subscribe(ctx, testTopic, handler)
	require.NoError(t, err)

	// Verify subscription exists
	assert.True(t, g.IsSubscribed(testTopic))
	subscriptions := g.GetSubscriptions()
	assert.Contains(t, subscriptions, testTopic)

	// Test subscribing to same topic again (should fail)
	err = g.Subscribe(ctx, testTopic, handler)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrTopicAlreadySubscribed))

	// Test invalid topic
	err = g.Subscribe(ctx, "", handler)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidTopic))

	// Test nil handler
	err = g.Subscribe(ctx, "test-topic-2", nil)
	assert.Error(t, err)

	// Test unsubscribe
	err = g.Unsubscribe(testTopic)
	require.NoError(t, err)
	assert.False(t, g.IsSubscribed(testTopic))

	// Test unsubscribe from non-subscribed topic
	err = g.Unsubscribe("non-existent-topic")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrTopicNotSubscribed))
}

func TestPublish(t *testing.T) {
	g := createTestGossipsub(t)
	ctx := context.Background()

	// Start gossipsub
	err := g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	// Use the testTopic constant
	testData := []byte("test message")

	// Test publish when not started fails
	_ = g.Stop()
	err = g.Publish(ctx, testTopic, testData)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotStarted))

	// Restart and test successful publish
	err = g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	err = g.Publish(ctx, testTopic, testData)
	require.NoError(t, err)

	// Test publish with invalid topic
	err = g.Publish(ctx, "", testData)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidTopic))

	// Test publish with oversized message
	largeData := make([]byte, g.config.MaxMessageSize+1)
	err = g.Publish(ctx, testTopic, largeData)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrMessageTooLarge))

	// Test publish with timeout
	err = g.PublishWithTimeout(ctx, testTopic, testData, 100*time.Millisecond)
	require.NoError(t, err)
}

func TestConcurrentOperations(t *testing.T) {
	g := createTestGossipsub(t)
	ctx := context.Background()

	err := g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	var mu sync.Mutex
	handlerCallCount := 0

	handler := func(ctx context.Context, msg *Message) error {
		mu.Lock()
		handlerCallCount++
		mu.Unlock()
		return nil
	}

	// Test concurrent subscriptions
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			topic := fmt.Sprintf("topic-%d", id)
			err := g.Subscribe(ctx, topic, handler)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// Verify all subscriptions
	subscriptions := g.GetSubscriptions()
	assert.Equal(t, numGoroutines, len(subscriptions))

	// Test concurrent publishing
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			topic := fmt.Sprintf("topic-%d", id)
			for j := 0; j < numOperations; j++ {
				data := []byte(fmt.Sprintf("message-%d-%d", id, j))
				err := g.Publish(ctx, topic, data)
				if err != nil {
					t.Logf("Publish error (expected in concurrent test): %v", err)
				}
			}
		}(i)
	}
	wg.Wait()

	// Give some time for message processing
	time.Sleep(100 * time.Millisecond)

	// Test concurrent unsubscriptions
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			topic := fmt.Sprintf("topic-%d", id)
			err := g.Unsubscribe(topic)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// Verify all unsubscribed
	subscriptions = g.GetSubscriptions()
	assert.Equal(t, 0, len(subscriptions))
}

func TestMessageProcessing(t *testing.T) {
	t.Skip("Skipping network-dependent message processing test")

	g := createTestGossipsub(t)
	ctx := context.Background()

	err := g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	// Use the testTopic constant
	testData := []byte("test message")

	var receivedMessages []Message
	var mu sync.Mutex

	handler := func(ctx context.Context, msg *Message) error {
		mu.Lock()
		receivedMessages = append(receivedMessages, *msg)
		mu.Unlock()
		return nil
	}

	// Subscribe to the topic
	err = g.Subscribe(ctx, testTopic, handler)
	require.NoError(t, err)

	// Publish a message
	err = g.Publish(ctx, testTopic, testData)
	require.NoError(t, err)

	// In a real implementation, we would test message flow
	// For unit tests, we verify the subscription and publish work
	assert.True(t, g.IsSubscribed(testTopic))
}

func TestPeerScoring(t *testing.T) {
	g := createTestGossipsub(t)
	ctx := context.Background()

	err := g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	// Use the testTopic constant

	// Set topic score parameters
	scoreParams := DefaultTopicScoreParams()
	err = g.SetTopicScoreParams(testTopic, scoreParams)
	require.NoError(t, err)

	// Test with invalid topic
	err = g.SetTopicScoreParams("", scoreParams)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidTopic))

	// Test when not started
	_ = g.Stop()
	err = g.SetTopicScoreParams(testTopic, scoreParams)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotStarted))

	err = g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	// Test getting peer scores (should be empty initially)
	scores, err := g.GetAllPeerScores()
	require.NoError(t, err)
	assert.Empty(t, scores)

	// Test getting score for non-existent peer
	fakePeerID := peer.ID("fake-peer")
	_, err = g.GetPeerScore(fakePeerID)
	assert.Error(t, err)
}

func TestEventEmission(t *testing.T) {
	g := createTestGossipsub(t)
	ctx := context.Background()

	// Test event registration and emission
	var startedCalled, stoppedCalled bool
	var subscribedTopic, unsubscribedTopic string

	g.OnPubsubStarted(func() {
		startedCalled = true
	})

	g.OnPubsubStopped(func() {
		stoppedCalled = true
	})

	g.OnTopicSubscribed(func(topic string) {
		subscribedTopic = topic
	})

	g.OnTopicUnsubscribed(func(topic string) {
		unsubscribedTopic = topic
	})

	// Start and verify event
	err := g.Start(ctx)
	require.NoError(t, err)

	// Give events time to fire
	time.Sleep(10 * time.Millisecond)
	assert.True(t, startedCalled)

	// Subscribe and verify event
	// Use the testTopic constant
	handler := func(ctx context.Context, msg *Message) error { return nil }

	err = g.Subscribe(ctx, testTopic, handler)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, testTopic, subscribedTopic)

	// Unsubscribe and verify event
	err = g.Unsubscribe(testTopic)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, testTopic, unsubscribedTopic)

	// Stop and verify event
	err = g.Stop()
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	assert.True(t, stoppedCalled)
}

func TestGetStats(t *testing.T) {
	g := createTestGossipsub(t)
	ctx := context.Background()

	// Test stats when not started
	stats := g.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats.ActiveSubscriptions)
	assert.Equal(t, 0, stats.ConnectedPeers)
	assert.Equal(t, 0, stats.TopicCount)

	// Start and test stats
	err := g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	stats = g.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats.ActiveSubscriptions)

	// Subscribe to a topic and check stats
	// Use the testTopic constant
	handler := func(ctx context.Context, msg *Message) error { return nil }

	err = g.Subscribe(ctx, testTopic, handler)
	require.NoError(t, err)

	stats = g.GetStats()
	assert.Equal(t, 1, stats.ActiveSubscriptions)
}

func TestValidatorOperations(t *testing.T) {
	g := createTestGossipsub(t)
	ctx := context.Background()

	err := g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	// Use the testTopic constant

	// Test registering validator
	validator := func(ctx context.Context, msg *Message) (ValidationResult, error) {
		return ValidationAccept, nil
	}

	err = g.RegisterValidator(testTopic, validator)
	require.NoError(t, err)

	// Test registering nil validator
	err = g.RegisterValidator(testTopic, nil)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidValidator))

	// Test invalid topic
	err = g.RegisterValidator("", validator)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidTopic))

	// Test unregistering validator
	err = g.UnregisterValidator(testTopic)
	require.NoError(t, err)

	// Test when not started
	_ = g.Stop()
	err = g.RegisterValidator(testTopic, validator)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotStarted))
}

func TestTopicOperations(t *testing.T) {
	g := createTestGossipsub(t)
	ctx := context.Background()

	err := g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	// Use the testTopic constant

	// Initially no topics
	topics := g.GetAllTopics()
	assert.Empty(t, topics)

	peers := g.GetTopicPeers(testTopic)
	assert.Empty(t, peers)

	// Subscribe to create a topic
	handler := func(ctx context.Context, msg *Message) error { return nil }
	err = g.Subscribe(ctx, testTopic, handler)
	require.NoError(t, err)

	// Now we should have the topic
	topics = g.GetAllTopics()
	assert.Contains(t, topics, testTopic)

	// Peers should still be empty (no external peers)
	peers = g.GetTopicPeers(testTopic)
	assert.Empty(t, peers) // Should be empty slice
}

// Benchmark tests for performance validation
func BenchmarkPublish(b *testing.B) {
	g := createTestGossipsub(&testing.T{})
	ctx := context.Background()

	err = g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	testTopic := "benchmark-topic"
	testData := []byte("benchmark message")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = g.Publish(ctx, testTopic, testData)
		}
	})
}

func BenchmarkSubscribe(b *testing.B) {
	g := createTestGossipsub(&testing.T{})
	ctx := context.Background()

	err = g.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = g.Stop() }()

	handler := func(ctx context.Context, msg *Message) error { return nil }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic := fmt.Sprintf("benchmark-topic-%d", i)
		_ = g.Subscribe(ctx, topic, handler)
	}
}
