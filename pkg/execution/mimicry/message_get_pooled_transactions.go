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
	GetPooledTransactionsCode = 0x19
)

type GetPooledTransactions eth.GetPooledTransactionsPacket

func (msg *GetPooledTransactions) Code() int { return GetPooledTransactionsCode }

func (msg *GetPooledTransactions) ReqID() uint64 { return msg.RequestId }

func (c *Client) sendGetPooledTransactions(ctx context.Context, pt *GetPooledTransactions) error {
	c.log.WithFields(logrus.Fields{
		"code":       GetPooledTransactionsCode,
		"request_id": pt.RequestId,
		"txs_count":  len(pt.GetPooledTransactionsRequest),
	}).Debug("sending GetPooledTransactions")

	encodedData, err := rlp.EncodeToBytes(pt)
	if err != nil {
		return fmt.Errorf("error encoding get block headers: %w", err)
	}

	if _, err := c.rlpxConn.Write(GetPooledTransactionsCode, encodedData); err != nil {
		return fmt.Errorf("error sending get block headers: %w", err)
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

	select {
	case res := <-c.pooledTransactionsMap[requestID]:
		return res, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout")
	}
}
