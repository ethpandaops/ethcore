package v1

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
)

// Client implements the Client interface for sending requests.
type client struct {
	host       host.Host
	encoder    Encoder
	compressor Compressor
	config     ClientConfig
	log        logrus.FieldLogger
}

// NewClient creates a new client.
func NewClient(h host.Host, config ClientConfig, log logrus.FieldLogger) Client {
	return &client{
		host:       h,
		encoder:    config.Encoder,
		compressor: config.Compressor,
		config:     config,
		log:        log.WithField("component", "reqresp_client"),
	}
}

// SendRequest sends a typed request to a peer and waits for a response.
func (c *client) SendRequest(ctx context.Context, peerID peer.ID, protocolID protocol.ID, req any, resp any) error {
	return c.SendRequestWithTimeout(ctx, peerID, protocolID, req, resp, c.config.DefaultTimeout)
}

// SendRequestWithTimeout sends a request with a custom timeout.
func (c *client) SendRequestWithTimeout(ctx context.Context, peerID peer.ID, protocolID protocol.ID, req any, resp any, timeout time.Duration) error {
	// Apply timeout to context
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	logCtx := c.log.WithFields(logrus.Fields{
		"peer":     peerID,
		"protocol": protocolID,
		"timeout":  timeout,
	})

	// Retry logic
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
			logCtx.WithField("attempt", attempt).Debug("Retrying request")
		}

		err := c.sendRequestOnce(ctx, peerID, protocolID, req, resp)
		if err == nil {
			return nil
		}

		lastErr = err
		logCtx.WithError(err).Debug("Request failed")
	}

	return fmt.Errorf("request failed after %d attempts: %w", c.config.MaxRetries+1, lastErr)
}

// sendRequestOnce sends a single request attempt.
func (c *client) sendRequestOnce(ctx context.Context, peerID peer.ID, protocolID protocol.ID, req any, resp any) error {
	// Open stream
	stream, err := c.host.NewStream(ctx, peerID, protocolID)
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Set deadline on stream
	deadline, ok := ctx.Deadline()
	if ok {
		if err := stream.SetDeadline(deadline); err != nil {
			return fmt.Errorf("failed to set stream deadline: %w", err)
		}
	}

	// Write request
	if err := c.writeRequest(stream, req); err != nil {
		return fmt.Errorf("failed to write request: %w", err)
	}

	// Close write side to signal end of request
	if err := stream.CloseWrite(); err != nil {
		return fmt.Errorf("failed to close write stream: %w", err)
	}

	// Read response
	if err := c.readResponse(stream, resp); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	return nil
}

// writeRequest writes a request to the stream.
func (c *client) writeRequest(stream network.Stream, req any) error {
	// Encode request
	data, err := c.encoder.Encode(req)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	// Compress if needed
	if c.compressor != nil {
		compressed, err := c.compressor.Compress(data)
		if err != nil {
			return fmt.Errorf("failed to compress request: %w", err)
		}

		data = compressed
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
		return fmt.Errorf("failed to write request data: %w", err)
	}

	return nil
}

// readResponse reads a response from the stream.
func (c *client) readResponse(stream network.Stream, resp any) error {
	// Read status byte
	var status [1]byte
	if _, err := io.ReadFull(stream, status[:]); err != nil {
		return fmt.Errorf("failed to read status: %w", err)
	}

	// Check status
	if Status(status[0]) != StatusSuccess {
		return fmt.Errorf("server returned error status: %s", Status(status[0]))
	}

	// Read size prefix
	var sizeBytes [4]byte
	if _, err := io.ReadFull(stream, sizeBytes[:]); err != nil {
		return fmt.Errorf("failed to read size prefix: %w", err)
	}

	size := binary.BigEndian.Uint32(sizeBytes[:])
	if size == 0 {
		return fmt.Errorf("received empty response")
	}

	// Read data
	data := make([]byte, size)
	if _, err := io.ReadFull(stream, data); err != nil {
		return fmt.Errorf("failed to read response data: %w", err)
	}

	// Decompress if needed
	if c.compressor != nil {
		decompressed, err := c.compressor.Decompress(data)
		if err != nil {
			return fmt.Errorf("failed to decompress response: %w", err)
		}

		data = decompressed
	}

	// Decode response
	if err := c.encoder.Decode(data, resp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// Request provides a fluent API for building requests.
type Request[TReq, TResp any] struct {
	client   Client
	protocol Protocol[TReq, TResp]
	peerID   peer.ID
	timeout  time.Duration
}

// NewRequest creates a new request builder.
func NewRequest[TReq, TResp any](client Client, protocol Protocol[TReq, TResp]) *Request[TReq, TResp] {
	return &Request[TReq, TResp]{
		client:   client,
		protocol: protocol,
	}
}

// To sets the target peer.
func (r *Request[TReq, TResp]) To(peerID peer.ID) *Request[TReq, TResp] {
	r.peerID = peerID

	return r
}

// WithTimeout sets a custom timeout.
func (r *Request[TReq, TResp]) WithTimeout(timeout time.Duration) *Request[TReq, TResp] {
	r.timeout = timeout

	return r
}

// Send sends the request and returns the response.
func (r *Request[TReq, TResp]) Send(ctx context.Context, req TReq) (TResp, error) {
	var resp TResp

	if r.peerID == "" {
		return resp, fmt.Errorf("peer ID not set")
	}

	err := r.client.SendRequestWithTimeout(ctx, r.peerID, r.protocol.ID(), req, &resp, r.timeout)

	return resp, err
}

// SendRequest is a convenience function for sending requests.
func SendRequest[TReq, TResp any](
	ctx context.Context,
	client Client,
	protocol Protocol[TReq, TResp],
	peerID peer.ID,
	req TReq,
) (TResp, error) {
	var resp TResp
	err := client.SendRequest(ctx, peerID, protocol.ID(), req, &resp)

	return resp, err
}

// SendChunkedRequest sends a request that expects multiple response chunks.
func SendChunkedRequest[TReq, TResp any](
	ctx context.Context,
	client Client,
	protocol Protocol[TReq, TResp],
	peerID peer.ID,
	req TReq,
	handler func(chunk TResp) error,
) error {
	// For now, we'll use the existing client interface with a wrapper
	// In a real implementation, we'd extend the client to support chunked responses natively
	return fmt.Errorf("chunked request sending not yet implemented in base client")
}

// ChunkedClient extends the base client with chunked response support.
type ChunkedClient interface {
	Client
	// SendChunkedRequest sends a request and processes multiple response chunks.
	SendChunkedRequest(ctx context.Context, peerID peer.ID, protocolID protocol.ID, req any, chunkHandler func(chunk any) error) error
}

// chunkedClient implements ChunkedClient.
type chunkedClient struct {
	*client
}

// NewChunkedClient creates a new client with chunked response support.
func NewChunkedClient(h host.Host, config ClientConfig, log logrus.FieldLogger) ChunkedClient {
	baseClient, ok := NewClient(h, config, log).(*client)
	if !ok {
		panic("failed to cast to concrete client type")
	}

	return &chunkedClient{
		client: baseClient,
	}
}

// SendChunkedRequest sends a request and processes multiple response chunks.
func (c *chunkedClient) SendChunkedRequest(ctx context.Context, peerID peer.ID, protocolID protocol.ID, req any, chunkHandler func(chunk any) error) error {
	// Apply timeout to context
	timeout := c.config.DefaultTimeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	logCtx := c.log.WithFields(logrus.Fields{
		"peer":     peerID,
		"protocol": protocolID,
		"chunked":  true,
	})

	// Open stream
	stream, err := c.host.NewStream(ctx, peerID, protocolID)
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Set deadline on stream
	deadline, ok := ctx.Deadline()
	if ok {
		if err := stream.SetDeadline(deadline); err != nil {
			return fmt.Errorf("failed to set stream deadline: %w", err)
		}
	}

	// Write request
	if err := c.writeRequest(stream, req); err != nil {
		return fmt.Errorf("failed to write request: %w", err)
	}

	// Close write side to signal end of request
	if err := stream.CloseWrite(); err != nil {
		return fmt.Errorf("failed to close write stream: %w", err)
	}

	// Read multiple response chunks
	chunkCount := 0

	for {
		// Read status byte
		var status [1]byte
		if _, err := io.ReadFull(stream, status[:]); err != nil {
			if err == io.EOF && chunkCount > 0 {
				// Normal end of chunked response
				break
			}

			return fmt.Errorf("failed to read chunk status: %w", err)
		}

		// Check status
		if Status(status[0]) != StatusSuccess {
			return fmt.Errorf("server returned error status: %s", Status(status[0]))
		}

		// Read size prefix
		var sizeBytes [4]byte
		if _, err := io.ReadFull(stream, sizeBytes[:]); err != nil {
			if err == io.EOF {
				// End of chunks
				break
			}

			return fmt.Errorf("failed to read size prefix: %w", err)
		}

		size := binary.BigEndian.Uint32(sizeBytes[:])
		if size == 0 {
			// Empty chunk might signal end
			continue
		}

		// Read data
		data := make([]byte, size)
		if _, err := io.ReadFull(stream, data); err != nil {
			return fmt.Errorf("failed to read chunk data: %w", err)
		}

		// Decompress if needed
		if c.compressor != nil {
			decompressed, err := c.compressor.Decompress(data)
			if err != nil {
				return fmt.Errorf("failed to decompress chunk: %w", err)
			}

			data = decompressed
		}

		// Process chunk
		if err := chunkHandler(data); err != nil {
			return fmt.Errorf("chunk handler error: %w", err)
		}

		chunkCount++
		logCtx.WithField("chunk", chunkCount).Debug("Processed response chunk")
	}

	logCtx.WithField("total_chunks", chunkCount).Debug("Completed chunked response")

	return nil
}
