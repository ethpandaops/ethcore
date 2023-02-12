// eth protocol get get block headers https://github.com/ethereum/devp2p/blob/master/caps/eth.md#getblockheaders-0x03
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	GetBlockHeadersCode = 0x13
)

type GetBlockHeaders eth.GetBlockHeadersPacket66

func (msg *GetBlockHeaders) Code() int { return GetBlockHeadersCode }

func (msg *GetBlockHeaders) ReqID() uint64 { return msg.RequestId }

func (m *Mimicry) receiveGetBlockHeaders(ctx context.Context, data []byte) (*GetBlockHeaders, error) {
	s := new(GetBlockHeaders)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding get block headers: %w", err)
	}

	return s, nil
}

func (m *Mimicry) handleGetBlockHeaders(ctx context.Context, code uint64, data []byte) error {
	m.log.WithField("code", code).Debug("received GetBlockHeaders")

	blockHeaders, err := m.receiveGetBlockHeaders(ctx, data)
	if err != nil {
		return err
	}

	err = m.sendGetBlockHeaders(ctx, blockHeaders)
	if err != nil {
		return err
	}

	return nil
}

func (m *Mimicry) sendGetBlockHeaders(ctx context.Context, bh *GetBlockHeaders) error {
	m.log.WithFields(logrus.Fields{
		"code":       GetBlockHeadersCode,
		"request_id": bh.RequestId,
		"headers":    bh.GetBlockHeadersPacket,
	}).Debug("sending GetBlockHeaders")

	encodedData, err := rlp.EncodeToBytes(bh)
	if err != nil {
		return fmt.Errorf("error encoding get block headers: %w", err)
	}

	if _, err := m.rlpxConn.Write(GetBlockHeadersCode, encodedData); err != nil {
		return fmt.Errorf("error sending get block headers: %w", err)
	}

	return nil
}
