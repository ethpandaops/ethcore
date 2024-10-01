package crawler

import (
	"context"
	"fmt"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/sirupsen/logrus"
)

func (c *Crawler) handleStatus(ctx context.Context, stream network.Stream) (common.SSZObj, error) {
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

	var theirStatus common.Status
	if err := sszNetworkEncoder.DecodeWithMaxLength(stream, p2p.WrapSSZObject(&theirStatus)); err != nil {
		logCtx.WithError(err).Debug("Failed to decode status message")

		return nil, err
	}

	agentVersion := "unknown"

	rawAgentVersion, err := c.node.Peerstore().Get(stream.Conn().RemotePeer(), "AgentVersion")
	if err != nil {
		logCtx.WithError(err).Debug("Failed to get agent version")
	} else {
		agentVersion = rawAgentVersion.(string)
	}

	logCtx.WithFields(logrus.Fields{
		"fork_version":    theirStatus.ForkDigest,
		"finalized_epoch": theirStatus.FinalizedEpoch,
		"finalized_root":  theirStatus.FinalizedRoot,
		"head_slot":       theirStatus.HeadSlot,
		"head_root":       theirStatus.HeadRoot,
		"agent":           agentVersion,
	}).Debug("Received status message")

	status := c.GetStatus()

	if status.ForkDigest != theirStatus.ForkDigest {
		c.emitPeerStatusUpdated(stream.Conn().RemotePeer(), &status)
	}

	return &status, nil
}

func (c *Crawler) handleGoodbye(ctx context.Context, stream network.Stream) (common.SSZObj, error) {
	logCtx := c.log.WithFields(logrus.Fields{
		"peer":      stream.Conn().RemotePeer().String(),
		"protocol":  stream.Protocol(),
		"direction": "incoming",
	})

	start := time.Now()
	defer func() {
		logCtx.WithField("duration", time.Since(start)).Debug("Handled goodbye message")

		if err := stream.Close(); err != nil {
			logCtx.WithError(err).Debug("Failed to close stream")
		}
	}()

	var theirGoodbye common.Goodbye
	if err := sszNetworkEncoder.DecodeWithMaxLength(stream, p2p.WrapSSZObject(&theirGoodbye)); err != nil {
		logCtx.WithError(err).Debug("Failed to decode goodbye message")

		return nil, err
	}

	logCtx.WithFields(logrus.Fields{
		"goodbye": theirGoodbye,
	}).Debug("Received goodbye message")

	var resp common.Goodbye

	return &resp, nil
}

func (c *Crawler) handlePing(ctx context.Context, stream network.Stream) (common.SSZObj, error) {
	logCtx := c.log.WithFields(logrus.Fields{
		"peer":      stream.Conn().RemotePeer().String(),
		"protocol":  stream.Protocol(),
		"direction": "incoming",
	})

	start := time.Now()
	defer func() {
		logCtx.WithField("duration", time.Since(start)).Debug("Handled ping message")

		if err := stream.Close(); err != nil {
			logCtx.WithError(err).Debug("Failed to close stream")
		}
	}()

	var theirPing common.Ping
	if err := sszNetworkEncoder.DecodeWithMaxLength(stream, p2p.WrapSSZObject(&theirPing)); err != nil {
		logCtx.WithError(err).Debug("Failed to decode ping message")

		return nil, err
	}

	logCtx.WithFields(logrus.Fields{
		"ping": fmt.Sprintf("%d", theirPing),
	}).Debug("Received ping message")

	ping := common.Ping(c.metadata.SeqNumber)

	return &ping, nil
}

func (c *Crawler) handleMetadata(ctx context.Context, stream network.Stream) (common.SSZObj, error) {
	logCtx := c.log.WithFields(logrus.Fields{
		"peer":      stream.Conn().RemotePeer().String(),
		"protocol":  stream.Protocol(),
		"direction": "incoming",
	})

	start := time.Now()
	defer func() {
		logCtx.WithField("duration", time.Since(start)).Debug("Handled metadata message")

		if err := stream.Close(); err != nil {
			logCtx.WithError(err).Debug("Failed to close stream")
		}
	}()

	var theirMetadata common.MetaData
	if err := sszNetworkEncoder.DecodeWithMaxLength(stream, p2p.WrapSSZObject(&theirMetadata)); err != nil {
		logCtx.WithError(err).Debug("Failed to decode metadata message")

		return nil, err
	}

	logCtx.WithFields(logrus.Fields{
		"metadata": theirMetadata,
	}).Debug("Received metadata message")

	resp := c.metadata

	return resp, nil
}
