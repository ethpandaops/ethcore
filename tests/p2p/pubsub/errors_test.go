package pubsub_test

import (
	"errors"
	"fmt"
	"testing"

	ethpubsub "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTopic            = "test/topic"
	alreadyRegisteredMsg = "already registered"
)

func TestNewError(t *testing.T) {
	baseErr := errors.New("base error")
	context := "test operation"

	err := ethpubsub.NewError(baseErr, context)

	assert.NotNil(t, err)
	assert.Equal(t, baseErr, err.Unwrap())
	assert.Contains(t, err.Error(), context)
	assert.Empty(t, err.Topic())
}

func TestNewTopicError(t *testing.T) {
	baseErr := errors.New("topic error")
	topic := testTopic
	context := "subscription"

	err := ethpubsub.NewTopicError(baseErr, topic, context)

	assert.NotNil(t, err)
	assert.Equal(t, baseErr, err.Unwrap())
	assert.Contains(t, err.Error(), context)
	assert.Equal(t, topic, err.Topic())
}

func TestErrorString(t *testing.T) {
	tests := []struct {
		name     string
		err      *ethpubsub.Error
		expected string
	}{
		{
			name:     "error without topic",
			err:      ethpubsub.NewError(errors.New("connection failed"), "connect"),
			expected: "pubsub connect: connection failed",
		},
		{
			name:     "error with topic",
			err:      ethpubsub.NewTopicError(errors.New("validation failed"), "beacon_block", "validate"),
			expected: "pubsub validate [topic: beacon_block]: validation failed",
		},
		{
			name:     "error with empty context",
			err:      ethpubsub.NewError(errors.New("unknown error"), ""),
			expected: "pubsub : unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	baseErr := errors.New("base error")
	err := ethpubsub.NewError(baseErr, "operation")

	unwrapped := err.Unwrap()
	assert.Equal(t, baseErr, unwrapped)

	// Test with errors.Is
	assert.True(t, errors.Is(err, baseErr))
}

func TestErrorTopic(t *testing.T) {
	tests := []struct {
		name     string
		err      *ethpubsub.Error
		expected string
	}{
		{
			name:     "error without topic",
			err:      ethpubsub.NewError(errors.New("error"), "context"),
			expected: "",
		},
		{
			name:     "error with topic",
			err:      ethpubsub.NewTopicError(errors.New("error"), "test/topic", "context"),
			expected: "test/topic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Topic())
		})
	}
}

func TestPredefinedErrors(t *testing.T) {
	// Test that all predefined errors are not nil
	assert.NotNil(t, ethpubsub.ErrNotStarted)
	assert.NotNil(t, ethpubsub.ErrAlreadyStarted)
	assert.NotNil(t, ethpubsub.ErrInvalidTopic)
	assert.NotNil(t, ethpubsub.ErrTopicNotSubscribed)
	assert.NotNil(t, ethpubsub.ErrTopicAlreadySubscribed)
	assert.NotNil(t, ethpubsub.ErrMessageTooLarge)
	assert.NotNil(t, ethpubsub.ErrValidationFailed)
	assert.NotNil(t, ethpubsub.ErrPublishTimeout)
	assert.NotNil(t, ethpubsub.ErrSubscriptionClosed)
	assert.NotNil(t, ethpubsub.ErrPeerNotFound)
	assert.NotNil(t, ethpubsub.ErrContextCanceled)
	assert.NotNil(t, ethpubsub.ErrAlreadyRegistered)

	// Test error messages
	assert.Equal(t, "pubsub not started", ethpubsub.ErrNotStarted.Error())
	assert.Equal(t, "pubsub already started", ethpubsub.ErrAlreadyStarted.Error())
	assert.Equal(t, alreadyRegisteredMsg, ethpubsub.ErrAlreadyRegistered.Error())
}

func TestIsAlreadyRegisteredError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     alreadyRegisteredMsg + " error",
			err:      ethpubsub.ErrAlreadyRegistered,
			expected: true,
		},
		{
			name:     "wrapped " + alreadyRegisteredMsg + " error",
			err:      fmt.Errorf("processor already registered for topic test"),
			expected: true,
		},
		{
			name:     "multi-processor " + alreadyRegisteredMsg,
			err:      fmt.Errorf("multi-processor already registered with name test"),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      errors.New("connection failed"),
			expected: false,
		},
		{
			name:     "pubsub Error with " + alreadyRegisteredMsg,
			err:      ethpubsub.NewError(errors.New(alreadyRegisteredMsg), "register"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ethpubsub.IsAlreadyRegisteredError(tt.err))
		})
	}
}

func TestErrorChaining(t *testing.T) {
	// Test error chaining with multiple wraps
	baseErr := errors.New("network error")
	pubsubErr := ethpubsub.NewError(baseErr, "publish")
	wrappedErr := fmt.Errorf("failed to send message: %w", pubsubErr)

	// Should be able to find the base error through the chain
	assert.True(t, errors.Is(wrappedErr, baseErr))

	// Should be able to extract the pubsub error
	var perr *ethpubsub.Error
	require.True(t, errors.As(wrappedErr, &perr))
	assert.Contains(t, perr.Error(), "publish")
}
