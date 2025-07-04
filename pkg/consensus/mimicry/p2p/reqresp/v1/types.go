package v1

import (
	"errors"
	"time"

	"github.com/libp2p/go-libp2p/core/protocol"
)

// Common errors.
var (
	// ErrInvalidRequest indicates the request is malformed or invalid.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrInvalidResponse indicates the response is malformed or invalid.
	ErrInvalidResponse = errors.New("invalid response")
	// ErrStreamReset indicates the stream was reset by the remote peer.
	ErrStreamReset = errors.New("stream reset")
	// ErrTimeout indicates the operation timed out.
	ErrTimeout = errors.New("operation timed out")
	// ErrNoHandler indicates no handler is registered for the protocol.
	ErrNoHandler = errors.New("no handler registered")
	// ErrHandlerExists indicates a handler is already registered for the protocol.
	ErrHandlerExists = errors.New("handler already registered")
	// ErrServiceStopped indicates the service has been stopped.
	ErrServiceStopped = errors.New("service stopped")
	// ErrMaxSizeExceeded indicates the message size exceeds the maximum allowed.
	ErrMaxSizeExceeded = errors.New("max size exceeded")
)

// Status represents a response status code.
type Status uint8

const (
	// StatusSuccess indicates successful processing.
	StatusSuccess Status = 0
	// StatusInvalidRequest indicates the request was invalid.
	StatusInvalidRequest Status = 1
	// StatusServerError indicates a server-side error.
	StatusServerError Status = 2
	// StatusResourceUnavailable indicates the requested resource is unavailable.
	StatusResourceUnavailable Status = 3
	// StatusRateLimited indicates the peer is rate limited.
	StatusRateLimited Status = 4
)

// String returns the string representation of the status.
func (s Status) String() string {
	switch s {
	case StatusSuccess:
		return "success"
	case StatusInvalidRequest:
		return "invalid_request"
	case StatusServerError:
		return "server_error"
	case StatusResourceUnavailable:
		return "resource_unavailable"
	case StatusRateLimited:
		return "rate_limited"
	default:
		return "unknown"
	}
}

// IsError returns true if the status indicates an error.
func (s Status) IsError() bool {
	return s != StatusSuccess
}

// ProtocolConfig contains configuration for a protocol.
type ProtocolConfig struct {
	// ID is the protocol identifier.
	ID protocol.ID
	// Version is the protocol version.
	Version string
	// MaxRequestSize is the maximum allowed request size in bytes.
	MaxRequestSize uint64
	// MaxResponseSize is the maximum allowed response size in bytes.
	MaxResponseSize uint64
	// Timeout is the default timeout for requests.
	Timeout time.Duration
}

// RequestMetadata contains metadata about a request.
type RequestMetadata struct {
	// Protocol is the protocol ID.
	Protocol protocol.ID
	// PeerID is the ID of the requesting peer.
	PeerID string
	// RequestedAt is when the request was received.
	RequestedAt time.Time
	// Size is the size of the request in bytes.
	Size int
}

// ResponseMetadata contains metadata about a response.
type ResponseMetadata struct {
	// Protocol is the protocol ID.
	Protocol protocol.ID
	// PeerID is the ID of the responding peer.
	PeerID string
	// Status is the response status.
	Status Status
	// RespondedAt is when the response was sent.
	RespondedAt time.Time
	// Size is the size of the response in bytes.
	Size int
	// Duration is how long it took to process the request.
	Duration time.Duration
}

// HandlerConfig contains configuration for a request handler.
type HandlerConfig struct {
	// Encoder is used for encoding/decoding messages.
	Encoder Encoder
	// Compressor is used for compressing/decompressing messages.
	Compressor Compressor
	// MaxConcurrentRequests limits concurrent request processing.
	MaxConcurrentRequests int
	// RequestTimeout is the timeout for processing individual requests.
	RequestTimeout time.Duration
	// EnableMetrics enables metrics collection.
	EnableMetrics bool
}

// ClientConfig contains configuration for the client.
type ClientConfig struct {
	// Encoder is used for encoding/decoding messages.
	Encoder Encoder
	// Compressor is used for compressing/decompressing messages.
	Compressor Compressor
	// DefaultTimeout is the default request timeout.
	DefaultTimeout time.Duration
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int
	// RetryDelay is the delay between retry attempts.
	RetryDelay time.Duration
	// EnableMetrics enables metrics collection.
	EnableMetrics bool
}

// ServiceConfig contains configuration for the reqresp service.
type ServiceConfig struct {
	// HandlerConfig is the configuration for handlers.
	HandlerConfig HandlerConfig
	// ClientConfig is the configuration for the client.
	ClientConfig ClientConfig
}

// DefaultServiceConfig returns a default service configuration.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		HandlerConfig: HandlerConfig{
			MaxConcurrentRequests: 100,
			RequestTimeout:        30 * time.Second,
			EnableMetrics:         false,
		},
		ClientConfig: ClientConfig{
			DefaultTimeout: 30 * time.Second,
			MaxRetries:     3,
			RetryDelay:     1 * time.Second,
			EnableMetrics:  false,
		},
	}
}
