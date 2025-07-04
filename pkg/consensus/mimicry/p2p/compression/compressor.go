package compression

// Compressor provides compression and decompression functionality.
type Compressor interface {
	// Compress compresses the input data.
	Compress(data []byte) ([]byte, error)

	// Decompress decompresses the input data.
	Decompress(data []byte) ([]byte, error)

	// MaxLength returns the maximum allowed length for decompressed data.
	// Returns 0 if there is no limit.
	MaxLength() uint64
}
