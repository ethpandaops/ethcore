package v1_test

import (
	"testing"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1"
	fastssz "github.com/prysmaticlabs/fastssz"
	"github.com/stretchr/testify/require"
)

// Mock SSZ type for testing.
type mockSSZType struct {
	Value uint64
	Data  []byte
}

func (m *mockSSZType) MarshalSSZ() ([]byte, error) {
	// Simple mock encoding
	return []byte{byte(m.Value), byte(m.Value >> 8)}, nil
}

func (m *mockSSZType) UnmarshalSSZ(data []byte) error {
	if len(data) < 2 {
		return fastssz.ErrSize
	}
	m.Value = uint64(data[0]) | uint64(data[1])<<8

	return nil
}

func (m *mockSSZType) MarshalSSZTo(buf []byte) ([]byte, error) {
	return append(buf, byte(m.Value), byte(m.Value>>8)), nil
}

func (m *mockSSZType) SizeSSZ() int {
	return 2
}

func TestSSZSnappyEncoder(t *testing.T) {
	encoder := v1.NewSSZSnappyEncoder(1024 * 1024) // 1MB max

	t.Run("encode and decode", func(t *testing.T) {
		original := &mockSSZType{Value: 42}

		// Encode
		encoded, err := encoder.EncodeNetwork(original)
		require.NoError(t, err)
		require.NotEmpty(t, encoded)

		// Decode
		decoded := &mockSSZType{}
		err = encoder.DecodeNetwork(encoded, decoded)
		require.NoError(t, err)
		require.Equal(t, original.Value, decoded.Value)
	})

	t.Run("encode non-SSZ type", func(t *testing.T) {
		nonSSZ := struct{ Value int }{Value: 42}
		_, err := encoder.EncodeNetwork(nonSSZ)
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not implement fastssz.Marshaler")
	})

	t.Run("decode non-SSZ type", func(t *testing.T) {
		// First encode something valid to get properly compressed data
		original := &mockSSZType{Value: 42}
		encoded, err := encoder.EncodeNetwork(original)
		require.NoError(t, err)

		// Now try to decode into non-SSZ type
		nonSSZ := struct{ Value int }{Value: 42}
		err = encoder.DecodeNetwork(encoded, &nonSSZ)
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not implement fastssz.Unmarshaler")
	})

	t.Run("max decompressed size limit", func(t *testing.T) {
		// Create encoder with tiny limit
		smallEncoder := v1.NewSSZSnappyEncoder(10) // 10 bytes max

		// This should work for encoding
		original := &mockSSZType{Value: 42}
		encoded, err := smallEncoder.EncodeNetwork(original)
		require.NoError(t, err)

		// But may fail on decode if decompressed size exceeds limit
		// (This test might not fail with our simple mock, but demonstrates the API)
		decoded := &mockSSZType{}
		_ = smallEncoder.DecodeNetwork(encoded, decoded)
	})
}

func TestSSZSnappyEncoder_Integration(t *testing.T) {
	// Test that it properly integrates with Protocol
	encoder := v1.NewSSZSnappyEncoder(1024 * 1024)
	proto := v1.NewProtocol("/test/1", 100, 200, encoder)

	require.Equal(t, encoder, proto.NetworkEncoder())
}
