package v1

import (
	"context"
	"testing"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/compression"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistryHandlerCompression verifies that handlers with compression
// are correctly stored and retrieved from the registry.
func TestRegistryHandlerCompression(t *testing.T) {
	// Create a new registry
	registry := NewRegistry()

	// Create topic and compressor
	topicName := "compression-test-topic"
	topic, err := NewTopic[string](topicName)
	require.NoError(t, err)

	compressor := compression.NewSnappyCompressor(1024)

	// Create handler with compressor
	handler := NewHandlerConfig[string](
		WithEncoder[string](&StringEncoder{}),
		WithCompressor[string](compressor),
		WithProcessor[string](func(ctx context.Context, msg string, from peer.ID) error {
			return nil
		}),
	)

	// Register the handler
	err = Register(registry, topic, handler)
	require.NoError(t, err)

	// Retrieve handler using GetHandler
	retrievedHandler := registry.GetHandler(topicName)
	require.NotNil(t, retrievedHandler, "Handler should be retrievable")

	// Verify compressor is present and correct
	require.NotNil(t, retrievedHandler.Compressor, "Compressor should be present")
	assert.Equal(t, uint64(1024), retrievedHandler.Compressor.MaxLength(), "Compressor should have correct max length")

	// Test compression functionality
	testData := []byte("test message for compression")
	compressed, err := retrievedHandler.Compressor.Compress(testData)
	require.NoError(t, err)
	assert.NotEqual(t, testData, compressed, "Compressed data should be different")

	// Test decompression
	decompressed, err := retrievedHandler.Compressor.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, testData, decompressed, "Decompressed data should match original")
}

// TestRegistryHandlerWithoutCompression verifies that handlers without compression
// work correctly and don't interfere with compression retrieval.
func TestRegistryHandlerWithoutCompression(t *testing.T) {
	// Create a new registry
	registry := NewRegistry()

	// Create topic without compressor
	topicName := "no-compression-topic"
	topic, err := NewTopic[string](topicName)
	require.NoError(t, err)

	// Create handler without compressor
	handler := NewHandlerConfig[string](
		WithEncoder[string](&StringEncoder{}),
		WithProcessor[string](func(ctx context.Context, msg string, from peer.ID) error {
			return nil
		}),
	)

	// Register the handler
	err = Register(registry, topic, handler)
	require.NoError(t, err)

	// Retrieve handler using GetHandler
	retrievedHandler := registry.GetHandler(topicName)
	require.NotNil(t, retrievedHandler, "Handler should be retrievable")

	// Verify compressor is nil
	assert.Nil(t, retrievedHandler.Compressor, "Compressor should be nil for handlers without compression")
}

// TestCompressorIntegration verifies that compression configuration
// is properly copied between typed and untyped handlers.
func TestCompressorIntegration(t *testing.T) {
	// Create registry
	registry := NewRegistry()

	// Test both regular topics and subnet topics
	tests := []struct {
		name      string
		topicType string
	}{
		{"regular_topic", "regular"},
		{"subnet_topic", "subnet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressor := compression.NewSnappyCompressor(2048)
			handler := NewHandlerConfig[string](
				WithEncoder[string](&StringEncoder{}),
				WithCompressor[string](compressor),
				WithProcessor[string](func(ctx context.Context, msg string, from peer.ID) error {
					return nil
				}),
			)

			var retrievedHandler *HandlerConfig[any]

			if tt.topicType == "regular" {
				topic, err := NewTopic[string]("test-regular-topic")
				require.NoError(t, err)
				err = Register(registry, topic, handler)
				require.NoError(t, err)
				retrievedHandler = registry.GetHandler("test-regular-topic")
			} else {
				subnetTopic, err := NewSubnetTopic[string]("test_subnet_%d", 8)
				require.NoError(t, err)
				err = RegisterSubnet(registry, subnetTopic, handler)
				require.NoError(t, err)
				// Get a specific subnet topic to test
				retrievedHandler = registry.GetHandler("test_subnet_0")
			}

			// Verify handler was retrieved
			require.NotNil(t, retrievedHandler, "Handler should be retrievable")

			// Verify compressor is correctly transferred
			require.NotNil(t, retrievedHandler.Compressor, "Compressor should be present")
			assert.Equal(t, uint64(2048), retrievedHandler.Compressor.MaxLength(), "Compressor max length should match")
		})
	}
}

// StringEncoder is a simple encoder for string messages used in tests.
type StringEncoder struct{}

func (e *StringEncoder) Encode(msg string) ([]byte, error) {
	return []byte(msg), nil
}

func (e *StringEncoder) Decode(data []byte) (string, error) {
	return string(data), nil
}
