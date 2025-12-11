package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/stretchr/testify/assert"
)

func TestGetBlockBodiesCode(t *testing.T) {
	msg := &GetBlockBodies{}
	assert.Equal(t, GetBlockBodiesCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.GetBlockBodiesMsg, msg.Code())
}

func TestGetBlockBodiesReqID(t *testing.T) {
	msg := &GetBlockBodies{
		RequestId: 33333,
	}
	assert.Equal(t, uint64(33333), msg.ReqID())
}

func TestGetBlockBodiesReqIDZero(t *testing.T) {
	msg := &GetBlockBodies{}
	assert.Equal(t, uint64(0), msg.ReqID())
}

func TestBlockBodiesCode(t *testing.T) {
	msg := &BlockBodies{}
	assert.Equal(t, BlockBodiesCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.BlockBodiesMsg, msg.Code())
}

func TestBlockBodiesReqID(t *testing.T) {
	msg := &BlockBodies{
		RequestId: 44444,
	}
	assert.Equal(t, uint64(44444), msg.ReqID())
}

func TestBlockBodiesCodeConstants(t *testing.T) {
	// GetBlockBodies should be 0x15 (RLPXOffset + 5)
	assert.Equal(t, RLPXOffset+eth.GetBlockBodiesMsg, GetBlockBodiesCode)
	// BlockBodies should be 0x16 (RLPXOffset + 6)
	assert.Equal(t, RLPXOffset+eth.BlockBodiesMsg, BlockBodiesCode)
}
