package v1

import (
	"fmt"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/compression"
	fastssz "github.com/prysmaticlabs/fastssz"
)

// SSZSnappyEncoder implements NetworkEncoder using SSZ marshaling and Snappy compression.
type SSZSnappyEncoder struct {
	maxDecompressedSize uint64
}

// NewSSZSnappyEncoder creates a new SSZ+Snappy encoder.
func NewSSZSnappyEncoder(maxDecompressedSize uint64) *SSZSnappyEncoder {
	return &SSZSnappyEncoder{
		maxDecompressedSize: maxDecompressedSize,
	}
}

// EncodeNetwork performs SSZ marshaling followed by Snappy compression.
func (e *SSZSnappyEncoder) EncodeNetwork(msg any) ([]byte, error) {
	// First, SSZ marshal
	marshaler, ok := msg.(fastssz.Marshaler)
	if !ok {
		return nil, fmt.Errorf("type %T does not implement fastssz.Marshaler", msg)
	}

	sszData, err := marshaler.MarshalSSZ()
	if err != nil {
		return nil, fmt.Errorf("failed to SSZ marshal: %w", err)
	}

	// Then, Snappy compress
	compressor := compression.NewSnappyCompressor(0) // No limit for compression

	return compressor.Compress(sszData)
}

// DecodeNetwork performs Snappy decompression followed by SSZ unmarshaling.
func (e *SSZSnappyEncoder) DecodeNetwork(data []byte, msgType any) error {
	// First, Snappy decompress
	compressor := compression.NewSnappyCompressor(e.maxDecompressedSize)

	decompressed, err := compressor.Decompress(data)
	if err != nil {
		return fmt.Errorf("failed to decompress: %w", err)
	}

	// Then, SSZ unmarshal
	unmarshaler, ok := msgType.(fastssz.Unmarshaler)
	if !ok {
		return fmt.Errorf("type %T does not implement fastssz.Unmarshaler", msgType)
	}

	return unmarshaler.UnmarshalSSZ(decompressed)
}
