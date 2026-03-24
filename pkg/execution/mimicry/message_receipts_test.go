package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/stretchr/testify/assert"
)

func TestReceipts69Code(t *testing.T) {
	msg := &Receipts69{}
	assert.Equal(t, ReceiptsCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.ReceiptsMsg, msg.Code())
}

func TestReceipts69ReqID(t *testing.T) {
	msg := &Receipts69{
		ReceiptsPacket69: eth.ReceiptsPacket69{
			RequestId: 66666,
		},
	}
	assert.Equal(t, uint64(66666), msg.ReqID())
}

func TestReceiptsInterfaceCompliance(t *testing.T) {
	var _ Receipts = (*Receipts69)(nil)
}

func TestGetReceiptsCode(t *testing.T) {
	msg := &GetReceipts{}
	assert.Equal(t, GetReceiptsCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.GetReceiptsMsg, msg.Code())
}

func TestGetReceiptsReqID(t *testing.T) {
	msg := &GetReceipts{
		RequestId: 77777,
	}
	assert.Equal(t, uint64(77777), msg.ReqID())
}

func TestReceiptsCodeConstant(t *testing.T) {
	assert.Equal(t, RLPXOffset+eth.GetReceiptsMsg, GetReceiptsCode)
	assert.Equal(t, RLPXOffset+eth.ReceiptsMsg, ReceiptsCode)
}
