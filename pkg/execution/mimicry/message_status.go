// eth protocol status https://github.com/ethereum/devp2p/blob/master/caps/eth.md#status-0x00
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	StatusCode = 0x10
)

type Status eth.StatusPacket

func (msg *Status) Code() int { return StatusCode }

func (msg *Status) ReqID() uint64 { return 0 }

func (m *Mimicry) receiveStatus(ctx context.Context, data []byte) (*Status, error) {
	s := new(Status)
	if err := rlp.DecodeBytes(data, &s); err != nil {
		return nil, fmt.Errorf("error decoding status: %w", err)
	}

	return s, nil
}

func (m *Mimicry) sendStatus(ctx context.Context, status *Status) error {
	m.log.WithFields(logrus.Fields{
		"code":   StatusCode,
		"status": status,
	}).Debug("sending Status")

	encodedData, err := rlp.EncodeToBytes(status)
	if err != nil {
		return fmt.Errorf("error encoding status: %w", err)
	}

	if _, err := m.rlpxConn.Write(StatusCode, encodedData); err != nil {
		return fmt.Errorf("error sending status: %w", err)
	}

	return nil
}

func (m *Mimicry) handleStatus(ctx context.Context, code uint64, data []byte) error {
	m.log.WithField("code", code).Debug("received Status")

	status, err := m.receiveStatus(ctx, data)
	if err != nil {
		return err
	}

	m.publishStatus(ctx, status)

	if err := m.sendStatus(ctx, status); err != nil {
		return err
	}

	return nil
}
