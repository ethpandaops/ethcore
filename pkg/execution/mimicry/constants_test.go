package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/stretchr/testify/assert"
)

func TestRLPXOffset(t *testing.T) {
	// RLPXOffset should be 0x10 per RLPx message multiplexing spec
	// https://github.com/ethereum/devp2p/blob/master/rlpx.md#message-id-based-multiplexing
	assert.Equal(t, 0x10, RLPXOffset)
}

func TestP2PProtocolVersionConstants(t *testing.T) {
	assert.Equal(t, 5, P2PProtocolVersion)
	assert.Equal(t, 5, minP2PProtocolVersion)
}

func TestETHProtocolVersionConstants(t *testing.T) {
	assert.Equal(t, uint(68), minETHProtocolVersion)
	assert.Equal(t, uint(69), maxETHProtocolVersion)
}

func TestETHCapNameConstant(t *testing.T) {
	assert.Equal(t, "eth", ETHCapName)
}

func TestRLPxMessageCodes(t *testing.T) {
	// RLPx base protocol codes (no offset)
	assert.Equal(t, 0x00, HelloCode)
	assert.Equal(t, 0x01, DisconnectCode)
	assert.Equal(t, 0x02, PingCode)
	assert.Equal(t, 0x03, PongCode)
}

func TestETHMessageCodes(t *testing.T) {
	// ETH protocol codes should be RLPXOffset + eth message constant
	tests := []struct {
		name     string
		code     int
		expected int
	}{
		{"StatusCode", StatusCode, RLPXOffset + eth.StatusMsg},
		{"TransactionsCode", TransactionsCode, RLPXOffset + eth.TransactionsMsg},
		{"GetBlockHeadersCode", GetBlockHeadersCode, RLPXOffset + eth.GetBlockHeadersMsg},
		{"BlockHeadersCode", BlockHeadersCode, RLPXOffset + eth.BlockHeadersMsg},
		{"GetBlockBodiesCode", GetBlockBodiesCode, RLPXOffset + eth.GetBlockBodiesMsg},
		{"BlockBodiesCode", BlockBodiesCode, RLPXOffset + eth.BlockBodiesMsg},
		{"NewPooledTransactionHashesCode", NewPooledTransactionHashesCode, RLPXOffset + eth.NewPooledTransactionHashesMsg},
		{"GetPooledTransactionsCode", GetPooledTransactionsCode, RLPXOffset + eth.GetPooledTransactionsMsg},
		{"PooledTransactionsCode", PooledTransactionsCode, RLPXOffset + eth.PooledTransactionsMsg},
		{"GetReceiptsCode", GetReceiptsCode, RLPXOffset + eth.GetReceiptsMsg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code)
		})
	}
}

func TestMessageCodeValues(t *testing.T) {
	// Verify actual numeric values for important codes
	// These are per the ETH protocol specification

	// Status should be 0x10 (RLPXOffset + 0)
	assert.Equal(t, 0x10, StatusCode)

	// Transactions should be 0x12 (RLPXOffset + 2)
	assert.Equal(t, 0x12, TransactionsCode)
}
