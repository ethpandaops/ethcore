package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chuckpreslar/emission"
	"github.com/sirupsen/logrus"
)

// handlerRegistry manages message handlers for different topics
type handlerRegistry struct {
	log      logrus.FieldLogger
	handlers map[string]MessageHandler
	metrics  *Metrics
	emitter  *emission.Emitter
	mutex    sync.RWMutex
}

// newHandlerRegistry creates a new message handler registry
func newHandlerRegistry(log logrus.FieldLogger, metrics *Metrics, emitter *emission.Emitter) *handlerRegistry {
	return &handlerRegistry{
		log:      log.WithField("component", "handlers"),
		handlers: make(map[string]MessageHandler),
		metrics:  metrics,
		emitter:  emitter,
	}
}

// register adds a message handler for a specific topic
func (hr *handlerRegistry) register(topic string, handler MessageHandler) {
	if handler == nil {
		hr.log.WithField("topic", topic).Warn("Attempted to register nil handler")
		return
	}

	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	hr.handlers[topic] = handler
	hr.log.WithField("topic", topic).Debug("Message handler registered")
}

// unregister removes a message handler for a specific topic
func (hr *handlerRegistry) unregister(topic string) {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	delete(hr.handlers, topic)
	hr.log.WithField("topic", topic).Debug("Message handler unregistered")
}

// get retrieves a message handler for a specific topic
func (hr *handlerRegistry) get(topic string) MessageHandler {
	hr.mutex.RLock()
	defer hr.mutex.RUnlock()

	return hr.handlers[topic]
}

// processMessage processes a message using the appropriate handler
func (hr *handlerRegistry) processMessage(ctx context.Context, msg *Message) error {
	start := time.Now()

	handler := hr.get(msg.Topic)
	if handler == nil {
		err := fmt.Errorf("no handler registered for topic: %s", msg.Topic)
		hr.log.WithField("topic", msg.Topic).Debug(err.Error())

		if hr.metrics != nil {
			hr.metrics.RecordHandlerError(msg.Topic)
		}

		hr.emitter.Emit(HandlerErrorEvent, msg.Topic, err)
		return err
	}

	// Create a timeout context for message handling
	handlerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Process the message
	err := hr.safeProcessMessage(handlerCtx, msg, handler)

	// Record metrics
	duration := time.Since(start)
	success := err == nil

	if hr.metrics != nil {
		hr.metrics.RecordMessageHandled(msg.Topic, success, duration)
		hr.metrics.RecordHandlerDuration(msg.Topic, duration)

		if !success {
			hr.metrics.RecordHandlerError(msg.Topic)
		}
	}

	// Emit events
	hr.emitter.Emit(MessageHandledEvent, msg.Topic, success)

	if err != nil {
		hr.log.WithError(err).WithFields(logrus.Fields{
			"topic":    msg.Topic,
			"from":     msg.From,
			"duration": duration,
		}).Error("Message handler failed")

		hr.emitter.Emit(HandlerErrorEvent, msg.Topic, err)
	} else {
		hr.log.WithFields(logrus.Fields{
			"topic":    msg.Topic,
			"from":     msg.From,
			"duration": duration,
		}).Debug("Message processed successfully")
	}

	return err
}

// safeProcessMessage safely executes a message handler with panic recovery
func (hr *handlerRegistry) safeProcessMessage(ctx context.Context, msg *Message, handler MessageHandler) (err error) {
	// Recover from panics in message handlers
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panicked: %v", r)
			hr.log.WithFields(logrus.Fields{
				"topic": msg.Topic,
				"from":  msg.From,
				"panic": r,
			}).Error("Message handler panicked")
		}
	}()

	// Execute the handler
	return handler(ctx, msg)
}

// Handler utility functions

// LoggingHandler wraps a handler to add automatic logging
func LoggingHandler(log logrus.FieldLogger, handler MessageHandler) MessageHandler {
	return func(ctx context.Context, msg *Message) error {
		start := time.Now()

		log.WithFields(logrus.Fields{
			"topic": msg.Topic,
			"from":  msg.From,
			"size":  len(msg.Data),
		}).Debug("Processing message")

		err := handler(ctx, msg)

		duration := time.Since(start)
		logEntry := log.WithFields(logrus.Fields{
			"topic":    msg.Topic,
			"from":     msg.From,
			"duration": duration,
		})

		if err != nil {
			logEntry.WithError(err).Error("Message processing failed")
		} else {
			logEntry.Debug("Message processed successfully")
		}

		return err
	}
}

// MetricsHandler wraps a handler to add automatic metrics collection
func MetricsHandler(metrics *Metrics, handler MessageHandler) MessageHandler {
	return func(ctx context.Context, msg *Message) error {
		start := time.Now()

		err := handler(ctx, msg)

		duration := time.Since(start)
		success := err == nil

		if metrics != nil {
			metrics.RecordMessageHandled(msg.Topic, success, duration)
			metrics.RecordHandlerDuration(msg.Topic, duration)

			if !success {
				metrics.RecordHandlerError(msg.Topic)
			}
		}

		return err
	}
}

// AsyncHandler wraps a handler to process messages asynchronously
// This can improve throughput but messages may be processed out of order
func AsyncHandler(log logrus.FieldLogger, handler MessageHandler) MessageHandler {
	return func(ctx context.Context, msg *Message) error {
		// Create a copy of the message for async processing
		msgCopy := &Message{
			Topic:        msg.Topic,
			Data:         make([]byte, len(msg.Data)),
			From:         msg.From,
			ReceivedTime: msg.ReceivedTime,
			Sequence:     msg.Sequence,
		}
		copy(msgCopy.Data, msg.Data)

		// Process asynchronously
		go func() {
			if err := handler(ctx, msgCopy); err != nil {
				log.WithError(err).WithFields(logrus.Fields{
					"topic": msgCopy.Topic,
					"from":  msgCopy.From,
				}).Error("Async message processing failed")
			}
		}()

		return nil
	}
}

// FilterHandler wraps a handler with a filter function
// Messages that don't pass the filter are ignored
func FilterHandler(filter func(*Message) bool, handler MessageHandler) MessageHandler {
	return func(ctx context.Context, msg *Message) error {
		if !filter(msg) {
			return nil // Silently ignore filtered messages
		}
		return handler(ctx, msg)
	}
}
