// RLPx ping https://github.com/ethereum/devp2p/blob/master/rlpx.md#ping-0x02
package mimicry

import "context"

const (
	PingCode = 0x02
)

type Ping struct{}

func (h *Ping) Code() int { return PingCode }

func (h *Ping) ReqID() uint64 { return 0 }

func (m *Mimicry) handlePing(ctx context.Context, code uint64, data []byte) error {
	m.log.WithField("code", code).Debug("received Ping")

	if err := m.sendPong(ctx); err != nil {
		return err
	}

	return nil
}
