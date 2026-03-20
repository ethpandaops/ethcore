package crawler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/sirupsen/logrus"
)

func (c *Crawler) handleStatus(ctx context.Context, stream network.Stream) error {
	logCtx := c.log.WithFields(logrus.Fields{
		"peer":      stream.Conn().RemotePeer().String(),
		"protocol":  stream.Protocol(),
		"direction": "incoming",
	})

	start := time.Now()

	defer func() {
		logCtx.WithField("duration", time.Since(start)).Debug("Handled status message")

		if err := stream.Close(); err != nil {
			logCtx.WithError(err).Debug("Failed to close stream")
		}
	}()

	// Decode the incoming status based on the negotiated protocol version.
	theirStatus, err := c.readIncomingStatus(ctx, stream, logCtx)
	if err != nil {
		return err
	}

	agentVersion := "unknown"

	rawAgentVersion, err := c.node.Peerstore().Get(stream.Conn().RemotePeer(), "AgentVersion")
	if err != nil {
		logCtx.WithError(err).Debug("Failed to get agent version")
	} else {
		a, ok := rawAgentVersion.(string)
		if !ok {
			logCtx.Debug("Agent version is not a string")
		} else {
			agentVersion = a
		}
	}

	logCtx.WithFields(logrus.Fields{
		"fork_version":    theirStatus.ForkDigest,
		"finalized_epoch": theirStatus.FinalizedEpoch,
		"finalized_root":  theirStatus.FinalizedRoot,
		"head_slot":       theirStatus.HeadSlot,
		"head_root":       theirStatus.HeadRoot,
		"agent":           agentVersion,
	}).Info("Received status message")

	status := c.GetStatus()

	if status.ForkDigest != theirStatus.ForkDigest {
		c.emitPeerStatusUpdated(&PeerStatusUpdated{
			PeerID: stream.Conn().RemotePeer(),
			ENR:    c.GetPeerENR(stream.Conn().RemotePeer()),
			Status: &status,
		})
	}

	// Respond with our status in the same protocol version the peer used.
	if err := c.writeStatusResponse(ctx, stream, &status, logCtx); err != nil {
		return err
	}

	return nil
}

// readIncomingStatus decodes a status request from the stream, handling both v1 and v2.
func (c *Crawler) readIncomingStatus(
	ctx context.Context,
	stream network.Stream,
	logCtx *logrus.Entry,
) (*common.Status, error) {
	var (
		payload common.SSZObj
		toV1    func() *common.Status
	)

	if stream.Protocol() == eth.StatusV2ProtocolID {
		v2 := &eth.StatusV2{}
		payload = v2
		toV1 = v2.ToV1
	} else {
		v1 := &common.Status{}
		payload = v1
		toV1 = func() *common.Status { return v1 }
	}

	if err := c.reqResp.ReadRequest(ctx, stream, payload); err != nil {
		logCtx.WithError(err).Error("Failed to decode status message")

		if errr := c.reqResp.WriteResponse(ctx, stream, nil, errors.New("failed to decode request body")); errr != nil {
			logCtx.WithError(errr).Debug("Failed to send error response")
		}

		return nil, err
	}

	return toV1(), nil
}

// writeStatusResponse writes our status in the protocol version the peer used.
func (c *Crawler) writeStatusResponse(
	ctx context.Context,
	stream network.Stream,
	status *common.Status,
	logCtx *logrus.Entry,
) error {
	var payload common.SSZObj = status

	if stream.Protocol() == eth.StatusV2ProtocolID {
		payload = eth.StatusV2FromV1(status)
	}

	if err := c.reqResp.WriteResponse(ctx, stream, payload, nil); err != nil {
		logCtx.WithError(err).Debug("Failed to send status response")

		return err
	}

	return nil
}

func (c *Crawler) handleGoodbye(ctx context.Context, stream network.Stream) error {
	var err error

	logCtx := c.log.WithFields(logrus.Fields{
		"peer":      stream.Conn().RemotePeer().String(),
		"protocol":  stream.Protocol(),
		"direction": "incoming",
	})

	start := time.Now()

	defer func() {
		logCtx.WithField("duration", time.Since(start)).Debug("Handled goodbye message")

		if err != nil {
			logCtx.WithError(err).Debug("Failed to handle goodbye message")

			if errr := c.reqResp.WriteResponse(ctx, stream, nil, err); errr != nil {
				logCtx.WithError(errr).Debug("Failed to send goodbye response in response to handle error")
			}
		}

		if errr := stream.Close(); errr != nil {
			logCtx.WithError(errr).Debug("Failed to close stream")
		}
	}()

	// Read the goodbye message
	var theirGoodbye common.Goodbye

	err = c.reqResp.ReadRequest(ctx, stream, &theirGoodbye)
	if err != nil {
		logCtx.WithError(err).Debug("Failed to read goodbye message")

		return err
	}

	logCtx.WithFields(logrus.Fields{
		"goodbye": theirGoodbye,
	}).Debug("Received goodbye message")

	var resp common.Goodbye

	// Send the goodbye response
	err = c.reqResp.WriteResponse(ctx, stream, &resp, nil)
	if err != nil {
		logCtx.WithError(err).Debug("Failed to send goodbye response")

		return err
	}

	return nil
}

func (c *Crawler) handlePing(ctx context.Context, stream network.Stream) error {
	var err error

	logCtx := c.log.WithFields(logrus.Fields{
		"peer":      stream.Conn().RemotePeer().String(),
		"protocol":  stream.Protocol(),
		"direction": "incoming",
	})

	start := time.Now()

	defer func() {
		logCtx.WithField("duration", time.Since(start)).Debug("Handled ping message")

		err = stream.Close()
		if err != nil {
			logCtx.WithError(err).Debug("Failed to close stream")
		}
	}()

	var theirPing common.Ping

	err = c.reqResp.ReadRequest(ctx, stream, &theirPing)
	if err != nil {
		logCtx.WithError(err).Debug("Failed to decode ping message")

		return err
	}

	logCtx.WithFields(logrus.Fields{
		"ping": fmt.Sprintf("%d", theirPing),
	}).Debug("Received ping message")

	ping := common.Ping(c.metadata.SeqNumber)

	err = c.reqResp.WriteResponse(ctx, stream, &ping, nil)
	if err != nil {
		logCtx.WithError(err).Debug("Failed to send ping response")

		return err
	}

	return nil
}

func (c *Crawler) handleMetadata(ctx context.Context, stream network.Stream) error {
	var err error

	logCtx := c.log.WithFields(logrus.Fields{
		"peer":      stream.Conn().RemotePeer().String(),
		"protocol":  stream.Protocol(),
		"direction": "incoming",
	})

	start := time.Now()

	defer func() {
		logCtx.WithField("duration", time.Since(start)).Debug("Handled metadata message")

		err = stream.Close()
		if err != nil {
			logCtx.WithError(err).Debug("Failed to close stream")
		}
	}()

	// Metadata requests have no content per the Ethereum consensus spec.
	// Respond with the appropriate version based on the negotiated protocol.
	logCtx.Debug("Received metadata request")

	if stream.Protocol() == eth.MetaDataV3ProtocolID {
		v3 := eth.MetaDataV3FromV2(c.metadata)

		if err := c.reqResp.WriteResponse(ctx, stream, v3, nil); err != nil {
			logCtx.WithError(err).Debug("Failed to send metadata v3 response")

			return err
		}

		return nil
	}

	if err := c.reqResp.WriteResponse(ctx, stream, c.metadata, nil); err != nil {
		logCtx.WithError(err).Debug("Failed to send metadata response")

		return err
	}

	return nil
}

// handleDummyRPC is a dummy handler for RPCs that are not yet implemented. It
// will always return an error.
func (c *Crawler) handleDummyRPC(ctx context.Context, stream network.Stream) error {
	logCtx := c.log.WithFields(logrus.Fields{
		"peer":      stream.Conn().RemotePeer().String(),
		"protocol":  stream.Protocol(),
		"direction": "incoming",
	})

	logCtx.Debug("Received dummy RPC")

	// Send an error response
	if err := c.reqResp.WriteResponse(ctx, stream, nil, errors.New("unknown error")); err != nil {
		logCtx.WithError(err).Debug("Failed to send dummy RPC response")

		return err
	}

	// Close the stream
	return stream.Close()
}
