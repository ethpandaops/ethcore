package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPooledTransactionHashesCode(t *testing.T) {
	msg := &NewPooledTransactionHashes{}
	assert.Equal(t, NewPooledTransactionHashesCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.NewPooledTransactionHashesMsg, msg.Code())
}

func TestNewPooledTransactionHashesReqID(t *testing.T) {
	msg := &NewPooledTransactionHashes{}
	// NewPooledTransactionHashes doesn't have a request ID, always returns 0
	assert.Equal(t, uint64(0), msg.ReqID())
}

func TestNewPooledTransactionHashesCodeConstant(t *testing.T) {
	// NewPooledTransactionHashes should be 0x18 (RLPXOffset + 8)
	assert.Equal(t, RLPXOffset+eth.NewPooledTransactionHashesMsg, NewPooledTransactionHashesCode)
}

func TestNewPooledTransactionHashesRLPEncoding(t *testing.T) {
	hash1 := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	hash2 := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")

	original := eth.NewPooledTransactionHashesPacket{
		Types:  []byte{0x02, 0x02},
		Sizes:  []uint32{100, 200},
		Hashes: []common.Hash{hash1, hash2},
	}

	// Encode
	encoded, err := rlp.EncodeToBytes(&original)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// Decode
	var decoded eth.NewPooledTransactionHashesPacket
	err = rlp.DecodeBytes(encoded, &decoded)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, original.Types, decoded.Types)
	assert.Equal(t, original.Sizes, decoded.Sizes)
	assert.Equal(t, original.Hashes, decoded.Hashes)
}

func TestNewPooledTransactionHashesWithData(t *testing.T) {
	hash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	msg := NewPooledTransactionHashes{
		Types:  []byte{0x02},
		Sizes:  []uint32{150},
		Hashes: []common.Hash{hash},
	}

	assert.Len(t, msg.Hashes, 1)
	assert.Equal(t, hash, msg.Hashes[0])
	assert.Equal(t, byte(0x02), msg.Types[0])
	assert.Equal(t, uint32(150), msg.Sizes[0])
}
