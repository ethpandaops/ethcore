package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockRangeUpdateCode(t *testing.T) {
	msg := &BlockRangeUpdate{}
	assert.Equal(t, BlockRangeUpdateCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.BlockRangeUpdateMsg, msg.Code())
}

func TestBlockRangeUpdateCodeConstant(t *testing.T) {
	// BlockRangeUpdate code should be RLPXOffset + BlockRangeUpdateMsg
	assert.Equal(t, RLPXOffset+eth.BlockRangeUpdateMsg, BlockRangeUpdateCode)
}

func TestBlockRangeUpdateWithData(t *testing.T) {
	hash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	msg := BlockRangeUpdate{
		EarliestBlock:   100,
		LatestBlock:     500,
		LatestBlockHash: hash,
	}

	assert.Equal(t, uint64(100), msg.EarliestBlock)
	assert.Equal(t, uint64(500), msg.LatestBlock)
	assert.Equal(t, hash, msg.LatestBlockHash)
}

func TestBlockRangeUpdateRLPEncoding(t *testing.T) {
	hash := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")

	original := eth.BlockRangeUpdatePacket{
		EarliestBlock:   1000,
		LatestBlock:     5000,
		LatestBlockHash: hash,
	}

	// Encode
	encoded, err := rlp.EncodeToBytes(&original)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// Decode
	var decoded eth.BlockRangeUpdatePacket
	err = rlp.DecodeBytes(encoded, &decoded)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, original.EarliestBlock, decoded.EarliestBlock)
	assert.Equal(t, original.LatestBlock, decoded.LatestBlock)
	assert.Equal(t, original.LatestBlockHash, decoded.LatestBlockHash)
}
