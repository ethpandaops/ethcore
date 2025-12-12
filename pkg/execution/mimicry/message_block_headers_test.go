package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/stretchr/testify/assert"
)

func TestGetBlockHeadersCode(t *testing.T) {
	msg := &GetBlockHeaders{}
	assert.Equal(t, GetBlockHeadersCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.GetBlockHeadersMsg, msg.Code())
}

func TestGetBlockHeadersReqID(t *testing.T) {
	msg := &GetBlockHeaders{
		RequestId: 11111,
	}
	assert.Equal(t, uint64(11111), msg.ReqID())
}

func TestGetBlockHeadersReqIDZero(t *testing.T) {
	msg := &GetBlockHeaders{}
	assert.Equal(t, uint64(0), msg.ReqID())
}

func TestBlockHeadersCode(t *testing.T) {
	msg := &BlockHeaders{}
	assert.Equal(t, BlockHeadersCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.BlockHeadersMsg, msg.Code())
}

func TestBlockHeadersReqID(t *testing.T) {
	msg := &BlockHeaders{
		RequestId: 22222,
	}
	assert.Equal(t, uint64(22222), msg.ReqID())
}

func TestBlockHeadersCodeConstants(t *testing.T) {
	// GetBlockHeaders should be 0x13 (RLPXOffset + 3)
	assert.Equal(t, RLPXOffset+eth.GetBlockHeadersMsg, GetBlockHeadersCode)
	// BlockHeaders should be 0x14 (RLPXOffset + 4)
	assert.Equal(t, RLPXOffset+eth.BlockHeadersMsg, BlockHeadersCode)
}
