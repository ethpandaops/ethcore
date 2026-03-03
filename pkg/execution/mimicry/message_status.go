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
	StatusCode = RLPXOffset + eth.StatusMsg
)

// Status is a wrapper interface for StatusPacket.
type Status interface {
	Code() int
	ReqID() uint64
	GetGenesis() []byte
	GetHead() []byte
	GetNetworkID() uint64
	GetForkIDHash() []byte
	GetForkIDNext() uint64
}

type Status69 struct {
	eth.StatusPacket
}

func (msg *Status69) Code() int { return StatusCode }

func (msg *Status69) ReqID() uint64 { return 0 }

func (msg *Status69) GetGenesis() []byte { return msg.Genesis[:] }

func (msg *Status69) GetHead() []byte { return msg.LatestBlockHash[:] }

func (msg *Status69) GetNetworkID() uint64 { return msg.NetworkID }

func (msg *Status69) GetForkIDHash() []byte { return msg.ForkID.Hash[:] }

func (msg *Status69) GetForkIDNext() uint64 { return msg.ForkID.Next }

func (c *Client) receiveStatus(ctx context.Context, data []byte) (Status, error) {
	s := new(Status69)
	if err := rlp.DecodeBytes(data, &s.StatusPacket); err != nil {
		return nil, fmt.Errorf("error decoding status: %w", err)
	}

	return s, nil
}

func (c *Client) sendStatus(ctx context.Context, status Status) error {
	c.log.WithFields(logrus.Fields{
		"code":          StatusCode,
		"status":        status,
		"ethCapVersion": c.ethCapVersion,
	}).Debug("sending Status")

	var encodedData []byte

	var err error

	switch s := status.(type) {
	case *Status69:
		encodedData, err = rlp.EncodeToBytes(&s.StatusPacket)
	default:
		return fmt.Errorf("unsupported status type: %T", status)
	}

	if err != nil {
		return fmt.Errorf("error encoding status: %w", err)
	}

	if _, err := c.rlpxConn.Write(StatusCode, encodedData); err != nil {
		return fmt.Errorf("error sending status: %w", err)
	}

	return nil
}

func (c *Client) handleStatus(ctx context.Context, code uint64, data []byte) error {
	c.log.WithFields(logrus.Fields{
		"code":          code,
		"ethCapVersion": c.ethCapVersion,
	}).Debug("received Status")

	status, err := c.receiveStatus(ctx, data)
	if err != nil {
		return err
	}

	c.publishStatus(ctx, status)

	if err := c.sendStatus(ctx, status); err != nil {
		return err
	}

	return nil
}
