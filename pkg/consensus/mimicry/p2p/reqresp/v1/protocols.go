package v1

import (
	"github.com/libp2p/go-libp2p/core/protocol"
)

// BaseProtocol provides a basic implementation of the Protocol interface.
type BaseProtocol struct {
	id              protocol.ID
	maxRequestSize  uint64
	maxResponseSize uint64
	networkEncoder  NetworkEncoder
}

// NewProtocol creates a new base protocol.
func NewProtocol(id protocol.ID, maxRequestSize, maxResponseSize uint64, networkEncoder NetworkEncoder) *BaseProtocol {
	return &BaseProtocol{
		id:              id,
		maxRequestSize:  maxRequestSize,
		maxResponseSize: maxResponseSize,
		networkEncoder:  networkEncoder,
	}
}

// ID returns the protocol ID.
func (p *BaseProtocol) ID() protocol.ID {
	return p.id
}

// MaxRequestSize returns the maximum allowed request size in bytes.
func (p *BaseProtocol) MaxRequestSize() uint64 {
	return p.maxRequestSize
}

// MaxResponseSize returns the maximum allowed response size in bytes.
func (p *BaseProtocol) MaxResponseSize() uint64 {
	return p.maxResponseSize
}

// NetworkEncoder returns the network encoder for this protocol.
func (p *BaseProtocol) NetworkEncoder() NetworkEncoder {
	return p.networkEncoder
}

// BaseChunkedProtocol provides a basic implementation of the ChunkedProtocol interface.
type BaseChunkedProtocol struct {
	*BaseProtocol
}

// NewChunkedProtocol creates a new chunked protocol.
func NewChunkedProtocol(id protocol.ID, maxRequestSize, maxResponseSize uint64, networkEncoder NetworkEncoder) *BaseChunkedProtocol {
	return &BaseChunkedProtocol{
		BaseProtocol: NewProtocol(id, maxRequestSize, maxResponseSize, networkEncoder),
	}
}

// IsChunked returns true indicating this protocol uses chunked responses.
func (p *BaseChunkedProtocol) IsChunked() bool {
	return true
}
