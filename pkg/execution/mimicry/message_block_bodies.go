// eth protocol block bodies https://github.com/ethereum/devp2p/blob/master/caps/eth.md#blockbodies-0x06
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	BlockBodiesCode = RLPXOffset + eth.BlockBodiesMsg
)

type BlockBodies eth.BlockBodiesPacket

func (msg *BlockBodies) Code() int { return BlockBodiesCode }

func (msg *BlockBodies) ReqID() uint64 { return msg.RequestId }

func (c *Client) sendBlockBodies(ctx context.Context, bh *BlockBodies) error {
	c.log.WithFields(logrus.Fields{
		"code":         BlockBodiesCode,
		"request_id":   bh.RequestId,
		"bodies_count": len(bh.BlockBodiesResponse),
	}).Debug("sending BlockBodies")

	encodedData, err := rlp.EncodeToBytes(bh)
	if err != nil {
		return fmt.Errorf("error encoding block bodies: %w", err)
	}

	if _, err := c.rlpxConn.Write(BlockBodiesCode, encodedData); err != nil {
		return fmt.Errorf("error sending block bodies: %w", err)
	}

	return nil
}
