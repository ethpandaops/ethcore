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

// ProcessorSubscription manages a subscription with a processor for a specific topic type
type ProcessorSubscription[T any] struct {
	processor    Processor[T]
	subscription *pubsub.Subscription
	metrics      *ProcessorMetrics
	emitter      *emission.Emitter
	log          logrus.FieldLogger

	// Lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
	stopped bool
	mu      sync.RWMutex
}

// newProcessorSubscription creates a new processor-based subscription
func newProcessorSubscription[T any](
	processor Processor[T],
	subscription *pubsub.Subscription,
	metrics *ProcessorMetrics,
	emitter *emission.Emitter,
	log logrus.FieldLogger,
) *ProcessorSubscription[T] {
	ctx, cancel := context.WithCancel(context.Background())

	return &ProcessorSubscription[T]{
		processor:    processor,
		subscription: subscription,
		metrics:      metrics,
		emitter:      emitter,
		log: log.WithFields(logrus.Fields{
			"component": "processor_subscription",
			"topic":     processor.Topic(),
		}),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins processing messages for this subscription
func (ps *ProcessorSubscription[T]) Start() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.started {
		return fmt.Errorf("subscription already started")
	}

	if ps.stopped {
		return fmt.Errorf("subscription has been stopped")
	}

	ps.log.Debug("Starting processor subscription")

	// Start the processing loop
	ps.wg.Add(1)
	go ps.processLoop()

	ps.started = true

	ps.log.Info("Processor subscription started")
	return nil
}

// Stop halts message processing gracefully
func (ps *ProcessorSubscription[T]) Stop() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.stopped {
		return nil
	}

	if !ps.started {
		ps.stopped = true
		return nil
	}

	ps.log.Debug("Stopping processor subscription")

	// Cancel context to signal goroutines to stop
	ps.cancel()

	// Wait for all goroutines to complete
	ps.wg.Wait()

	ps.stopped = true

	ps.log.Info("Processor subscription stopped")
	return nil
}

// processLoop handles incoming messages in a goroutine
func (ps *ProcessorSubscription[T]) processLoop() {
	defer ps.wg.Done()

	ps.log.Debug("Starting message processing loop")

	for {
		select {
		case <-ps.ctx.Done():
			ps.log.Debug("Processing loop terminated by context cancellation")
			return
		default:
			// Get next message from subscription
			msg, err := ps.subscription.Next(ps.ctx)
			if err != nil {
				if ps.ctx.Err() != nil {
					// Context was cancelled, this is expected
					ps.log.Debug("Message subscription closed due to context cancellation")
					return
				}

				ps.log.WithError(err).Error("Failed to get next message from subscription")
				// Continue processing despite error
				continue
			}

			// Handle the message
			if err := ps.handleMessage(ps.ctx, msg); err != nil {
				ps.log.WithError(err).Error("Failed to handle message")
				// Continue processing despite error
			}
		}
	}
}

// handleMessage processes individual messages
func (ps *ProcessorSubscription[T]) handleMessage(ctx context.Context, msg *pubsub.Message) error {
	start := time.Now()

	topic := ps.processor.Topic()
	peerID := msg.GetFrom()

	ps.log.WithFields(logrus.Fields{
		"peer":         peerID,
		"message_size": len(msg.Data),
	}).Debug("Processing message")

	// Record message received
	if ps.metrics != nil {
		ps.metrics.RecordMessage()
	}

	// Emit message received event
	if ps.emitter != nil {
		ps.emitter.Emit(MessageReceivedEvent, topic, peerID)
	}

	// Decode the message (first decode)
	decodedMsg, err := ps.processor.Decode(ctx, msg.Data)
	if err != nil {
		ps.log.WithError(err).WithField("peer", peerID).Warn("Failed to decode message")

		if ps.metrics != nil {
			ps.metrics.RecordDecodingError()
		}

		return fmt.Errorf("decode error: %w", err)
	}

	// Validate the message
	validationResult, err := ps.processor.Validate(ctx, decodedMsg, peerID.String())
	if err != nil {
		ps.log.WithError(err).WithField("peer", peerID).Warn("Message validation failed")

		if ps.metrics != nil {
			ps.metrics.RecordValidationError()
		}

		if ps.emitter != nil {
			ps.emitter.Emit(ValidationFailedEvent, topic, peerID, err)
		}

		return fmt.Errorf("validation error: %w", err)
	}

	// Record validation result
	if ps.metrics != nil {
		ps.metrics.RecordValidationResult(validationResult)
	}

	// Only process accepted messages
	if validationResult != ValidationAccept {
		ps.log.WithFields(logrus.Fields{
			"peer":   peerID,
			"result": validationResult.String(),
		}).Debug("Message not accepted, skipping processing")

		return nil
	}

	// Process the message
	if err := ps.processor.Process(ctx, decodedMsg, peerID.String()); err != nil {
		ps.log.WithError(err).WithField("peer", peerID).Error("Failed to process message")

		if ps.metrics != nil {
			ps.metrics.RecordProcessingError()
		}

		if ps.emitter != nil {
			ps.emitter.Emit(HandlerErrorEvent, topic, err)
		}

		return fmt.Errorf("processing error: %w", err)
	}

	// Record successful processing
	processingDuration := time.Since(start)

	if ps.metrics != nil {
		ps.metrics.RecordProcessed()
		ps.metrics.RecordProcessingTime(processingDuration)
	}

	if ps.emitter != nil {
		ps.emitter.Emit(MessageHandledEvent, topic, true)
	}

	ps.log.WithFields(logrus.Fields{
		"peer":               peerID,
		"processing_time_ms": processingDuration.Milliseconds(),
	}).Debug("Message processed successfully")

	return nil
}

// createValidator creates a libp2p pubsub validator function for this processor
func (ps *ProcessorSubscription[T]) createValidator() pubsub.ValidatorEx {
	return func(ctx context.Context, peerID peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		start := time.Now()

		topic := ps.processor.Topic()

		ps.log.WithFields(logrus.Fields{
			"peer":         peerID,
			"message_size": len(msg.Data),
		}).Debug("Validating message")

		// Decode the message (double-decode pattern: decode in validator, decode again in handler)
		// We don't cache the result to ensure clean separation between validation and processing
		decodedMsg, err := ps.processor.Decode(ctx, msg.Data)
		if err != nil {
			ps.log.WithError(err).WithField("peer", peerID).Debug("Failed to decode message during validation")

			if ps.metrics != nil {
				ps.metrics.RecordDecodingError()
				ps.metrics.RecordValidationResult(ValidationReject)
			}

			return pubsub.ValidationReject
		}

		// Validate the message
		validationResult, err := ps.processor.Validate(ctx, decodedMsg, peerID.String())
		if err != nil {
			ps.log.WithError(err).WithField("peer", peerID).Debug("Message validation failed")

			if ps.metrics != nil {
				ps.metrics.RecordValidationError()
				ps.metrics.RecordValidationResult(ValidationReject)
			}

			if ps.emitter != nil {
				ps.emitter.Emit(ValidationFailedEvent, topic, peerID, err)
			}

			return pubsub.ValidationReject
		}

		// Record validation timing
		validationDuration := time.Since(start)

		if ps.metrics != nil {
			ps.metrics.RecordValidationResult(validationResult)
		}

		if ps.emitter != nil {
			ps.emitter.Emit(MessageValidatedEvent, topic, validationResult)
		}

		ps.log.WithFields(logrus.Fields{
			"peer":               peerID,
			"result":             validationResult.String(),
			"validation_time_ms": validationDuration.Milliseconds(),
		}).Debug("Message validation completed")

		// Convert our validation result to libp2p result
		switch validationResult {
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
}

// Metrics returns current processing metrics
func (ps *ProcessorSubscription[T]) Metrics() ProcessorStats {
	if ps.metrics == nil {
		return ProcessorStats{}
	}

	return ps.metrics.GetStats()
}

// Topic returns the topic this subscription handles
func (ps *ProcessorSubscription[T]) Topic() string {
	return ps.processor.Topic()
}

// IsStarted returns whether the subscription is currently started
func (ps *ProcessorSubscription[T]) IsStarted() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.started && !ps.stopped
}

// IsStopped returns whether the subscription has been stopped
func (ps *ProcessorSubscription[T]) IsStopped() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.stopped
}
