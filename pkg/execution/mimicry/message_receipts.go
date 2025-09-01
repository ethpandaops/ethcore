// eth protocol block receipts https://github.com/ethereum/devp2p/blob/master/caps/eth.md#blockreceipts-0x06
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	ReceiptsCode = RLPXOffset + eth.ReceiptsMsg
)

// Receipts is a wrapper interface for both ReceiptsPacket68 and ReceiptsPacket69.
type Receipts interface {
	Code() int
	ReqID() uint64
}

type Receipts68 struct {
	eth.ReceiptsPacket[*eth.ReceiptList68]
}

type Receipts69 struct {
	eth.ReceiptsPacket[*eth.ReceiptList69]
}

func (msg *Receipts68) Code() int { return ReceiptsCode }

func (msg *Receipts68) ReqID() uint64 { return msg.RequestId }

func (msg *Receipts69) Code() int { return ReceiptsCode }

func (msg *Receipts69) ReqID() uint64 { return msg.RequestId }

func (c *Client) sendReceipts(ctx context.Context, r Receipts) error {
	var requestID uint64

	var listCount int

	var encodedData []byte

	var err error

	switch receipts := r.(type) {
	case *Receipts68:
		requestID = receipts.RequestId
		listCount = len(receipts.List)
		encodedData, err = rlp.EncodeToBytes(&receipts.ReceiptsPacket)
	case *Receipts69:
		requestID = receipts.RequestId
		listCount = len(receipts.List)
		encodedData, err = rlp.EncodeToBytes(&receipts.ReceiptsPacket)
	default:
		return fmt.Errorf("unsupported receipts type: %T", r)
	}

	c.log.WithFields(logrus.Fields{
		"code":           ReceiptsCode,
		"request_id":     requestID,
		"receipts_count": listCount,
		"ethCapVersion":  c.ethCapVersion,
	}).Debug("sending Receipts")

	if err != nil {
		return fmt.Errorf("error encoding block receipts: %w", err)
	}

	if _, err := c.rlpxConn.Write(ReceiptsCode, encodedData); err != nil {
		return fmt.Errorf("error sending block receipts: %w", err)
	}

	return nil
}
