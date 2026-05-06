// eth protocol status https://github.com/ethereum/devp2p/blob/master/caps/eth.md#status-0x00
package mimicry

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	StatusCode = RLPXOffset + eth.StatusMsg
)

// Status is a wrapper interface for the StatusPacket.
type Status interface {
	Code() int
	ReqID() uint64
	GetProtocolVersion() uint32
	GetGenesis() []byte
	GetHead() []byte
	GetNetworkID() uint64
	GetForkIDHash() []byte
	GetForkIDNext() uint64
}

type Status68Packet struct {
	ProtocolVersion uint32
	NetworkID       uint64
	TD              *big.Int
	Head            common.Hash
	Genesis         common.Hash
	ForkID          forkid.ID
}

type Status68 struct {
	Status68Packet
}

type Status69 struct {
	eth.StatusPacket
}

func (msg *Status68) Code() int { return StatusCode }

func (msg *Status68) ReqID() uint64 { return 0 }

func (msg *Status68) GetProtocolVersion() uint32 { return msg.ProtocolVersion }

func (msg *Status68) GetGenesis() []byte { return msg.Genesis[:] }

func (msg *Status68) GetHead() []byte { return msg.Head[:] }

func (msg *Status68) GetNetworkID() uint64 { return msg.NetworkID }

func (msg *Status68) GetForkIDHash() []byte { return msg.ForkID.Hash[:] }

func (msg *Status68) GetForkIDNext() uint64 { return msg.ForkID.Next }

func (msg *Status69) Code() int { return StatusCode }

func (msg *Status69) ReqID() uint64 { return 0 }

func (msg *Status69) GetProtocolVersion() uint32 { return msg.ProtocolVersion }

func (msg *Status69) GetGenesis() []byte { return msg.Genesis[:] }

func (msg *Status69) GetHead() []byte { return msg.LatestBlockHash[:] }

func (msg *Status69) GetNetworkID() uint64 { return msg.NetworkID }

func (msg *Status69) GetForkIDHash() []byte { return msg.ForkID.Hash[:] }

func (msg *Status69) GetForkIDNext() uint64 { return msg.ForkID.Next }

func (c *Client) receiveStatus(ctx context.Context, data []byte) (Status, error) {
	switch c.ethCapVersion {
	case 68:
		s := new(Status68)
		if err := rlp.DecodeBytes(data, &s.Status68Packet); err != nil {
			return nil, fmt.Errorf("error decoding eth/68 status: %w", err)
		}

		return s, nil
	case 69, 70:
		s := new(Status69)
		if err := rlp.DecodeBytes(data, &s.StatusPacket); err != nil {
			return nil, fmt.Errorf("error decoding eth/%d status: %w", c.ethCapVersion, err)
		}

		return s, nil
	default:
		return nil, fmt.Errorf("unsupported eth protocol version for status: %d", c.ethCapVersion)
	}
}

func (c *Client) sendStatus(ctx context.Context, status Status) error {
	c.log.WithFields(logrus.Fields{
		logFieldCode:   StatusCode,
		"status":       status,
		logFieldETHCap: c.ethCapVersion,
	}).Debug("sending Status")

	var encodedData []byte

	var err error

	switch s := status.(type) {
	case *Status68:
		encodedData, err = rlp.EncodeToBytes(&s.Status68Packet)
	case *Status69:
		encodedData, err = rlp.EncodeToBytes(&s.StatusPacket)
	default:
		return fmt.Errorf("unsupported status type: %T", status)
	}

	if err != nil {
		return fmt.Errorf("error encoding status: %w", err)
	}

	if err := c.writeRLPx(StatusCode, encodedData); err != nil {
		return fmt.Errorf("error sending status: %w", err)
	}

	c.statusSent = true

	return nil
}

func (c *Client) handleStatus(ctx context.Context, code uint64, data []byte) error {
	c.log.WithFields(logrus.Fields{
		logFieldCode:   code,
		logFieldETHCap: c.ethCapVersion,
	}).Debug("received Status")

	status, err := c.receiveStatus(ctx, data)
	if err != nil {
		return err
	}

	response := status
	if c.statusProvider != nil {
		provided, perr := c.statusProvider(ctx, c.ethCapVersion, status)
		if perr != nil {
			return fmt.Errorf("failed to build provided Status: %w", perr)
		}

		if provided != nil {
			response = provided
		}
	}

	c.publishStatus(ctx, status)

	if !c.statusSent {
		if err := c.sendStatus(ctx, response); err != nil {
			return err
		}
	}

	return nil
}
