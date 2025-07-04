package compression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnappyCompressor(t *testing.T) {
	t.Run("compress and decompress", func(t *testing.T) {
		compressor := NewSnappyCompressor(0)

		testData := []byte("Hello, this is a test message that will be compressed")

		// Compress
		compressed, err := compressor.Compress(testData)
		require.NoError(t, err)
		assert.NotEqual(t, testData, compressed)
		// Snappy compression may not always result in smaller data for small inputs

		// Decompress
		decompressed, err := compressor.Decompress(compressed)
		require.NoError(t, err)
		assert.Equal(t, testData, decompressed)
	})

	t.Run("max length validation", func(t *testing.T) {
		maxLen := uint64(50)
		compressor := NewSnappyCompressor(maxLen)

		// Create data that will exceed max length when decompressed
		largeData := make([]byte, 100)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		// Compress should succeed
		compressed, err := compressor.Compress(largeData)
		require.NoError(t, err)

		// Decompress should fail due to max length
		_, err = compressor.Decompress(compressed)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds max length")
	})

	t.Run("invalid compressed data", func(t *testing.T) {
		compressor := NewSnappyCompressor(0)

		// Try to decompress invalid data
		invalidData := []byte{0xFF, 0xFF, 0xFF, 0xFF}
		_, err := compressor.Decompress(invalidData)
		assert.Error(t, err)
	})

	t.Run("empty data", func(t *testing.T) {
		compressor := NewSnappyCompressor(0)

		// Compress empty data
		compressed, err := compressor.Compress([]byte{})
		require.NoError(t, err)

		// Decompress
		decompressed, err := compressor.Decompress(compressed)
		require.NoError(t, err)
		assert.Empty(t, decompressed)
	})

	t.Run("max length getter", func(t *testing.T) {
		maxLen := uint64(1024 * 1024)
		compressor := NewSnappyCompressor(maxLen)
		assert.Equal(t, maxLen, compressor.MaxLength())

		// Zero max length
		compressor2 := NewSnappyCompressor(0)
		assert.Equal(t, uint64(0), compressor2.MaxLength())
	})
}
