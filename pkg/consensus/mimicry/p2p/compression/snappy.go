package compression

import (
	"github.com/golang/snappy"
	"github.com/pkg/errors"
)

// SnappyCompressor implements Compressor using Snappy compression.
type SnappyCompressor struct {
	maxLength uint64
}

// NewSnappyCompressor creates a new SnappyCompressor with the specified max length.
func NewSnappyCompressor(maxLength uint64) *SnappyCompressor {
	return &SnappyCompressor{maxLength: maxLength}
}

// Compress compresses the input data using Snappy.
func (s *SnappyCompressor) Compress(data []byte) ([]byte, error) {
	return snappy.Encode(nil, data), nil
}

// Decompress decompresses the input data using Snappy.
func (s *SnappyCompressor) Decompress(data []byte) ([]byte, error) {
	decodedLen, err := snappy.DecodedLen(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get decoded length")
	}

	if s.maxLength > 0 && uint64(decodedLen) > s.maxLength {
		return nil, errors.Errorf("decompressed data exceeds max length: %d > %d", decodedLen, s.maxLength)
	}

	return snappy.Decode(nil, data)
}

// MaxLength returns the maximum allowed length for decompressed data.
func (s *SnappyCompressor) MaxLength() uint64 {
	return s.maxLength
}

// Ensure SnappyCompressor implements Compressor.
var _ Compressor = (*SnappyCompressor)(nil)
