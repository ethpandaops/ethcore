package v1_test

import (
	"testing"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"
)

func TestSimpleProtocol(t *testing.T) {
	encoder := &mockNetworkEncoder{}
	protocolID := protocol.ID("/test/1")
	maxReq := uint64(100)
	maxResp := uint64(200)

	t.Run("non-chunked protocol", func(t *testing.T) {
		proto := v1.NewProtocol(protocolID, maxReq, maxResp, encoder)

		require.Equal(t, protocolID, proto.ID())
		require.Equal(t, maxReq, proto.MaxRequestSize())
		require.Equal(t, maxResp, proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.False(t, proto.IsChunked())
	})

	t.Run("chunked protocol", func(t *testing.T) {
		proto := v1.NewChunkedProtocol(protocolID, maxReq, maxResp, encoder)

		require.Equal(t, protocolID, proto.ID())
		require.Equal(t, maxReq, proto.MaxRequestSize())
		require.Equal(t, maxResp, proto.MaxResponseSize())
		require.Equal(t, encoder, proto.NetworkEncoder())
		require.True(t, proto.IsChunked())
	})
}

func TestProtocolInterface(t *testing.T) {
	// Ensure SimpleProtocol implements Protocol interface
	var _ v1.Protocol[testRequest, testResponse] = &v1.SimpleProtocol{}

	// Test that we can use it in generic functions
	proto := v1.NewProtocol(
		protocol.ID("/test/1"),
		100,
		200,
		&mockNetworkEncoder{},
	)

	// This should compile and work with the Protocol interface
	testGenericFunction[testRequest, testResponse](t, proto)
}

func testGenericFunction[TReq, TResp any](t *testing.T, proto v1.Protocol[TReq, TResp]) {
	t.Helper()

	require.NotNil(t, proto.ID())
	require.Greater(t, proto.MaxRequestSize(), uint64(0))
	require.Greater(t, proto.MaxResponseSize(), uint64(0))
	require.NotNil(t, proto.NetworkEncoder())
}
