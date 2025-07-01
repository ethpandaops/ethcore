package v1_test

import (
	"context"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGossipsub_New(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	tests := []struct {
		name    string
		ctx     context.Context
		host    host.Host
		opts    []v1.Option
		wantErr bool
	}{
		{
			name: "valid configuration",
			ctx:  ctx,
			host: h,
			opts: []v1.Option{
				v1.WithLogger(logrus.StandardLogger()),
				v1.WithMaxMessageSize(5 << 20),
				v1.WithPublishTimeout(10 * time.Second),
				v1.WithValidationConcurrency(5),
			},
			wantErr: false,
		},
		{
			name:    "nil context",
			ctx:     nil,
			host:    h,
			wantErr: true,
		},
		{
			name:    "nil host",
			ctx:     ctx,
			host:    nil,
			wantErr: true,
		},
		{
			name: "invalid option",
			ctx:  ctx,
			host: h,
			opts: []v1.Option{
				v1.WithMaxMessageSize(-1),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs, err := v1.New(tt.ctx, tt.host, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, gs)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, gs)
				assert.True(t, gs.IsStarted())

				// Clean up
				err = gs.Stop()
				assert.NoError(t, err)
			}
		})
	}
}

func TestGossipsub_RegisterAndSubscribe(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h, v1.WithLogger(logrus.StandardLogger()))
	require.NoError(t, err)
	defer gs.Stop()

	// Create a test topic
	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("test-topic", encoder)
	require.NoError(t, err)

	// Create a handler
	messageReceived := make(chan GossipTestMessage, 1)
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithProcessor[GossipTestMessage](func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			messageReceived <- msg
			return nil
		}),
	)

	// Register the handler
	err = registerTopic(gs, topic, handler)
	assert.NoError(t, err)

	// Subscribe to the topic
	sub, err := subscribeTopic(gs, topic)
	assert.NoError(t, err)
	assert.NotNil(t, sub)
	assert.Equal(t, "test-topic", sub.Topic())
	assert.False(t, sub.IsCancelled())

	// Check subscription count
	assert.Equal(t, 1, gs.TopicCount())
	assert.Contains(t, gs.ActiveTopics(), "test-topic")

	// Cancel subscription
	sub.Cancel()
	assert.True(t, sub.IsCancelled())

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)
}

func TestGossipsub_SubnetTopics(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h, v1.WithLogger(logrus.StandardLogger()))
	require.NoError(t, err)
	defer gs.Stop()

	// Create a subnet topic
	encoder := &GossipTestEncoder{}
	subnetTopic, err := v1.NewSubnetTopic[GossipTestMessage]("test-subnet-%d", 64, encoder)
	require.NoError(t, err)

	// Create a handler
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithValidator[GossipTestMessage](func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			return v1.ValidationAccept
		}),
	)

	// Register the subnet handler
	err = registerSubnetTopic(gs, subnetTopic, handler)
	assert.NoError(t, err)

	// Subscribe to a specific subnet
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	sub, err := subscribeSubnetTopic(gs, subnetTopic, 5, forkDigest)
	assert.NoError(t, err)
	assert.NotNil(t, sub)
	assert.Contains(t, sub.Topic(), "test-subnet-5")

	// Create a subnet subscription manager
	subnetSub, err := v1.NewSubnetSubscription(subnetTopic)
	assert.NoError(t, err)
	assert.NotNil(t, subnetSub)
}

func TestGossipsub_Options(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	gs, err := v1.New(ctx, h,
		v1.WithLogger(logger.WithField("test", "gossipsub")),
		v1.WithMaxMessageSize(20<<20),
		v1.WithPublishTimeout(30*time.Second),
		v1.WithValidationConcurrency(20),
		v1.WithGossipSubParams(pubsub.DefaultGossipSubParams()),
	)
	require.NoError(t, err)
	assert.NotNil(t, gs)
	// Can't check private fields directly anymore

	err = gs.Stop()
	assert.NoError(t, err)
}

func TestGossipsub_Lifecycle(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h)
	require.NoError(t, err)

	// Check initial state
	assert.True(t, gs.IsStarted())
	assert.Equal(t, 0, gs.TopicCount())
	assert.Empty(t, gs.ActiveTopics())

	// Stop the gossipsub
	err = gs.Stop()
	assert.NoError(t, err)
	assert.False(t, gs.IsStarted())

	// Operations should fail after stop
	encoder := &GossipTestEncoder{}
	topic, err := v1.NewTopic[GossipTestMessage]("test", encoder)
	require.NoError(t, err)

	_, err = subscribeTopic(gs, topic)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}
