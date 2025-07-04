package v1

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testString = "test"

func TestNewChunkedHandler(t *testing.T) {
	proto := testChunkedProtocol{
		testProtocol: testProtocol{
			id:              "/test/chunked/1.0.0",
			maxRequestSize:  1024,
			maxResponseSize: 2048,
		},
		chunked: true,
	}

	handler := func(ctx context.Context, req testRequest, from peer.ID, writer ChunkedResponseWriter[testResponse]) error {
		return writer.WriteChunk(testResponse{Message: "chunk1", ID: req.ID})
	}

	opts := HandlerOptions{
		Encoder:        &mockEncoder{},
		RequestTimeout: 10 * time.Second,
	}

	logger := logrus.New()

	h := NewChunkedHandler(proto, handler, opts, logger)
	require.NotNil(t, h)

	// Verify it implements StreamHandler
	var _ StreamHandler = h
}

func TestChunkedHandler_HandleStream(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name             string
		setupStream      func() *mockStream
		handler          ChunkedRequestHandler[testRequest, testResponse]
		encoder          Encoder
		compressor       Compressor
		maxRequestSize   uint64
		expectedChunks   int
		expectedStatus   []Status
		expectedMessages []string
		expectedError    bool
	}{
		{
			name: "successful_single_chunk",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/chunked/1.0.0", "remote", "local")
				// Prepare request data
				reqData := []byte("ping")
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(reqData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, reqData...)
				stream.setReadData(buf)

				return stream
			},
			handler: func(ctx context.Context, req testRequest, from peer.ID, writer ChunkedResponseWriter[testResponse]) error {
				return writer.WriteChunk(testResponse{Message: "pong", ID: req.ID})
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
			maxRequestSize:   1024,
			expectedChunks:   1,
			expectedStatus:   []Status{StatusSuccess},
			expectedMessages: []string{"pong"},
		},
		{
			name: "successful_multiple_chunks",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/chunked/1.0.0", "remote", "local")
				reqData := []byte("ping")
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(reqData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, reqData...)
				stream.setReadData(buf)

				return stream
			},
			handler: func(ctx context.Context, req testRequest, from peer.ID, writer ChunkedResponseWriter[testResponse]) error {
				// Write multiple chunks
				chunks := []string{"chunk1", "chunk2", "chunk3"}
				for i, chunk := range chunks {
					if err := writer.WriteChunk(testResponse{Message: chunk, ID: i}); err != nil {
						return err
					}
				}

				return nil
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
			maxRequestSize:   1024,
			expectedChunks:   3,
			expectedStatus:   []Status{StatusSuccess, StatusSuccess, StatusSuccess},
			expectedMessages: []string{"chunk1", "chunk2", "chunk3"},
		},
		{
			name: "handler_error",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/chunked/1.0.0", "remote", "local")
				reqData := []byte("ping")
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(reqData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, reqData...)
				stream.setReadData(buf)

				return stream
			},
			handler: func(ctx context.Context, req testRequest, from peer.ID, writer ChunkedResponseWriter[testResponse]) error {
				return errors.New("handler error")
			},
			encoder: &mockEncoder{
				decodeFunc: func(data []byte, msgType any) error {
					if req, ok := msgType.(*testRequest); ok {
						req.Message = string(data)
						req.ID = 1

						return nil
					}

					return nil
				},
			},
			maxRequestSize: 1024,
			expectedChunks: 1,
			expectedStatus: []Status{StatusServerError},
			expectedError:  false,
		},
		{
			name: "write_chunk_after_error",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/chunked/1.0.0", "remote", "local")
				reqData := []byte("ping")
				var buf []byte
				sizeBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(sizeBuf, uint32(len(reqData)))
				buf = append(buf, sizeBuf...)
				buf = append(buf, reqData...)
				stream.setReadData(buf)

				return stream
			},
			handler: func(ctx context.Context, req testRequest, from peer.ID, writer ChunkedResponseWriter[testResponse]) error {
				// Write first chunk successfully
				if err := writer.WriteChunk(testResponse{Message: "chunk1", ID: 1}); err != nil {
					return err
				}
				// Simulate an error occurring
				// The writer should handle subsequent writes gracefully
				return errors.New("error after first chunk")
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
			maxRequestSize:   1024,
			expectedChunks:   2,
			expectedStatus:   []Status{StatusSuccess, StatusServerError},
			expectedMessages: []string{"chunk1"},
			expectedError:    false,
		},
		{
			name: "with_compression",
			setupStream: func() *mockStream {
				stream := newMockStream("test-stream", "/test/chunked/1.0.0", "remote", "local")
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
			handler: func(ctx context.Context, req testRequest, from peer.ID, writer ChunkedResponseWriter[testResponse]) error {
				return writer.WriteChunk(testResponse{Message: "pong", ID: req.ID})
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
			compressor:       &mockCompressor{},
			maxRequestSize:   1024,
			expectedChunks:   1,
			expectedStatus:   []Status{StatusSuccess},
			expectedMessages: []string{"COMPRESSED:pong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := tt.setupStream()
			ctx := context.Background()

			proto := testChunkedProtocol{
				testProtocol: testProtocol{
					id:              "/test/chunked/1.0.0",
					maxRequestSize:  tt.maxRequestSize,
					maxResponseSize: 2048,
				},
				chunked: true,
			}

			opts := HandlerOptions{
				Encoder:        tt.encoder,
				Compressor:     tt.compressor,
				RequestTimeout: 5 * time.Second,
			}

			h := NewChunkedHandler(proto, tt.handler, opts, logger)

			// Handle the stream
			h.HandleStream(ctx, stream)

			// Parse written data to extract chunks
			written := stream.getWrittenData()
			chunks := parseChunkedResponse(t, written)

			// Verify chunk count
			assert.Equal(t, tt.expectedChunks, len(chunks))

			// Verify each chunk
			for i, chunk := range chunks {
				if i < len(tt.expectedStatus) {
					assert.Equal(t, tt.expectedStatus[i], chunk.status)
				}
				if i < len(tt.expectedMessages) {
					assert.Equal(t, tt.expectedMessages[i], string(chunk.data))
				}
			}

			// If we expect an error status at the end
			if tt.expectedError && len(written) > 0 {
				// The last byte might be an error status if handler returned error
				lastByte := written[len(written)-1]
				if lastByte == byte(StatusServerError) {
					// This is expected for handler errors
					assert.True(t, true)
				}
			}
		})
	}
}

type parsedChunk struct {
	status Status
	data   []byte
}

func parseChunkedResponse(t *testing.T, data []byte) []parsedChunk {
	t.Helper()

	var chunks []parsedChunk
	offset := 0

	for offset < len(data) {
		// Read status byte
		if offset >= len(data) {
			break
		}
		status := Status(data[offset])
		offset++

		// If error status, no data follows
		if status != StatusSuccess {
			chunks = append(chunks, parsedChunk{status: status})

			continue
		}

		// Read size
		if offset+4 > len(data) {
			break
		}
		size := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Read data
		if offset+int(size) > len(data) {
			break
		}
		chunkData := data[offset : offset+int(size)]
		offset += int(size)

		chunks = append(chunks, parsedChunk{
			status: status,
			data:   chunkData,
		})
	}

	return chunks
}

func TestChunkedResponseWriter(t *testing.T) {
	logger := logrus.New()

	tests := []struct {
		name          string
		setupWriter   func() *streamChunkedWriter[testResponse]
		chunks        []testResponse
		expectedError string
		verifyWrite   func(t *testing.T, stream *mockStream)
	}{
		{
			name: "write_single_chunk",
			setupWriter: func() *streamChunkedWriter[testResponse] {
				stream := newMockStream("test", "/test/1.0.0", "local", "remote")

				return &streamChunkedWriter[testResponse]{
					stream: stream,
					encoder: &mockEncoder{
						encodeFunc: func(msg any) ([]byte, error) {
							if resp, ok := msg.(testResponse); ok {
								return []byte(resp.Message), nil
							}

							return nil, errors.New("unknown type")
						},
					},
					maxSize: 1024,
					log:     logger,
				}
			},
			chunks: []testResponse{
				{Message: "test chunk", ID: 1},
			},
			verifyWrite: func(t *testing.T, stream *mockStream) {
				t.Helper()
				data := stream.getWrittenData()
				chunks := parseChunkedResponse(t, data)
				require.Equal(t, 1, len(chunks))
				assert.Equal(t, StatusSuccess, chunks[0].status)
				assert.Equal(t, "test chunk", string(chunks[0].data))
			},
		},
		{
			name: "write_multiple_chunks",
			setupWriter: func() *streamChunkedWriter[testResponse] {
				stream := newMockStream("test", "/test/1.0.0", "local", "remote")

				return &streamChunkedWriter[testResponse]{
					stream: stream,
					encoder: &mockEncoder{
						encodeFunc: func(msg any) ([]byte, error) {
							if resp, ok := msg.(testResponse); ok {
								return []byte(resp.Message), nil
							}

							return nil, errors.New("unknown type")
						},
					},
					maxSize: 1024,
					log:     logger,
				}
			},
			chunks: []testResponse{
				{Message: "chunk1", ID: 1},
				{Message: "chunk2", ID: 2},
				{Message: "chunk3", ID: 3},
			},
			verifyWrite: func(t *testing.T, stream *mockStream) {
				t.Helper()
				data := stream.getWrittenData()
				chunks := parseChunkedResponse(t, data)
				require.Equal(t, 3, len(chunks))
				for i, chunk := range chunks {
					assert.Equal(t, StatusSuccess, chunk.status)
					assert.Equal(t, fmt.Sprintf("chunk%d", i+1), string(chunk.data))
				}
			},
		},
		{
			name: "chunk_exceeds_max_size",
			setupWriter: func() *streamChunkedWriter[testResponse] {
				stream := newMockStream("test", "/test/1.0.0", "local", "remote")

				return &streamChunkedWriter[testResponse]{
					stream: stream,
					encoder: &mockEncoder{
						encodeFunc: func(msg any) ([]byte, error) {
							// Return data that exceeds max size
							return make([]byte, 2048), nil
						},
					},
					maxSize: 1024,
					log:     logger,
				}
			},
			chunks: []testResponse{
				{Message: "too large", ID: 1},
			},
			expectedError: "exceeds max",
		},
		{
			name: "encoder_error",
			setupWriter: func() *streamChunkedWriter[testResponse] {
				stream := newMockStream("test", "/test/1.0.0", "local", "remote")

				return &streamChunkedWriter[testResponse]{
					stream: stream,
					encoder: &mockEncoder{
						encodeFunc: func(msg any) ([]byte, error) {
							return nil, errors.New("encode error")
						},
					},
					maxSize: 1024,
					log:     logger,
				}
			},
			chunks: []testResponse{
				{Message: "test", ID: 1},
			},
			expectedError: "encode error",
		},
		{
			name: "with_compression",
			setupWriter: func() *streamChunkedWriter[testResponse] {
				stream := newMockStream("test", "/test/1.0.0", "local", "remote")

				return &streamChunkedWriter[testResponse]{
					stream: stream,
					encoder: &mockEncoder{
						encodeFunc: func(msg any) ([]byte, error) {
							if resp, ok := msg.(testResponse); ok {
								return []byte(resp.Message), nil
							}

							return nil, errors.New("unknown type")
						},
					},
					compressor: &mockCompressor{},
					maxSize:    1024,
					log:        logger,
				}
			},
			chunks: []testResponse{
				{Message: "test chunk", ID: 1},
			},
			verifyWrite: func(t *testing.T, stream *mockStream) {
				t.Helper()
				data := stream.getWrittenData()
				chunks := parseChunkedResponse(t, data)
				require.Equal(t, 1, len(chunks))
				assert.Equal(t, StatusSuccess, chunks[0].status)
				assert.Equal(t, "COMPRESSED:test chunk", string(chunks[0].data))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := tt.setupWriter()

			var err error
			for _, chunk := range tt.chunks {
				if e := writer.WriteChunk(chunk); e != nil {
					err = e

					break
				}
			}

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				if tt.verifyWrite != nil {
					if stream, ok := writer.stream.(*mockStream); ok {
						tt.verifyWrite(t, stream)
					}
				}
			}
		})
	}
}

func TestChunkedHandler_TimeoutHandling(t *testing.T) {
	proto := testChunkedProtocol{
		testProtocol: testProtocol{
			id:              "/test/chunked/1.0.0",
			maxRequestSize:  1024,
			maxResponseSize: 2048,
		},
		chunked: true,
	}

	// Handler that takes longer than timeout
	slowHandler := func(ctx context.Context, req testRequest, from peer.ID, writer ChunkedResponseWriter[testResponse]) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return writer.WriteChunk(testResponse{Message: "too late"})
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	opts := HandlerOptions{
		Encoder: &mockEncoder{
			decodeFunc: func(data []byte, msgType any) error {
				if req, ok := msgType.(*testRequest); ok {
					req.Message = testString

					return nil
				}

				return nil
			},
		},
		RequestTimeout: 50 * time.Millisecond, // Very short timeout
	}

	logger := logrus.New()
	h := NewChunkedHandler(proto, slowHandler, opts, logger)

	// Setup stream with valid request
	stream := newMockStream("test-stream", "/test/chunked/1.0.0", "remote", "local")
	reqData := []byte(testString + " request")
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

func TestChunkedHandler_PanicRecovery(t *testing.T) {
	proto := testChunkedProtocol{
		testProtocol: testProtocol{
			id:              "/test/chunked/1.0.0",
			maxRequestSize:  1024,
			maxResponseSize: 2048,
		},
		chunked: true,
	}

	// Handler that panics
	panicHandler := func(ctx context.Context, req testRequest, from peer.ID, writer ChunkedResponseWriter[testResponse]) error {
		panic("handler panic")
	}

	opts := HandlerOptions{
		Encoder: &mockEncoder{
			decodeFunc: func(data []byte, msgType any) error {
				if req, ok := msgType.(*testRequest); ok {
					req.Message = testString

					return nil
				}

				return nil
			},
		},
		RequestTimeout: 5 * time.Second,
	}

	logger := logrus.New()
	h := NewChunkedHandler(proto, panicHandler, opts, logger)

	// Setup stream with valid request
	stream := newMockStream("test-stream", "/test/chunked/1.0.0", "remote", "local")
	reqData := []byte(testString + " request")
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
