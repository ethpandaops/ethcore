package pubsub

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// Processor defines the interface for processing messages from a specific topic
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

	// Subscribe starts subscription to this processor's topic
	Subscribe(ctx context.Context) error

	// Unsubscribe stops subscription to this processor's topic
	Unsubscribe(ctx context.Context) error
}

// MultiProcessor defines the interface for processors that handle multiple related topics
// This is useful for subnet-based topics like beacon_attestation_XX where XX is the subnet ID
type MultiProcessor[T any] interface {
	// AllPossibleTopics returns all topics this processor might handle (for topic scoring prewarming)
	// For subnet-based processors, this returns all possible subnet topics (e.g., all 64 attestation subnets)
	AllPossibleTopics() []string

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

	// Subscribe starts subscription to the specified subnets
	Subscribe(ctx context.Context, subnets []uint64) error

	// Unsubscribe stops subscription to the specified subnets
	Unsubscribe(ctx context.Context, subnets []uint64) error

	// GetActiveSubnets returns the currently subscribed subnets
	GetActiveSubnets() []uint64
}

// ProcessorMetrics tracks performance metrics for message processing
type ProcessorMetrics struct {
	// Processing metrics
	messagesReceived    uint64
	messagesProcessed   uint64
	messagesAccepted    uint64
	messagesRejected    uint64
	messagesIgnored     uint64
	processingErrors    uint64
	validationErrors    uint64
	decodingErrors      uint64

	// Timing metrics
	totalProcessingTime time.Duration
	avgProcessingTime   time.Duration

	// Per-topic metrics (for MultiProcessor)
	topicMetrics map[string]*ProcessorMetrics

	log logrus.FieldLogger
}

// NewProcessorMetrics creates a new ProcessorMetrics instance
func NewProcessorMetrics(log logrus.FieldLogger) *ProcessorMetrics {
	return &ProcessorMetrics{
		topicMetrics: make(map[string]*ProcessorMetrics),
		log:          log.WithField("component", "processor_metrics"),
	}
}

// RecordMessage increments the messages received counter
func (m *ProcessorMetrics) RecordMessage() {
	m.messagesReceived++
}

// RecordProcessed increments the messages processed counter
func (m *ProcessorMetrics) RecordProcessed() {
	m.messagesProcessed++
}

// RecordValidationResult records the outcome of message validation
func (m *ProcessorMetrics) RecordValidationResult(result ValidationResult) {
	switch result {
	case ValidationAccept:
		m.messagesAccepted++
	case ValidationReject:
		m.messagesRejected++
	case ValidationIgnore:
		m.messagesIgnored++
	}
}

// RecordProcessingError increments the processing error counter
func (m *ProcessorMetrics) RecordProcessingError() {
	m.processingErrors++
}

// RecordValidationError increments the validation error counter
func (m *ProcessorMetrics) RecordValidationError() {
	m.validationErrors++
}

// RecordDecodingError increments the decoding error counter
func (m *ProcessorMetrics) RecordDecodingError() {
	m.decodingErrors++
}

// RecordProcessingTime adds processing time to the total and updates average
func (m *ProcessorMetrics) RecordProcessingTime(duration time.Duration) {
	m.totalProcessingTime += duration
	if m.messagesProcessed > 0 {
		m.avgProcessingTime = m.totalProcessingTime / time.Duration(m.messagesProcessed)
	}
}

// GetTopicMetrics returns metrics for a specific topic (creates if not exists)
func (m *ProcessorMetrics) GetTopicMetrics(topic string) *ProcessorMetrics {
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

// GetStats returns current processing statistics
func (m *ProcessorMetrics) GetStats() ProcessorStats {
	return ProcessorStats{
		MessagesReceived:     m.messagesReceived,
		MessagesProcessed:    m.messagesProcessed,
		MessagesAccepted:     m.messagesAccepted,
		MessagesRejected:     m.messagesRejected,
		MessagesIgnored:      m.messagesIgnored,
		ProcessingErrors:     m.processingErrors,
		ValidationErrors:     m.validationErrors,
		DecodingErrors:       m.decodingErrors,
		TotalProcessingTime:  m.totalProcessingTime,
		AvgProcessingTime:    m.avgProcessingTime,
		TopicCount:           len(m.topicMetrics),
	}
}

// ProcessorStats contains processor performance statistics
type ProcessorStats struct {
	MessagesReceived     uint64
	MessagesProcessed    uint64
	MessagesAccepted     uint64
	MessagesRejected     uint64
	MessagesIgnored      uint64
	ProcessingErrors     uint64
	ValidationErrors     uint64
	DecodingErrors       uint64
	TotalProcessingTime  time.Duration
	AvgProcessingTime    time.Duration
	TopicCount           int
}

// LogStats logs the current processor statistics
func (m *ProcessorMetrics) LogStats() {
	stats := m.GetStats()
	m.log.WithFields(logrus.Fields{
		"messages_received":      stats.MessagesReceived,
		"messages_processed":     stats.MessagesProcessed,
		"messages_accepted":      stats.MessagesAccepted,
		"messages_rejected":      stats.MessagesRejected,
		"messages_ignored":       stats.MessagesIgnored,
		"processing_errors":      stats.ProcessingErrors,
		"validation_errors":      stats.ValidationErrors,
		"decoding_errors":        stats.DecodingErrors,
		"total_processing_time":  stats.TotalProcessingTime,
		"avg_processing_time":    stats.AvgProcessingTime,
		"topic_count":            stats.TopicCount,
	}).Info("processor metrics")
}

// multiProcessorWrapper wraps a MultiProcessor to act as a single-topic Processor
// This allows reusing the single-topic subscription logic for multi-topic processors
type multiProcessorWrapper[T any] struct {
	multiProcessor MultiProcessor[T]
	topic          string
}

// Topic returns the specific topic this wrapper handles
func (w *multiProcessorWrapper[T]) Topic() string {
	return w.topic
}

// AllPossibleTopics returns just this wrapper's topic
func (w *multiProcessorWrapper[T]) AllPossibleTopics() []string {
	return []string{w.topic}
}

// GetTopicScoreParams delegates to the multi-processor
func (w *multiProcessorWrapper[T]) GetTopicScoreParams() *TopicScoreParams {
	return w.multiProcessor.GetTopicScoreParams(w.topic)
}

// Decode delegates to the multi-processor with the specific topic
func (w *multiProcessorWrapper[T]) Decode(ctx context.Context, data []byte) (T, error) {
	return w.multiProcessor.Decode(ctx, w.topic, data)
}

// Validate delegates to the multi-processor with the specific topic
func (w *multiProcessorWrapper[T]) Validate(ctx context.Context, msg T, from string) (ValidationResult, error) {
	return w.multiProcessor.Validate(ctx, w.topic, msg, from)
}

// Process delegates to the multi-processor with the specific topic
func (w *multiProcessorWrapper[T]) Process(ctx context.Context, msg T, from string) error {
	return w.multiProcessor.Process(ctx, w.topic, msg, from)
}

// Subscribe is not used as subscription is managed by SubscribeMultiWithProcessor
func (w *multiProcessorWrapper[T]) Subscribe(ctx context.Context) error {
	// This is handled by the parent SubscribeMultiWithProcessor function
	return nil
}

// Unsubscribe is not used as unsubscription is managed by the gossipsub instance
func (w *multiProcessorWrapper[T]) Unsubscribe(ctx context.Context) error {
	// This is handled by the gossipsub Unsubscribe function
	return nil
}