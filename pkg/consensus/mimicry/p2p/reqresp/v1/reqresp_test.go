package v1

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	host := newMockHost("test-peer")
	config := DefaultServiceConfig()
	logger := logrus.New()

	service := New(host, config, logger)
	require.NotNil(t, service)

	// Verify it implements Service interface
	var _ Service = service
}

func TestService_StartStop(t *testing.T) {
	host := newMockHost("test-peer")
	config := DefaultServiceConfig()
	logger := logrus.New()

	service := New(host, config, logger)

	// Start the service
	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)

	// Start again should return error
	err = service.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	// Stop the service
	err = service.Stop()
	require.NoError(t, err)

	// Stop again should be safe
	err = service.Stop()
	require.NoError(t, err)
}

func TestService_RegisterUnregister(t *testing.T) {
	host := newMockHost("test-peer")
	config := DefaultServiceConfig()
	logger := logrus.New()

	service := New(host, config, logger)

	// Start the service
	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop()

	protocolID := protocol.ID("/test/1.0.0")
	handler := &mockStreamHandler{
		handleFunc: func(ctx context.Context, stream network.Stream) {
			// Do nothing
		},
	}

	// Register handler
	err = service.Register(protocolID, handler)
	require.NoError(t, err)

	// Register again should return error
	err = service.Register(protocolID, handler)
	require.Error(t, err)
	assert.Equal(t, ErrHandlerExists, err)

	// Unregister handler
	err = service.Unregister(protocolID)
	require.NoError(t, err)

	// Unregister again should return error
	err = service.Unregister(protocolID)
	require.Error(t, err)
	assert.Equal(t, ErrNoHandler, err)

	// Can register again after unregister
	err = service.Register(protocolID, handler)
	require.NoError(t, err)
}

func TestService_RegisterBeforeStart(t *testing.T) {
	host := newMockHost("test-peer")
	config := DefaultServiceConfig()
	logger := logrus.New()

	service := New(host, config, logger)

	protocolID := protocol.ID("/test/1.0.0")
	handler := &mockStreamHandler{}

	// Register should fail before start
	err := service.Register(protocolID, handler)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

func TestService_SendRequestAfterStop(t *testing.T) {
	host := newMockHost("test-peer")
	config := DefaultServiceConfig()
	logger := logrus.New()

	service := New(host, config, logger)

	// Start and stop the service
	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)
	err = service.Stop()
	require.NoError(t, err)

	// Send request should fail
	var req, resp string
	err = service.SendRequest(ctx, "peer123", "/test/1.0.0", &req, &resp)
	require.Error(t, err)
	assert.Equal(t, ErrServiceStopped, err)
}

func TestService_ConcurrentOperations(t *testing.T) {
	host := newMockHost("test-peer")
	config := DefaultServiceConfig()
	logger := logrus.New()

	service := New(host, config, logger)

	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop()

	// Run concurrent operations
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent registrations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			protocolID := protocol.ID(fmt.Sprintf("/test/%d/1.0.0", idx))
			handler := &mockStreamHandler{}
			if err := service.Register(protocolID, handler); err != nil {
				errors <- err
			}
		}(i)
	}

	// Concurrent unregistrations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Wait a bit to let registrations happen
			time.Sleep(10 * time.Millisecond)
			protocolID := protocol.ID(fmt.Sprintf("/test/%d/1.0.0", idx))
			if err := service.Unregister(protocolID); err != nil && err != ErrNoHandler {
				errors <- err
			}
		}(i)
	}

	// Wait for all operations to complete
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}
}

func TestRegisterProtocol(t *testing.T) {
	host := newMockHost("test-peer")
	config := DefaultServiceConfig()
	logger := logrus.New()

	service := New(host, config, logger)

	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop()

	proto := testProtocol{
		id:              "/test/1.0.0",
		maxRequestSize:  1024,
		maxResponseSize: 2048,
	}

	handler := func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
		return testResponse{Message: "pong", ID: req.ID}, nil
	}

	opts := HandlerOptions{
		Encoder:        &mockEncoder{},
		RequestTimeout: 10 * time.Second,
	}

	// Register the protocol
	err = RegisterProtocol(service, proto, handler, opts)
	require.NoError(t, err)

	// Verify handler was registered
	err = service.Unregister(proto.ID())
	require.NoError(t, err)
}

func TestRegisterChunkedProtocol(t *testing.T) {
	host := newMockHost("test-peer")
	config := DefaultServiceConfig()
	logger := logrus.New()

	service := New(host, config, logger)

	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop()

	proto := testChunkedProtocol{
		testProtocol: testProtocol{
			id:              "/test/chunked/1.0.0",
			maxRequestSize:  1024,
			maxResponseSize: 2048,
		},
		chunked: true,
	}

	handler := func(ctx context.Context, req testRequest, from peer.ID, writer ChunkedResponseWriter[testResponse]) error {
		return writer.WriteChunk(testResponse{Message: "chunk", ID: req.ID})
	}

	opts := HandlerOptions{
		Encoder:        &mockEncoder{},
		RequestTimeout: 10 * time.Second,
	}

	// Register the chunked protocol
	err = RegisterChunkedProtocol(service, proto, handler, opts)
	require.NoError(t, err)

	// Verify handler was registered
	err = service.Unregister(proto.ID())
	require.NoError(t, err)
}

func TestNewProtocol(t *testing.T) {
	proto := NewProtocol("/test/1.0.0", 1024, 2048)

	assert.Equal(t, protocol.ID("/test/1.0.0"), proto.ID())
	assert.Equal(t, uint64(1024), proto.MaxRequestSize())
	assert.Equal(t, uint64(2048), proto.MaxResponseSize())
}

func TestNewChunkedProtocol(t *testing.T) {
	proto := NewChunkedProtocol("/test/chunked/1.0.0", 1024, 2048)

	assert.Equal(t, protocol.ID("/test/chunked/1.0.0"), proto.ID())
	assert.Equal(t, uint64(1024), proto.MaxRequestSize())
	assert.Equal(t, uint64(2048), proto.MaxResponseSize())
	assert.True(t, proto.IsChunked())
}

func TestService_IntegrationScenario(t *testing.T) {
	// Create two mock hosts that can communicate
	host1, host2 := createConnectedMockHosts()

	config := DefaultServiceConfig()
	logger := logrus.New()

	// Create services for both hosts
	service1 := New(host1, config, logger)
	service2 := New(host2, config, logger)

	ctx := context.Background()

	// Start both services
	err := service1.Start(ctx)
	require.NoError(t, err)
	defer service1.Stop()

	err = service2.Start(ctx)
	require.NoError(t, err)
	defer service2.Stop()

	// Register a handler on service2
	proto := NewProtocol("/echo/1.0.0", 1024, 1024)
	echoHandler := func(ctx context.Context, req string, from peer.ID) (string, error) {
		return "Echo: " + req, nil
	}

	opts := HandlerOptions{
		Encoder: &mockEncoder{
			encodeFunc: func(msg any) ([]byte, error) {
				if str, ok := msg.(string); ok {
					return []byte(str), nil
				}
				return nil, errors.New("unsupported type")
			},
			decodeFunc: func(data []byte, msgType any) error {
				if ptr, ok := msgType.(*string); ok {
					*ptr = string(data)
					return nil
				}
				return errors.New("unsupported type")
			},
		},
		RequestTimeout: 5 * time.Second,
	}

	err = RegisterProtocol(service2, proto, echoHandler, opts)
	require.NoError(t, err)

	// Send request from service1 to service2
	req := "Hello, World!"
	var resp string

	reqOpts := RequestOptions{
		Encoder: opts.Encoder,
		Timeout: 5 * time.Second,
	}

	err = service1.SendRequestWithOptions(ctx, host2.ID(), proto.ID(), &req, &resp, reqOpts)
	require.NoError(t, err)
	assert.Equal(t, "Echo: Hello, World!", resp)
}

func TestService_ChunkedIntegrationScenario(t *testing.T) {
	// Create two mock hosts that can communicate
	host1, host2 := createConnectedMockHosts()

	config := DefaultServiceConfig()
	logger := logrus.New()

	// Create services for both hosts
	service1 := New(host1, config, logger)
	service2 := New(host2, config, logger)

	ctx := context.Background()

	// Start both services
	err := service1.Start(ctx)
	require.NoError(t, err)
	defer service1.Stop()

	err = service2.Start(ctx)
	require.NoError(t, err)
	defer service2.Stop()

	// Register a chunked handler on service2
	proto := NewChunkedProtocol("/blocks/1.0.0", 1024, 1024)
	blocksHandler := func(ctx context.Context, req int, from peer.ID, writer ChunkedResponseWriter[string]) error {
		// Send multiple chunks
		for i := 0; i < req; i++ {
			if err := writer.WriteChunk(fmt.Sprintf("Block %d", i)); err != nil {
				return err
			}
		}
		return nil
	}

	opts := HandlerOptions{
		Encoder: &mockEncoder{
			encodeFunc: func(msg any) ([]byte, error) {
				if n, ok := msg.(int); ok {
					return []byte(fmt.Sprintf("%d", n)), nil
				}
				if str, ok := msg.(string); ok {
					return []byte(str), nil
				}
				return nil, errors.New("unsupported type")
			},
			decodeFunc: func(data []byte, msgType any) error {
				if ptr, ok := msgType.(*int); ok {
					_, err := fmt.Sscanf(string(data), "%d", ptr)
					return err
				}
				if ptr, ok := msgType.(*string); ok {
					*ptr = string(data)
					return nil
				}
				return errors.New("unsupported type")
			},
		},
		RequestTimeout: 5 * time.Second,
	}

	err = RegisterChunkedProtocol(service2, proto, blocksHandler, opts)
	require.NoError(t, err)

	// Send chunked request from service1 to service2
	req := 3 // Request 3 blocks
	receivedChunks := []string{}

	chunkHandler := func(chunk any) error {
		if data, ok := chunk.([]byte); ok {
			var str string
			if err := opts.Encoder.Decode(data, &str); err == nil {
				receivedChunks = append(receivedChunks, str)
			}
		}
		return nil
	}

	chunkedClient := NewChunkedClient(host1, config.ClientConfig, logger)

	reqOpts := RequestOptions{
		Encoder: opts.Encoder,
		Timeout: 5 * time.Second,
	}

	err = chunkedClient.SendChunkedRequestWithOptions(ctx, host2.ID(), proto.ID(), &req, chunkHandler, reqOpts)
	require.NoError(t, err)

	// Verify we received the expected chunks
	assert.Equal(t, 3, len(receivedChunks))
	assert.Equal(t, "Block 0", receivedChunks[0])
	assert.Equal(t, "Block 1", receivedChunks[1])
	assert.Equal(t, "Block 2", receivedChunks[2])
}
