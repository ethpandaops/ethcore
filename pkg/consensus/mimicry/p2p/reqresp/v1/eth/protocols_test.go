package eth_test

import (
	"testing"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	ethProtocols "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1/eth"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"
)

// Mock types for testing.
type Status struct {
	ForkDigest     [4]byte
	FinalizedRoot  [32]byte
	FinalizedEpoch uint64
	HeadRoot       [32]byte
	HeadSlot       uint64
}

type Goodbye struct {
	Reason uint64
}

type Ping struct {
	SeqNumber uint64
}

type Metadata struct {
	SeqNumber uint64
	Attnets   [8]byte
}

type BeaconBlocksByRangeRequest struct {
	StartSlot uint64
	Count     uint64
	Step      uint64
}

type BeaconBlock struct {
	Slot          uint64
	ProposerIndex uint64
}

type BlobSidecar struct {
	Index uint8
	Blob  [131072]byte
}

// Mock NetworkEncoder.
type mockNetworkEncoder struct{}

func (m *mockNetworkEncoder) EncodeNetwork(msg any) ([]byte, error) {
	return []byte("encoded"), nil
}

func (m *mockNetworkEncoder) DecodeNetwork(data []byte, msgType any) error {
	return nil
}

func TestEthProtocols(t *testing.T) {
	encoder := &mockNetworkEncoder{}

	t.Run("Status protocol", func(t *testing.T) {
		proto := ethProtocols.NewStatus[Status, Status](84, 84, encoder)

		require.Equal(t, protocol.ID(eth.StatusV1ProtocolID), proto.ID())
		require.Equal(t, uint64(84), proto.MaxRequestSize())
		require.Equal(t, uint64(84), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.False(t, proto.IsChunked())
	})

	t.Run("Goodbye protocol", func(t *testing.T) {
		proto := ethProtocols.NewGoodbye[Goodbye, Goodbye](8, 8, encoder)

		require.Equal(t, protocol.ID(eth.GoodbyeV1ProtocolID), proto.ID())
		require.Equal(t, uint64(8), proto.MaxRequestSize())
		require.Equal(t, uint64(8), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.False(t, proto.IsChunked())
	})

	t.Run("Ping protocol", func(t *testing.T) {
		proto := ethProtocols.NewPing[Ping, Ping](8, 8, encoder)

		require.Equal(t, protocol.ID(eth.PingV1ProtocolID), proto.ID())
		require.Equal(t, uint64(8), proto.MaxRequestSize())
		require.Equal(t, uint64(8), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.False(t, proto.IsChunked())
	})

	t.Run("MetadataV1 protocol", func(t *testing.T) {
		proto := ethProtocols.NewMetadataV1[struct{}, Metadata](0, 17, encoder)

		require.Equal(t, protocol.ID(eth.MetaDataV1ProtocolID), proto.ID())
		require.Equal(t, uint64(0), proto.MaxRequestSize())
		require.Equal(t, uint64(17), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.False(t, proto.IsChunked())
	})

	t.Run("MetadataV2 protocol", func(t *testing.T) {
		proto := ethProtocols.NewMetadataV2[struct{}, Metadata](0, 17, encoder)

		require.Equal(t, protocol.ID(eth.MetaDataV2ProtocolID), proto.ID())
		require.Equal(t, uint64(0), proto.MaxRequestSize())
		require.Equal(t, uint64(17), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.False(t, proto.IsChunked())
	})

	t.Run("BeaconBlocksByRangeV1 protocol", func(t *testing.T) {
		proto := ethProtocols.NewBeaconBlocksByRangeV1[BeaconBlocksByRangeRequest, BeaconBlock](
			24,
			1<<20, // 1MB
			encoder,
		)

		require.Equal(t, protocol.ID(eth.BeaconBlocksByRangeV1ProtocolID), proto.ID())
		require.Equal(t, uint64(24), proto.MaxRequestSize())
		require.Equal(t, uint64(1<<20), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.True(t, proto.IsChunked())
	})

	t.Run("BeaconBlocksByRangeV2 protocol", func(t *testing.T) {
		proto := ethProtocols.NewBeaconBlocksByRangeV2[BeaconBlocksByRangeRequest, BeaconBlock](
			24,
			10*(1<<20), // 10MB
			encoder,
		)

		require.Equal(t, protocol.ID(eth.BeaconBlocksByRangeV2ProtocolID), proto.ID())
		require.Equal(t, uint64(24), proto.MaxRequestSize())
		require.Equal(t, uint64(10*(1<<20)), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.True(t, proto.IsChunked())
	})

	t.Run("BeaconBlocksByRootV1 protocol", func(t *testing.T) {
		proto := ethProtocols.NewBeaconBlocksByRootV1[[][32]byte, BeaconBlock](
			1024,
			1<<20, // 1MB
			encoder,
		)

		require.Equal(t, protocol.ID(eth.BeaconBlocksByRootV1ProtocolID), proto.ID())
		require.Equal(t, uint64(1024), proto.MaxRequestSize())
		require.Equal(t, uint64(1<<20), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.True(t, proto.IsChunked())
	})

	t.Run("BeaconBlocksByRootV2 protocol", func(t *testing.T) {
		proto := ethProtocols.NewBeaconBlocksByRootV2[[][32]byte, BeaconBlock](
			1024,
			10*(1<<20), // 10MB
			encoder,
		)

		require.Equal(t, protocol.ID(eth.BeaconBlocksByRootV2ProtocolID), proto.ID())
		require.Equal(t, uint64(1024), proto.MaxRequestSize())
		require.Equal(t, uint64(10*(1<<20)), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.True(t, proto.IsChunked())
	})

	t.Run("BlobSidecarsByRangeV1 protocol", func(t *testing.T) {
		proto := ethProtocols.NewBlobSidecarsByRangeV1[BeaconBlocksByRangeRequest, BlobSidecar](
			24,
			1<<17, // 128KB
			encoder,
		)

		require.Equal(t, protocol.ID(eth.BlobSidecarsByRangeV1ProtocolID), proto.ID())
		require.Equal(t, uint64(24), proto.MaxRequestSize())
		require.Equal(t, uint64(1<<17), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.True(t, proto.IsChunked())
	})

	t.Run("BlobSidecarsByRootV1 protocol", func(t *testing.T) {
		proto := ethProtocols.NewBlobSidecarsByRootV1[[][32]byte, BlobSidecar](
			1024,
			1<<17, // 128KB
			encoder,
		)

		require.Equal(t, protocol.ID(eth.BlobSidecarsByRootV1ProtocolID), proto.ID())
		require.Equal(t, uint64(1024), proto.MaxRequestSize())
		require.Equal(t, uint64(1<<17), proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.True(t, proto.IsChunked())
	})
}

// Test that protocols are properly typed.
func TestProtocolTypes(t *testing.T) {
	encoder := &mockNetworkEncoder{}

	// These should compile without issues
	var _ = ethProtocols.NewStatus[Status, Status](84, 84, encoder)
	var _ = ethProtocols.NewBeaconBlocksByRangeV2[BeaconBlocksByRangeRequest, BeaconBlock](24, 1<<20, encoder)
}
