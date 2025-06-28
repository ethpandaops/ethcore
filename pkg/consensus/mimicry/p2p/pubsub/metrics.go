package pubsub

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics contains prometheus metrics for the pubsub system.
type Metrics struct {
	// Message metrics
	MessagesReceived  *prometheus.CounterVec
	MessagesPublished *prometheus.CounterVec
	MessagesValidated *prometheus.CounterVec
	MessagesHandled   *prometheus.CounterVec

	// Error metrics
	ValidationErrors *prometheus.CounterVec
	HandlerErrors    *prometheus.CounterVec
	PublishErrors    *prometheus.CounterVec

	// Performance metrics
	ValidationDuration *prometheus.HistogramVec
	HandlerDuration    *prometheus.HistogramVec
	PublishDuration    *prometheus.HistogramVec

	// State metrics
	ActiveSubscriptions *prometheus.GaugeVec
	PeersPerTopic       *prometheus.GaugeVec
	MessageQueueSize    *prometheus.GaugeVec

	// Peer metrics
	PeerScores     *prometheus.GaugeVec
	PeersConnected prometheus.Gauge
}

// NewMetrics creates a new Metrics instance with the given namespace.
func NewMetrics(namespace string) *Metrics {
	return &Metrics{
		// Message metrics
		MessagesReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_received_total",
				Help:      "Total number of messages received by topic",
			},
			[]string{"topic"},
		),
		MessagesPublished: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_published_total",
				Help:      "Total number of messages published by topic and status",
			},
			[]string{"topic", "status"},
		),
		MessagesValidated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_validated_total",
				Help:      "Total number of messages validated by topic and result",
			},
			[]string{"topic", "result"},
		),
		MessagesHandled: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_handled_total",
				Help:      "Total number of messages handled by topic and status",
			},
			[]string{"topic", "status"},
		),

		// Error metrics
		ValidationErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "validation_errors_total",
				Help:      "Total number of validation errors by topic",
			},
			[]string{"topic"},
		),
		HandlerErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "handler_errors_total",
				Help:      "Total number of handler errors by topic",
			},
			[]string{"topic"},
		),
		PublishErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "publish_errors_total",
				Help:      "Total number of publish errors by topic",
			},
			[]string{"topic"},
		),

		// Performance metrics
		ValidationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "validation_duration_seconds",
				Help:      "Time spent validating messages by topic",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"topic"},
		),
		HandlerDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "handler_duration_seconds",
				Help:      "Time spent handling messages by topic",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"topic"},
		),
		PublishDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "publish_duration_seconds",
				Help:      "Time spent publishing messages by topic",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"topic"},
		),

		// State metrics
		ActiveSubscriptions: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_subscriptions",
				Help:      "Number of active subscriptions",
			},
			[]string{},
		),
		PeersPerTopic: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "peers_per_topic",
				Help:      "Number of peers per topic",
			},
			[]string{"topic"},
		),
		MessageQueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "message_queue_size",
				Help:      "Size of message queue by topic",
			},
			[]string{"topic"},
		),

		// Peer metrics
		PeerScores: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "peer_scores",
				Help:      "Peer scores by peer ID",
			},
			[]string{"peer_id"},
		),
		PeersConnected: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "peers_connected",
				Help:      "Number of connected peers",
			},
		),
	}
}

// Register registers all metrics with the given prometheus registry.
func (m *Metrics) Register(registry *prometheus.Registry) error {
	collectors := []prometheus.Collector{
		m.MessagesReceived,
		m.MessagesPublished,
		m.MessagesValidated,
		m.MessagesHandled,
		m.ValidationErrors,
		m.HandlerErrors,
		m.PublishErrors,
		m.ValidationDuration,
		m.HandlerDuration,
		m.PublishDuration,
		m.ActiveSubscriptions,
		m.PeersPerTopic,
		m.MessageQueueSize,
		m.PeerScores,
		m.PeersConnected,
	}

	for _, collector := range collectors {
		if err := registry.Register(collector); err != nil {
			return err
		}
	}

	return nil
}

// Metric update methods.
func (m *Metrics) RecordMessageReceived(topic string) {
	if m.MessagesReceived != nil {
		m.MessagesReceived.WithLabelValues(topic).Inc()
	}
}

func (m *Metrics) RecordMessagePublished(topic string, success bool) {
	if m.MessagesPublished != nil {
		status := "success"
		if !success {
			status = "error"
		}

		m.MessagesPublished.WithLabelValues(topic, status).Inc()
	}
}

func (m *Metrics) RecordMessageValidated(topic string, result ValidationResult) {
	if m.MessagesValidated != nil {
		m.MessagesValidated.WithLabelValues(topic, result.String()).Inc()
	}
}

func (m *Metrics) RecordMessageHandled(topic string, success bool, duration time.Duration) {
	if m.MessagesHandled != nil {
		status := "success"
		if !success {
			status = "error"
		}

		m.MessagesHandled.WithLabelValues(topic, status).Inc()
	}

	if m.HandlerDuration != nil {
		m.HandlerDuration.WithLabelValues(topic).Observe(duration.Seconds())
	}
}

func (m *Metrics) RecordValidationError(topic string) {
	if m.ValidationErrors != nil {
		m.ValidationErrors.WithLabelValues(topic).Inc()
	}
}

func (m *Metrics) RecordHandlerError(topic string) {
	if m.HandlerErrors != nil {
		m.HandlerErrors.WithLabelValues(topic).Inc()
	}
}

func (m *Metrics) RecordPublishError(topic string) {
	if m.PublishErrors != nil {
		m.PublishErrors.WithLabelValues(topic).Inc()
	}
}

func (m *Metrics) RecordValidationDuration(topic string, duration time.Duration) {
	if m.ValidationDuration != nil {
		m.ValidationDuration.WithLabelValues(topic).Observe(duration.Seconds())
	}
}

func (m *Metrics) RecordHandlerDuration(topic string, duration time.Duration) {
	if m.HandlerDuration != nil {
		m.HandlerDuration.WithLabelValues(topic).Observe(duration.Seconds())
	}
}

func (m *Metrics) RecordPublishDuration(topic string, duration time.Duration) {
	if m.PublishDuration != nil {
		m.PublishDuration.WithLabelValues(topic).Observe(duration.Seconds())
	}
}

func (m *Metrics) SetActiveSubscriptions(count int) {
	if m.ActiveSubscriptions != nil {
		m.ActiveSubscriptions.WithLabelValues().Set(float64(count))
	}
}

func (m *Metrics) SetPeersPerTopic(topic string, count int) {
	if m.PeersPerTopic != nil {
		m.PeersPerTopic.WithLabelValues(topic).Set(float64(count))
	}
}

func (m *Metrics) SetMessageQueueSize(topic string, size int) {
	if m.MessageQueueSize != nil {
		m.MessageQueueSize.WithLabelValues(topic).Set(float64(size))
	}
}

func (m *Metrics) SetPeerScore(peerID string, score float64) {
	if m.PeerScores != nil {
		m.PeerScores.WithLabelValues(peerID).Set(score)
	}
}

func (m *Metrics) SetPeersConnected(count int) {
	if m.PeersConnected != nil {
		m.PeersConnected.Set(float64(count))
	}
}
