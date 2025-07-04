package v1

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRequestString = "test request"

func TestNewClient(t *testing.T) {
	host := newMockHost("test-peer")
	config := ClientConfig{
		DefaultTimeout: 10 * time.Second,
		MaxRetries:     3,
		RetryDelay:     100 * time.Millisecond,
	}
	logger := logrus.New()

	client := NewClient(host, config, logger)
	require.NotNil(t, client)

	// Verify client is not nil and implements the interface
	var _ = client
}

func TestClient_SendRequest(t *testing.T) {
	ctx := context.Background()
	host := newMockHost("test-peer")
	config := ClientConfig{
		DefaultTimeout: 1 * time.Second,
		MaxRetries:     0,
	}
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	client := NewClient(host, config, logger)

	tests := []struct {
		name          string
		setupStream   func() *mockStream
		encoder       Encoder
		expectedError string
	}{
		{
			name: "successful_request",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")
				// Prepare response data
				respData := []byte("test response")
				var buf []byte
				buf = append(buf, byte(StatusSuccess))
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(respData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, respData...)
				stream.setReadData(buf)

				return stream
			},
			encoder: &mockEncoder{},
		},
		{
			name: "stream_creation_fails",
			setupStream: func() *mockStream {
				return nil
			},
			encoder:       &mockEncoder{},
			expectedError: "failed to open stream",
		},
		{
			name: "encoder_not_provided",
			setupStream: func() *mockStream {
				return newMockStream("test-stream", "/test/1.0.0", "local", "remote")
			},
			encoder:       nil,
			expectedError: "encoder must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock host behavior
			if tt.setupStream != nil {
				stream := tt.setupStream()
				host.newStreamFunc = func(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
					if stream == nil {
						return nil, errors.New("stream creation failed")
					}

					return stream, nil
				}
			}

			req := testRequestString
			var resp string

			opts := RequestOptions{
				Encoder: tt.encoder,
			}

			err := client.SendRequestWithOptions(ctx, "remote-peer", "/test/1.0.0", &req, &resp, opts)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "test response", resp)
			}
		})
	}
}

func TestClient_SendRequestWithTimeout(t *testing.T) {
	ctx := context.Background()
	host := newMockHost("test-peer")
	config := ClientConfig{
		DefaultTimeout: 1 * time.Second,
		MaxRetries:     0,
	}
	logger := logrus.New()

	client := NewClient(host, config, logger)

	// Test that custom timeout is applied
	customTimeout := 500 * time.Millisecond
	startTime := time.Now()

	// Setup a stream that delays response
	stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")
	host.newStreamFunc = func(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
		// Check if context has timeout
		deadline, ok := ctx.Deadline()
		if ok {
			timeUntilDeadline := time.Until(deadline)
			// Verify timeout is approximately what we set
			assert.InDelta(t, customTimeout.Milliseconds(), timeUntilDeadline.Milliseconds(), 100)
		}

		return stream, nil
	}

	req := testRequestString
	var resp string

	err := client.SendRequestWithTimeout(ctx, "remote-peer", "/test/1.0.0", &req, &resp, customTimeout)
	elapsed := time.Since(startTime)

	// Should fail because no encoder is provided by default
	require.Error(t, err)
	assert.Contains(t, err.Error(), "encoder must be provided")
	assert.Less(t, elapsed, customTimeout+100*time.Millisecond)
}

func TestClient_RetryLogic(t *testing.T) {
	ctx := context.Background()
	host := newMockHost("test-peer")
	config := ClientConfig{
		DefaultTimeout: 500 * time.Millisecond,
		MaxRetries:     2,
		RetryDelay:     50 * time.Millisecond,
	}
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	client := NewClient(host, config, logger)

	attemptCount := 0
	host.newStreamFunc = func(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
		attemptCount++
		if attemptCount <= 2 {
			return nil, errors.New("temporary failure")
		}
		// Success on third attempt
		stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")
		// Prepare successful response
		respData := []byte("success after retries")
		var buf []byte
		buf = append(buf, byte(StatusSuccess))
		sizeBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(sizeBuf, uint32(len(respData)))
		buf = append(buf, sizeBuf...)
		buf = append(buf, respData...)
		stream.setReadData(buf)

		return stream, nil
	}

	req := testRequestString
	var resp string

	opts := RequestOptions{
		Encoder: &mockEncoder{},
	}

	err := client.SendRequestWithOptions(ctx, "remote-peer", "/test/1.0.0", &req, &resp, opts)
	require.NoError(t, err)
	assert.Equal(t, "success after retries", resp)
	assert.Equal(t, 3, attemptCount)
}

func TestClient_WriteRequest(t *testing.T) {
	client := &client{
		log: logrus.New(),
	}

	tests := []struct {
		name          string
		request       any
		encoder       Encoder
		compressor    Compressor
		expectedError string
		verifyWrite   func(t *testing.T, stream *mockStream)
	}{
		{
			name:    "successful_write_no_compression",
			request: testRequestString,
			encoder: &mockEncoder{},
			verifyWrite: func(t *testing.T, stream *mockStream) {
				t.Helper()
				data := stream.getWrittenData()
				require.GreaterOrEqual(t, len(data), 4)

				// Check size prefix
				size := binary.BigEndian.Uint32(data[:4])
				assert.Equal(t, uint32(len(testRequestString)), size)

				// Check data
				assert.Equal(t, testRequestString, string(data[4:]))
			},
		},
		{
			name:       "successful_write_with_compression",
			request:    "test request",
			encoder:    &mockEncoder{},
			compressor: &mockCompressor{},
			verifyWrite: func(t *testing.T, stream *mockStream) {
				t.Helper()
				data := stream.getWrittenData()
				require.GreaterOrEqual(t, len(data), 4)

				// Check size prefix
				size := binary.BigEndian.Uint32(data[:4])
				expectedCompressed := "COMPRESSED:" + testRequestString
				assert.Equal(t, uint32(len(expectedCompressed)), size)

				// Check compressed data
				assert.Equal(t, expectedCompressed, string(data[4:]))
			},
		},
		{
			name:    "encoder_fails",
			request: testRequestString,
			encoder: &mockEncoder{
				encodeFunc: func(msg any) ([]byte, error) {
					return nil, errors.New("encode error")
				},
			},
			expectedError: "failed to encode request",
		},
		{
			name:    "compressor_fails",
			request: testRequestString,
			encoder: &mockEncoder{},
			compressor: &mockCompressor{
				compressFunc: func(data []byte) ([]byte, error) {
					return nil, errors.New("compress error")
				},
			},
			expectedError: "failed to compress request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")
			opts := RequestOptions{
				Encoder:    tt.encoder,
				Compressor: tt.compressor,
			}

			err := client.writeRequest(stream, tt.request, opts)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				if tt.verifyWrite != nil {
					tt.verifyWrite(t, stream)
				}
			}
		})
	}
}

func TestClient_ReadResponse(t *testing.T) {
	client := &client{
		log: logrus.New(),
	}

	tests := []struct {
		name          string
		setupStream   func() *mockStream
		encoder       Encoder
		compressor    Compressor
		expectedResp  string
		expectedError string
	}{
		{
			name: "successful_read_no_compression",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")
				respData := []byte("test response")
				var buf []byte
				buf = append(buf, byte(StatusSuccess))
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(respData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, respData...)
				stream.setReadData(buf)

				return stream
			},
			encoder:      &mockEncoder{},
			expectedResp: "test response",
		},
		{
			name: "successful_read_with_compression",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")
				respData := []byte("COMPRESSED:test response")
				var buf []byte
				buf = append(buf, byte(StatusSuccess))
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(respData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, respData...)
				stream.setReadData(buf)

				return stream
			},
			encoder:      &mockEncoder{},
			compressor:   &mockCompressor{},
			expectedResp: "test response",
		},
		{
			name: "error_status",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")
				stream.setReadData([]byte{byte(StatusServerError)})

				return stream
			},
			encoder:       &mockEncoder{},
			expectedError: "server returned error status: server_error",
		},
		{
			name: "empty_response",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")
				var buf []byte
				buf = append(buf, byte(StatusSuccess))
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, 0)
				buf = append(buf, sizeBuf...)
				stream.setReadData(buf)

				return stream
			},
			encoder:       &mockEncoder{},
			expectedError: "received empty response",
		},
		{
			name: "decoder_fails",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")
				respData := []byte("test response")
				var buf []byte
				buf = append(buf, byte(StatusSuccess))
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(respData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, respData...)
				stream.setReadData(buf)

				return stream
			},
			encoder: &mockEncoder{
				decodeFunc: func(data []byte, msgType any) error {
					return errors.New("decode error")
				},
			},
			expectedError: "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := tt.setupStream()
			opts := RequestOptions{
				Encoder:    tt.encoder,
				Compressor: tt.compressor,
			}

			var resp string
			err := client.readResponse(stream, &resp, opts)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResp, resp)
			}
		})
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	host := newMockHost("test-peer")
	config := ClientConfig{
		DefaultTimeout: 5 * time.Second,
		MaxRetries:     2,
		RetryDelay:     100 * time.Millisecond,
	}
	logger := logrus.New()

	client := NewClient(host, config, logger)

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	attemptCount := 0
	host.newStreamFunc = func(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
		attemptCount++
		if attemptCount == 1 {
			// Cancel context after first attempt
			cancel()
		}

		return nil, errors.New("temporary failure")
	}

	req := testRequestString
	var resp string

	opts := RequestOptions{
		Encoder: &mockEncoder{},
	}

	err := client.SendRequestWithOptions(ctx, "remote-peer", "/test/1.0.0", &req, &resp, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
	assert.Equal(t, 1, attemptCount) // Should not retry after context cancellation
}

func TestRequest_FluentAPI(t *testing.T) {
	host := newMockHost("test-peer")
	config := ClientConfig{
		DefaultTimeout: 1 * time.Second,
	}
	logger := logrus.New()

	client := NewClient(host, config, logger)
	proto := testProtocol{
		id:              "/test/1.0.0",
		maxRequestSize:  1024,
		maxResponseSize: 1024,
	}

	t.Run("successful_request", func(t *testing.T) {
		req := NewRequest[testRequest, testResponse](client, proto)
		require.NotNil(t, req)

		// Test fluent API
		req = req.To("remote-peer").WithTimeout(500 * time.Millisecond)
		assert.Equal(t, peer.ID("remote-peer"), req.peerID)
		assert.Equal(t, 500*time.Millisecond, req.timeout)
	})

	t.Run("missing_peer_id", func(t *testing.T) {
		req := NewRequest[testRequest, testResponse](client, proto)

		ctx := context.Background()
		testReq := testRequest{Message: "test", ID: 1}

		_, err := req.Send(ctx, testReq)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "peer ID not set")
	})
}

func TestChunkedClient(t *testing.T) {
	host := newMockHost("test-peer")
	config := ClientConfig{
		DefaultTimeout: 1 * time.Second,
		MaxRetries:     0,
	}
	logger := logrus.New()

	chunkedClient := NewChunkedClient(host, config, logger)
	require.NotNil(t, chunkedClient)

	t.Run("send_chunked_request", func(t *testing.T) {
		// Setup mock stream with multiple chunks
		stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")

		// Prepare multiple chunks
		chunks := []string{"chunk1", "chunk2", "chunk3"}
		var buf []byte
		for _, chunk := range chunks {
			buf = append(buf, byte(StatusSuccess))
			sizeBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(sizeBuf, uint32(len(chunk)))
			buf = append(buf, sizeBuf...)
			buf = append(buf, chunk...)
		}
		stream.setReadData(buf)

		host.newStreamFunc = func(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
			return stream, nil
		}

		ctx := context.Background()
		req := testRequestString

		receivedChunks := []string{}
		chunkHandler := func(chunk any) error {
			// In real implementation, chunk would be decoded
			// For now, we just store the raw data
			if data, ok := chunk.([]byte); ok {
				receivedChunks = append(receivedChunks, string(data))
			}

			return nil
		}

		opts := RequestOptions{
			Encoder: &mockEncoder{},
		}

		err := chunkedClient.SendChunkedRequestWithOptions(ctx, "remote-peer", "/test/1.0.0", &req, chunkHandler, opts)
		require.NoError(t, err)
		assert.Equal(t, 3, len(receivedChunks))
	})

	t.Run("chunk_handler_error", func(t *testing.T) {
		stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")

		// Prepare a chunk
		var buf []byte
		buf = append(buf, byte(StatusSuccess))
		sizeBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(sizeBuf, uint32(len("chunk1")))
		buf = append(buf, sizeBuf...)
		buf = append(buf, []byte("chunk1")...)
		stream.setReadData(buf)

		host.newStreamFunc = func(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
			return stream, nil
		}

		ctx := context.Background()
		req := testRequestString

		chunkHandler := func(chunk any) error {
			return errors.New("handler error")
		}

		opts := RequestOptions{
			Encoder: &mockEncoder{},
		}

		err := chunkedClient.SendChunkedRequestWithOptions(ctx, "remote-peer", "/test/1.0.0", &req, chunkHandler, opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chunk handler error")
	})
}

func TestClient_DataSizeValidation(t *testing.T) {
	client := &client{
		log: logrus.New(),
	}

	// Create a very large message that would exceed uint32 max when encoded
	largeData := make([]byte, 1<<32) // 4GB

	stream := newMockStream("test-stream", "/test/1.0.0", "local", "remote")
	opts := RequestOptions{
		Encoder: &mockEncoder{
			encodeFunc: func(msg any) ([]byte, error) {
				return largeData, nil
			},
		},
	}

	err := client.writeRequest(stream, "test", opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data size")
}
