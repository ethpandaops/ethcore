package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
)

type ReqResp struct {
	log       logrus.FieldLogger
	host      host.Host
	encoder   encoder.SszNetworkEncoder
	protocols []protocol.ID
	config    *ReqRespConfig
}

type ReqRespConfig struct {
	WriteTimeout    time.Duration
	ReadTimeout     time.Duration
	TimeToFirstByte time.Duration
}

func NewReqResp(log logrus.FieldLogger, h host.Host, e encoder.SszNetworkEncoder, config *ReqRespConfig) *ReqResp {
	return &ReqResp{
		log:       log,
		host:      h,
		encoder:   e,
		protocols: []protocol.ID{},
		config:    config,
	}
}

func (r *ReqResp) SupportedProtocols() []protocol.ID {
	return r.protocols
}

func (r *ReqResp) RegisterHandler(ctx context.Context, proto protocol.ID, handler func(ctx context.Context, stream network.Stream) error) error {
	r.log.WithField("protocol", proto).Info("Registering protocol handler")

	for _, p := range r.protocols {
		if p == proto {
			return fmt.Errorf("protocol already registered: %s", proto)
		}
	}

	r.protocols = append(r.protocols, proto)

	r.host.SetStreamHandler(proto, r.wrapper(ctx, handler))

	return nil
}

func (r *ReqResp) wrapper(ctx context.Context, handler func(ctx context.Context, stream network.Stream) error) network.StreamHandler {
	return func(stream network.Stream) {
		logCtx := r.log.WithFields(logrus.Fields{
			"peer":      stream.Conn().RemotePeer().String(),
			"protocol":  stream.Protocol(),
			"direction": "incoming",
		})

		if err := stream.SetReadDeadline(time.Now().Add(r.config.ReadTimeout)); err != nil {
			logCtx.WithError(err).Debug("Failed to set read deadline")

			return
		}

		start := time.Now()
		defer func() {
			logCtx.WithField("duration", time.Since(start)).Debug("Handled message")
		}()

		err := handler(ctx, stream)
		if err != nil {
			logCtx.WithError(err).Debug("Req/Resp handler returned error")
		}
	}
}

// GetHost returns the underlying libp2p host.
func (r *ReqResp) GetHost() host.Host {
	return r.host
}
