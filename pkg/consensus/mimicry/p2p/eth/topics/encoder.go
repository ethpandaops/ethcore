package topics

import (
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/compression"
	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
)

// NewSSZSnappyEncoder creates a new SSZ encoder for compatibility.
// Note: This is for backward compatibility. In the new architecture,
// compression is configured separately on the handler.
func NewSSZSnappyEncoder[T any]() v1.Encoder[T] {
	return v1.NewSSZEncoder[T]()
}

// NewSSZSnappyEncoderWithMaxLen creates a new SSZ encoder with max length for compatibility.
// Note: This is for backward compatibility. In the new architecture,
// compression with max length is configured separately on the handler.
func NewSSZSnappyEncoderWithMaxLen[T any](maxLen uint64) v1.Encoder[T] {
	// Just return SSZ encoder, max length should be enforced via compressor
	return v1.NewSSZEncoder[T]()
}

// NewSnappyCompressor creates a new Snappy compressor with the given max length.
// This should be used with WithCompressor option on handler configuration.
func NewSnappyCompressor(maxLen uint64) compression.Compressor {
	return compression.NewSnappyCompressor(maxLen)
}
