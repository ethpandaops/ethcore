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

type Receipts eth.ReceiptsPacket66

func (msg *Receipts) Code() int { return ReceiptsCode }

func (msg *Receipts) ReqID() uint64 { return msg.RequestId }

func (m *Mimicry) handleReceipts(ctx context.Context, data []byte) (*Receipts, error) {
	s := new(Receipts)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding block receipts: %w", err)
	}

	return s, nil
}

func (m *Mimicry) sendReceipts(ctx context.Context, r *Receipts) error {
	m.log.WithFields(logrus.Fields{
		"code":           ReceiptsCode,
		"request_id":     r.RequestId,
		"receipts_count": len(r.ReceiptsPacket),
	}).Debug("sending Receipts")

	encodedData, err := rlp.EncodeToBytes(r)
	if err != nil {
		return fmt.Errorf("error encoding block receipts: %w", err)
	}

	if _, err := m.rlpxConn.Write(ReceiptsCode, encodedData); err != nil {
		return fmt.Errorf("error sending block receipts: %w", err)
	}

	return nil
}
