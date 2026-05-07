// eth protocol get get block headers https://github.com/ethereum/devp2p/blob/master/caps/eth.md#getblockheaders-0x03
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	GetBlockHeadersCode = RLPXOffset + eth.GetBlockHeadersMsg
)

type GetBlockHeaders eth.GetBlockHeadersPacket

func (msg *GetBlockHeaders) Code() int { return GetBlockHeadersCode }

func (msg *GetBlockHeaders) ReqID() uint64 { return msg.RequestId }

func (c *Client) receiveGetBlockHeaders(ctx context.Context, data []byte) (*GetBlockHeaders, error) {
	s := new(GetBlockHeaders)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding get block headers: %w", err)
	}

	return s, nil
}

func (c *Client) handleGetBlockHeaders(ctx context.Context, code uint64, data []byte) error {
	c.log.WithField(logFieldCode, code).Debug("received GetBlockHeaders")

	blockHeaders, err := c.receiveGetBlockHeaders(ctx, data)
	if err != nil {
		return err
	}

	var headersList rlp.RawList[*types.Header]

	if c.headerProvider != nil && blockHeaders.GetBlockHeadersRequest != nil {
		headers, herr := c.headerProvider(ctx, blockHeaders.GetBlockHeadersRequest)
		if herr != nil {
			c.log.WithError(herr).Debug("failed to provide block headers")
		} else {
			headersList, err = rlp.EncodeToRawList(headers)
			if err != nil {
				return fmt.Errorf("error encoding provided block headers: %w", err)
			}
		}
	}

	c.log.WithFields(logrus.Fields{
		logFieldRequestID:    blockHeaders.RequestId,
		logFieldHeadersCount: headersList.Len(),
		"amount":             blockHeaders.Amount,
		"origin_hash":        blockHeaders.Origin.Hash,
		"origin_number":      blockHeaders.Origin.Number,
		"skip":               blockHeaders.Skip,
		"reverse":            blockHeaders.Reverse,
	}).Debug("responding to GetBlockHeaders")

	err = c.sendBlockHeaders(ctx, &BlockHeaders{
		RequestId: blockHeaders.RequestId,
		List:      headersList,
	})
	if err != nil {
		return err
	}

	return nil
}
