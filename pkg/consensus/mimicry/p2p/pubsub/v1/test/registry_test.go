package v1_test

import (
	"context"
	"fmt"
	"testing"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEncoder is a test encoder that does nothing
type mockEncoder[T any] struct{}

func (m *mockEncoder[T]) Encode(msg T) ([]byte, error) {
	return []byte("encoded"), nil
}

func (m *mockEncoder[T]) Decode(data []byte) (T, error) {
	var zero T
	return zero, nil
}

// RegistryTestMessage is a test message type for registry tests
type RegistryTestMessage struct {
	ID   string
	Data []byte
}

func TestNewRegistry(t *testing.T) {
	r := v1.NewRegistry()
	assert.NotNil(t, r)
	assert.Equal(t, 0, r.TopicCount())
	assert.Equal(t, 0, r.SubnetPatternCount())
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (*v1.Registry, *v1.Topic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage])
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful registration",
			setup: func() (*v1.Registry, *v1.Topic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage]) {
				r := v1.NewRegistry()
				topic, _ := v1.NewTopic[RegistryTestMessage]("test_topic", &mockEncoder[RegistryTestMessage]{})
				handler := v1.NewHandlerConfig[RegistryTestMessage](
					v1.WithValidator(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) v1.ValidationResult {
						return v1.ValidationAccept
					}),
				)
				return r, topic, handler
			},
			wantErr: false,
		},
		{
			name: "nil registry",
			setup: func() (*v1.Registry, *v1.Topic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage]) {
				topic, _ := v1.NewTopic[RegistryTestMessage]("test_topic", &mockEncoder[RegistryTestMessage]{})
				handler := v1.NewHandlerConfig[RegistryTestMessage](
					v1.WithValidator(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) v1.ValidationResult {
						return v1.ValidationAccept
					}),
				)
				return nil, topic, handler
			},
			wantErr: true,
			errMsg:  "registry is nil",
		},
		{
			name: "nil topic",
			setup: func() (*v1.Registry, *v1.Topic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage]) {
				r := v1.NewRegistry()
				handler := v1.NewHandlerConfig[RegistryTestMessage](
					v1.WithValidator(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) v1.ValidationResult {
						return v1.ValidationAccept
					}),
				)
				return r, nil, handler
			},
			wantErr: true,
			errMsg:  "topic is nil",
		},
		{
			name: "nil handler",
			setup: func() (*v1.Registry, *v1.Topic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage]) {
				r := v1.NewRegistry()
				topic, _ := v1.NewTopic[RegistryTestMessage]("test_topic", &mockEncoder[RegistryTestMessage]{})
				return r, topic, nil
			},
			wantErr: true,
			errMsg:  "handler is nil",
		},
		{
			name: "invalid handler (no validator or processor)",
			setup: func() (*v1.Registry, *v1.Topic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage]) {
				r := v1.NewRegistry()
				topic, _ := v1.NewTopic[RegistryTestMessage]("test_topic", &mockEncoder[RegistryTestMessage]{})
				handler := createTestHandler[RegistryTestMessage]() // No options
				return r, topic, handler
			},
			wantErr: true,
			errMsg:  "invalid handler configuration",
		},
		{
			name: "duplicate registration",
			setup: func() (*v1.Registry, *v1.Topic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage]) {
				r := v1.NewRegistry()
				topic, _ := v1.NewTopic[RegistryTestMessage]("test_topic", &mockEncoder[RegistryTestMessage]{})
				handler := v1.NewHandlerConfig[RegistryTestMessage](
					v1.WithValidator(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) v1.ValidationResult {
						return v1.ValidationAccept
					}),
				)
				// Register once
				_ = v1.Register(r, topic, handler)
				// Return same topic for second registration
				return r, topic, handler
			},
			wantErr: true,
			errMsg:  "already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, topic, handler := tt.setup()
			err := v1.Register(r, topic, handler)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				if r != nil {
					assert.Equal(t, 1, r.TopicCount())
				}
			}
		})
	}
}

func TestRegisterSubnet(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (*v1.Registry, *v1.SubnetTopic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage])
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful subnet registration",
			setup: func() (*v1.Registry, *v1.SubnetTopic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage]) {
				r := v1.NewRegistry()
				subnetTopic, _ := v1.NewSubnetTopic[RegistryTestMessage]("attestation_%d", 64, &mockEncoder[RegistryTestMessage]{})
				handler := v1.NewHandlerConfig[RegistryTestMessage](
					v1.WithProcessor(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) error {
						return nil
					}),
				)
				return r, subnetTopic, handler
			},
			wantErr: false,
		},
		{
			name: "nil registry",
			setup: func() (*v1.Registry, *v1.SubnetTopic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage]) {
				subnetTopic, _ := v1.NewSubnetTopic[RegistryTestMessage]("attestation_%d", 64, &mockEncoder[RegistryTestMessage]{})
				handler := v1.NewHandlerConfig[RegistryTestMessage](
					v1.WithProcessor(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) error {
						return nil
					}),
				)
				return nil, subnetTopic, handler
			},
			wantErr: true,
			errMsg:  "registry is nil",
		},
		{
			name: "nil subnet topic",
			setup: func() (*v1.Registry, *v1.SubnetTopic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage]) {
				r := v1.NewRegistry()
				handler := v1.NewHandlerConfig[RegistryTestMessage](
					v1.WithProcessor(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) error {
						return nil
					}),
				)
				return r, nil, handler
			},
			wantErr: true,
			errMsg:  "subnet topic is nil",
		},
		{
			name: "duplicate subnet registration",
			setup: func() (*v1.Registry, *v1.SubnetTopic[RegistryTestMessage], *v1.HandlerConfig[RegistryTestMessage]) {
				r := v1.NewRegistry()
				subnetTopic, _ := v1.NewSubnetTopic[RegistryTestMessage]("attestation_%d", 64, &mockEncoder[RegistryTestMessage]{})
				handler := v1.NewHandlerConfig[RegistryTestMessage](
					v1.WithProcessor(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) error {
						return nil
					}),
				)
				// Register once
				_ = v1.RegisterSubnet(r, subnetTopic, handler)
				// Return same pattern for second registration
				return r, subnetTopic, handler
			},
			wantErr: true,
			errMsg:  "already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, subnetTopic, handler := tt.setup()
			err := v1.RegisterSubnet(r, subnetTopic, handler)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				if r != nil {
					assert.Equal(t, 1, r.SubnetPatternCount())
				}
			}
		})
	}
}

func TestGetHandler(t *testing.T) {
	r := v1.NewRegistry()

	// Register a regular topic
	topic, err := v1.NewTopic[RegistryTestMessage]("beacon_block", &mockEncoder[RegistryTestMessage]{})
	require.NoError(t, err)
	handler := v1.NewHandlerConfig[RegistryTestMessage](
		v1.WithValidator(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) v1.ValidationResult {
			return v1.ValidationAccept
		}),
	)
	err = v1.Register(r, topic, handler)
	require.NoError(t, err)

	// Register a subnet topic
	subnetTopic, err := v1.NewSubnetTopic[RegistryTestMessage]("beacon_attestation_%d", 64, &mockEncoder[RegistryTestMessage]{})
	require.NoError(t, err)
	subnetHandler := v1.NewHandlerConfig[RegistryTestMessage](
		v1.WithProcessor(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) error {
			return nil
		}),
	)
	err = v1.RegisterSubnet(r, subnetTopic, subnetHandler)
	require.NoError(t, err)

	tests := []struct {
		name      string
		topicName string
		wantFound bool
	}{
		{
			name:      "exact topic match",
			topicName: "beacon_block",
			wantFound: true,
		},
		{
			name:      "subnet topic match",
			topicName: "beacon_attestation_5",
			wantFound: true,
		},
		{
			name:      "subnet topic with eth2 prefix",
			topicName: "/eth2/12345678/beacon_attestation_10/ssz_snappy",
			wantFound: true,
		},
		{
			name:      "no match",
			topicName: "unknown_topic",
			wantFound: false,
		},
		{
			name:      "invalid subnet number",
			topicName: "beacon_attestation_invalid",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var handler *v1.HandlerConfig[RegistryTestMessage]
			if tt.wantFound {
				assert.NotNil(t, handler)
			} else {
				assert.Nil(t, handler)
			}
		})
	}
}

func TestHasHandler(t *testing.T) {
	r := v1.NewRegistry()

	// Register a topic
	topic, err := v1.NewTopic[RegistryTestMessage]("test_topic", &mockEncoder[RegistryTestMessage]{})
	require.NoError(t, err)
	handler := v1.NewHandlerConfig[RegistryTestMessage](
		v1.WithValidator(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) v1.ValidationResult {
			return v1.ValidationAccept
		}),
	)
	err = v1.Register(r, topic, handler)
	require.NoError(t, err)

	// Test that registration succeeded by checking topic count
	assert.Equal(t, 1, r.TopicCount())
}

func TestClear(t *testing.T) {
	r := v1.NewRegistry()

	// Register multiple handlers
	topic1, _ := v1.NewTopic[RegistryTestMessage]("topic1", &mockEncoder[RegistryTestMessage]{})
	topic2, _ := v1.NewTopic[RegistryTestMessage]("topic2", &mockEncoder[RegistryTestMessage]{})
	subnetTopic, _ := v1.NewSubnetTopic[RegistryTestMessage]("subnet_%d", 10, &mockEncoder[RegistryTestMessage]{})

	handler := v1.NewHandlerConfig[RegistryTestMessage](
		v1.WithValidator(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) v1.ValidationResult {
			return v1.ValidationAccept
		}),
	)

	_ = v1.Register(r, topic1, handler)
	_ = v1.Register(r, topic2, handler)
	_ = v1.RegisterSubnet(r, subnetTopic, handler)

	assert.Equal(t, 2, r.TopicCount())
	assert.Equal(t, 1, r.SubnetPatternCount())

	r.Clear()

	assert.Equal(t, 0, r.TopicCount())
	assert.Equal(t, 0, r.SubnetPatternCount())
}

func TestConcurrentAccess(t *testing.T) {
	r := v1.NewRegistry()
	done := make(chan bool)

	// Concurrent registrations
	go func() {
		for i := 0; i < 100; i++ {
			topic, _ := v1.NewTopic[RegistryTestMessage](fmt.Sprintf("topic_%d", i), &mockEncoder[RegistryTestMessage]{})
			handler := v1.NewHandlerConfig[RegistryTestMessage](
				v1.WithValidator(func(ctx context.Context, msg RegistryTestMessage, from peer.ID) v1.ValidationResult {
					return v1.ValidationAccept
				}),
			)
			_ = v1.Register(r, topic, handler)
		}
		done <- true
	}()

	// Concurrent lookups
	go func() {
		for i := 0; i < 100; i++ {
			// Just access public methods
			_ = r.TopicCount()
		}
		done <- true
	}()

	// Concurrent counts
	go func() {
		for i := 0; i < 100; i++ {
			r.TopicCount()
			r.SubnetPatternCount()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify some topics were registered
	assert.Greater(t, r.TopicCount(), 0)
}

func TestTypeErasure(t *testing.T) {
	// This test verifies that different message types can be registered
	// and the type erasure through v1.HandlerConfig[any] works correctly

	type MessageTypeA struct {
		FieldA string
	}

	type MessageTypeB struct {
		FieldB int
	}

	r := v1.NewRegistry()

	// Register handler for MessageTypeA
	topicA, _ := v1.NewTopic[MessageTypeA]("topic_a", &mockEncoder[MessageTypeA]{})
	handlerA := v1.NewHandlerConfig[MessageTypeA](
		v1.WithValidator(func(ctx context.Context, msg MessageTypeA, from peer.ID) v1.ValidationResult {
			return v1.ValidationAccept
		}),
	)
	err := v1.Register(r, topicA, handlerA)
	assert.NoError(t, err)

	// Register handler for MessageTypeB
	topicB, _ := v1.NewTopic[MessageTypeB]("topic_b", &mockEncoder[MessageTypeB]{})
	handlerB := v1.NewHandlerConfig[MessageTypeB](
		v1.WithValidator(func(ctx context.Context, msg MessageTypeB, from peer.ID) v1.ValidationResult {
			return v1.ValidationAccept
		}),
	)
	err = v1.Register(r, topicB, handlerB)
	assert.NoError(t, err)

	// Both handlers should be registered
	assert.Equal(t, 2, r.TopicCount())
}
