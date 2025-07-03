package v1_test

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMessage is a simple message type for testing.
type TestMessage struct {
	Content string
	Value   int
}

// TestMessageEncoder implements the Encoder interface for TestMessage.
type TestMessageEncoder struct{}

func (e *TestMessageEncoder) Encode(msg TestMessage) ([]byte, error) {
	return []byte(msg.Content), nil
}

func (e *TestMessageEncoder) Decode(data []byte) (TestMessage, error) {
	return TestMessage{Content: string(data)}, nil
}

func TestHandlerConfig(t *testing.T) {
	t.Run("NewHandlerConfig with options", func(t *testing.T) {
		decoder := func(data []byte) (TestMessage, error) {
			return TestMessage{Content: string(data)}, nil
		}

		validator := func(ctx context.Context, msg TestMessage, from peer.ID) v1.ValidationResult {
			if msg.Content == "invalid" {
				return v1.ValidationReject
			}

			return v1.ValidationAccept
		}

		processor := func(ctx context.Context, msg TestMessage, from peer.ID) error {
			if msg.Content == "error" {
				return errors.New("processing error")
			}

			return nil
		}

		scoreParams := &pubsub.TopicScoreParams{
			TopicWeight: 1.0,
		}

		events := make(chan v1.Event, 10)

		config := v1.NewHandlerConfig[TestMessage](
			v1.WithDecoder(decoder),
			v1.WithValidator(validator),
			v1.WithProcessor(processor),
			v1.WithScoreParams[TestMessage](scoreParams),
			v1.WithEvents[TestMessage](events),
		)

		assert.NotNil(t, config)

		// Test that validation works properly
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Validate configuration", func(t *testing.T) {
		// Empty config should fail validation
		config := v1.NewHandlerConfig[TestMessage]()
		err := config.Validate()
		require.Error(t, err)
		assert.Equal(t, v1.ErrNoHandler, err)

		// Config with only validator should pass
		config = v1.NewHandlerConfig[TestMessage](
			v1.WithEncoder(&TestMessageEncoder{}),
			v1.WithValidator(func(ctx context.Context, msg TestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
		)
		err = config.Validate()
		assert.NoError(t, err)

		// Config with only processor should pass
		config = v1.NewHandlerConfig[TestMessage](
			v1.WithEncoder(&TestMessageEncoder{}),
			v1.WithProcessor(func(ctx context.Context, msg TestMessage, from peer.ID) error {
				return nil
			}),
		)
		err = config.Validate()
		assert.NoError(t, err)
	})

	t.Run("v1.ValidationResult values", func(t *testing.T) {
		assert.Equal(t, v1.ValidationResult(0), v1.ValidationAccept)
		assert.Equal(t, v1.ValidationResult(1), v1.ValidationReject)
		assert.Equal(t, v1.ValidationResult(2), v1.ValidationIgnore)
	})

	t.Run("SubnetValidator and SubnetProcessor", func(t *testing.T) {
		subnetValidatorCalled := false
		subnetProcessorCalled := false

		subnetValidator := v1.SubnetValidator[TestMessage](func(ctx context.Context, msg TestMessage, from peer.ID, subnet uint64) v1.ValidationResult {
			subnetValidatorCalled = true
			assert.Equal(t, uint64(42), subnet)

			return v1.ValidationAccept
		})

		subnetProcessor := v1.SubnetProcessor[TestMessage](func(ctx context.Context, msg TestMessage, from peer.ID, subnet uint64) error {
			subnetProcessorCalled = true
			assert.Equal(t, uint64(42), subnet)

			return nil
		})

		// Call the subnet functions
		result := subnetValidator(context.Background(), TestMessage{}, peer.ID("test"), 42)
		assert.Equal(t, v1.ValidationAccept, result)
		assert.True(t, subnetValidatorCalled)

		err := subnetProcessor(context.Background(), TestMessage{}, peer.ID("test"), 42)
		assert.NoError(t, err)
		assert.True(t, subnetProcessorCalled)
	})
}

func TestWithSSZDecoding(t *testing.T) {
	t.Run("WithSSZDecoding creates config for non-SSZ types", func(t *testing.T) {
		config := v1.NewHandlerConfig[TestMessage](
			v1.WithSSZDecoding[TestMessage](),
			v1.WithValidator(func(ctx context.Context, msg TestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
		)

		// Just test that config is created properly
		assert.NotNil(t, config)
		err := config.Validate()
		assert.NoError(t, err)
	})
}
