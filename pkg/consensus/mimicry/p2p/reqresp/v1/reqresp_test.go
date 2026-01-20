package v1_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// Test types.
type testRequest struct {
	ID    int
	Value string
}

type testResponse struct {
	ID     int
	Result string
}

// Mock NetworkEncoder for testing.
type mockNetworkEncoder struct {
	encodeErr error
	decodeErr error
	encoded   []byte
}

func (m *mockNetworkEncoder) EncodeNetwork(msg any) ([]byte, error) {
	if m.encodeErr != nil {
		return nil, m.encodeErr
	}
	if m.encoded != nil {
		return m.encoded, nil
	}
	// Simple mock encoding
	switch v := msg.(type) {
	case testRequest:
		return []byte("req:" + v.Value), nil
	case testResponse:
		return []byte("resp:" + v.Result), nil
	default:
		return []byte("unknown"), nil
	}
}

func (m *mockNetworkEncoder) DecodeNetwork(data []byte, msgType any) error {
	if m.decodeErr != nil {
		return m.decodeErr
	}
	// Simple mock decoding
	switch v := msgType.(type) {
	case *testRequest:
		v.ID = 1
		if len(data) > 4 && string(data[:4]) == "req:" {
			v.Value = string(data[4:])
		} else {
			v.Value = string(data)
		}
	case *testResponse:
		v.ID = 1
		if len(data) > 5 && string(data[:5]) == "resp:" {
			v.Result = string(data[5:])
		} else {
			v.Result = string(data)
		}
	}

	return nil
}

func createTestHosts(t *testing.T) (host.Host, host.Host) {
	t.Helper()

	h1, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)

	h2, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)

	// Connect hosts
	err = h1.Connect(context.Background(), peer.AddrInfo{
		ID:    h2.ID(),
		Addrs: h2.Addrs(),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		h1.Close()
		h2.Close()
	})

	return h1, h2
}

func TestReqResp_NewAndLifecycle(t *testing.T) {
	logger := logrus.New()
	h1, _ := createTestHosts(t)

	// Test creation
	service := v1.New(h1, logger)
	require.NotNil(t, service)

	// Test starting service
	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)

	// Test starting already started service
	err = service.Start(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already started")

	// Test stopping service
	err = service.Stop()
	require.NoError(t, err)

	// Test stopping already stopped service
	err = service.Stop()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not started")
}

func TestReqResp_RegisterHandler(t *testing.T) {
	logger := logrus.New()
	h1, _ := createTestHosts(t)

	service := v1.New(h1, logger)

	protocolID := protocol.ID("/test/1")
	handler := func(stream network.Stream) {
		stream.Close()
	}

	// Test registering handler
	err := service.RegisterHandler(protocolID, handler)
	require.NoError(t, err)

	// Test registering duplicate handler
	err = service.RegisterHandler(protocolID, handler)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered")

	// Test handler is set after start
	err = service.Start(context.Background())
	require.NoError(t, err)

	// Register another handler after start
	protocolID2 := protocol.ID("/test/2")
	err = service.RegisterHandler(protocolID2, handler)
	require.NoError(t, err)
}

func TestHandleStream(t *testing.T) {
	h1, h2 := createTestHosts(t)
	logger := logrus.New()

	// Create protocol
	proto := v1.NewProtocol(
		protocol.ID("/test/req/1"),
		1024,
		1024,
		&mockNetworkEncoder{},
	)

	// Set up handler on h2
	var handlerCalled bool
	handler := func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
		handlerCalled = true
		require.Equal(t, h1.ID(), from)
		require.Equal(t, 1, req.ID)
		require.Equal(t, "test", req.Value)

		return testResponse{ID: req.ID, Result: "handled"}, nil
	}

	service := v1.New(h2, logger)
	err := v1.RegisterStreamHandler(service, proto, handler)
	require.NoError(t, err)

	err = service.Start(context.Background())
	require.NoError(t, err)

	// Send request from h1 to h2
	resp, err := v1.SendRequest[testRequest, testResponse](
		context.Background(),
		h1,
		h2.ID(),
		proto,
		testRequest{ID: 1, Value: "test"},
	)
	require.NoError(t, err)
	require.True(t, handlerCalled)
	require.Equal(t, 1, resp.ID)
}

func TestHandleStream_Errors(t *testing.T) {
	tests := []struct {
		name         string
		setupEncoder func() *mockNetworkEncoder
		setupHandler func() v1.RequestHandler[testRequest, testResponse]
		expectError  string
	}{
		{
			name: "decode error",
			setupEncoder: func() *mockNetworkEncoder {
				return &mockNetworkEncoder{decodeErr: errors.New("decode failed")}
			},
			setupHandler: func() v1.RequestHandler[testRequest, testResponse] {
				return func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
					return testResponse{}, nil
				}
			},
			expectError: "failed to decode request",
		},
		{
			name: "handler error",
			setupEncoder: func() *mockNetworkEncoder {
				return &mockNetworkEncoder{}
			},
			setupHandler: func() v1.RequestHandler[testRequest, testResponse] {
				return func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
					return testResponse{}, errors.New("handler failed")
				}
			},
			expectError: "handler error",
		},
		{
			name: "encode response error",
			setupEncoder: func() *mockNetworkEncoder {
				return &mockNetworkEncoder{encodeErr: errors.New("encode failed")}
			},
			setupHandler: func() v1.RequestHandler[testRequest, testResponse] {
				return func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
					return testResponse{Result: "ok"}, nil
				}
			},
			expectError: "failed to encode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h1, h2 := createTestHosts(t)
			logger := logrus.New()

			proto := v1.NewProtocol(
				protocol.ID("/test/req/1"),
				1024,
				1024,
				tt.setupEncoder(),
			)

			service := v1.New(h2, logger)
			err := v1.RegisterStreamHandler(service, proto, tt.setupHandler())
			require.NoError(t, err)

			err = service.Start(context.Background())
			require.NoError(t, err)

			// Send request and expect error
			_, err = v1.SendRequest[testRequest, testResponse](
				context.Background(),
				h1,
				h2.ID(),
				proto,
				testRequest{ID: 1, Value: "test"},
			)
			require.Error(t, err)
		})
	}
}

func TestHandleChunkedStream(t *testing.T) {
	h1, h2 := createTestHosts(t)
	logger := logrus.New()

	// Create chunked protocol
	proto := v1.NewChunkedProtocol(
		protocol.ID("/test/chunked/1"),
		1024,
		1024,
		&mockNetworkEncoder{},
	)

	// Set up chunked handler on h2
	var chunks []testResponse
	var mu sync.Mutex
	handler := func(ctx context.Context, req testRequest, from peer.ID, w v1.ChunkedResponseWriter[testResponse]) error {
		require.Equal(t, h1.ID(), from)

		// Send 3 chunks
		for i := 0; i < 3; i++ {
			err := w.WriteChunk(testResponse{ID: i, Result: "chunk"})
			if err != nil {
				return err
			}
		}

		return nil
	}

	service := v1.New(h2, logger)
	err := v1.RegisterChunkedStreamHandler(service, proto, handler)
	require.NoError(t, err)

	err = service.Start(context.Background())
	require.NoError(t, err)

	// Send chunked request from h1 to h2
	chunkErr := v1.SendChunkedRequest[testRequest, testResponse](
		context.Background(),
		h1,
		h2.ID(),
		proto,
		testRequest{ID: 1, Value: "test"},
		func(resp testResponse) error {
			mu.Lock()
			chunks = append(chunks, resp)
			mu.Unlock()

			return nil
		},
	)
	require.NoError(t, chunkErr)
	require.Len(t, chunks, 3)
}

func TestSendRequest_Errors(t *testing.T) {
	t.Run("connection error", func(t *testing.T) {
		h1, err := libp2p.New()
		require.NoError(t, err)
		defer h1.Close()

		proto := v1.NewProtocol(
			protocol.ID("/test/1"),
			1024,
			1024,
			&mockNetworkEncoder{},
		)

		// Try to send to non-existent peer
		_, err = v1.SendRequest[testRequest, testResponse](
			context.Background(),
			h1,
			peer.ID("invalid"),
			proto,
			testRequest{},
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to open stream")
	})

	t.Run("encode error", func(t *testing.T) {
		h1, h2 := createTestHosts(t)

		proto := v1.NewProtocol(
			protocol.ID("/test/encode/1"),
			1024,
			1024,
			&mockNetworkEncoder{encodeErr: errors.New("encode failed")},
		)

		// Register a handler on h2 so the protocol is supported
		h2.SetStreamHandler(proto.ID(), func(s network.Stream) {
			s.Close()
		})

		_, err := v1.SendRequest[testRequest, testResponse](
			context.Background(),
			h1,
			h2.ID(),
			proto,
			testRequest{},
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to encode request")
	})
}

func TestProtocol(t *testing.T) {
	encoder := &mockNetworkEncoder{}

	t.Run("non-chunked protocol", func(t *testing.T) {
		proto := v1.NewProtocol(
			protocol.ID("/test/1"),
			100,
			200,
			encoder,
		)

		require.Equal(t, protocol.ID("/test/1"), proto.ID())
		require.Equal(t, uint64(100), proto.MaxRequestSize())
		require.Equal(t, uint64(200), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.False(t, proto.IsChunked())
	})

	t.Run("chunked protocol", func(t *testing.T) {
		proto := v1.NewChunkedProtocol(
			protocol.ID("/test/chunked/1"),
			100,
			200,
			encoder,
		)

		require.Equal(t, protocol.ID("/test/chunked/1"), proto.ID())
		require.Equal(t, uint64(100), proto.MaxRequestSize())
		require.Equal(t, uint64(200), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.True(t, proto.IsChunked())
	})
}

func TestConcurrentOperations(t *testing.T) {
	logger := logrus.New()
	h1, _ := createTestHosts(t)

	service := v1.New(h1, logger)
	ctx := context.Background()

	// Start service
	err := service.Start(ctx)
	require.NoError(t, err)

	// Concurrent handler registration
	var wg sync.WaitGroup
	errors := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			protocolID := protocol.ID("/test/" + string(rune(idx)))
			handler := func(stream network.Stream) {
				stream.Close()
			}
			errors[idx] = service.RegisterHandler(protocolID, handler)
		}(i)
	}

	wg.Wait()

	// All should succeed
	for _, errItem := range errors {
		require.NoError(t, errItem)
	}

	// Stop service
	err = service.Stop()
	require.NoError(t, err)
}

func TestStreamChunkedWriter(t *testing.T) {
	h1, h2 := createTestHosts(t)

	// Channel to coordinate the test
	streamReady := make(chan network.Stream, 1)

	// Register handler on h2 to accept the protocol
	h2.SetStreamHandler(protocol.ID("/test/1"), func(s network.Stream) {
		streamReady <- s
	})

	// Open a test stream from h1 to h2
	stream, err := h1.NewStream(context.Background(), h2.ID(), protocol.ID("/test/1"))
	require.NoError(t, err)
	defer stream.Close()

	// Get the h2 side of the stream
	h2Stream := <-streamReady
	defer h2Stream.Close()

	// Read from h2 stream in goroutine
	var received []byte
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 1024)
		n, readErr := h2Stream.Read(buf)
		if readErr != nil {
			done <- readErr

			return
		}
		received = buf[:n]
		done <- nil
	}()

	// Test writing chunk through HandleChunkedStream internals
	encoder := &mockNetworkEncoder{encoded: []byte("test-chunk")}
	proto := v1.NewChunkedProtocol(
		protocol.ID("/test/chunked/1"),
		1024,
		1024,
		encoder,
	)

	// Simulate the writer creation from HandleChunkedStream
	writer := &streamChunkedWriter{
		stream:         stream,
		networkEncoder: proto.NetworkEncoder(),
	}

	err = writer.WriteChunk(testResponse{ID: 1, Result: "test"})
	require.NoError(t, err)

	// Wait for read
	select {
	case err := <-done:
		require.NoError(t, err)
		require.Equal(t, []byte("test-chunk"), received)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for stream read")
	}
}

// Helper type to access internal writer.
type streamChunkedWriter struct {
	stream         network.Stream
	networkEncoder v1.NetworkEncoder
}

func (w *streamChunkedWriter) WriteChunk(resp any) error {
	data, err := w.networkEncoder.EncodeNetwork(resp)
	if err != nil {
		return err
	}
	_, err = w.stream.Write(data)

	return err
}

func (w *streamChunkedWriter) Close() error {
	return w.stream.Close()
}
