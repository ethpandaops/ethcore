// eth protocol get get block headers https://github.com/ethereum/devp2p/blob/master/caps/eth.md#getblockreceipts-0x05
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	GetReceiptsCode = RLPXOffset + eth.GetReceiptsMsg
)

type GetReceipts eth.GetReceiptsPacket

func (msg *GetReceipts) Code() int { return GetReceiptsCode }

func (msg *GetReceipts) ReqID() uint64 { return msg.RequestId }

func (msg *GetReceipts) ReceiptRequest() ReceiptRequest {
	return ReceiptRequest{
		Hashes: msg.GetReceiptsRequest,
	}
}

type GetReceipts70Packet struct {
	RequestId              uint64
	FirstBlockReceiptIndex uint64
	eth.GetReceiptsRequest
}

type GetReceipts70 struct {
	GetReceipts70Packet
}

func (msg *GetReceipts70) Code() int { return GetReceiptsCode }

func (msg *GetReceipts70) ReqID() uint64 { return msg.RequestId }

func (msg *GetReceipts70) ReceiptRequest() ReceiptRequest {
	return ReceiptRequest{
		Hashes:                 msg.GetReceiptsRequest,
		FirstBlockReceiptIndex: msg.FirstBlockReceiptIndex,
	}
}

type GetReceiptsMessage interface {
	Code() int
	ReqID() uint64
	ReceiptRequest() ReceiptRequest
}

func (c *Client) receiveGetReceipts(ctx context.Context, data []byte) (GetReceiptsMessage, error) {
	switch c.ethCapVersion {
	case 70:
		s := new(GetReceipts70)
		if err := rlp.DecodeBytes(data, &s.GetReceipts70Packet); err != nil {
			return nil, fmt.Errorf("error decoding eth/70 get block receipts: %w", err)
		}

		return s, nil
	default:
		s := new(GetReceipts)
		if err := rlp.DecodeBytes(data, &s); err != nil {
			return nil, fmt.Errorf("error decoding get block receipts: %w", err)
		}

		return s, nil
	}
}

func (c *Client) handleGetReceipts(ctx context.Context, code uint64, data []byte) error {
	c.log.WithField(logFieldCode, code).Debug("received GetReceipts")

	request, err := c.receiveGetReceipts(ctx, data)
	if err != nil {
		return err
	}

	var list rlp.RawList[*eth.ReceiptList]

	receiptRequest := request.ReceiptRequest()

	if c.receiptProvider != nil {
		receipts, rerr := c.receiptProvider(ctx, receiptRequest)
		if rerr != nil {
			return fmt.Errorf("fetch receipts: %w", rerr)
		}

		list, err = rlp.EncodeToRawList(receipts)
		if err != nil {
			return fmt.Errorf("encode receipts: %w", err)
		}
	}

	c.log.WithFields(logrus.Fields{
		logFieldRequestID:           request.ReqID(),
		"first_block_receipt_index": receiptRequest.FirstBlockReceiptIndex,
		"requested_count":           len(receiptRequest.Hashes),
		"response_count":            list.Len(),
	}).Debug("responding to GetReceipts")

	var receipts Receipts
	if c.ethCapVersion >= 70 {
		receipts = &Receipts70{Receipts70Packet: Receipts70Packet{
			RequestId:           request.ReqID(),
			LastBlockIncomplete: false,
			List:                list,
		}}
	} else {
		receipts = &Receipts69{ReceiptsPacket: eth.ReceiptsPacket{
			RequestId: request.ReqID(),
			List:      list,
		}}
	}

	err = c.sendReceipts(ctx, receipts)
	if err != nil {
		c.handleSessionError(ctx, err)

		return err
	}

	return nil
}
