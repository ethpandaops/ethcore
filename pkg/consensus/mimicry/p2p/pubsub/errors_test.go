package pubsub

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewError(t *testing.T) {
	baseErr := errors.New("base error")
	context := "test operation"

	err := NewError(baseErr, context)

	assert.NotNil(t, err)
	assert.Equal(t, baseErr, err.err)
	assert.Equal(t, context, err.context)
	assert.Empty(t, err.topic)
}

func TestNewTopicError(t *testing.T) {
	baseErr := errors.New("topic error")
	topic := "test/topic"
	context := "subscription"

	err := NewTopicError(baseErr, topic, context)

	assert.NotNil(t, err)
	assert.Equal(t, baseErr, err.err)
	assert.Equal(t, context, err.context)
	assert.Equal(t, topic, err.topic)
}

func TestErrorString(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected string
	}{
		{
			name: "error without topic",
			err: &Error{
				err:     errors.New("connection failed"),
				context: "connect",
			},
			expected: "pubsub connect: connection failed",
		},
		{
			name: "error with topic",
			err: &Error{
				err:     errors.New("validation failed"),
				context: "validate",
				topic:   "beacon_block",
			},
			expected: "pubsub validate [topic: beacon_block]: validation failed",
		},
		{
			name: "error with empty context",
			err: &Error{
				err:     errors.New("unknown error"),
				context: "",
			},
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
	err := NewError(baseErr, "operation")

	unwrapped := err.Unwrap()
	assert.Equal(t, baseErr, unwrapped)

	// Test with errors.Is
	assert.True(t, errors.Is(err, baseErr))
}

func TestErrorTopic(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected string
	}{
		{
			name:     "error without topic",
			err:      NewError(errors.New("error"), "context"),
			expected: "",
		},
		{
			name:     "error with topic",
			err:      NewTopicError(errors.New("error"), "test/topic", "context"),
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
	assert.NotNil(t, ErrNotStarted)
	assert.NotNil(t, ErrAlreadyStarted)
	assert.NotNil(t, ErrInvalidTopic)
	assert.NotNil(t, ErrTopicNotSubscribed)
	assert.NotNil(t, ErrTopicAlreadySubscribed)
	assert.NotNil(t, ErrMessageTooLarge)
	assert.NotNil(t, ErrValidationFailed)
	assert.NotNil(t, ErrPublishTimeout)
	assert.NotNil(t, ErrSubscriptionClosed)
	assert.NotNil(t, ErrPeerNotFound)
	assert.NotNil(t, ErrContextCanceled)
	assert.NotNil(t, ErrAlreadyRegistered)

	// Test error messages
	assert.Equal(t, "pubsub not started", ErrNotStarted.Error())
	assert.Equal(t, "pubsub already started", ErrAlreadyStarted.Error())
	assert.Equal(t, "already registered", ErrAlreadyRegistered.Error())
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
			name:     "already registered error",
			err:      ErrAlreadyRegistered,
			expected: true,
		},
		{
			name:     "wrapped already registered error",
			err:      fmt.Errorf("processor already registered for topic test"),
			expected: true,
		},
		{
			name:     "multi-processor already registered",
			err:      fmt.Errorf("multi-processor already registered with name test"),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      errors.New("connection failed"),
			expected: false,
		},
		{
			name:     "pubsub Error with already registered",
			err:      NewError(errors.New("already registered"), "register"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsAlreadyRegisteredError(tt.err))
		})
	}
}

func TestErrorChaining(t *testing.T) {
	// Test error chaining with multiple wraps
	baseErr := errors.New("network error")
	pubsubErr := NewError(baseErr, "publish")
	wrappedErr := fmt.Errorf("failed to send message: %w", pubsubErr)

	// Should be able to find the base error through the chain
	assert.True(t, errors.Is(wrappedErr, baseErr))
	
	// Should be able to extract the pubsub error
	var perr *Error
	require.True(t, errors.As(wrappedErr, &perr))
	assert.Equal(t, "publish", perr.context)
}