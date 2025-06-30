package pubsub

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// Processor defines the interface for processing messages from a specific topic.
type Processor[T any] interface {
	// Topic returns the topic this processor handles
	Topic() string

	// AllPossibleTopics returns all topics this processor might handle (for topic scoring prewarming)
	// For single-topic processors, this typically returns []string{Topic()}
	AllPossibleTopics() []string

	// GetTopicScoreParams returns scoring parameters for this processor's topic
	// Return nil to use default scoring or disable scoring for this topic
	GetTopicScoreParams() *TopicScoreParams

	// Decode converts raw message data into the typed message
	Decode(ctx context.Context, data []byte) (T, error)

	// Validate performs validation on the decoded message
	Validate(ctx context.Context, msg T, from string) (ValidationResult, error)

	// Process handles the validated message
	Process(ctx context.Context, msg T, from string) error
}

// This is useful for subnet-based topics like beacon_attestation_XX where XX is the subnet ID.
type MultiProcessor[T any] interface {
	// AllPossibleTopics returns all topics this processor might handle (for topic scoring prewarming)
	// For subnet-based processors, this returns all possible subnet topics (e.g., all 64 attestation subnets)
	AllPossibleTopics() []string

	// ActiveTopics returns the currently active topics this processor is interested in
	// This subset of AllPossibleTopics() determines which topics should have active subscriptions
	ActiveTopics() []string

	// UpdateActiveTopics dynamically updates which topics this processor is interested in
	// Returns the list of topics to subscribe to and unsubscribe from
	UpdateActiveTopics(newActiveTopics []string) (toSubscribe []string, toUnsubscribe []string, err error)

	// GetTopicScoreParams returns scoring parameters for the given topic
	// Return nil to use default scoring or disable scoring for this topic
	GetTopicScoreParams(topic string) *TopicScoreParams

	// TopicIndex returns the index of the topic within this processor's topic set
	TopicIndex(topic string) (int, error)

	// Decode converts raw message data into the typed message for the given topic
	Decode(ctx context.Context, topic string, data []byte) (T, error)

	// Validate performs validation on the decoded message for the given topic
	Validate(ctx context.Context, topic string, msg T, from string) (ValidationResult, error)

	// Process handles the validated message for the given topic
	Process(ctx context.Context, topic string, msg T, from string) error
}


// ProcessorMetrics tracks performance metrics for message processing.
type ProcessorMetrics struct {
	// Processing metrics (use atomic operations)
	messagesReceived  uint64
	messagesProcessed uint64
	messagesAccepted  uint64
	messagesRejected  uint64
	messagesIgnored   uint64
	processingErrors  uint64
	validationErrors  uint64
	decodingErrors    uint64

	// Timing metrics (protected by mutex)
	mu                  sync.RWMutex
	totalProcessingTime time.Duration
	avgProcessingTime   time.Duration

	// Per-topic metrics (for MultiProcessor)
	topicMetrics map[string]*ProcessorMetrics

	log logrus.FieldLogger
}

// NewProcessorMetrics creates a new ProcessorMetrics instance.
func NewProcessorMetrics(log logrus.FieldLogger) *ProcessorMetrics {
	return &ProcessorMetrics{
		topicMetrics: make(map[string]*ProcessorMetrics),
		log:          log.WithField("component", "processor_metrics"),
	}
}

// RecordMessage increments the messages received counter.
func (m *ProcessorMetrics) RecordMessage() {
	atomic.AddUint64(&m.messagesReceived, 1)
}

// RecordProcessed increments the messages processed counter.
func (m *ProcessorMetrics) RecordProcessed() {
	atomic.AddUint64(&m.messagesProcessed, 1)
}

// RecordValidationResult records the outcome of message validation.
func (m *ProcessorMetrics) RecordValidationResult(result ValidationResult) {
	switch result {
	case ValidationAccept:
		atomic.AddUint64(&m.messagesAccepted, 1)
	case ValidationReject:
		atomic.AddUint64(&m.messagesRejected, 1)
	case ValidationIgnore:
		atomic.AddUint64(&m.messagesIgnored, 1)
	}
}

// RecordProcessingError increments the processing error counter.
func (m *ProcessorMetrics) RecordProcessingError() {
	atomic.AddUint64(&m.processingErrors, 1)
}

// RecordValidationError increments the validation error counter.
func (m *ProcessorMetrics) RecordValidationError() {
	atomic.AddUint64(&m.validationErrors, 1)
}

// RecordDecodingError increments the decoding error counter.
func (m *ProcessorMetrics) RecordDecodingError() {
	atomic.AddUint64(&m.decodingErrors, 1)
}

// RecordProcessingTime adds processing time to the total and updates average.
func (m *ProcessorMetrics) RecordProcessingTime(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalProcessingTime += duration
	processed := atomic.LoadUint64(&m.messagesProcessed)

	if processed > 0 && processed <= uint64(9223372036854775807) { // Check for overflow (max int64)
		m.avgProcessingTime = m.totalProcessingTime / time.Duration(processed)
	}
}

// GetTopicMetrics returns metrics for a specific topic (creates if not exists).
func (m *ProcessorMetrics) GetTopicMetrics(topic string) *ProcessorMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	if metrics, exists := m.topicMetrics[topic]; exists {
		return metrics
	}

	topicMetrics := &ProcessorMetrics{
		topicMetrics: make(map[string]*ProcessorMetrics),
		log:          m.log.WithField("topic", topic),
	}
	m.topicMetrics[topic] = topicMetrics

	return topicMetrics
}

// GetStats returns current processing statistics.
func (m *ProcessorMetrics) GetStats() ProcessorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ProcessorStats{
		MessagesReceived:    atomic.LoadUint64(&m.messagesReceived),
		MessagesProcessed:   atomic.LoadUint64(&m.messagesProcessed),
		MessagesAccepted:    atomic.LoadUint64(&m.messagesAccepted),
		MessagesRejected:    atomic.LoadUint64(&m.messagesRejected),
		MessagesIgnored:     atomic.LoadUint64(&m.messagesIgnored),
		ProcessingErrors:    atomic.LoadUint64(&m.processingErrors),
		ValidationErrors:    atomic.LoadUint64(&m.validationErrors),
		DecodingErrors:      atomic.LoadUint64(&m.decodingErrors),
		TotalProcessingTime: m.totalProcessingTime,
		AvgProcessingTime:   m.avgProcessingTime,
		TopicCount:          len(m.topicMetrics),
	}
}

// ProcessorStats contains processor performance statistics.
type ProcessorStats struct {
	MessagesReceived    uint64
	MessagesProcessed   uint64
	MessagesAccepted    uint64
	MessagesRejected    uint64
	MessagesIgnored     uint64
	ProcessingErrors    uint64
	ValidationErrors    uint64
	DecodingErrors      uint64
	TotalProcessingTime time.Duration
	AvgProcessingTime   time.Duration
	TopicCount          int
}

// LogStats logs the current processor statistics.
func (m *ProcessorMetrics) LogStats() {
	stats := m.GetStats()
	m.log.WithFields(logrus.Fields{
		"messages_received":     stats.MessagesReceived,
		"messages_processed":    stats.MessagesProcessed,
		"messages_accepted":     stats.MessagesAccepted,
		"messages_rejected":     stats.MessagesRejected,
		"messages_ignored":      stats.MessagesIgnored,
		"processing_errors":     stats.ProcessingErrors,
		"validation_errors":     stats.ValidationErrors,
		"decoding_errors":       stats.DecodingErrors,
		"total_processing_time": stats.TotalProcessingTime,
		"avg_processing_time":   stats.AvgProcessingTime,
		"topic_count":           stats.TopicCount,
	}).Info("processor metrics")
}

// This allows reusing the single-topic subscription logic for multi-topic processors.
type multiProcessorWrapper[T any] struct {
	multiProcessor MultiProcessor[T]
	topic          string
}

// Topic returns the specific topic this wrapper handles.
func (w *multiProcessorWrapper[T]) Topic() string {
	return w.topic
}

// AllPossibleTopics returns just this wrapper's topic.
func (w *multiProcessorWrapper[T]) AllPossibleTopics() []string {
	return []string{w.topic}
}

// GetTopicScoreParams delegates to the multi-processor.
func (w *multiProcessorWrapper[T]) GetTopicScoreParams() *TopicScoreParams {
	return w.multiProcessor.GetTopicScoreParams(w.topic)
}

// Decode delegates to the multi-processor with the specific topic.
func (w *multiProcessorWrapper[T]) Decode(ctx context.Context, data []byte) (T, error) {
	return w.multiProcessor.Decode(ctx, w.topic, data)
}

// Validate delegates to the multi-processor with the specific topic.
func (w *multiProcessorWrapper[T]) Validate(ctx context.Context, msg T, from string) (ValidationResult, error) {
	return w.multiProcessor.Validate(ctx, w.topic, msg, from)
}

// Process delegates to the multi-processor with the specific topic.
func (w *multiProcessorWrapper[T]) Process(ctx context.Context, msg T, from string) error {
	return w.multiProcessor.Process(ctx, w.topic, msg, from)
}


