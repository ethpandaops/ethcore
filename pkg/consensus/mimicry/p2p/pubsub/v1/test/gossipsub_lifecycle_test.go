package v1_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGossipsub_FullLifecycle tests the complete lifecycle of gossipsub
func TestGossipsub_FullLifecycle(t *testing.T) {
	t.Run("single topic lifecycle", func(t *testing.T) {
		ctx := context.Background()
		h := createTestHost(t)
		defer h.Close()

		// Create gossipsub
		gs, err := v1.New(ctx, h)
		require.NoError(t, err)
		assert.True(t, gs.IsStarted())

		// Create topic and handler
		encoder := &GossipTestEncoder{}
		topic, err := v1.NewTopic[GossipTestMessage]("lifecycle-test", encoder)
		require.NoError(t, err)

		messageReceived := make(chan GossipTestMessage, 1)
		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				select {
				case messageReceived <- msg:
				default:
				}
				return nil
			}),
		)

		// Register handler
		err = registerTopic(gs, topic, handler)
		require.NoError(t, err)

		// Subscribe
		sub, err := subscribeTopic(gs, topic)
		require.NoError(t, err)
		assert.Equal(t, "lifecycle-test", sub.Topic())
		assert.False(t, sub.IsCancelled())
		assert.Equal(t, 1, gs.TopicCount())

		// Publish message
		testMsg := GossipTestMessage{ID: "1", Content: "test"}
		err = publishTopic(gs, topic, testMsg)
		require.NoError(t, err)

		// Wait for message
		select {
		case msg := <-messageReceived:
			assert.Equal(t, testMsg.ID, msg.ID)
			assert.Equal(t, testMsg.Content, msg.Content)
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for message")
		}

		// Unsubscribe
		sub.Cancel()
		assert.True(t, sub.IsCancelled())

		// Wait for cleanup
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, 0, gs.TopicCount())

		// Stop gossipsub
		err = gs.Stop()
		require.NoError(t, err)
		assert.False(t, gs.IsStarted())
	})

	t.Run("multiple topics lifecycle", func(t *testing.T) {
		ctx := context.Background()
		h := createTestHost(t)
		defer h.Close()

		gs, err := v1.New(ctx, h)
		require.NoError(t, err)

		encoder := &GossipTestEncoder{}
		numTopics := 5
		topics := make([]*v1.Topic[GossipTestMessage], numTopics)
		subscriptions := make([]*v1.Subscription, numTopics)
		handlers := make([]*v1.HandlerConfig[GossipTestMessage], numTopics)

		// Create and register multiple topics
		for i := 0; i < numTopics; i++ {
			topic, err := v1.NewTopic[GossipTestMessage](fmt.Sprintf("topic-%d", i), encoder)
			require.NoError(t, err)
			topics[i] = topic

			handler := createTestHandler[GossipTestMessage]()
			handlers[i] = handler
			err = registerTopic(gs, topic, handler)
			require.NoError(t, err)
		}

		// Subscribe to all topics
		for i := 0; i < numTopics; i++ {
			sub, err := subscribeTopic(gs, topics[i])
			require.NoError(t, err)
			subscriptions[i] = sub
		}

		assert.Equal(t, numTopics, gs.TopicCount())

		// Unsubscribe from half the topics
		for i := 0; i < numTopics/2; i++ {
			subscriptions[i].Cancel()
		}

		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, numTopics-numTopics/2, gs.TopicCount())

		// Stop gossipsub
		err = gs.Stop()
		require.NoError(t, err)

		// Verify all subscriptions are cancelled
		for _, sub := range subscriptions {
			assert.True(t, sub.IsCancelled())
		}
	})

	t.Run("restart after stop", func(t *testing.T) {
		ctx := context.Background()
		h := createTestHost(t)
		defer h.Close()

		// First instance
		gs1, err := v1.New(ctx, h)
		require.NoError(t, err)
		err = gs1.Stop()
		require.NoError(t, err)

		// Second instance with same host should work
		gs2, err := v1.New(ctx, h)
		require.NoError(t, err)
		defer gs2.Stop()

		assert.True(t, gs2.IsStarted())
	})
}

// TestGossipsub_SubnetLifecycle tests subnet subscription lifecycle
func TestGossipsub_SubnetLifecycle(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h)
	require.NoError(t, err)
	defer gs.Stop()

	encoder := &GossipTestEncoder{}
	subnetTopic, err := v1.NewSubnetTopic[GossipTestMessage]("test_subnet_%d", 64, encoder)
	require.NoError(t, err)

	// Register subnet handler
	handler := createTestHandler[GossipTestMessage]()
	err = registerSubnetTopic(gs, subnetTopic, handler)
	require.NoError(t, err)

	// Create subnet subscription manager
	subnetSubs, err := v1.CreateSubnetSubscription(gs, subnetTopic)
	require.NoError(t, err)

	// Subscribe to multiple subnets
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	subnets := []uint64{0, 5, 10, 15, 20}

	for _, subnet := range subnets {
		sub, err := subscribeSubnetTopic(gs, subnetTopic, subnet, forkDigest)
		require.NoError(t, err)
		err = subnetSubs.Add(subnet, sub)
		require.NoError(t, err)
	}

	// Verify active subnets
	active := subnetSubs.Active()
	assert.Len(t, active, len(subnets))

	// Remove some subnets
	for i := 0; i < len(subnets)/2; i++ {
		removed := subnetSubs.Remove(subnets[i])
		assert.True(t, removed)
	}

	// Verify count
	assert.Equal(t, len(subnets)-len(subnets)/2, subnetSubs.Count())

	// Clear all
	subnetSubs.Clear()
	assert.Equal(t, 0, subnetSubs.Count())
}

// TestGossipsub_ContextPropagation tests context cancellation propagation
func TestGossipsub_ContextPropagation(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(parentCtx, h)
	require.NoError(t, err)

	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("context-test", encoder)
	require.NoError(t, err)

	processorStarted := make(chan struct{})
	processorStopped := make(chan struct{})

	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			select {
			case processorStarted <- struct{}{}:
			default:
			}
			<-ctx.Done()
			select {
			case processorStopped <- struct{}{}:
			default:
			}
			return ctx.Err()
		}),
	)

	err = registerTopic(gs, topic, handler)
	require.NoError(t, err)

	sub, err := subscribeTopic(gs, topic)
	require.NoError(t, err)

	// Publish a message to trigger processor
	msg := GossipTestMessage{ID: "1", Content: "test"}
	err = publishTopic(gs, topic, msg)
	require.NoError(t, err)

	// Wait for processor to start
	select {
	case <-processorStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("processor didn't start")
	}

	// Cancel parent context
	parentCancel()

	// Processor should stop
	select {
	case <-processorStopped:
	case <-time.After(2 * time.Second):
		t.Fatal("processor didn't stop after context cancellation")
	}

	// v1.Subscription should be cancelled
	time.Sleep(100 * time.Millisecond)
	assert.True(t, sub.IsCancelled())
}

// TestGossipsub_MultiNodeLifecycle tests lifecycle with multiple nodes
func TestGossipsub_MultiNodeLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-node test in short mode")
	}

	ctx := context.Background()

	// Create multiple hosts
	numNodes := 3
	hosts := make([]host.Host, numNodes)
	gossipsubs := make([]*v1.Gossipsub, numNodes)

	for i := 0; i < numNodes; i++ {
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		hosts[i] = h

		gs, err := v1.New(ctx, h)
		require.NoError(t, err)
		gossipsubs[i] = gs
	}

	// Connect hosts
	for i := 0; i < numNodes; i++ {
		for j := i + 1; j < numNodes; j++ {
			err := hosts[i].Connect(ctx, peer.AddrInfo{
				ID:    hosts[j].ID(),
				Addrs: hosts[j].Addrs(),
			})
			require.NoError(t, err)
		}
	}

	// Create common topic
	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("multi-node-test", encoder)
	require.NoError(t, err)

	// Track received messages per node
	receivedMessages := make([]chan GossipTestMessage, numNodes)
	for i := 0; i < numNodes; i++ {
		ch := make(chan GossipTestMessage, 10)
		receivedMessages[i] = ch
		nodeID := i

		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				select {
				case receivedMessages[nodeID] <- msg:
				default:
				}
				return nil
			}),
		)

		err = registerTopic(gossipsubs[i], topic, handler)
		require.NoError(t, err)

		_, err = subscribeTopic(gossipsubs[i], topic)
		require.NoError(t, err)
	}

	// Allow mesh to form
	time.Sleep(500 * time.Millisecond)

	// Publish from each node
	for i := 0; i < numNodes; i++ {
		msg := GossipTestMessage{
			ID:      fmt.Sprintf("node-%d", i),
			Content: fmt.Sprintf("message from node %d", i),
		}
		err := publishTopic(gossipsubs[i], topic, msg)
		require.NoError(t, err)
	}

	// Verify all nodes receive all messages
	timeout := time.After(5 * time.Second)
	expectedMessages := numNodes * (numNodes - 1) // Each node receives from all others

	totalReceived := 0
	for totalReceived < expectedMessages {
		select {
		case <-timeout:
			t.Fatalf("timeout: received %d/%d messages", totalReceived, expectedMessages)
		default:
			for i := 0; i < numNodes; i++ {
				select {
				case msg := <-receivedMessages[i]:
					totalReceived++
					t.Logf("Node %d received message: %s", i, msg.ID)
				default:
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Clean shutdown
	var wg sync.WaitGroup
	for i := 0; i < numNodes; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := gossipsubs[idx].Stop()
			assert.NoError(t, err)
			err = hosts[idx].Close()
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()
}

// TestGossipsub_ErrorInjection tests error handling during lifecycle
func TestGossipsub_ErrorInjection(t *testing.T) {
	t.Run("validator errors", func(t *testing.T) {
		ctx := context.Background()
		h := createTestHost(t)
		defer h.Close()

		gs, err := v1.New(ctx, h)
		require.NoError(t, err)
		defer gs.Stop()

		encoder := &GossipTestEncoder{}
		topic, err := v1.NewTopic[GossipTestMessage]("error-test", encoder)
		require.NoError(t, err)

		validationAttempts := 0
		processingAttempts := 0

		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
				validationAttempts++
				if msg.ID == "reject" {
					return v1.ValidationReject
				}
				if msg.ID == "ignore" {
					return v1.ValidationIgnore
				}
				return v1.ValidationAccept
			}),
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				processingAttempts++
				return nil
			}),
		)

		err = registerTopic(gs, topic, handler)
		require.NoError(t, err)

		sub, err := subscribeTopic(gs, topic)
		require.NoError(t, err)
		defer sub.Cancel()

		// Test different validation results
		messages := []struct {
			msg           GossipTestMessage
			shouldProcess bool
		}{
			{GossipTestMessage{ID: "accept", Content: "should process"}, true},
			{GossipTestMessage{ID: "reject", Content: "should not process"}, false},
			{GossipTestMessage{ID: "ignore", Content: "should not process"}, false},
		}

		for _, test := range messages {
			err := publishTopic(gs, topic, test.msg)
			require.NoError(t, err)
		}

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		assert.Equal(t, 3, validationAttempts)
		assert.Equal(t, 1, processingAttempts) // Only "accept" should be processed
	})

	t.Run("processor panic recovery", func(t *testing.T) {
		ctx := context.Background()
		h := createTestHost(t)
		defer h.Close()

		gs, err := v1.New(ctx, h)
		require.NoError(t, err)
		defer gs.Stop()

		encoder := &GossipTestEncoder{}
		topic, err := v1.NewTopic[GossipTestMessage]("panic-test", encoder)
		require.NoError(t, err)

		panicMessage := "test panic"
		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				if msg.ID == "panic" {
					panic(panicMessage)
				}
				return nil
			}),
		)

		err = registerTopic(gs, topic, handler)
		require.NoError(t, err)

		sub, err := subscribeTopic(gs, topic)
		require.NoError(t, err)
		defer sub.Cancel()

		// Publish message that causes panic
		msg := GossipTestMessage{ID: "panic", Content: "should panic"}
		err = publishTopic(gs, topic, msg)
		require.NoError(t, err)

		// v1.Gossipsub should continue working after panic
		time.Sleep(100 * time.Millisecond)
		assert.True(t, gs.IsStarted())

		// Should be able to process more messages
		msg2 := GossipTestMessage{ID: "normal", Content: "should work"}
		err = publishTopic(gs, topic, msg2)
		require.NoError(t, err)
	})
}

// TestGossipsub_GracefulShutdown tests graceful shutdown scenarios
func TestGossipsub_GracefulShutdown(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h)
	require.NoError(t, err)

	encoder := &GossipTestEncoder{}

	// Create multiple active subscriptions
	numTopics := 10
	subscriptions := make([]*v1.Subscription, numTopics)

	for i := 0; i < numTopics; i++ {
		topic, err := v1.NewTopic[GossipTestMessage](fmt.Sprintf("shutdown-topic-%d", i), encoder)
		require.NoError(t, err)

		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				// Simulate some work
				select {
				case <-time.After(10 * time.Millisecond):
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			}),
		)

		err = registerTopic(gs, topic, handler)
		require.NoError(t, err)

		sub, err := subscribeTopic(gs, topic)
		require.NoError(t, err)
		subscriptions[i] = sub

		// Start publishing messages
		go func(t *v1.Topic[GossipTestMessage]) {
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					msg := GossipTestMessage{ID: "test", Content: "shutdown test"}
					_ = publishTopic(gs, t, msg)
				case <-ctx.Done():
					return
				}
			}
		}(topic)
	}

	// Let the system run for a bit
	time.Sleep(200 * time.Millisecond)

	// Graceful shutdown
	shutdownStart := time.Now()
	err = gs.Stop()
	require.NoError(t, err)
	shutdownDuration := time.Since(shutdownStart)

	// Verify shutdown completed in reasonable time
	assert.Less(t, shutdownDuration, 5*time.Second, "Shutdown took too long")

	// Verify all subscriptions are cancelled
	for i, sub := range subscriptions {
		assert.True(t, sub.IsCancelled(), "v1.Subscription %d not cancelled", i)
	}

	// Verify gossipsub is stopped
	assert.False(t, gs.IsStarted())
	assert.Equal(t, 0, gs.TopicCount())
}
