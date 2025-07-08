package v1

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// NetworkEncoder defines the interface for network encoding and decoding messages.
// This combines SSZ marshaling + Snappy compression in one step, matching
// how Ethereum consensus protocols actually work (ssz_snappy).
type NetworkEncoder interface {
	// EncodeNetwork does SSZ marshaling + Snappy compression in one step
	EncodeNetwork(msg any) ([]byte, error)

	// DecodeNetwork does Snappy decompression + SSZ unmarshaling in one step
	DecodeNetwork(data []byte, msgType any) error
}

// RequestHandler handles incoming requests and returns responses.
type RequestHandler[TReq, TResp any] func(ctx context.Context, req TReq, from peer.ID) (TResp, error)

// ChunkedRequestHandler handles requests that produce multiple response chunks.
type ChunkedRequestHandler[TReq, TResp any] func(ctx context.Context, req TReq, from peer.ID, w ChunkedResponseWriter[TResp]) error

// ChunkedResponseWriter allows writing response chunks for chunked protocols.
type ChunkedResponseWriter[TResp any] interface {
	WriteChunk(resp TResp) error
	Close() error
}

// Protocol represents a request-response protocol with typed requests and responses.
type Protocol[TReq, TResp any] interface {
	ID() protocol.ID
	MaxRequestSize() uint64
	MaxResponseSize() uint64
	NetworkEncoder() NetworkEncoder
}

// ChunkedProtocol represents a protocol that supports chunked responses.
type ChunkedProtocol[TReq, TResp any] interface {
	Protocol[TReq, TResp]
	IsChunked() bool
}
