package v1

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// Encoder defines the interface for encoding and decoding messages.
type Encoder interface {
	// Encode encodes the message into bytes.
	Encode(msg any) ([]byte, error)
	// Decode decodes bytes into the message type.
	Decode(data []byte, msgType any) error
}

// Compressor defines the interface for compressing and decompressing data.
type Compressor interface {
	// Compress compresses the input data.
	Compress(data []byte) ([]byte, error)
	// Decompress decompresses the input data.
	Decompress(data []byte) ([]byte, error)
}

// StreamHandler handles individual request-response streams.
type StreamHandler interface {
	// HandleStream processes an incoming stream.
	HandleStream(ctx context.Context, stream network.Stream)
}

// RequestHandler is a function type that handles incoming requests.
// It receives the request and returns a response or an error.
type RequestHandler[TReq, TResp any] func(ctx context.Context, req TReq, from peer.ID) (TResp, error)

// ResponseValidator is a function type that validates responses.
// It returns an error if the response is invalid.
type ResponseValidator[TResp any] func(ctx context.Context, resp TResp, from peer.ID) error

// Protocol represents a request-response protocol with typed requests and responses.
type Protocol[TReq, TResp any] interface {
	// ID returns the protocol ID.
	ID() protocol.ID
	// MaxRequestSize returns the maximum allowed request size in bytes.
	MaxRequestSize() uint64
	// MaxResponseSize returns the maximum allowed response size in bytes.
	MaxResponseSize() uint64
}

// ChunkedProtocol represents a protocol that supports chunked responses.
type ChunkedProtocol[TReq, TResp any] interface {
	Protocol[TReq, TResp]
	// IsChunked returns true if this protocol uses chunked responses.
	IsChunked() bool
}

// Client provides methods for sending requests.
type Client interface {
	// SendRequest sends a request to a peer and waits for a response.
	// The req and resp parameters must be pointers to the request and response types.
	SendRequest(ctx context.Context, peerID peer.ID, protocolID protocol.ID, req any, resp any) error
	// SendRequestWithTimeout sends a request with a custom timeout.
	// The req and resp parameters must be pointers to the request and response types.
	SendRequestWithTimeout(ctx context.Context, peerID peer.ID, protocolID protocol.ID, req any, resp any, timeout time.Duration) error
	// SendRequestWithOptions sends a request with custom options including encoding.
	// The req and resp parameters must be pointers to the request and response types.
	SendRequestWithOptions(ctx context.Context, peerID peer.ID, protocolID protocol.ID, req any, resp any, opts RequestOptions) error
}

// Registry manages request handlers for different protocols.
type Registry interface {
	// Register registers a handler for a protocol.
	Register(protocolID protocol.ID, handler StreamHandler) error
	// Unregister removes a handler for a protocol.
	Unregister(protocolID protocol.ID) error
}

// Service combines client and registry functionality.
type Service interface {
	Client
	Registry
	// Start starts the service.
	Start(ctx context.Context) error
	// Stop stops the service.
	Stop() error
}
