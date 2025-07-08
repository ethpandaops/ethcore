package v1

import (
	"github.com/libp2p/go-libp2p/core/protocol"
)

// SimpleProtocol provides a simple implementation of the Protocol interface.
type SimpleProtocol struct {
	id              protocol.ID
	maxRequestSize  uint64
	maxResponseSize uint64
	networkEncoder  NetworkEncoder
	isChunked       bool
}

// NewProtocol creates a new protocol.
func NewProtocol(id protocol.ID, maxRequestSize, maxResponseSize uint64, networkEncoder NetworkEncoder) *SimpleProtocol {
	return &SimpleProtocol{
		id:              id,
		maxRequestSize:  maxRequestSize,
		maxResponseSize: maxResponseSize,
		networkEncoder:  networkEncoder,
		isChunked:       false,
	}
}

// NewChunkedProtocol creates a new chunked protocol.
func NewChunkedProtocol(id protocol.ID, maxRequestSize, maxResponseSize uint64, networkEncoder NetworkEncoder) *SimpleProtocol {
	return &SimpleProtocol{
		id:              id,
		maxRequestSize:  maxRequestSize,
		maxResponseSize: maxResponseSize,
		networkEncoder:  networkEncoder,
		isChunked:       true,
	}
}

// ID returns the protocol ID.
func (p *SimpleProtocol) ID() protocol.ID {
	return p.id
}

// MaxRequestSize returns the maximum allowed request size in bytes.
func (p *SimpleProtocol) MaxRequestSize() uint64 {
	return p.maxRequestSize
}

// MaxResponseSize returns the maximum allowed response size in bytes.
func (p *SimpleProtocol) MaxResponseSize() uint64 {
	return p.maxResponseSize
}

// NetworkEncoder returns the network encoder for this protocol.
func (p *SimpleProtocol) NetworkEncoder() NetworkEncoder {
	return p.networkEncoder
}

// IsChunked returns whether this protocol uses chunked responses.
func (p *SimpleProtocol) IsChunked() bool {
	return p.isChunked
}
