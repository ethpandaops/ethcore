// RLPx disconnect https://github.com/ethereum/devp2p/blob/master/rlpx.md#disconnect-0x01
package mimicry

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/golang/snappy"
)

const (
	DisconnectCode = 0x01
)

type Disconnect struct {
	Reason p2p.DiscReason
}

func (h *Disconnect) Code() int { return DisconnectCode }

func (h *Disconnect) ReqID() uint64 { return 0 }

func (c *Client) receiveDisconnect(ctx context.Context, data []byte) *Disconnect {
	reason, err := decodeDisconnectReason(data)
	if err != nil {
		c.log.WithError(err).WithField("raw", hexutil.Encode(data)).Debug("Error decoding disconnect")
	}

	return &Disconnect{Reason: reason}
}

func decodeDisconnectReason(data []byte) (p2p.DiscReason, error) {
	if len(data) == 0 {
		return p2p.DiscInvalid, nil
	}

	reason, err := decodeDisconnectReasonRLP(data)
	if err == nil {
		return reason, nil
	}

	if decoded, snappyErr := snappy.Decode(nil, data); snappyErr == nil {
		if reason, snappyDecodeErr := decodeDisconnectReasonRLP(decoded); snappyDecodeErr == nil {
			return reason, nil
		}
	}

	return p2p.DiscInvalid, err
}

func decodeDisconnectReasonRLP(data []byte) (p2p.DiscReason, error) {
	if len(data) == 0 {
		return p2p.DiscInvalid, nil
	}

	if data[0] >= 0xc0 {
		var elements []rlp.RawValue
		if err := rlp.DecodeBytes(data, &elements); err != nil {
			return p2p.DiscInvalid, err
		}

		if len(elements) == 0 {
			return p2p.DiscInvalid, nil
		}

		var reasonBytes []byte
		if err := rlp.DecodeBytes(elements[0], &reasonBytes); err != nil {
			return p2p.DiscInvalid, err
		}

		if len(reasonBytes) != 1 {
			return p2p.DiscInvalid, nil
		}

		return p2p.DiscReason(reasonBytes[0]), nil
	}

	reason := new(p2p.DiscReason)
	if err := rlp.DecodeBytes(data, reason); err != nil {
		return p2p.DiscInvalid, fmt.Errorf("decode legacy disconnect reason: %w", err)
	}

	return *reason, nil
}

func (c *Client) handleDisconnect(ctx context.Context, code uint64, data []byte) {
	c.log.WithFields(map[string]any{
		logFieldCode: code,
		"raw":        hexutil.Encode(data),
	}).Debug("received Disconnect")

	disconnect := c.receiveDisconnect(ctx, data)

	c.disconnect(ctx, disconnect)
}
