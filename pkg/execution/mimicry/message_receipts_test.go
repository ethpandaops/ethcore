package mimicry

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestReceipts70ReqID(t *testing.T) {
	msg := &Receipts70{
		Receipts70Packet: Receipts70Packet{
			RequestId:           66666,
			LastBlockIncomplete: false,
		},
	}
	assert.Equal(t, uint64(66666), msg.ReqID())
}

func TestReceiptsInterfaceCompliance(t *testing.T) {
	var _ Receipts = (*Receipts69)(nil)
	var _ Receipts = (*Receipts70)(nil)
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

func TestReceiveGetReceipts70(t *testing.T) {
	hash := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	packet := GetReceipts70Packet{
		RequestId:              77777,
		FirstBlockReceiptIndex: 5,
		GetReceiptsRequest:     eth.GetReceiptsRequest{hash},
	}
	encoded, err := rlp.EncodeToBytes(&packet)
	require.NoError(t, err)

	client := &Client{ethCapVersion: 70}
	msg, err := client.receiveGetReceipts(context.Background(), encoded)
	require.NoError(t, err)

	assert.Equal(t, uint64(77777), msg.ReqID())
	assert.Equal(t, ReceiptRequest{
		Hashes:                 []common.Hash{hash},
		FirstBlockReceiptIndex: 5,
	}, msg.ReceiptRequest())
}

func TestReceiptsCodeConstant(t *testing.T) {
	assert.Equal(t, RLPXOffset+eth.GetReceiptsMsg, GetReceiptsCode)
	assert.Equal(t, RLPXOffset+eth.ReceiptsMsg, ReceiptsCode)
}
