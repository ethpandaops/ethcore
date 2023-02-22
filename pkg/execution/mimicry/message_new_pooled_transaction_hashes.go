// eth protocol new pooled transaction hashes https://github.com/ethereum/devp2p/blob/master/caps/eth.md#newpooledtransactionhashes-0x08
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	NewPooledTransactionHashesCode = 0x18
)

type NewPooledTransactionHashes66 eth.NewPooledTransactionHashesPacket66

type NewPooledTransactionHashes68 eth.NewPooledTransactionHashesPacket68

func (msg *NewPooledTransactionHashes66) Code() int { return NewPooledTransactionHashesCode }
func (msg *NewPooledTransactionHashes68) Code() int { return NewPooledTransactionHashesCode }

func (msg *NewPooledTransactionHashes66) ReqID() uint64 { return 0 }
func (msg *NewPooledTransactionHashes68) ReqID() uint64 { return 0 }

func (c *Client) receiveNewPooledTransactionHashes66(ctx context.Context, data []byte) (*NewPooledTransactionHashes66, error) {
	s := new(NewPooledTransactionHashes66)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding new pooled transaction hashes: %w", err)
	}

	return s, nil
}

func (c *Client) receiveNewPooledTransactionHashes68(ctx context.Context, data []byte) (*NewPooledTransactionHashes68, error) {
	s := new(NewPooledTransactionHashes68)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding new pooled transaction hashes: %w", err)
	}

	return s, nil
}

func (c *Client) sendNewPooledTransactionHashes66(ctx context.Context, pth *NewPooledTransactionHashes66) error {
	c.log.WithFields(logrus.Fields{
		"code":         NewPooledTransactionHashesCode,
		"hashes_count": len(*pth),
	}).Debug("sending NewPooledTransactionHashes")

	encodedData, err := rlp.EncodeToBytes(pth)
	if err != nil {
		return fmt.Errorf("error encoding new pooled transaction hashes: %w", err)
	}

	if _, err := c.rlpxConn.Write(NewPooledTransactionHashesCode, encodedData); err != nil {
		return fmt.Errorf("error sending new pooled transaction hashes: %w", err)
	}

	return nil
}

func (c *Client) sendNewPooledTransactionHashes68(ctx context.Context, pth *NewPooledTransactionHashes68) error {
	c.log.WithFields(logrus.Fields{
		"code":         NewPooledTransactionHashesCode,
		"hashes_count": len(pth.Hashes),
	}).Debug("sending NewPooledTransactionHashes")

	encodedData, err := rlp.EncodeToBytes(pth)
	if err != nil {
		return fmt.Errorf("error encoding new pooled transaction hashes: %w", err)
	}

	if _, err := c.rlpxConn.Write(NewPooledTransactionHashesCode, encodedData); err != nil {
		return fmt.Errorf("error sending new pooled transaction hashes: %w", err)
	}

	return nil
}

func (c *Client) handleNewPooledTransactionHashes(ctx context.Context, code uint64, data []byte) error {
	c.log.WithField("code", code).Debug("received NewPooledTransactionHashes")

	if c.ethCapVersion < 68 {
		hashes, err := c.receiveNewPooledTransactionHashes66(ctx, data)
		if err != nil {
			return err
		}

		c.publishNewPooledTransactionHashes66(ctx, hashes)
	} else {
		hashes, err := c.receiveNewPooledTransactionHashes68(ctx, data)
		if err != nil {
			return err
		}

		c.publishNewPooledTransactionHashes68(ctx, hashes)
	}

	return nil
}
