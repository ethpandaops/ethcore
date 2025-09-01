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
	BlockRangeUpdateCode = 0x21
)

type BlockRangeUpdate eth.BlockRangeUpdatePacket

func (msg *BlockRangeUpdate) Code() int { return BlockRangeUpdateCode }

func (c *Client) receiveBlockRangeUpdate(ctx context.Context, data []byte) (*BlockRangeUpdate, error) {
	s := new(BlockRangeUpdate)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding block range update: %w", err)
	}

	return s, nil
}

func (c *Client) handleBlockRangeUpdate(ctx context.Context, code uint64, data []byte) error {
	c.log.WithField("code", code).Debug("received BlockRangeUpdate")

	blockRangeUpdate, err := c.receiveBlockRangeUpdate(ctx, data)
	if err != nil {
		return err
	}

	c.log.WithFields(logrus.Fields{
		"earliest_block":    blockRangeUpdate.EarliestBlock,
		"latest_block":      blockRangeUpdate.LatestBlock,
		"latest_block_hash": blockRangeUpdate.LatestBlockHash,
	}).Debug("received BlockRangeUpdate")

	return nil
}
