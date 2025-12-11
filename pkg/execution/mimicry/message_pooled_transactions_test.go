package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/stretchr/testify/assert"
)

func TestGetPooledTransactionsCode(t *testing.T) {
	msg := &GetPooledTransactions{}
	assert.Equal(t, GetPooledTransactionsCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.GetPooledTransactionsMsg, msg.Code())
}

func TestGetPooledTransactionsReqID(t *testing.T) {
	msg := &GetPooledTransactions{
		RequestId: 12345,
	}
	assert.Equal(t, uint64(12345), msg.ReqID())
}

func TestGetPooledTransactionsReqIDZero(t *testing.T) {
	msg := &GetPooledTransactions{}
	assert.Equal(t, uint64(0), msg.ReqID())
}

func TestPooledTransactionsCode(t *testing.T) {
	msg := &PooledTransactions{}
	assert.Equal(t, PooledTransactionsCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.PooledTransactionsMsg, msg.Code())
}

func TestPooledTransactionsReqID(t *testing.T) {
	msg := &PooledTransactions{
		RequestId: 67890,
	}
	assert.Equal(t, uint64(67890), msg.ReqID())
}

func TestPooledTransactionsCodeConstant(t *testing.T) {
	// GetPooledTransactions should be 0x19 (RLPXOffset + 9)
	assert.Equal(t, RLPXOffset+eth.GetPooledTransactionsMsg, GetPooledTransactionsCode)
	// PooledTransactions should be 0x1a (RLPXOffset + 10)
	assert.Equal(t, RLPXOffset+eth.PooledTransactionsMsg, PooledTransactionsCode)
}
