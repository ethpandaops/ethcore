package topics_test

import (
	"testing"

	"github.com/OffchainLabs/go-bitfield"
	eth "github.com/OffchainLabs/prysm/v7/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/topics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSZSnappyEncoder(t *testing.T) {
	encoder := topics.NewSSZSnappyEncoder[*eth.Attestation]()

	// Create a test attestation
	// Use bitlist format: the last byte contains the length marker
	aggregationBits := bitfield.Bitlist{0xFF, 0x01} // 8 bits set, length marker

	attestation := &eth.Attestation{
		AggregationBits: aggregationBits,
		Data: &eth.AttestationData{
			Slot:            100,
			CommitteeIndex:  1,
			BeaconBlockRoot: make([]byte, 32),
			Source: &eth.Checkpoint{
				Epoch: 10,
				Root:  make([]byte, 32),
			},
			Target: &eth.Checkpoint{
				Epoch: 11,
				Root:  make([]byte, 32),
			},
		},
		Signature: make([]byte, 96),
	}

	t.Run("encode and decode", func(t *testing.T) {
		// Encode
		encoded, err := encoder.Encode(attestation)
		require.NoError(t, err)
		assert.NotEmpty(t, encoded)

		// Decode
		decoded, err := encoder.Decode(encoded)
		require.NoError(t, err)
		assert.Equal(t, attestation.Data.Slot, decoded.Data.Slot)
		assert.Equal(t, attestation.Data.CommitteeIndex, decoded.Data.CommitteeIndex)
		assert.Equal(t, attestation.AggregationBits, decoded.AggregationBits)
	})

	t.Run("decode invalid data", func(t *testing.T) {
		// Try to decode invalid SSZ data
		_, err := encoder.Decode([]byte{0xFF, 0xFE, 0xFD})
		assert.Error(t, err)
		// The encoder no longer handles compression, so error is from SSZ
		assert.Contains(t, err.Error(), "SSZ")
	})
}

func TestSSZEncoderCompatibility(t *testing.T) {
	// Note: NewSSZSnappyEncoderWithMaxLen now returns an SSZ-only encoder
	// Max length enforcement should be done via compressor in handler configuration

	t.Run("create encoders", func(t *testing.T) {
		// Create encoder without max length
		encoder1 := topics.NewSSZSnappyEncoder[*eth.Attestation]()
		assert.NotNil(t, encoder1)

		// Create encoder with max length (parameter is ignored)
		encoder2 := topics.NewSSZSnappyEncoderWithMaxLen[*eth.Attestation](10 * 1024 * 1024)
		assert.NotNil(t, encoder2)

		// Both should work the same way
		attestation := &eth.Attestation{
			AggregationBits: bitfield.Bitlist{0xFF, 0x01},
			Data: &eth.AttestationData{
				Slot:            100,
				CommitteeIndex:  1,
				BeaconBlockRoot: make([]byte, 32),
				Source: &eth.Checkpoint{
					Epoch: 10,
					Root:  make([]byte, 32),
				},
				Target: &eth.Checkpoint{
					Epoch: 11,
					Root:  make([]byte, 32),
				},
			},
			Signature: make([]byte, 96),
		}

		encoded1, err := encoder1.Encode(attestation)
		require.NoError(t, err)

		encoded2, err := encoder2.Encode(attestation)
		require.NoError(t, err)

		// Should produce the same output
		assert.Equal(t, encoded1, encoded2)
	})
}

func TestSnappyCompressor(t *testing.T) {
	// Test the new Snappy compressor that should be used with handler configuration
	maxLen := uint64(1000)
	compressor := topics.NewSnappyCompressor(maxLen)

	t.Run("compress and decompress", func(t *testing.T) {
		data := []byte("test data for compression")

		// Compress
		compressed, err := compressor.Compress(data)
		require.NoError(t, err)
		assert.NotEmpty(t, compressed)
		assert.NotEqual(t, data, compressed) // Should be different after compression

		// Decompress
		decompressed, err := compressor.Decompress(compressed)
		require.NoError(t, err)
		assert.Equal(t, data, decompressed)
	})

	t.Run("max length check", func(t *testing.T) {
		// The max length is enforced during decompression
		assert.Equal(t, maxLen, compressor.MaxLength())
	})
}
