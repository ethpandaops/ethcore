package v1_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"unsafe"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// GossipTestMessage is a simple test message type for gossipsub tests.
type GossipTestMessage struct {
	ID      string
	Content string
}

// GossipTestEncoder is a simple encoder for gossipsub testing.
type GossipTestEncoder struct{}

func (e *GossipTestEncoder) Encode(msg GossipTestMessage) ([]byte, error) {
	// Simple encoding: ID|Content
	// Use fmt.Sprintf to avoid potential string concatenation issues
	return []byte(fmt.Sprintf("%s|%s", msg.ID, msg.Content)), nil
}

func (e *GossipTestEncoder) Decode(data []byte) (GossipTestMessage, error) {
	// Simple decoding
	parts := string(data)
	idx := 0
	for i, b := range data {
		if b == '|' {
			idx = i
			break
		}
	}
	if idx == 0 {
		return GossipTestMessage{}, nil
	}
	return GossipTestMessage{
		ID:      parts[:idx],
		Content: parts[idx+1:],
	}, nil
}

// createTestHost creates a libp2p host for testing.
func createTestHost(t *testing.T) host.Host {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	return h
}

// Wrapper functions to work around Go generic type conversion limitations
// These functions use unsafe package internally but provide a cleaner API for tests

// registerTopic is a helper function that converts typed Topic to any Topic for registration
func registerTopic[T any](gs *v1.Gossipsub, topic *v1.Topic[T], handler *v1.HandlerConfig[T]) error {
	// Use unsafe conversion to convert *Topic[T] to *Topic[any]
	// This is safe because the underlying struct layout is identical
	anyTopic := (*v1.Topic[any])(unsafe.Pointer(topic))
	anyHandler := (*v1.HandlerConfig[any])(unsafe.Pointer(handler))
	return gs.Register(anyTopic, anyHandler)
}

// subscribeTopic is a helper function that converts typed Topic to any Topic for subscription
func subscribeTopic[T any](gs *v1.Gossipsub, topic *v1.Topic[T]) (*v1.Subscription, error) {
	// Use unsafe conversion to convert *Topic[T] to *Topic[any]
	anyTopic := (*v1.Topic[any])(unsafe.Pointer(topic))
	return v1.Subscribe(context.Background(), gs, anyTopic)
}

// publishTopic is a helper function that converts typed Topic for publishing
func publishTopic[T any](gs *v1.Gossipsub, topic *v1.Topic[T], msg T) error {
	// Use unsafe conversion to convert *Topic[T] to *Topic[any]
	anyTopic := (*v1.Topic[any])(unsafe.Pointer(topic))
	return v1.Publish(gs, anyTopic, any(msg))
}

// registerSubnetTopic is a helper function that converts typed SubnetTopic to any SubnetTopic for registration
func registerSubnetTopic[T any](gs *v1.Gossipsub, subnetTopic *v1.SubnetTopic[T], handler *v1.HandlerConfig[T]) error {
	// Use unsafe conversion to convert *SubnetTopic[T] to *SubnetTopic[any]
	anySubnetTopic := (*v1.SubnetTopic[any])(unsafe.Pointer(subnetTopic))
	anyHandler := (*v1.HandlerConfig[any])(unsafe.Pointer(handler))
	return gs.RegisterSubnet(anySubnetTopic, anyHandler)
}

// subscribeSubnetTopic is a helper function that converts typed SubnetTopic for subscription
func subscribeSubnetTopic[T any](gs *v1.Gossipsub, subnetTopic *v1.SubnetTopic[T], subnet uint64, forkDigest [4]byte) (*v1.Subscription, error) {
	// Use unsafe conversion to convert *SubnetTopic[T] to *SubnetTopic[any]
	anySubnetTopic := (*v1.SubnetTopic[any])(unsafe.Pointer(subnetTopic))
	return v1.SubscribeSubnet(context.Background(), gs, anySubnetTopic, subnet, forkDigest)
}

// createTestHandler creates a basic handler config with a no-op processor for testing
func createTestHandler[T any]() *v1.HandlerConfig[T] {
	return v1.NewHandlerConfig[T](
		v1.WithProcessor[T](func(ctx context.Context, msg T, from peer.ID) error {
			// Simple no-op processor for testing
			return nil
		}),
	)
}

// createMockSubscription creates a mock subscription for testing (using unsafe access to private fields)
func createMockSubscription(topic string, cancelFunc func()) *v1.Subscription {
	// This is unsafe but necessary for testing since Subscription fields are private
	// and there's no public constructor
	sub := &v1.Subscription{}

	// Use unsafe to set private fields
	type subscriptionFields struct {
		topic     string
		cancel    func()
		mu        sync.RWMutex
		cancelled bool
	}

	fields := (*subscriptionFields)(unsafe.Pointer(sub))
	fields.topic = topic
	if cancelFunc != nil {
		fields.cancel = cancelFunc
	} else {
		fields.cancel = func() {} // no-op cancel function
	}
	fields.cancelled = false

	return sub
}
