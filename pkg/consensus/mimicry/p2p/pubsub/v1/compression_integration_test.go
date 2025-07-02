package v1_test

import (
	"context"
	"testing"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/compression"
	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressionIntegration(t *testing.T) {
	t.Run("handler with compression", func(t *testing.T) {

		// Create topic with SSZ encoder
		_, err := v1.NewTopic[*SSZTestMessage]("test-topic")
		require.NoError(t, err)

		// Create SSZ encoder for testing
		encoder := v1.NewSSZEncoder[*SSZTestMessage]()

		// Create handler config with compression
		config := v1.NewHandlerConfig[*SSZTestMessage](
			v1.WithEncoder[*SSZTestMessage](encoder),
			v1.WithCompressor[*SSZTestMessage](compression.NewSnappyCompressor(1024*1024)),
			v1.WithValidator[*SSZTestMessage](func(ctx context.Context, msg *SSZTestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
			v1.WithProcessor[*SSZTestMessage](func(ctx context.Context, msg *SSZTestMessage, from peer.ID) error {
				// Process message
				return nil
			}),
		)

		// Test message
		testMsg := &SSZTestMessage{
			Value: 123,
			Data:  []byte("test message data"),
		}

		// Encode message
		encoded, err := encoder.Encode(testMsg)
		require.NoError(t, err)

		// Compress message (simulating what would happen during publish)
		compressed, err := config.Compressor.Compress(encoded)
		require.NoError(t, err)

		// Verify compression actually happened
		assert.NotEqual(t, encoded, compressed)
		assert.Less(t, len(compressed), len(encoded))

		// Simulate receiving compressed message
		// First decompress
		decompressed, err := config.Compressor.Decompress(compressed)
		require.NoError(t, err)
		assert.Equal(t, encoded, decompressed)

		// Then decode
		decoded, err := encoder.Decode(decompressed)
		require.NoError(t, err)
		assert.Equal(t, testMsg.Value, decoded.Value)
		assert.Equal(t, testMsg.Data, decoded.Data)
	})

	t.Run("handler without compression", func(t *testing.T) {

		// Create topic with SSZ encoder
		topic, err := v1.NewTopic[*SSZTestMessage]("test-topic-no-compression")
		require.NoError(t, err)

		// Create SSZ encoder for testing
		encoder := v1.NewSSZEncoder[*SSZTestMessage]()

		// Create handler config WITHOUT compression
		config := v1.NewHandlerConfig[*SSZTestMessage](
			v1.WithEncoder[*SSZTestMessage](encoder),
			v1.WithValidator[*SSZTestMessage](func(ctx context.Context, msg *SSZTestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
		)

		// Verify no compressor is set
		assert.Nil(t, config.Compressor)

		// Test message
		testMsg := &SSZTestMessage{
			Value: 456,
			Data:  []byte("uncompressed test message"),
		}

		// Encode message
		encoded, err := encoder.Encode(testMsg)
		require.NoError(t, err)

		// Without compression, encoded data is used directly
		decoded, err := encoder.Decode(encoded)
		require.NoError(t, err)
		assert.Equal(t, testMsg.Value, decoded.Value)
		assert.Equal(t, testMsg.Data, decoded.Data)

		_ = topic // Suppress unused variable warning
	})

	t.Run("compression with size limit", func(t *testing.T) {
		maxLen := uint64(50)
		compressor := compression.NewSnappyCompressor(maxLen)

		// Create large message that exceeds limit when decompressed
		largeMsg := &SSZTestMessage{
			Value: 999,
			Data:  make([]byte, 100), // This will exceed the 50 byte limit
		}

		encoder := v1.NewSSZEncoder[*SSZTestMessage]()

		// Encode and compress
		encoded, err := encoder.Encode(largeMsg)
		require.NoError(t, err)

		compressed, err := compressor.Compress(encoded)
		require.NoError(t, err)

		// Decompression should fail due to size limit
		_, err = compressor.Decompress(compressed)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds max length")
	})
}

func TestCompressionWithRegistry(t *testing.T) {
	registry := v1.NewRegistry()

	t.Run("register handler with compression", func(t *testing.T) {
		topic, err := v1.NewTopic[*SSZTestMessage]("compressed-topic")
		require.NoError(t, err)

		compressor := compression.NewSnappyCompressor(10 * 1024 * 1024)

		config := v1.NewHandlerConfig[*SSZTestMessage](
			v1.WithEncoder[*SSZTestMessage](v1.NewSSZEncoder[*SSZTestMessage]()),
			v1.WithCompressor[*SSZTestMessage](compressor),
			v1.WithValidator[*SSZTestMessage](func(ctx context.Context, msg *SSZTestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
		)

		err = v1.Register(registry, topic, config)
		require.NoError(t, err)

		// Verify handler is registered with compression
		assert.True(t, registry.HasHandler(topic.Name()))

		handler := registry.GetHandler(topic.Name())
		require.NotNil(t, handler)
		assert.NotNil(t, handler.Compressor)
		assert.Equal(t, uint64(10*1024*1024), handler.Compressor.MaxLength())
	})

	t.Run("multiple topics with different compression settings", func(t *testing.T) {
		// Topic 1: With compression
		encoder1 := v1.NewSSZEncoder[*SSZTestMessage]()
		topic1, err := v1.NewTopic[*SSZTestMessage]("topic-with-compression")
		require.NoError(t, err)

		config1 := v1.NewHandlerConfig[*SSZTestMessage](
			v1.WithEncoder[*SSZTestMessage](encoder1),
			v1.WithCompressor[*SSZTestMessage](compression.NewSnappyCompressor(1024*1024)),
			v1.WithValidator[*SSZTestMessage](func(ctx context.Context, msg *SSZTestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
		)

		err = v1.Register(registry, topic1, config1)
		require.NoError(t, err)

		// Topic 2: Without compression
		encoder2 := v1.NewSSZEncoder[*SSZTestMessage]()
		topic2, err := v1.NewTopic[*SSZTestMessage]("topic-without-compression")
		require.NoError(t, err)

		config2 := v1.NewHandlerConfig[*SSZTestMessage](
			v1.WithEncoder[*SSZTestMessage](encoder2),
			v1.WithValidator[*SSZTestMessage](func(ctx context.Context, msg *SSZTestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
		)

		err = v1.Register(registry, topic2, config2)
		require.NoError(t, err)

		// Verify different compression settings
		handler1 := registry.GetHandler(topic1.Name())
		require.NotNil(t, handler1)
		assert.NotNil(t, handler1.Compressor)

		handler2 := registry.GetHandler(topic2.Name())
		require.NotNil(t, handler2)
		assert.Nil(t, handler2.Compressor)
	})
}
