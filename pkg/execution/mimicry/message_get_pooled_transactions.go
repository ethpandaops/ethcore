// eth protocol get get block headers https://github.com/ethereum/devp2p/blob/master/caps/eth.md#getblockheaders-0x03
package mimicry

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	GetPooledTransactionsCode = RLPXOffset + eth.GetPooledTransactionsMsg
)

type GetPooledTransactions eth.GetPooledTransactionsPacket

func (msg *GetPooledTransactions) Code() int { return GetPooledTransactionsCode }

func (msg *GetPooledTransactions) ReqID() uint64 { return msg.RequestId }

func (c *Client) receiveGetPooledTransactions(ctx context.Context, data []byte) (*GetPooledTransactions, error) {
	s := new(GetPooledTransactions)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding get pooled transactions: %w", err)
	}

	return s, nil
}

func (c *Client) handleGetPooledTransactions(ctx context.Context, code uint64, data []byte) error {
	c.log.WithField(logFieldCode, code).Debug("received GetPooledTransactions")

	txs, err := c.receiveGetPooledTransactions(ctx, data)
	if err != nil {
		return err
	}

	return c.sendPooledTransactions(ctx, &PooledTransactions{
		RequestId: txs.RequestId,
	})
}

func (c *Client) sendGetPooledTransactions(ctx context.Context, pt *GetPooledTransactions) error {
	c.log.WithFields(logrus.Fields{
		logFieldCode:      GetPooledTransactionsCode,
		logFieldRequestID: pt.RequestId,
		"txs_count":       len(pt.GetPooledTransactionsRequest),
	}).Debug("sending GetPooledTransactions")

	encodedData, err := rlp.EncodeToBytes(pt)
	if err != nil {
		return fmt.Errorf("error encoding get pooled transactions: %w", err)
	}

	if err := c.writeRLPx(GetPooledTransactionsCode, encodedData); err != nil {
		return fmt.Errorf("error sending get pooled transactions: %w", err)
	}

	return nil
}

func (c *Client) GetPooledTransactions(ctx context.Context, hashes []common.Hash) (*PooledTransactions, error) {
	//nolint:gosec // not a security issue
	requestID := uint64(rand.Uint32())<<32 + uint64(rand.Uint32())

	c.pooledTransactionsMux.Lock()
	c.pooledTransactionsMap[requestID] = make(chan *PooledTransactions)
	c.pooledTransactionsMux.Unlock()

	defer func() {
		c.pooledTransactionsMux.Lock()
		defer c.pooledTransactionsMux.Unlock()

		if ch, exists := c.pooledTransactionsMap[requestID]; exists {
			close(ch)
			delete(c.pooledTransactionsMap, requestID)
		}
	}()

	if err := c.sendGetPooledTransactions(ctx, &GetPooledTransactions{
		RequestId:                    requestID,
		GetPooledTransactionsRequest: hashes,
	}); err != nil {
		return nil, err
	}

	// Read the channel reference under the lock rather than indexing the
	// map directly inside the select: the map is also written (creation
	// above, delete in the deferred cleanup) under this same mutex, and a
	// second concurrent GetPooledTransactions call reaching either of
	// those while this select evaluates the map index unlocked is a fatal
	// concurrent map read/write.
	c.pooledTransactionsMux.Lock()
	ch := c.pooledTransactionsMap[requestID]
	c.pooledTransactionsMux.Unlock()

	select {
	case res := <-ch:
		return res, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout")
	}
}
