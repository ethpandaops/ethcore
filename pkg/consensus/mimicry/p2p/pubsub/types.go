package pubsub

import (
	"context"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// ValidationResult represents the outcome of message validation
type ValidationResult int

const (
	// ValidationAccept indicates the message should be accepted and propagated
	ValidationAccept ValidationResult = iota
	// ValidationReject indicates the message should be rejected and not propagated
	ValidationReject
	// ValidationIgnore indicates the message should be ignored (not propagated, but not rejected)
	ValidationIgnore
)

// String returns a string representation of the validation result
func (v ValidationResult) String() string {
	switch v {
	case ValidationAccept:
		return "accept"
	case ValidationReject:
		return "reject"
	case ValidationIgnore:
		return "ignore"
	default:
		return "unknown"
	}
}

// Message represents a generic pubsub message with metadata
type Message struct {
	// Topic is the topic this message was received on
	Topic string
	// Data is the raw message data
	Data []byte
	// From is the peer ID of the message sender
	From peer.ID
	// ReceivedTime is when the message was received
	ReceivedTime time.Time
	// Sequence is a unique sequence number for the message
	Sequence uint64
}

// MessageHandler processes received messages for a specific topic
type MessageHandler func(ctx context.Context, msg *Message) error

// Validator validates messages and returns a validation result
// The validator should be fast and non-blocking for optimal performance
type Validator func(ctx context.Context, msg *Message) (ValidationResult, error)

// TopicScoreParams allows configuration of topic-specific scoring parameters
// These parameters are used by the gossipsub protocol to score peers based on their behavior
type TopicScoreParams struct {
	// TopicWeight is the weight of the topic in the overall score
	TopicWeight float64
	// TimeInMeshWeight is the weight for time spent in the mesh
	TimeInMeshWeight float64
	// TimeInMeshQuantum is the time quantum for mesh time calculations
	TimeInMeshQuantum time.Duration
	// TimeInMeshCap is the maximum time-in-mesh score
	TimeInMeshCap float64
	// FirstMessageDeliveriesWeight is the weight for first message deliveries
	FirstMessageDeliveriesWeight float64
	// FirstMessageDeliveriesDecay is the decay factor for first message deliveries
	FirstMessageDeliveriesDecay float64
	// FirstMessageDeliveriesCap is the maximum first message deliveries score
	FirstMessageDeliveriesCap float64
	// InvalidMessageDeliveriesWeight is the weight for invalid message deliveries (negative)
	InvalidMessageDeliveriesWeight float64
	// InvalidMessageDeliveriesDecay is the decay factor for invalid message deliveries
	InvalidMessageDeliveriesDecay float64
}

// DefaultTopicScoreParams returns default topic scoring parameters suitable for Ethereum
func DefaultTopicScoreParams() *TopicScoreParams {
	return &TopicScoreParams{
		TopicWeight:                    0.5,
		TimeInMeshWeight:               1.0,
		TimeInMeshQuantum:              time.Second,
		TimeInMeshCap:                  3600.0, // 1 hour
		FirstMessageDeliveriesWeight:   1.0,
		FirstMessageDeliveriesDecay:    0.99,
		FirstMessageDeliveriesCap:      100.0,
		InvalidMessageDeliveriesWeight: -1.0,
		InvalidMessageDeliveriesDecay:  0.99,
	}
}

// Subscription represents an active topic subscription
type Subscription struct {
	topic        string
	subscription *pubsub.Subscription
	handler      MessageHandler
	cancel       context.CancelFunc
}

// Topic returns the topic name for this subscription
func (s *Subscription) Topic() string {
	return s.topic
}

// PeerScoreSnapshot represents a peer's current score across all topics
type PeerScoreSnapshot struct {
	// PeerID is the peer's identifier
	PeerID peer.ID
	// Score is the overall score for the peer
	Score float64
	// Topics contains per-topic scores
	Topics map[string]float64
	// AppSpecific is the application-specific score component
	AppSpecific float64
	// IPColocation is the IP colocation penalty
	IPColocation float64
	// Behavioural is the behavioral penalty
	Behavioural float64
}

// TopicInfo contains information about a topic's current state
type TopicInfo struct {
	// Name is the topic name
	Name string
	// Peers contains the peer IDs currently subscribed to this topic
	Peers []peer.ID
	// MessageCount is the number of messages received on this topic
	MessageCount uint64
	// LastMessageTime is when the last message was received
	LastMessageTime time.Time
}

// PeerInfo contains information about a peer's pubsub state
type PeerInfo struct {
	// ID is the peer's identifier
	ID peer.ID
	// Topics contains the topics this peer is subscribed to
	Topics []string
	// Score is the peer's current score
	Score float64
	// Connected indicates if the peer is currently connected
	Connected bool
}

// Stats contains runtime statistics for the pubsub system
type Stats struct {
	// TotalMessages is the total number of messages processed
	TotalMessages uint64
	// TotalValidationErrors is the number of validation errors
	TotalValidationErrors uint64
	// TotalHandlerErrors is the number of handler errors
	TotalHandlerErrors uint64
	// ActiveSubscriptions is the number of active subscriptions
	ActiveSubscriptions int
	// ConnectedPeers is the number of connected peers
	ConnectedPeers int
	// TopicCount is the number of topics with active subscriptions
	TopicCount int
}
