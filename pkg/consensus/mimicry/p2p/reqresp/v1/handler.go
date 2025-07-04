package v1

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
)

// Handler wraps a typed request handler to work with streams.
type Handler[TReq, TResp any] struct {
	handler    RequestHandler[TReq, TResp]
	encoder    Encoder
	compressor Compressor
	protocol   Protocol[TReq, TResp]
	log        logrus.FieldLogger
	config     HandlerOptions
}

// NewHandler creates a new handler.
func NewHandler[TReq, TResp any](
	protocol Protocol[TReq, TResp],
	handler RequestHandler[TReq, TResp],
	config HandlerOptions,
	log logrus.FieldLogger,
) *Handler[TReq, TResp] {
	return &Handler[TReq, TResp]{
		handler:    handler,
		encoder:    config.Encoder,
		compressor: config.Compressor,
		protocol:   protocol,
		log:        log.WithField("protocol", protocol.ID()),
		config:     config,
	}
}

// HandleStream implements StreamHandler.
func (h *Handler[TReq, TResp]) HandleStream(ctx context.Context, stream network.Stream) {
	defer stream.Close()

	// Set deadline if configured
	if h.config.RequestTimeout > 0 {
		deadline := time.Now().Add(h.config.RequestTimeout)
		if err := stream.SetDeadline(deadline); err != nil {
			h.log.WithError(err).Debug("Failed to set stream deadline")
		}
	}

	// Get peer ID
	peerID := stream.Conn().RemotePeer()
	h.log.WithField("peer", peerID).Debug("Handling request")

	// Read request
	req, err := h.readRequest(stream)
	if err != nil {
		h.log.WithError(err).WithField("peer", peerID).Debug("Failed to read request")
		_ = h.writeErrorResponse(stream, StatusInvalidRequest)

		return
	}

	// Process request
	resp, err := h.handler(ctx, req, peerID)
	if err != nil {
		h.log.WithError(err).WithField("peer", peerID).Debug("Handler returned error")
		_ = h.writeErrorResponse(stream, StatusServerError)

		return
	}

	// Write response
	if err := h.writeResponse(stream, StatusSuccess, resp); err != nil {
		h.log.WithError(err).WithField("peer", peerID).Debug("Failed to write response")
	}
}

// readRequest reads and decodes a request from the stream.
func (h *Handler[TReq, TResp]) readRequest(stream network.Stream) (TReq, error) {
	var req TReq

	// Read size prefix (4 bytes)
	var sizeBytes [4]byte
	if _, err := io.ReadFull(stream, sizeBytes[:]); err != nil {
		return req, fmt.Errorf("failed to read size prefix: %w", err)
	}

	size := binary.BigEndian.Uint32(sizeBytes[:])
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

// writeResponse writes a response to the stream.
func (h *Handler[TReq, TResp]) writeResponse(stream network.Stream, status Status, resp TResp) error {
	// Write status byte
	if _, err := stream.Write([]byte{byte(status)}); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	// Only write response data if status is success
	if status != StatusSuccess {
		return nil
	}

	// Encode response
	data, err := h.encoder.Encode(resp)
	if err != nil {
		return fmt.Errorf("failed to encode response: %w", err)
	}

	// Compress if needed
	if h.compressor != nil {
		compressed, err := h.compressor.Compress(data)
		if err != nil {
			return fmt.Errorf("failed to compress response: %w", err)
		}

		data = compressed
	}

	// Check size
	if uint64(len(data)) > h.protocol.MaxResponseSize() {
		return fmt.Errorf("response size %d exceeds max %d", len(data), h.protocol.MaxResponseSize())
	}

	// Write size prefix
	var sizeBytes [4]byte

	dataLen := len(data)

	if dataLen > int(^uint32(0)) {
		return fmt.Errorf("data size %d exceeds uint32 max", dataLen)
	}

	binary.BigEndian.PutUint32(sizeBytes[:], uint32(dataLen))

	if _, err := stream.Write(sizeBytes[:]); err != nil {
		return fmt.Errorf("failed to write size prefix: %w", err)
	}

	// Write data
	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("failed to write response data: %w", err)
	}

	return nil
}

// writeErrorResponse writes an error response with just a status code.
func (h *Handler[TReq, TResp]) writeErrorResponse(stream network.Stream, status Status) error {
	if _, err := stream.Write([]byte{byte(status)}); err != nil {
		h.log.WithError(err).Debug("Failed to write error status")

		return err
	}

	return nil
}

// HandlerRegistry manages protocol handlers.
type HandlerRegistry struct {
	handlers map[protocol.ID]StreamHandler
	log      logrus.FieldLogger
}

// NewHandlerRegistry creates a new handler registry.
func NewHandlerRegistry(log logrus.FieldLogger) *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[protocol.ID]StreamHandler),
		log:      log.WithField("component", "handler_registry"),
	}
}

// Register registers a handler for a protocol.
func (r *HandlerRegistry) Register(protocolID protocol.ID, handler StreamHandler) error {
	if _, exists := r.handlers[protocolID]; exists {
		return ErrHandlerExists
	}

	r.handlers[protocolID] = handler
	r.log.WithField("protocol", protocolID).Debug("Registered handler")

	return nil
}

// Unregister removes a handler for a protocol.
func (r *HandlerRegistry) Unregister(protocolID protocol.ID) error {
	if _, exists := r.handlers[protocolID]; !exists {
		return ErrNoHandler
	}

	delete(r.handlers, protocolID)
	r.log.WithField("protocol", protocolID).Debug("Unregistered handler")

	return nil
}

// Get returns the handler for a protocol.
func (r *HandlerRegistry) Get(protocolID protocol.ID) (StreamHandler, bool) {
	handler, ok := r.handlers[protocolID]

	return handler, ok
}

// RegisterHandler registers a typed handler for a protocol.
func RegisterHandler[TReq, TResp any](
	registry *HandlerRegistry,
	protocol Protocol[TReq, TResp],
	handler RequestHandler[TReq, TResp],
	config HandlerOptions,
	log logrus.FieldLogger,
) error {
	h := NewHandler(protocol, handler, config, log)

	return registry.Register(protocol.ID(), h)
}
