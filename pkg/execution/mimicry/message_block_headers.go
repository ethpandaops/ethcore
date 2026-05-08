// eth protocol block headers https://github.com/ethereum/devp2p/blob/master/caps/eth.md#blockheaders-0x04
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	BlockHeadersCode = RLPXOffset + eth.BlockHeadersMsg
)

type BlockHeaders eth.BlockHeadersPacket

func (msg *BlockHeaders) Code() int { return BlockHeadersCode }

func (msg *BlockHeaders) ReqID() uint64 { return msg.RequestId }

func (c *Client) receiveBlockHeaders(ctx context.Context, data []byte) (*BlockHeaders, error) {
	s := new(BlockHeaders)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding block headers: %w", err)
	}

	return s, nil
}

func (c *Client) handleBlockHeaders(ctx context.Context, code uint64, data []byte) error {
	c.log.WithField(logFieldCode, code).Debug("received BlockHeaders")

	blockHeaders, err := c.receiveBlockHeaders(ctx, data)
	if err != nil {
		return err
	}

	c.log.WithFields(logrus.Fields{
		logFieldRequestID:    blockHeaders.RequestId,
		logFieldHeadersCount: blockHeaders.List.Len(),
	}).Debug("ignoring unsolicited BlockHeaders")

	return nil
}

func (c *Client) sendBlockHeaders(ctx context.Context, bh *BlockHeaders) error {
	c.log.WithFields(logrus.Fields{
		logFieldCode:         BlockHeadersCode,
		logFieldRequestID:    bh.RequestId,
		logFieldHeadersCount: bh.List.Len(),
	}).Debug("sending BlockHeaders")

	encodedData, err := rlp.EncodeToBytes(bh)
	if err != nil {
		return fmt.Errorf("error encoding block headers: %w", err)
	}

	if err := c.writeRLPx(BlockHeadersCode, encodedData); err != nil {
		return fmt.Errorf("error sending block headers: %w", err)
	}

	return nil
}
