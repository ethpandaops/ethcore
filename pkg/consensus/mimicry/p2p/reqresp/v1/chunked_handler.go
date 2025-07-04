package v1

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// ChunkedRequestHandler handles requests that produce multiple response chunks.
type ChunkedRequestHandler[TReq, TResp any] func(
	ctx context.Context,
	req TReq,
	from peer.ID,
	writer ChunkedResponseWriter[TResp],
) error

// ChunkedResponseWriter allows writing multiple response chunks to a stream.
type ChunkedResponseWriter[TResp any] interface {
	// WriteChunk writes a single response chunk to the stream.
	// Each chunk is sent with its own status byte and length prefix.
	WriteChunk(resp TResp) error
	// Close finalizes the chunked response.
	Close() error
}

// streamChunkedWriter implements ChunkedResponseWriter for a network stream.
type streamChunkedWriter[TResp any] struct {
	stream     network.Stream
	encoder    Encoder
	compressor Compressor
	maxSize    uint64
	log        logrus.FieldLogger
	closed     bool
}

// WriteChunk writes a single response chunk.
func (w *streamChunkedWriter[TResp]) WriteChunk(resp TResp) error {
	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// Write success status byte for this chunk
	if _, err := w.stream.Write([]byte{byte(StatusSuccess)}); err != nil {
		return fmt.Errorf("failed to write chunk status: %w", err)
	}

	// Encode response
	data, err := w.encoder.Encode(resp)
	if err != nil {
		return fmt.Errorf("failed to encode response chunk: %w", err)
	}

	// Compress if needed
	if w.compressor != nil {
		compressed, err := w.compressor.Compress(data)
		if err != nil {
			return fmt.Errorf("failed to compress response chunk: %w", err)
		}

		data = compressed
	}

	// Check size
	if uint64(len(data)) > w.maxSize {
		return fmt.Errorf("response chunk size %d exceeds max %d", len(data), w.maxSize)
	}

	// Write size prefix
	var sizeBytes [4]byte

	dataLen := len(data)
	if dataLen > int(^uint32(0)) {
		return fmt.Errorf("data size %d exceeds uint32 max", dataLen)
	}

	binary.BigEndian.PutUint32(sizeBytes[:], uint32(dataLen))

	if _, err := w.stream.Write(sizeBytes[:]); err != nil {
		return fmt.Errorf("failed to write size prefix: %w", err)
	}

	// Write data
	if _, err := w.stream.Write(data); err != nil {
		return fmt.Errorf("failed to write response data: %w", err)
	}

	w.log.WithField("chunk_size", len(data)).Debug("Wrote response chunk")

	return nil
}

// Close finalizes the chunked response.
func (w *streamChunkedWriter[TResp]) Close() error {
	if w.closed {
		return nil
	}

	w.closed = true

	return nil
}

// ChunkedHandler wraps a chunked request handler to work with streams.
type ChunkedHandler[TReq, TResp any] struct {
	handler    ChunkedRequestHandler[TReq, TResp]
	encoder    Encoder
	compressor Compressor
	protocol   Protocol[TReq, TResp]
	log        logrus.FieldLogger
	config     HandlerOptions
}

// NewChunkedHandler creates a new chunked handler.
func NewChunkedHandler[TReq, TResp any](
	protocol Protocol[TReq, TResp],
	handler ChunkedRequestHandler[TReq, TResp],
	config HandlerOptions,
	log logrus.FieldLogger,
) *ChunkedHandler[TReq, TResp] {
	return &ChunkedHandler[TReq, TResp]{
		handler:    handler,
		encoder:    config.Encoder,
		compressor: config.Compressor,
		protocol:   protocol,
		log:        log.WithField("protocol", protocol.ID()),
		config:     config,
	}
}

// HandleStream implements StreamHandler.
func (h *ChunkedHandler[TReq, TResp]) HandleStream(ctx context.Context, stream network.Stream) {
	defer stream.Close()

	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			h.log.WithField("panic", r).Error("Chunked handler panicked")
			_ = h.writeErrorResponse(stream, StatusServerError)
		}
	}()

	// Create context with timeout if configured
	handlerCtx := ctx

	var cancel context.CancelFunc
	if h.config.RequestTimeout > 0 {
		handlerCtx, cancel = context.WithTimeout(ctx, h.config.RequestTimeout)
		defer cancel()

		deadline := time.Now().Add(h.config.RequestTimeout)
		if err := stream.SetDeadline(deadline); err != nil {
			h.log.WithError(err).Debug("Failed to set stream deadline")
		}
	}

	// Get peer ID
	peerID := stream.Conn().RemotePeer()
	h.log.WithField("peer", peerID).Debug("Handling chunked request")

	// Read request
	req, err := h.readRequest(stream)
	if err != nil {
		h.log.WithError(err).WithField("peer", peerID).Debug("Failed to read request")
		_ = h.writeErrorResponse(stream, StatusInvalidRequest)

		return
	}

	// Create response writer
	writer := &streamChunkedWriter[TResp]{
		stream:     stream,
		encoder:    h.encoder,
		compressor: h.compressor,
		maxSize:    h.protocol.MaxResponseSize(),
		log:        h.log,
	}

	// Process request with chunked writer
	err = h.handler(handlerCtx, req, peerID, writer)
	if err != nil {
		h.log.WithError(err).WithField("peer", peerID).Debug("Chunked handler returned error")
		// Try to send error status if writer hasn't written anything yet
		if !writer.closed {
			_ = h.writeErrorResponse(stream, StatusServerError)
		}

		return
	}

	// Ensure writer is closed
	_ = writer.Close()
}

// readRequest reads and decodes a request from the stream.
func (h *ChunkedHandler[TReq, TResp]) readRequest(stream network.Stream) (TReq, error) {
	var req TReq

	// Read size prefix (4 bytes)
	var sizeBytes [4]byte
	if _, err := io.ReadFull(stream, sizeBytes[:]); err != nil {
		return req, fmt.Errorf("failed to read size prefix: %w", err)
	}

	size := binary.BigEndian.Uint32(sizeBytes[:])
	if size == 0 {
		return req, fmt.Errorf("empty request")
	}

	if uint64(size) > h.protocol.MaxRequestSize() {
		return req, fmt.Errorf("request size %d exceeds max %d", size, h.protocol.MaxRequestSize())
	}

	// Read data
	data := make([]byte, size)
	if _, err := io.ReadFull(stream, data); err != nil {
		return req, fmt.Errorf("failed to read request data: %w", err)
	}

	// Decompress if needed
	if h.compressor != nil {
		decompressed, err := h.compressor.Decompress(data)
		if err != nil {
			return req, fmt.Errorf("failed to decompress request: %w", err)
		}

		data = decompressed
	}

	// Decode request
	if err := h.encoder.Decode(data, &req); err != nil {
		return req, fmt.Errorf("failed to decode request: %w", err)
	}

	return req, nil
}

// writeErrorResponse writes an error response with just a status code.
func (h *ChunkedHandler[TReq, TResp]) writeErrorResponse(stream network.Stream, status Status) error {
	if _, err := stream.Write([]byte{byte(status)}); err != nil {
		h.log.WithError(err).Debug("Failed to write error status")

		return err
	}

	return nil
}

// RegisterChunkedHandler registers a chunked handler for a protocol.
func RegisterChunkedHandler[TReq, TResp any](
	registry *HandlerRegistry,
	protocol Protocol[TReq, TResp],
	handler ChunkedRequestHandler[TReq, TResp],
	config HandlerOptions,
	log logrus.FieldLogger,
) error {
	h := NewChunkedHandler(protocol, handler, config, log)

	return registry.Register(protocol.ID(), h)
}
