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
	TransactionsCode = 0x12
)

type Transactions eth.TransactionsPacket

func (msg *Transactions) Code() int { return TransactionsCode }

func (msg *Transactions) ReqID() uint64 { return 0 }

func (m *Mimicry) receiveTransactions(ctx context.Context, data []byte) (*Transactions, error) {
	s := new(Transactions)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding transactions: %w", err)
	}

	return s, nil
}

func (m *Mimicry) handleTransactions(ctx context.Context, code uint64, data []byte) error {
	m.log.WithField("code", code).Debug("received Transactions")

	txs, err := m.receiveTransactions(ctx, data)
	if err != nil {
		return err
	}

	m.publishTransactions(ctx, txs)

	return err
}

func (m *Mimicry) sendTransactions(ctx context.Context, transactions *Transactions) error {
	m.log.WithFields(logrus.Fields{
		"code":         TransactionsCode,
		"transactions": transactions,
	}).Debug("sending Transactions")

	encodedData, err := rlp.EncodeToBytes(transactions)
	if err != nil {
		return fmt.Errorf("error encoding transactions: %w", err)
	}

	if _, err := m.rlpxConn.Write(TransactionsCode, encodedData); err != nil {
		return fmt.Errorf("error sending transactions: %w", err)
	}

	return nil
}

func (m *Mimicry) Transactions(ctx context.Context, transactions *Transactions) error {
	return m.sendTransactions(ctx, transactions)
}
