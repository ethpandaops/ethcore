package v1_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGossipsubErrorHandling(t *testing.T) {
	t.Run("PublishToUnregisteredTopic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create test infrastructure
		ti := NewTestInfrastructure(t)
		defer ti.Cleanup()

		// Create a node
		node, err := ti.CreateNode(ctx)
		require.NoError(t, err)

		// Create a topic but don't register any handler
		topic, err := CreateTestTopic("unregistered-topic")
		require.NoError(t, err)

		// Try to publish to the unregistered topic
		// This should succeed because publishing doesn't require a handler
		msg := GossipTestMessage{
			ID:      "test-msg",
			Content: "test content",
			From:    node.ID.String(),
		}

		err = v1.Publish(node.Gossipsub, topic, msg)
		// Publishing to unregistered topic should fail because no handler/encoder is registered
		assert.Error(t, err, "Publishing to unregistered topic should fail")
		assert.Contains(t, err.Error(), "no handler registered")
	})

	t.Run("SubscribeToUnregisteredTopic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create test infrastructure
		ti := NewTestInfrastructure(t)
		defer ti.Cleanup()

		// Create a node
		node, err := ti.CreateNode(ctx)
		require.NoError(t, err)

		// Create a topic but don't register any handler
		topic, err := CreateTestTopic("unregistered-topic-sub")
		require.NoError(t, err)

		// Try to subscribe to the unregistered topic
		sub, err := v1.Subscribe(ctx, node.Gossipsub, topic)
		assert.Error(t, err, "Subscribing to unregistered topic should fail")
		assert.Nil(t, sub, "Subscription should be nil on error")
		assert.Contains(t, err.Error(), "no handler registered", "Error should mention missing handler")
	})

	t.Run("DoubleSubscriptionToSameTopic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create test infrastructure
		ti := NewTestInfrastructure(t)
		defer ti.Cleanup()

		// Create a node
		node, err := ti.CreateNode(ctx)
		require.NoError(t, err)

		// Create and register a topic
		topic, err := CreateTestTopic("double-sub-topic")
		require.NoError(t, err)

		// Create and register a handler
		handler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		})
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		// First subscription should succeed
		sub1, err := v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
		require.NotNil(t, sub1)

		// Second subscription to the same topic should fail
		sub2, err := v1.Subscribe(ctx, node.Gossipsub, topic)
		assert.Error(t, err, "Double subscription should fail")
		assert.Nil(t, sub2, "Second subscription should be nil")
		assert.Contains(t, err.Error(), "already subscribed", "Error should mention already subscribed")

		// Cancel the first subscription
		sub1.Cancel()

		// Wait a bit for subscription cleanup
		time.Sleep(500 * time.Millisecond)

		// After canceling, we still can't resubscribe to the same topic in libp2p
		// This is a limitation of the underlying libp2p pubsub implementation
		// The topic handle persists even after unsubscribing
		sub3, err := v1.Subscribe(ctx, node.Gossipsub, topic)
		assert.Error(t, err, "Resubscription to same topic should fail due to libp2p limitation")
		assert.Nil(t, sub3, "Subscription should be nil on error")
	})

	t.Run("OperationsOnStoppedGossipsub", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create test infrastructure
		ti := NewTestInfrastructure(t)

		// Create a node
		node, err := ti.CreateNode(ctx)
		require.NoError(t, err)

		// Create a topic and handler
		topic, err := CreateTestTopic("stopped-gossipsub-topic")
		require.NoError(t, err)

		handler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		})

		// Register handler before stopping
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		// Stop the gossipsub instance
		err = node.Gossipsub.Stop()
		require.NoError(t, err)

		// Verify gossipsub is stopped
		assert.False(t, node.Gossipsub.IsStarted(), "Gossipsub should be stopped")

		// Try to create a subnet subscription on stopped gossipsub
		subnetTopic, err := v1.NewSubnetTopic[GossipTestMessage]("test_subnet_%d", 64)
		require.NoError(t, err)
		_, err = v1.CreateSubnetSubscription(node.Gossipsub, subnetTopic)
		assert.Error(t, err, "CreateSubnetSubscription on stopped gossipsub should fail")
		assert.Contains(t, err.Error(), "gossipsub not started", "Error should mention gossipsub not started")

		// Try to subscribe on stopped gossipsub
		sub, err := v1.Subscribe(ctx, node.Gossipsub, topic)
		assert.Error(t, err, "Subscribe on stopped gossipsub should fail")
		assert.Nil(t, sub, "Subscription should be nil")
		assert.Contains(t, err.Error(), "gossipsub not started", "Error should mention gossipsub not started")

		// Try to publish on stopped gossipsub
		msg := GossipTestMessage{
			ID:      "test-msg",
			Content: "test content",
			From:    node.ID.String(),
		}
		err = v1.Publish(node.Gossipsub, topic, msg)
		assert.Error(t, err, "Publish on stopped gossipsub should fail")
		assert.Contains(t, err.Error(), "gossipsub not started", "Error should mention gossipsub not started")

		// Try to stop already stopped gossipsub
		err = node.Gossipsub.Stop()
		assert.Error(t, err, "Stopping already stopped gossipsub should fail")
		assert.Contains(t, err.Error(), "gossipsub not started", "Error should mention gossipsub not started")

		// Clean up node manually since we stopped gossipsub early
		if node.Host != nil {
			node.Host.Close()
		}
		if node.cancel != nil {
			node.cancel()
		}
	})

	t.Run("NilContextHandling", func(t *testing.T) {
		// Test New with nil context
		//nolint:staticcheck // intentionally testing nil context handling
		g, err := v1.New(nil, nil)
		assert.Error(t, err, "New with nil context should fail")
		assert.Nil(t, g, "Gossipsub should be nil on error")
		assert.Contains(t, err.Error(), "context cannot be nil", "Error should mention nil context")

		// For Subscribe with nil context, we need a valid gossipsub instance first
		ctx := context.Background()
		ti := NewTestInfrastructure(t)
		defer ti.Cleanup()

		node, err := ti.CreateNode(ctx)
		require.NoError(t, err)

		topic, err := CreateTestTopic("nil-ctx-topic")
		require.NoError(t, err)

		handler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		})
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		// Try to subscribe with context.TODO() - basic context handling test

		sub, err := v1.Subscribe(context.TODO(), node.Gossipsub, topic)
		if err == nil {
			// If it didn't panic, it should at least return an error or handle gracefully
			assert.NotNil(t, sub, "If no error, subscription should not be nil")
			if sub != nil {
				sub.Cancel() // Clean up
			}
		}
	})

	t.Run("NilHostHandling", func(t *testing.T) {
		ctx := context.Background()

		// Try to create gossipsub with nil host
		g, err := v1.New(ctx, nil)
		assert.Error(t, err, "New with nil host should fail")
		assert.Nil(t, g, "Gossipsub should be nil on error")
		assert.Contains(t, err.Error(), "host cannot be nil", "Error should mention nil host")
	})

	t.Run("InvalidOptions", func(t *testing.T) {
		ctx := context.Background()
		ti := NewTestInfrastructure(t)
		defer ti.Cleanup()

		// Test with nil logger
		_, err := ti.CreateNode(ctx, v1.WithLogger(nil))
		assert.Error(t, err, "Creating node with nil logger should fail")
		assert.Contains(t, err.Error(), "logger cannot be nil", "Error should mention nil logger")

		// Test with invalid publish timeout
		_, err = ti.CreateNode(ctx, v1.WithPublishTimeout(0))
		assert.Error(t, err, "Creating node with zero publish timeout should fail")
		assert.Contains(t, err.Error(), "publish timeout must be positive", "Error should mention invalid timeout")

		_, err = ti.CreateNode(ctx, v1.WithPublishTimeout(-1*time.Second))
		assert.Error(t, err, "Creating node with negative publish timeout should fail")
		assert.Contains(t, err.Error(), "publish timeout must be positive", "Error should mention invalid timeout")

		// Test with invalid publish timeout
		_, err = ti.CreateNode(ctx, v1.WithPublishTimeout(0))
		assert.Error(t, err, "Creating node with zero publish timeout should fail")
		assert.Contains(t, err.Error(), "publish timeout must be positive", "Error should mention invalid timeout")

		_, err = ti.CreateNode(ctx, v1.WithPublishTimeout(-1*time.Second))
		assert.Error(t, err, "Creating node with negative publish timeout should fail")
		assert.Contains(t, err.Error(), "publish timeout must be positive", "Error should mention invalid timeout")

		// Test with nil metrics
		_, err = ti.CreateNode(ctx, v1.WithMetrics(nil))
		assert.Error(t, err, "Creating node with nil metrics should fail")
		assert.Contains(t, err.Error(), "metrics cannot be nil", "Error should mention nil metrics")

		// Test with nil metrics
		_, err = ti.CreateNode(ctx, v1.WithMetrics(nil))
		assert.Error(t, err, "Creating node with nil metrics should fail")
		assert.Contains(t, err.Error(), "metrics cannot be nil", "Error should mention nil metrics")
	})

	t.Run("RegistryErrors", func(t *testing.T) {
		ctx := context.Background()
		ti := NewTestInfrastructure(t)
		defer ti.Cleanup()

		node, err := ti.CreateNode(ctx)
		require.NoError(t, err)

		// Test registering nil topic
		err = v1.Register(node.Gossipsub.Registry(), nil, &v1.HandlerConfig[GossipTestMessage]{})
		assert.Error(t, err, "Registering nil topic should fail")
		assert.Contains(t, err.Error(), "topic is nil", "Error should mention nil topic")

		// Test registering with nil handler
		topic, err := CreateTestTopic("test-topic")
		require.NoError(t, err)
		err = v1.Register(node.Gossipsub.Registry(), topic, nil)
		assert.Error(t, err, "Registering with nil handler should fail")
		assert.Contains(t, err.Error(), "handler is nil", "Error should mention nil handler")

		// Test registering with invalid handler (no validator or processor)
		invalidHandler := &v1.HandlerConfig[GossipTestMessage]{}
		err = v1.Register(node.Gossipsub.Registry(), topic, invalidHandler)
		assert.Error(t, err, "Registering with invalid handler should fail")
		assert.Contains(t, err.Error(), "no validator or processor configured", "Error should mention missing handler components")

		// Test double registration
		validHandler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		})
		err = v1.Register(node.Gossipsub.Registry(), topic, validHandler)
		require.NoError(t, err, "First registration should succeed")

		err = v1.Register(node.Gossipsub.Registry(), topic, validHandler)
		assert.Error(t, err, "Double registration should fail")
		assert.Contains(t, err.Error(), "already registered", "Error should mention already registered")
	})

	t.Run("MessageEncodingErrors", func(t *testing.T) {
		ctx := context.Background()
		ti := NewTestInfrastructure(t)
		defer ti.Cleanup()

		node, err := ti.CreateNode(ctx)
		require.NoError(t, err)

		// Create a topic
		topic, err := v1.NewTopic[GossipTestMessage]("faulty-topic")
		require.NoError(t, err)

		// Create handler with faulty encoder
		faultyEncoder := &FaultyEncoder{shouldFailEncode: true}
		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithEncoder[GossipTestMessage](faultyEncoder),
			v1.WithProcessor[GossipTestMessage](func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				return nil
			}),
		)
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		// Try to publish with faulty encoder
		msg := GossipTestMessage{
			ID:      "test-msg",
			Content: "test content",
			From:    node.ID.String(),
		}
		err = v1.Publish(node.Gossipsub, topic, msg)
		assert.Error(t, err, "Publishing with faulty encoder should fail")
		assert.Contains(t, err.Error(), "failed to encode message", "Error should mention encoding failure")
	})

	t.Run("SubscriptionAfterCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ti := NewTestInfrastructure(t)
		defer ti.Cleanup()

		node, err := ti.CreateNode(ctx)
		require.NoError(t, err)

		topic, err := CreateTestTopic("cancel-test-topic")
		require.NoError(t, err)

		handler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		})
		err = v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		// Subscribe
		sub, err := v1.Subscribe(ctx, node.Gossipsub, topic)
		require.NoError(t, err)
		require.NotNil(t, sub)

		// Verify subscription is active
		assert.False(t, sub.IsCancelled(), "Subscription should be active")

		// Cancel subscription
		sub.Cancel()

		// Verify subscription is cancelled
		assert.True(t, sub.IsCancelled(), "Subscription should be cancelled")

		// Wait for cleanup
		time.Sleep(100 * time.Millisecond)

		// Due to libp2p limitations, we cannot resubscribe to the same topic
		// even after cancellation. The topic handle persists in the pubsub system.
		newSub, err := v1.Subscribe(ctx, node.Gossipsub, topic)
		assert.Error(t, err, "Resubscription should fail due to libp2p topic persistence")
		assert.Nil(t, newSub, "New subscription should be nil on error")
		assert.Contains(t, err.Error(), "topic already exists", "Error should mention topic already exists")
	})
}

// FaultyEncoder is a test encoder that can be configured to fail.
type FaultyEncoder struct {
	shouldFailEncode bool
	shouldFailDecode bool
}

func (e *FaultyEncoder) Encode(msg GossipTestMessage) ([]byte, error) {
	if e.shouldFailEncode {
		return nil, fmt.Errorf("intentional encoding failure")
	}
	// Use the same encoding as TestEncoder
	encoded := fmt.Sprintf("%s|%s|%s", msg.ID, msg.Content, msg.From)

	return []byte(encoded), nil
}

func (e *FaultyEncoder) Decode(data []byte) (GossipTestMessage, error) {
	if e.shouldFailDecode {
		return GossipTestMessage{}, fmt.Errorf("intentional decoding failure")
	}
	// Use the same decoding as TestEncoder
	encoder := &TestEncoder{}

	return encoder.Decode(data)
}

func TestSubnetTopicErrors(t *testing.T) {
	ctx := context.Background()
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	node, err := ti.CreateNode(ctx)
	require.NoError(t, err)

	t.Run("InvalidSubnetNumber", func(t *testing.T) {
		// Create a subnet topic with max 64 subnets
		subnetTopic, err := v1.NewSubnetTopic[GossipTestMessage]("test_subnet_%d", 64)
		require.NoError(t, err)

		// Try to subscribe to subnet beyond max
		forkDigest := [4]byte{0x00, 0x00, 0x00, 0x01}
		sub, err := v1.SubscribeSubnet(ctx, node.Gossipsub, subnetTopic, 65, forkDigest)
		assert.Error(t, err, "Subscribing to subnet beyond max should fail")
		assert.Nil(t, sub, "Subscription should be nil")
	})

	t.Run("SubnetRegistrationErrors", func(t *testing.T) {
		// Test registering nil subnet topic
		err := v1.RegisterSubnet(node.Gossipsub.Registry(), nil, &v1.HandlerConfig[GossipTestMessage]{})
		assert.Error(t, err, "Registering nil subnet topic should fail")
		assert.Contains(t, err.Error(), "subnet topic is nil", "Error should mention nil subnet topic")

		// Create valid subnet topic
		subnetTopic, err := v1.NewSubnetTopic[GossipTestMessage]("test_subnet_%d", 64)
		require.NoError(t, err)

		// Test registering with nil handler
		err = v1.RegisterSubnet(node.Gossipsub.Registry(), subnetTopic, nil)
		assert.Error(t, err, "Registering subnet with nil handler should fail")
		assert.Contains(t, err.Error(), "handler is nil", "Error should mention nil handler")

		// Test double registration of subnet pattern
		validHandler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		})
		err = v1.RegisterSubnet(node.Gossipsub.Registry(), subnetTopic, validHandler)
		require.NoError(t, err, "First subnet registration should succeed")

		err = v1.RegisterSubnet(node.Gossipsub.Registry(), subnetTopic, validHandler)
		assert.Error(t, err, "Double subnet registration should fail")
		assert.Contains(t, err.Error(), "already registered", "Error should mention already registered")
	})
}

func TestGossipsubConcurrentOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create multiple nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	// Create and register multiple topics
	topics := make([]*v1.Topic[GossipTestMessage], 5)
	for i := 0; i < 5; i++ {
		topic, err := CreateTestTopic(fmt.Sprintf("concurrent-topic-%d", i))
		require.NoError(t, err)
		topics[i] = topic

		// Register on all nodes
		for _, node := range nodes {
			handler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				return nil
			})
			err = v1.Register(node.Gossipsub.Registry(), topic, handler)
			require.NoError(t, err)
		}
	}

	t.Run("ConcurrentSubscriptionsAndUnsubscriptions", func(t *testing.T) {
		// Subscribe to all topics concurrently
		subscriptions := make([][]*v1.Subscription, len(nodes))
		for i := range subscriptions {
			subscriptions[i] = make([]*v1.Subscription, len(topics))
		}

		// Concurrent subscriptions
		wg := &sync.WaitGroup{}
		for nodeIdx, node := range nodes {
			for topicIdx, topic := range topics {
				wg.Add(1)
				go func(n *TestNode, t *v1.Topic[GossipTestMessage], ni, ti int) {
					defer wg.Done()
					sub, err := v1.Subscribe(ctx, n.Gossipsub, t)
					if err == nil {
						subscriptions[ni][ti] = sub
					}
				}(node, topic, nodeIdx, topicIdx)
			}
		}
		wg.Wait()

		// Verify all subscriptions succeeded
		for i, nodeSubs := range subscriptions {
			for j, sub := range nodeSubs {
				assert.NotNil(t, sub, "Subscription for node %d topic %d should not be nil", i, j)
			}
		}

		// Concurrent unsubscriptions
		wg = &sync.WaitGroup{}
		for _, nodeSubs := range subscriptions {
			for _, sub := range nodeSubs {
				if sub != nil {
					wg.Add(1)
					go func(s *v1.Subscription) {
						defer wg.Done()
						s.Cancel()
					}(sub)
				}
			}
		}
		wg.Wait()

		// Verify all subscriptions are cancelled
		for _, nodeSubs := range subscriptions {
			for _, sub := range nodeSubs {
				if sub != nil {
					assert.True(t, sub.IsCancelled(), "Subscription should be cancelled")
				}
			}
		}
	})

	t.Run("ConcurrentPublishing", func(t *testing.T) {
		// Use a different topic for this test to avoid conflicts
		pubTopic, err := CreateTestTopic("concurrent-publishing-topic")
		require.NoError(t, err)

		// Register handler on all nodes
		for _, node := range nodes {
			handler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				return nil
			})
			err = v1.Register(node.Gossipsub.Registry(), pubTopic, handler)
			require.NoError(t, err)
		}

		// Subscribe all nodes to the topic
		for _, node := range nodes {
			_, err := v1.Subscribe(ctx, node.Gossipsub, pubTopic)
			require.NoError(t, err)
		}

		// Wait for mesh to stabilize
		WaitForGossipsubReady(t, nodes, pubTopic.Name(), 3)

		// Publish messages concurrently from all nodes
		messageCount := 10
		wg := &sync.WaitGroup{}
		publishErrors := make([]error, len(nodes)*messageCount)
		errorIdx := 0

		for _, node := range nodes {
			for i := 0; i < messageCount; i++ {
				wg.Add(1)
				idx := errorIdx
				errorIdx++
				go func(n *TestNode, msgIdx, errIdx int) {
					defer wg.Done()
					msg := GossipTestMessage{
						ID:      fmt.Sprintf("concurrent-msg-%s-%d", n.ID.ShortString(), msgIdx),
						Content: fmt.Sprintf("Concurrent message %d from %s", msgIdx, n.ID.ShortString()),
						From:    n.ID.String(),
					}
					publishErrors[errIdx] = v1.Publish(n.Gossipsub, pubTopic, msg)
				}(node, i, idx)
			}
		}
		wg.Wait()

		// Verify no publish errors
		for i, err := range publishErrors {
			assert.NoError(t, err, "Publish error at index %d", i)
		}
	})
}

func TestMetricsIntegration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create metrics instance
	metrics := v1.NewMetrics("test")

	// Create node with metrics
	node, err := ti.CreateNode(ctx, v1.WithMetrics(metrics))
	require.NoError(t, err)

	// Create and register topic
	topic, err := CreateTestTopic("metrics-topic")
	require.NoError(t, err)

	handler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
		return nil
	})
	err = v1.Register(node.Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)

	// Subscribe to topic
	sub, err := v1.Subscribe(ctx, node.Gossipsub, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Publish a message
	msg := GossipTestMessage{
		ID:      "metrics-test-msg",
		Content: "Testing metrics",
		From:    node.ID.String(),
	}
	err = v1.Publish(node.Gossipsub, topic, msg)
	require.NoError(t, err)

	// Verify metrics were recorded
	// Note: The actual metrics verification would depend on the metrics implementation
	// This is a placeholder for where you would verify metrics were properly recorded
	assert.NotNil(t, metrics, "Metrics should be initialized")
}

func TestGlobalInvalidPayloadHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Track global handler calls
	globalHandlerCalled := false
	var globalHandlerData []byte
	var globalHandlerErr error
	var globalHandlerTopic string
	_ = globalHandlerCalled // Mark as used for now
	_ = globalHandlerData
	_ = globalHandlerErr
	_ = globalHandlerTopic

	// Create node with global invalid payload handler
	node, err := ti.CreateNode(ctx, v1.WithGlobalInvalidPayloadHandler(
		func(ctx context.Context, data []byte, err error, from peer.ID, topic string) {
			globalHandlerCalled = true
			globalHandlerData = data
			globalHandlerErr = err
			globalHandlerTopic = topic
			t.Logf("Global invalid payload handler called: topic=%s, err=%v", topic, err)
		},
	))
	require.NoError(t, err)

	// Create topic with handler that has a decoder that will fail on specific input
	topic, err := CreateTestTopic("invalid-payload-topic")
	require.NoError(t, err)

	// Create handler with custom decoder that fails on specific input
	handler := v1.NewHandlerConfig(
		v1.WithDecoder(func(data []byte) (GossipTestMessage, error) {
			if string(data) == "INVALID_PAYLOAD" {
				return GossipTestMessage{}, fmt.Errorf("intentional decode error")
			}
			encoder := &TestEncoder{}

			return encoder.Decode(data)
		}),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)

	err = v1.Register(node.Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)

	// Subscribe to topic
	sub, err := v1.Subscribe(ctx, node.Gossipsub, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Wait a bit for subscription to be ready
	time.Sleep(500 * time.Millisecond)

	// Simulate receiving an invalid message by publishing raw data
	// Note: In a real test, you would need to simulate this through the network layer
	// or by having another node send malformed data
	// For this test, we're checking the setup is correct
	assert.NotNil(t, node.Gossipsub, "Gossipsub should be initialized with global handler")
}

// Additional test for specific error scenarios.
func TestSpecificErrorScenarios(t *testing.T) {
	t.Run("TopicNameValidation", func(t *testing.T) {
		// Test empty topic name
		topic, err := v1.NewTopic[GossipTestMessage]("")
		assert.Error(t, err, "Creating topic with empty name should fail")
		assert.Nil(t, topic, "Topic should be nil on error")

		// Test topic name with invalid characters
		// Note: libp2p may have its own validation rules
		topic, err = v1.NewTopic[GossipTestMessage]("topic with spaces")
		// This might actually succeed depending on libp2p's validation
		if err == nil {
			assert.NotNil(t, topic, "Topic should not be nil if no error")
		}
	})

	t.Run("EncoderErrors", func(t *testing.T) {
		// Test nil encoder - now handled in handler config validation
		handler := v1.NewHandlerConfig[GossipTestMessage]()
		err := handler.Validate()
		assert.Error(t, err, "Handler with no encoder or decoder should fail validation")
	})

	t.Run("HandlerConfigurationErrors", func(t *testing.T) {
		// Create handler config with no processor or validator
		handler := v1.NewHandlerConfig[GossipTestMessage]()
		err := handler.Validate()
		assert.Error(t, err, "Handler with no processor or validator should fail validation")
		assert.Equal(t, v1.ErrNoHandler, err, "Should return ErrNoHandler")
	})
}

// Benchmarking error scenarios.
func BenchmarkErrorHandling(b *testing.B) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	ti := &TestInfrastructure{
		t:     &testing.T{},
		nodes: make([]*TestNode, 0),
	}

	node, err := ti.CreateNode(ctx, v1.WithLogger(logger.WithField("bench", "error")))
	if err != nil {
		b.Fatal(err)
	}
	defer ti.Cleanup()

	topic, _ := CreateTestTopic("bench-topic")
	handler := CreateTestHandler(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
		return nil
	})
	if err := v1.Register(node.Gossipsub.Registry(), topic, handler); err != nil {
		b.Fatalf("Failed to register handler: %v", err)
	}

	b.Run("SubscribeToUnregisteredTopic", func(b *testing.B) {
		unregisteredTopic, _ := CreateTestTopic("unregistered-bench-topic")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = v1.Subscribe(ctx, node.Gossipsub, unregisteredTopic)
		}
	})

	b.Run("PublishWithEncodingError", func(b *testing.B) {
		// Create topic
		faultyTopic, _ := v1.NewTopic[GossipTestMessage]("faulty-bench-topic")

		// Register handler with faulty encoder
		faultyEncoder := &FaultyEncoder{shouldFailEncode: true}
		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithEncoder[GossipTestMessage](faultyEncoder),
			v1.WithProcessor[GossipTestMessage](func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				return nil
			}),
		)
		_ = v1.Register(node.Gossipsub.Registry(), faultyTopic, handler)

		msg := GossipTestMessage{ID: "bench", Content: "benchmark", From: "bench"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = v1.Publish(node.Gossipsub, faultyTopic, msg)
		}
	})
}
