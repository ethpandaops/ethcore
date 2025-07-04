package v1

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHandler(t *testing.T) {
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

	logger := logrus.New()

	h := NewHandler(proto, handler, opts, logger)
	require.NotNil(t, h)

	// Verify it implements StreamHandler
	var _ StreamHandler = h
}

func TestHandler_HandleStream(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name           string
		setupStream    func() *mockStream
		handler        RequestHandler[testRequest, testResponse]
		encoder        Encoder
		compressor     Compressor
		maxRequestSize uint64
		expectedStatus Status
		expectedResp   string
	}{
		{
			name: "successful_request",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "remote", "local")
				// Prepare request data
				reqData := []byte(`{"Message":"ping","ID":1}`)
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(reqData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, reqData...)
				stream.setReadData(buf)
				return stream
			},
			handler: func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
				return testResponse{Message: "pong", ID: req.ID, Time: time.Now()}, nil
			},
			encoder: &mockEncoder{
				encodeFunc: func(msg any) ([]byte, error) {
					if req, ok := msg.(testRequest); ok {
						return []byte(`{"Message":"` + req.Message + `","ID":1}`), nil
					}
					if resp, ok := msg.(testResponse); ok {
						return []byte(`{"Message":"` + resp.Message + `","ID":1}`), nil
					}
					return nil, errors.New("unknown type")
				},
				decodeFunc: func(data []byte, msgType any) error {
					if req, ok := msgType.(*testRequest); ok {
						req.Message = "ping"
						req.ID = 1
						return nil
					}
					return errors.New("unknown type")
				},
			},
			maxRequestSize: 1024,
			expectedStatus: StatusSuccess,
			expectedResp:   `{"Message":"pong","ID":1}`,
		},
		{
			name: "request_too_large",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "remote", "local")
				// Prepare oversized request
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, 2048) // Exceeds max size
				buf = append(buf, sizeBuf...)
				stream.setReadData(buf)
				return stream
			},
			handler: func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
				return testResponse{}, nil
			},
			encoder:        &mockEncoder{},
			maxRequestSize: 1024,
			expectedStatus: StatusInvalidRequest,
		},
		{
			name: "handler_returns_error",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "remote", "local")
				reqData := []byte(`{"Message":"ping","ID":1}`)
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(reqData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, reqData...)
				stream.setReadData(buf)
				return stream
			},
			handler: func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
				return testResponse{}, errors.New("handler error")
			},
			encoder: &mockEncoder{
				decodeFunc: func(data []byte, msgType any) error {
					if req, ok := msgType.(*testRequest); ok {
						req.Message = "ping"
						req.ID = 1
						return nil
					}
					return errors.New("unknown type")
				},
			},
			maxRequestSize: 1024,
			expectedStatus: StatusServerError,
		},
		{
			name: "decode_error",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "remote", "local")
				reqData := []byte("invalid json")
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(reqData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, reqData...)
				stream.setReadData(buf)
				return stream
			},
			handler: func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
				return testResponse{}, nil
			},
			encoder: &mockEncoder{
				decodeFunc: func(data []byte, msgType any) error {
					return errors.New("decode error")
				},
			},
			maxRequestSize: 1024,
			expectedStatus: StatusInvalidRequest,
		},
		{
			name: "with_compression",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/1.0.0", "remote", "local")
				// Prepare compressed request
				reqData := []byte("COMPRESSED:ping")
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(reqData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, reqData...)
				stream.setReadData(buf)
				return stream
			},
			handler: func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
				return testResponse{Message: "pong", ID: req.ID}, nil
			},
			encoder: &mockEncoder{
				encodeFunc: func(msg any) ([]byte, error) {
					if resp, ok := msg.(testResponse); ok {
						return []byte(resp.Message), nil
					}
					return []byte("ping"), nil
				},
				decodeFunc: func(data []byte, msgType any) error {
					if req, ok := msgType.(*testRequest); ok {
						req.Message = string(data)
						req.ID = 1
						return nil
					}
					return nil
				},
			},
			compressor:     &mockCompressor{},
			maxRequestSize: 1024,
			expectedStatus: StatusSuccess,
			expectedResp:   "COMPRESSED:pong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := tt.setupStream()
			ctx := context.Background()

			proto := testProtocol{
				id:              "/test/1.0.0",
				maxRequestSize:  tt.maxRequestSize,
				maxResponseSize: 2048,
			}

			opts := HandlerOptions{
				Encoder:        tt.encoder,
				Compressor:     tt.compressor,
				RequestTimeout: 5 * time.Second,
			}

			h := NewHandler(proto, tt.handler, opts, logger)

			// Handle the stream
			h.HandleStream(ctx, stream)

			// Check what was written to the stream
			written := stream.getWrittenData()
			require.GreaterOrEqual(t, len(written), 1, "Should have written at least status byte")

			// Check status
			status := Status(written[0])
			assert.Equal(t, tt.expectedStatus, status)

			if tt.expectedStatus == StatusSuccess && tt.expectedResp != "" {
				// Check response data
				require.GreaterOrEqual(t, len(written), 5, "Should have status + size + data")
				size := binary.BigEndian.Uint32(written[1:5])
				require.Equal(t, len(written)-5, int(size), "Size should match data length")

				respData := written[5:]
				assert.Equal(t, tt.expectedResp, string(respData))
			}
		})
	}
}

func TestHandler_ReadRequest(t *testing.T) {
	h := &Handler[testRequest, testResponse]{
		log: logrus.New(),
		protocol: testProtocol{
			id:              "/test/1.0.0",
			maxRequestSize:  1024,
			maxResponseSize: 2048,
		},
	}

	tests := []struct {
		name          string
		setupStream   func() network.Stream
		maxSize       uint64
		encoder       Encoder
		compressor    Compressor
		expectedReq   testRequest
		expectedError string
	}{
		{
			name: "successful_read",
			setupStream: func() network.Stream {
				stream := newMockStream("test", "/test/1.0.0", "local", "remote")
				data := []byte("test request")
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(data)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, data...)
				stream.setReadData(buf)
				return stream
			},
			maxSize: 1024,
			encoder: &mockEncoder{
				decodeFunc: func(data []byte, msgType any) error {
					if req, ok := msgType.(*testRequest); ok {
						req.Message = string(data)
						req.ID = 123
						return nil
					}
					return errors.New("unknown type")
				},
			},
			expectedReq: testRequest{Message: "test request", ID: 123},
		},
		{
			name: "size_exceeds_max",
			setupStream: func() network.Stream {
				stream := newMockStream("test", "/test/1.0.0", "local", "remote")
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, 2048)
				buf = append(buf, sizeBuf...)
				stream.setReadData(buf)
				return stream
			},
			maxSize:       1024,
			encoder:       &mockEncoder{},
			expectedError: "exceeds maximum",
		},
		{
			name: "empty_request",
			setupStream: func() network.Stream {
				stream := newMockStream("test", "/test/1.0.0", "local", "remote")
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, 0)
				buf = append(buf, sizeBuf...)
				stream.setReadData(buf)
				return stream
			},
			maxSize:       1024,
			encoder:       &mockEncoder{},
			expectedError: "empty request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := tt.setupStream()
			h.encoder = tt.encoder
			h.compressor = tt.compressor
			h.protocol = testProtocol{
				id:              "/test/1.0.0",
				maxRequestSize:  tt.maxSize,
				maxResponseSize: 2048,
			}

			req, err := h.readRequest(stream)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedReq, req)
			}
		})
	}
}

func TestHandler_WriteResponse(t *testing.T) {
	h := &Handler[testRequest, testResponse]{
		log: logrus.New(),
		protocol: testProtocol{
			id:              "/test/1.0.0",
			maxRequestSize:  1024,
			maxResponseSize: 2048,
		},
	}

	tests := []struct {
		name          string
		response      testResponse
		encoder       Encoder
		compressor    Compressor
		expectedError string
		verifyWrite   func(t *testing.T, data []byte)
	}{
		{
			name:     "successful_write",
			response: testResponse{Message: "test response", ID: 123},
			encoder: &mockEncoder{
				encodeFunc: func(msg any) ([]byte, error) {
					if resp, ok := msg.(testResponse); ok {
						return []byte(resp.Message), nil
					}
					return nil, errors.New("unknown type")
				},
			},
			verifyWrite: func(t *testing.T, data []byte) {
				t.Helper()
				require.GreaterOrEqual(t, len(data), 5)
				assert.Equal(t, byte(StatusSuccess), data[0])
				size := binary.BigEndian.Uint32(data[1:5])
				assert.Equal(t, uint32(len("test response")), size)
				assert.Equal(t, "test response", string(data[5:]))
			},
		},
		{
			name:     "encode_error",
			response: testResponse{Message: "test", ID: 123},
			encoder: &mockEncoder{
				encodeFunc: func(msg any) ([]byte, error) {
					return nil, errors.New("encode error")
				},
			},
			expectedError: "encode error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := newMockStream("test", "/test/1.0.0", "local", "remote")
			h.encoder = tt.encoder
			h.compressor = tt.compressor

			err := h.writeResponse(stream, StatusSuccess, tt.response)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				if tt.verifyWrite != nil {
					data := stream.getWrittenData()
					tt.verifyWrite(t, data)
				}
			}
		})
	}
}

func TestHandler_WriteErrorResponse(t *testing.T) {
	h := &Handler[testRequest, testResponse]{
		log: logrus.New(),
		protocol: testProtocol{
			id:              "/test/1.0.0",
			maxRequestSize:  1024,
			maxResponseSize: 2048,
		},
	}

	tests := []struct {
		name        string
		status      Status
		verifyWrite func(t *testing.T, data []byte)
	}{
		{
			name:   "invalid_request_error",
			status: StatusInvalidRequest,
			verifyWrite: func(t *testing.T, data []byte) {
				t.Helper()
				require.Equal(t, 1, len(data))
				assert.Equal(t, byte(StatusInvalidRequest), data[0])
			},
		},
		{
			name:   "server_error",
			status: StatusServerError,
			verifyWrite: func(t *testing.T, data []byte) {
				t.Helper()
				require.Equal(t, 1, len(data))
				assert.Equal(t, byte(StatusServerError), data[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := newMockStream("test", "/test/1.0.0", "local", "remote")
			// Test the standalone writeErrorResponse which just writes status
			err := h.writeResponse(stream, tt.status, testResponse{})
			require.NoError(t, err)

			if tt.verifyWrite != nil {
				data := stream.getWrittenData()
				tt.verifyWrite(t, data)
			}
		})
	}
}

func TestHandler_TimeoutHandling(t *testing.T) {
	proto := testProtocol{
		id:              "/test/1.0.0",
		maxRequestSize:  1024,
		maxResponseSize: 2048,
	}

	// Handler that takes longer than timeout
	slowHandler := func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
		select {
		case <-time.After(200 * time.Millisecond):
			return testResponse{Message: "too late"}, nil
		case <-ctx.Done():
			return testResponse{}, ctx.Err()
		}
	}

	opts := HandlerOptions{
		Encoder: &mockEncoder{
			decodeFunc: func(data []byte, msgType any) error {
				if req, ok := msgType.(*testRequest); ok {
					req.Message = "test"
					return nil
				}
				return nil
			},
		},
		RequestTimeout: 50 * time.Millisecond, // Very short timeout
	}

	logger := logrus.New()
	h := NewHandler(proto, slowHandler, opts, logger)

	// Setup stream with valid request
	stream := newMockStream("test-stream", "/test/1.0.0", "remote", "local")
	reqData := []byte("test request")
	var buf []byte
	sizeBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBuf, uint32(len(reqData)))
	buf = append(buf, sizeBuf...)
	buf = append(buf, reqData...)
	stream.setReadData(buf)

	ctx := context.Background()
	h.HandleStream(ctx, stream)

	// Verify error response was written
	written := stream.getWrittenData()
	require.GreaterOrEqual(t, len(written), 1)
	assert.Equal(t, byte(StatusServerError), written[0])
}

func TestHandler_PanicRecovery(t *testing.T) {
	proto := testProtocol{
		id:              "/test/1.0.0",
		maxRequestSize:  1024,
		maxResponseSize: 2048,
	}

	// Handler that panics
	panicHandler := func(ctx context.Context, req testRequest, from peer.ID) (testResponse, error) {
		panic("handler panic")
	}

	opts := HandlerOptions{
		Encoder: &mockEncoder{
			decodeFunc: func(data []byte, msgType any) error {
				if req, ok := msgType.(*testRequest); ok {
					req.Message = "test"
					return nil
				}
				return nil
			},
		},
		RequestTimeout: 5 * time.Second,
	}

	logger := logrus.New()
	h := NewHandler(proto, panicHandler, opts, logger)

	// Setup stream with valid request
	stream := newMockStream("test-stream", "/test/1.0.0", "remote", "local")
	reqData := []byte("test request")
	var buf []byte
	sizeBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBuf, uint32(len(reqData)))
	buf = append(buf, sizeBuf...)
	buf = append(buf, reqData...)
	stream.setReadData(buf)

	ctx := context.Background()

	// Should not panic
	assert.NotPanics(t, func() {
		h.HandleStream(ctx, stream)
	})

	// Verify error response was written
	written := stream.getWrittenData()
	require.GreaterOrEqual(t, len(written), 1)
	assert.Equal(t, byte(StatusServerError), written[0])
}
