package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/stretchr/testify/assert"
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
