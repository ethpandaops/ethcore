package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/stretchr/testify/assert"
)

func TestReceipts68Code(t *testing.T) {
	msg := &Receipts68{}
	assert.Equal(t, ReceiptsCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.ReceiptsMsg, msg.Code())
}

func TestReceipts68ReqID(t *testing.T) {
	msg := &Receipts68{
		ReceiptsPacket: eth.ReceiptsPacket[*eth.ReceiptList68]{
			RequestId: 55555,
		},
	}
	assert.Equal(t, uint64(55555), msg.ReqID())
}

func TestReceipts69Code(t *testing.T) {
	msg := &Receipts69{}
	assert.Equal(t, ReceiptsCode, msg.Code())
	assert.Equal(t, RLPXOffset+eth.ReceiptsMsg, msg.Code())
}

func TestReceipts69ReqID(t *testing.T) {
	msg := &Receipts69{
		ReceiptsPacket: eth.ReceiptsPacket[*eth.ReceiptList69]{
			RequestId: 66666,
		},
	}
	assert.Equal(t, uint64(66666), msg.ReqID())
}

func TestReceiptsInterfaceCompliance(t *testing.T) {
	// Ensure both Receipts68 and Receipts69 implement the Receipts interface
	var _ Receipts = (*Receipts68)(nil)
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
	// GetReceipts should be 0x1d
	assert.Equal(t, RLPXOffset+eth.GetReceiptsMsg, GetReceiptsCode)
	// Receipts should be 0x1e
	assert.Equal(t, RLPXOffset+eth.ReceiptsMsg, ReceiptsCode)
}
