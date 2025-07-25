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
	ReceiptsCode = 0x20
)

type Receipts eth.ReceiptsPacket[*eth.ReceiptList69]

func (msg *Receipts) Code() int { return ReceiptsCode }

func (msg *Receipts) ReqID() uint64 { return msg.RequestId }

func (c *Client) sendReceipts(ctx context.Context, r *Receipts) error {
	c.log.WithFields(logrus.Fields{
		"code":           ReceiptsCode,
		"request_id":     r.RequestId,
		"receipts_count": len(r.List),
	}).Debug("sending Receipts")

	encodedData, err := rlp.EncodeToBytes(r)
	if err != nil {
		return fmt.Errorf("error encoding block receipts: %w", err)
	}

	if _, err := c.rlpxConn.Write(ReceiptsCode, encodedData); err != nil {
		return fmt.Errorf("error sending block receipts: %w", err)
	}

	return nil
}
