package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/stretchr/testify/assert"
)

func TestReceiptsCode(t *testing.T) {
	msg := &ReceiptsData{}
	assert.Equal(t, ReceiptsCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.ReceiptsMsg, msg.Code())
}

func TestReceiptsReqID(t *testing.T) {
	msg := &ReceiptsData{
		ReceiptsPacket: eth.ReceiptsPacket{
			RequestId: 55555,
		},
	}
	assert.Equal(t, uint64(55555), msg.ReqID())
}

func TestReceiptsInterfaceCompliance(t *testing.T) {
	var _ Receipts = (*ReceiptsData)(nil)
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
	// GetReceipts should be 0x1d
	assert.Equal(t, RLPXOffset+eth.GetReceiptsMsg, GetReceiptsCode)
	// Receipts should be 0x1e
	assert.Equal(t, RLPXOffset+eth.ReceiptsMsg, ReceiptsCode)
}
