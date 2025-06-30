package pubsub

import (
	"errors"
	"fmt"
	"strings"
)

// Error wraps an error with additional context for pubsub operations.
type Error struct {
	err     error
	context string
	topic   string
}

// NewError creates a new Error with context.
func NewError(err error, context string) *Error {
	return &Error{
		err:     err,
		context: context,
	}
}

// NewTopicError creates a new Error with topic and context.
func NewTopicError(err error, topic string, context string) *Error {
	return &Error{
		err:     err,
		context: context,
		topic:   topic,
	}
}

// Error returns the error message.
func (e *Error) Error() string {
	if e.topic != "" {
		return fmt.Sprintf("pubsub %s [topic: %s]: %v", e.context, e.topic, e.err)
	}

	return fmt.Sprintf("pubsub %s: %v", e.context, e.err)
}

// Unwrap returns the wrapped error.
func (e *Error) Unwrap() error {
	return e.err
}

// Topic returns the topic associated with the error, if any.
func (e *Error) Topic() string {
	return e.topic
}

// Predefined error variables for common error conditions.
var (
	ErrNotStarted             = errors.New("pubsub not started")
	ErrAlreadyStarted         = errors.New("pubsub already started")
	ErrInvalidTopic           = errors.New("invalid topic")
	ErrTopicNotSubscribed     = errors.New("topic not subscribed")
	ErrTopicAlreadySubscribed = errors.New("topic already subscribed")
	ErrMessageTooLarge        = errors.New("message exceeds maximum size")
	ErrValidationFailed       = errors.New("message validation failed")
	ErrPublishTimeout         = errors.New("publish operation timed out")
	ErrSubscriptionClosed     = errors.New("subscription closed")
	ErrPeerNotFound           = errors.New("peer not found")
	ErrContextCanceled        = errors.New("context canceled")
	ErrAlreadyRegistered      = errors.New("already registered")
)

// IsAlreadyRegisteredError checks if the error indicates a processor is already registered.
func IsAlreadyRegisteredError(err error) bool {
	if err == nil {
		return false
	}
	// Check if it's our error type and contains "already registered"
	return strings.Contains(err.Error(), "already registered")
}
