package v1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
)

// Example request and response types.
type PingRequest struct {
	Message string
	Nonce   uint64
}

type PingResponse struct {
	Message string
	Nonce   uint64
	Time    time.Time
}

// Example protocol implementation.
type PingProtocol struct{}

func (p PingProtocol) ID() protocol.ID {
	return "/ping/1.0.0"
}

func (p PingProtocol) MaxRequestSize() uint64 {
	return 1024 // 1KB
}

func (p PingProtocol) MaxResponseSize() uint64 {
	return 1024 // 1KB
}

// Example encoder implementation using JSON.
type JSONEncoder struct{}

func (e JSONEncoder) Encode(msg any) ([]byte, error) {
	return json.Marshal(msg)
}

func (e JSONEncoder) Decode(data []byte, msgType any) error {
	return json.Unmarshal(data, msgType)
}

// Example compressor implementation (no compression).
type NoopCompressor struct{}

func (c NoopCompressor) Compress(data []byte) ([]byte, error) {
	return data, nil
}

func (c NoopCompressor) Decompress(data []byte) ([]byte, error) {
	return data, nil
}

// Example demonstrates basic usage of the reqresp package.
func Example_basicUsage() {
	// This example assumes you have a libp2p host set up
	var h host.Host // = ... initialize your host

	// Create service configuration
	config := v1.ServiceConfig{
		HandlerConfig: v1.HandlerConfig{
			Encoder:               JSONEncoder{},
			Compressor:            NoopCompressor{},
			MaxConcurrentRequests: 100,
			RequestTimeout:        30 * time.Second,
		},
		ClientConfig: v1.ClientConfig{
			Encoder:        JSONEncoder{},
			Compressor:     NoopCompressor{},
			DefaultTimeout: 30 * time.Second,
			MaxRetries:     3,
			RetryDelay:     time.Second,
		},
	}

	// Create the service
	logger := logrus.New()
	service := v1.New(h, config, logger)

	// Start the service
	ctx := context.Background()
	if err := service.Start(ctx); err != nil {
		panic(err)
	}
	defer func() {
		if err := service.Stop(); err != nil {
			panic(err)
		}
	}()

	// Register a handler for the ping protocol
	pingProto := PingProtocol{}
	handler := func(ctx context.Context, req PingRequest, from peer.ID) (PingResponse, error) {
		fmt.Printf("Received ping from %s: %s\n", from, req.Message)

		return PingResponse{
			Message: "pong",
			Nonce:   req.Nonce,
			Time:    time.Now(),
		}, nil
	}

	if err := v1.RegisterProtocol(service, pingProto, handler); err != nil {
		panic(err)
	}

	// Send a request using the fluent API
	targetPeer := peer.ID("QmTargetPeer")

	resp, err := v1.NewRequest[PingRequest, PingResponse](service, pingProto).
		To(targetPeer).
		WithTimeout(5*time.Second).
		Send(ctx, PingRequest{
			Message: "ping",
			Nonce:   12345,
		})

	if err != nil {
		fmt.Printf("Request failed: %v\n", err)

		return
	}

	fmt.Printf("Got response: %s at %v\n", resp.Message, resp.Time)
}

// Example demonstrates using custom protocols.
func Example_customProtocol() {
	// Define a custom protocol for file transfer
	type FileProtocol struct {
		version string
	}

	// Methods need to be defined outside the function
	// Create protocol instance
	fileProto := FileProtocol{version: "1.0.0"}

	// This example shows how the protocol can be used
	fmt.Printf("File protocol version: %s\n", fileProto.version)
}

// Example demonstrates error handling.
func Example_errorHandling() {
	// Example of handling different error types
	var service v1.Service // = ... initialized service

	ctx := context.Background()
	targetPeer := peer.ID("QmTargetPeer")

	// Send a request with timeout
	var req PingRequest
	var resp PingResponse

	err := service.SendRequestWithTimeout(ctx, targetPeer, "/ping/1.0.0", &req, &resp, 100*time.Millisecond)

	switch err {
	case nil:
		fmt.Println("Request succeeded")
	case v1.ErrTimeout:
		fmt.Println("Request timed out")
	case v1.ErrStreamReset:
		fmt.Println("Stream was reset by peer")
	default:
		fmt.Printf("Request failed: %v\n", err)
	}
}

// LoggingMiddleware is an example middleware for logging.
type LoggingMiddleware struct {
	log logrus.FieldLogger
}

func (m LoggingMiddleware) WrapHandler(handler v1.StreamHandler) v1.StreamHandler {
	return &wrappedHandler{
		handler: handler,
		log:     m.log,
	}
}

type wrappedHandler struct {
	handler v1.StreamHandler
	log     logrus.FieldLogger
}

func (w *wrappedHandler) HandleStream(ctx context.Context, stream network.Stream) {
	start := time.Now()
	w.handler.HandleStream(ctx, stream)
	w.log.WithFields(logrus.Fields{
		"duration": time.Since(start),
	}).Debug("Handler completed")
}

// Example demonstrates using middleware.
func Example_middleware() {
	// Create a logging middleware
	logger := logrus.New()
	middleware := LoggingMiddleware{
		log: logger,
	}

	// Usage would involve wrapping handlers before registration
	fmt.Printf("Middleware has logger: %v\n", middleware.log != nil)
	fmt.Println("Middleware example completed")
}

// Example demonstrates chunked responses.
func Example_chunkedResponses() {
	// This example assumes you have a libp2p host set up
	var h host.Host // = ... initialize your host

	// Create service configuration
	config := v1.ServiceConfig{
		HandlerConfig: v1.HandlerConfig{
			Encoder:               JSONEncoder{},
			Compressor:            NoopCompressor{},
			MaxConcurrentRequests: 100,
			RequestTimeout:        30 * time.Second,
		},
		ClientConfig: v1.ClientConfig{
			Encoder:        JSONEncoder{},
			Compressor:     NoopCompressor{},
			DefaultTimeout: 30 * time.Second,
			MaxRetries:     3,
			RetryDelay:     time.Second,
		},
	}

	// Create the service
	logger := logrus.New()
	service := v1.New(h, config, logger)

	// Start the service
	ctx := context.Background()
	if err := service.Start(ctx); err != nil {
		panic(err)
	}
	defer func() {
		if err := service.Stop(); err != nil {
			fmt.Printf("Failed to stop service: %v\n", err)
		}
	}()

	// Define a chunked protocol for streaming data
	type BlockRequest struct {
		StartSlot uint64
		Count     uint64
	}

	type Block struct {
		Slot uint64
		Data []byte
	}

	// Create a chunked protocol using the helper
	blocksProtocol := v1.NewChunkedProtocol(
		"/blocks/stream/1.0.0",
		1024,         // 1KB max request
		1024*1024*10, // 10MB max per chunk
	)

	// Register a chunked handler
	chunkedHandler := func(ctx context.Context, req BlockRequest, from peer.ID, writer v1.ChunkedResponseWriter[Block]) error {
		fmt.Printf("Received block request from %s: start=%d, count=%d\n", from, req.StartSlot, req.Count)

		// Send blocks as separate chunks
		for i := uint64(0); i < req.Count; i++ {
			block := Block{
				Slot: req.StartSlot + i,
				Data: []byte(fmt.Sprintf("block data for slot %d", req.StartSlot+i)),
			}

			if err := writer.WriteChunk(block); err != nil {
				return fmt.Errorf("failed to write block chunk: %w", err)
			}
		}

		return nil
	}

	if err := v1.RegisterChunkedProtocol(service, blocksProtocol, chunkedHandler); err != nil {
		panic(err)
	}

	// Client side - receive chunked responses
	chunkedClient := v1.NewChunkedClient(h, config.ClientConfig, logger)
	targetPeer := peer.ID("QmTargetPeer")

	// Process blocks as they arrive
	var receivedBlocks []Block
	err := chunkedClient.SendChunkedRequest(
		ctx,
		targetPeer,
		blocksProtocol.ID(),
		BlockRequest{StartSlot: 100, Count: 10},
		func(chunk any) error {
			// In a real implementation, the chunk would be decoded to Block type
			fmt.Printf("Received block chunk\n")
			// receivedBlocks = append(receivedBlocks, decodedBlock)
			return nil
		},
	)

	if err != nil {
		fmt.Printf("Chunked request failed: %v\n", err)

		return
	}

	fmt.Printf("Received %d blocks\n", len(receivedBlocks))
}
