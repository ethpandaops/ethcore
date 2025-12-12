package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/stretchr/testify/assert"
)

func TestTransactionsCode(t *testing.T) {
	msg := &Transactions{}
	assert.Equal(t, TransactionsCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.TransactionsMsg, msg.Code())
}

func TestTransactionsReqID(t *testing.T) {
	msg := &Transactions{}
	assert.Equal(t, uint64(0), msg.ReqID())
}

func TestTransactionsCodeConstant(t *testing.T) {
	// Transactions message code should be 0x12 (RLPXOffset + 2)
	assert.Equal(t, 0x12, TransactionsCode)
}
