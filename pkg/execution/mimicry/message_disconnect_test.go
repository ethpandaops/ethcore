package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisconnectCode(t *testing.T) {
	d := &Disconnect{}
	assert.Equal(t, DisconnectCode, d.Code())
	assert.Equal(t, 0x01, d.Code())
}

func TestDisconnectReqID(t *testing.T) {
	d := &Disconnect{}
	assert.Equal(t, uint64(0), d.ReqID())
}

func TestDisconnectReasons(t *testing.T) {
	tests := []struct {
		name   string
		reason p2p.DiscReason
	}{
		{"DiscRequested", p2p.DiscRequested},
		{"DiscNetworkError", p2p.DiscNetworkError},
		{"DiscProtocolError", p2p.DiscProtocolError},
		{"DiscUselessPeer", p2p.DiscUselessPeer},
		{"DiscTooManyPeers", p2p.DiscTooManyPeers},
		{"DiscAlreadyConnected", p2p.DiscAlreadyConnected},
		{"DiscIncompatibleVersion", p2p.DiscIncompatibleVersion},
		{"DiscInvalidIdentity", p2p.DiscInvalidIdentity},
		{"DiscQuitting", p2p.DiscQuitting},
		{"DiscUnexpectedIdentity", p2p.DiscUnexpectedIdentity},
		{"DiscSelf", p2p.DiscSelf},
		{"DiscReadTimeout", p2p.DiscReadTimeout},
		{"DiscSubprotocolError", p2p.DiscSubprotocolError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Disconnect{Reason: tt.reason}
			assert.Equal(t, tt.reason, d.Reason)
		})
	}
}

func TestDisconnectCodeConstant(t *testing.T) {
	// DisconnectCode should be 0x01 per RLPx spec
	assert.Equal(t, 0x01, DisconnectCode)
}

func TestDecodeDisconnectReason(t *testing.T) {
	legacy, err := rlp.EncodeToBytes(p2p.DiscTooManyPeers)
	require.NoError(t, err)

	tests := []struct {
		name string
		data []byte
		want p2p.DiscReason
	}{
		{
			name: "legacy scalar reason",
			data: legacy,
			want: p2p.DiscTooManyPeers,
		},
		{
			name: "besu list reason",
			data: []byte{0xc1, byte(p2p.DiscTooManyPeers)},
			want: p2p.DiscTooManyPeers,
		},
		{
			name: "besu unknown empty reason",
			data: []byte{0xc1, 0x80},
			want: p2p.DiscInvalid,
		},
		{
			name: "snappy compressed list reason",
			data: []byte{0x02, 0x04, 0xc1, byte(p2p.DiscTooManyPeers)},
			want: p2p.DiscTooManyPeers,
		},
		{
			name: "empty payload",
			data: nil,
			want: p2p.DiscInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeDisconnectReason(tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeDisconnectReasonHandlesSnappyEmptyPayload(t *testing.T) {
	require.NotPanics(t, func() {
		got, err := decodeDisconnectReason([]byte{0x00})
		require.NoError(t, err)
		assert.Equal(t, p2p.DiscReason(p2p.DiscInvalid), got)
	})
}
