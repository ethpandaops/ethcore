// eth protocol transactions https://github.com/ethereum/devp2p/blob/master/caps/eth.md#transactions-0x02
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	TransactionsCode = RLPXOffset + eth.TransactionsMsg
)

type Transactions eth.TransactionsPacket

func (msg *Transactions) Code() int { return TransactionsCode }

func (msg *Transactions) ReqID() uint64 { return 0 }

func (c *Client) receiveTransactions(ctx context.Context, data []byte) (*Transactions, error) {
	s := new(Transactions)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding transactions: %w", err)
	}

	return s, nil
}

func (c *Client) handleTransactions(ctx context.Context, code uint64, data []byte) error {
	c.log.WithField("code", code).Debug("received Transactions")

	txs, err := c.receiveTransactions(ctx, data)
	if err != nil {
		return err
	}

	c.publishTransactions(ctx, txs)

	return err
}

func (c *Client) sendTransactions(ctx context.Context, transactions *Transactions) error {
	c.log.WithFields(logrus.Fields{
		"code":         TransactionsCode,
		"transactions": transactions,
	}).Debug("sending Transactions")

	encodedData, err := rlp.EncodeToBytes(transactions)
	if err != nil {
		return fmt.Errorf("error encoding transactions: %w", err)
	}

	if _, err := c.rlpxConn.Write(TransactionsCode, encodedData); err != nil {
		return fmt.Errorf("error sending transactions: %w", err)
	}

	return nil
}

func (c *Client) Transactions(ctx context.Context, transactions *Transactions) error {
	return c.sendTransactions(ctx, transactions)
}
