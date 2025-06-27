package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chuckpreslar/emission"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// ValidationPipeline manages message validation for gossipsub topics
type ValidationPipeline struct {
	log        logrus.FieldLogger
	validators map[string]Validator
	metrics    *Metrics
	emitter    *emission.Emitter
	mutex      sync.RWMutex
}

// newValidationPipeline creates a new validation pipeline
func newValidationPipeline(log logrus.FieldLogger, metrics *Metrics, emitter *emission.Emitter) *ValidationPipeline {
	return &ValidationPipeline{
		log:        log.WithField("component", "validation"),
		validators: make(map[string]Validator),
		metrics:    metrics,
		emitter:    emitter,
	}
}

// addValidator registers a validator for a specific topic
func (vp *ValidationPipeline) addValidator(topic string, validator Validator) {
	vp.mutex.Lock()
	defer vp.mutex.Unlock()

	vp.validators[topic] = validator
	vp.log.WithField("topic", topic).Debug("Validator added for topic")
}

// removeValidator removes a validator for a specific topic
func (vp *ValidationPipeline) removeValidator(topic string) {
	vp.mutex.Lock()
	defer vp.mutex.Unlock()

	delete(vp.validators, topic)
	vp.log.WithField("topic", topic).Debug("Validator removed for topic")
}

// getValidator retrieves a validator for a specific topic
func (vp *ValidationPipeline) getValidator(topic string) Validator {
	vp.mutex.RLock()
	defer vp.mutex.RUnlock()

	return vp.validators[topic]
}

// createLibp2pValidator creates a libp2p-compatible validator function for a topic
func (vp *ValidationPipeline) createLibp2pValidator(topic string) pubsub.ValidatorEx {
	return func(ctx context.Context, peerID peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		start := time.Now()
		defer func() {
			duration := time.Since(start)
			if vp.metrics != nil {
				vp.metrics.RecordValidationDuration(topic, duration)
			}
		}()

		// Get the validator for this topic
		validator := vp.getValidator(topic)
		if validator == nil {
			// No validator registered - accept by default
			vp.log.WithField("topic", topic).Debug("No validator registered, accepting message")
			return pubsub.ValidationAccept
		}

		// Convert libp2p message to our generic message format
		genericMsg := &Message{
			Topic:        topic,
			Data:         msg.Data,
			From:         peerID,
			ReceivedTime: time.Now(),
			Sequence:     uint64(len(msg.Data)), // Simple sequence based on data length
		}

		// Run validation
		result, err := vp.validateMessage(ctx, topic, genericMsg, validator)

		// Record metrics and emit events
		if vp.metrics != nil {
			vp.metrics.RecordMessageValidated(topic, result)
		}

		if err != nil {
			vp.log.WithError(err).WithFields(logrus.Fields{
				"topic":  topic,
				"peer":   peerID,
				"result": result.String(),
			}).Warn("Message validation failed")

			if vp.metrics != nil {
				vp.metrics.RecordValidationError(topic)
			}

			// Emit validation failed event
			vp.emitter.Emit(ValidationFailedEvent, topic, peerID, err)
		} else {
			vp.log.WithFields(logrus.Fields{
				"topic":  topic,
				"peer":   peerID,
				"result": result.String(),
			}).Debug("Message validation completed")
		}

		// Emit validation completed event
		vp.emitter.Emit(MessageValidatedEvent, topic, result)

		// Convert our validation result to libp2p result
		return convertValidationResult(result)
	}
}

// validateMessage runs validation on a message using the appropriate validator
func (vp *ValidationPipeline) validateMessage(ctx context.Context, topic string, msg *Message, validator Validator) (ValidationResult, error) {
	if validator == nil {
		return ValidationAccept, nil
	}

	// Create a timeout context for validation
	validationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Run the validator
	result, err := validator(validationCtx, msg)
	if err != nil {
		return ValidationReject, fmt.Errorf("validation error: %w", err)
	}

	return result, nil
}

// convertValidationResult converts our ValidationResult to libp2p's ValidationResult
func convertValidationResult(result ValidationResult) pubsub.ValidationResult {
	switch result {
	case ValidationAccept:
		return pubsub.ValidationAccept
	case ValidationReject:
		return pubsub.ValidationReject
	case ValidationIgnore:
		return pubsub.ValidationIgnore
	default:
		// Default to reject for unknown results
		return pubsub.ValidationReject
	}
}

// Validation helper methods for common validation patterns

// ValidateMessageSize creates a validator that checks message size limits
func ValidateMessageSize(maxSize int) Validator {
	return func(ctx context.Context, msg *Message) (ValidationResult, error) {
		if len(msg.Data) > maxSize {
			return ValidationReject, fmt.Errorf("message size %d exceeds limit %d", len(msg.Data), maxSize)
		}
		return ValidationAccept, nil
	}
}

// ValidateNonEmpty creates a validator that rejects empty messages
func ValidateNonEmpty() Validator {
	return func(ctx context.Context, msg *Message) (ValidationResult, error) {
		if len(msg.Data) == 0 {
			return ValidationReject, fmt.Errorf("message is empty")
		}
		return ValidationAccept, nil
	}
}

// CombineValidators creates a validator that runs multiple validators in sequence
// If any validator rejects or returns an error, the combined validator fails
func CombineValidators(validators ...Validator) Validator {
	return func(ctx context.Context, msg *Message) (ValidationResult, error) {
		for i, validator := range validators {
			if validator == nil {
				continue
			}

			result, err := validator(ctx, msg)
			if err != nil {
				return ValidationReject, fmt.Errorf("validator %d failed: %w", i, err)
			}

			switch result {
			case ValidationReject:
				return ValidationReject, fmt.Errorf("validator %d rejected message", i)
			case ValidationIgnore:
				return ValidationIgnore, nil
			}
			// Continue if ValidationAccept
		}

		return ValidationAccept, nil
	}
}
