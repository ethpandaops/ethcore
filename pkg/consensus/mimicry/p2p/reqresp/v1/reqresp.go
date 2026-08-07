package v1

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
)

// ReqResp implements request/response functionality.
type ReqResp struct {
	host host.Host
	log  logrus.FieldLogger

	mu       sync.RWMutex
	started  bool
	handlers map[protocol.ID]func(network.Stream)
}

// New creates a new ReqResp service.
func New(h host.Host, log logrus.FieldLogger) *ReqResp {
	return &ReqResp{
		host:     h,
		log:      log.WithField("component", "reqresp"),
		handlers: make(map[protocol.ID]func(network.Stream)),
	}
}

// Start starts the service.
func (r *ReqResp) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return fmt.Errorf("service already started")
	}

	for protocolID, handler := range r.handlers {
		r.host.SetStreamHandler(protocolID, handler)
	}

	r.started = true
	return nil
}

// Stop stops the service.
func (r *ReqResp) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return fmt.Errorf("service not started")
	}

	for protocolID := range r.handlers {
		r.host.RemoveStreamHandler(protocolID)
	}

	r.started = false
	return nil
}

// HandleStream provides a convenient wrapper for handling req/resp streams with marshalling.
func HandleStream[TReq, TResp any](
	stream network.Stream,
	protocol Protocol[TReq, TResp],
	handler RequestHandler[TReq, TResp],
) error {
	defer stream.Close()

	// Read request from stream
	reqData := make([]byte, protocol.MaxRequestSize())

	n, err := stream.Read(reqData)
	if err != nil {
		return fmt.Errorf("failed to read request: %w", err)
	}

	reqData = reqData[:n]

	// Decode request (includes decompression)
	var req TReq
	if err = protocol.NetworkEncoder().DecodeNetwork(reqData, &req); err != nil {
		return fmt.Errorf("failed to decode request: %w", err)
	}

	// Handle request
	resp, err := handler(context.Background(), req, stream.Conn().RemotePeer())
	if err != nil {
		return fmt.Errorf("handler error: %w", err)
	}

	// Encode response (includes compression)
	respData, err := protocol.NetworkEncoder().EncodeNetwork(resp)
	if err != nil {
		return fmt.Errorf("failed to encode response: %w", err)
	}

	// Write response to stream
	_, err = stream.Write(respData)

	return err
}

// HandleChunkedStream provides a convenient wrapper for handling chunked req/resp streams.
func HandleChunkedStream[TReq, TResp any](
	stream network.Stream,
	protocol Protocol[TReq, TResp],
	handler ChunkedRequestHandler[TReq, TResp],
) error {
	defer stream.Close()

	// Read request from stream
	reqData := make([]byte, protocol.MaxRequestSize())

	n, err := stream.Read(reqData)
	if err != nil {
		return fmt.Errorf("failed to read request: %w", err)
	}

	reqData = reqData[:n]

	// Decode request (includes decompression)
	var req TReq
	if err = protocol.NetworkEncoder().DecodeNetwork(reqData, &req); err != nil {
		return fmt.Errorf("failed to decode request: %w", err)
	}

	// Create response writer
	writer := &streamChunkedWriter[TResp]{
		stream:         stream,
		networkEncoder: protocol.NetworkEncoder(),
	}

	// Handle request
	return handler(context.Background(), req, stream.Conn().RemotePeer(), writer)
}

// streamChunkedWriter implements ChunkedResponseWriter for streams.
type streamChunkedWriter[TResp any] struct {
	stream         network.Stream
	networkEncoder NetworkEncoder
}

func (w *streamChunkedWriter[TResp]) WriteChunk(resp TResp) error {
	data, err := w.networkEncoder.EncodeNetwork(resp)
	if err != nil {
		return fmt.Errorf("failed to encode chunk: %w", err)
	}

	_, err = w.stream.Write(data)

	return err
}

func (w *streamChunkedWriter[TResp]) Close() error {
	return w.stream.Close()
}

// RegisterHandler registers a raw stream handler for a protocol.
func (r *ReqResp) RegisterHandler(protocolID protocol.ID, handler func(network.Stream)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[protocolID]; exists {
		return fmt.Errorf("handler already registered for protocol %s", protocolID)
	}

	r.handlers[protocolID] = handler

	if r.started {
		r.host.SetStreamHandler(protocolID, handler)
	}

	return nil
}

// RegisterStreamHandler registers a handler using the convenient stream wrapper.
func RegisterStreamHandler[TReq, TResp any](
	service *ReqResp,
	protocol Protocol[TReq, TResp],
	handler RequestHandler[TReq, TResp],
) error {
	return service.RegisterHandler(protocol.ID(), func(stream network.Stream) {
		if err := HandleStream(stream, protocol, handler); err != nil {
			// Log error but don't crash - let the stream close gracefully
			service.log.WithError(err).WithField("protocol", protocol.ID()).Error("Stream handler error")
		}
	})
}

// RegisterChunkedStreamHandler registers a chunked handler using the convenient stream wrapper.
func RegisterChunkedStreamHandler[TReq, TResp any](
	service *ReqResp,
	protocol Protocol[TReq, TResp],
	handler ChunkedRequestHandler[TReq, TResp],
) error {
	return service.RegisterHandler(protocol.ID(), func(stream network.Stream) {
		if err := HandleChunkedStream(stream, protocol, handler); err != nil {
			// Log error but don't crash - let the stream close gracefully
			service.log.WithError(err).WithField("protocol", protocol.ID()).Error("Chunked stream handler error")
		}
	})
}

// SendRequest provides a convenient wrapper for making outbound requests.
func SendRequest[TReq, TResp any](
	ctx context.Context,
	h host.Host,
	peerID peer.ID,
	protocol Protocol[TReq, TResp],
	req TReq,
) (TResp, error) {
	var resp TResp

	// Open stream to peer
	stream, err := h.NewStream(ctx, peerID, protocol.ID())
	if err != nil {
		return resp, fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Encode request (includes compression)
	reqData, err := protocol.NetworkEncoder().EncodeNetwork(req)
	if err != nil {
		return resp, fmt.Errorf("failed to encode request: %w", err)
	}

	// Send request
	_, err = stream.Write(reqData)
	if err != nil {
		return resp, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response
	respData := make([]byte, protocol.MaxResponseSize())

	n, err := stream.Read(respData)
	if err != nil {
		return resp, fmt.Errorf("failed to read response: %w", err)
	}

	respData = respData[:n]

	// Decode response (includes decompression)
	if err = protocol.NetworkEncoder().DecodeNetwork(respData, &resp); err != nil {
		return resp, fmt.Errorf("failed to decode response: %w", err)
	}

	return resp, nil
}

// SendChunkedRequest provides a convenient wrapper for making chunked requests.
func SendChunkedRequest[TReq, TResp any](
	ctx context.Context,
	h host.Host,
	peerID peer.ID,
	protocol Protocol[TReq, TResp],
	req TReq,
	chunkHandler func(TResp) error,
) error {
	// Open stream to peer
	stream, err := h.NewStream(ctx, peerID, protocol.ID())
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Encode request (includes compression)
	reqData, err := protocol.NetworkEncoder().EncodeNetwork(req)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	// Send request
	_, err = stream.Write(reqData)
	if err != nil {
		return fmt.Errorf("failed to write request: %w", err)
	}

	// Read chunked responses
	for {
		// Read chunk
		chunkData := make([]byte, protocol.MaxResponseSize())

		n, err := stream.Read(chunkData)
		if err != nil {
			// End of stream is expected for chunked responses
			if err.Error() == "EOF" {
				break
			}

			return fmt.Errorf("failed to read chunk: %w", err)
		}

		if n == 0 {
			break // No more data
		}

		chunkData = chunkData[:n]

		// Decode chunk (includes decompression)
		var chunk TResp
		if err = protocol.NetworkEncoder().DecodeNetwork(chunkData, &chunk); err != nil {
			return fmt.Errorf("failed to decode chunk: %w", err)
		}

		// Handle chunk
		if err = chunkHandler(chunk); err != nil {
			return fmt.Errorf("chunk handler error: %w", err)
		}
	}

	return nil
}
