// eth protocol new pooled transaction hashes https://github.com/ethereum/devp2p/blob/master/caps/eth.md#newpooledtransactionhashes-0x08
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	NewPooledTransactionHashesCode = RLPXOffset + eth.NewPooledTransactionHashesMsg
)

type NewPooledTransactionHashes eth.NewPooledTransactionHashesPacket

func (msg *NewPooledTransactionHashes) Code() int { return NewPooledTransactionHashesCode }

func (msg *NewPooledTransactionHashes) ReqID() uint64 { return 0 }

func (c *Client) receiveNewPooledTransactionHashes(ctx context.Context, data []byte) (*NewPooledTransactionHashes, error) {
	s := new(NewPooledTransactionHashes)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding new pooled transaction hashes: %w", err)
	}

	return s, nil
}

func (c *Client) handleNewPooledTransactionHashes(ctx context.Context, code uint64, data []byte) error {
	c.log.WithField("code", code).Debug("received NewPooledTransactionHashes")

	hashes, err := c.receiveNewPooledTransactionHashes(ctx, data)
	if err != nil {
		return err
	}

	c.publishNewPooledTransactionHashes(ctx, hashes)

	return nil
}
