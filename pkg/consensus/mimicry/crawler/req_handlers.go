package crawler

import (
	"context"
	"errors"
	"fmt"
	"time"

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

	var theirStatus common.Status

	err := c.reqResp.ReadRequest(ctx, stream, &theirStatus)
	if err != nil {
		logCtx.WithError(err).Debug("Failed to decode status message")

		if errr := c.reqResp.WriteResponse(ctx, stream, nil, errors.New("failed to decode request body")); errr != nil {
			logCtx.WithError(errr).Debug("Failed to send status response in response to decode error")
		}

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
	}).Debug("Received status message")

	status := c.GetStatus()

	if status.ForkDigest != theirStatus.ForkDigest {
		c.emitPeerStatusUpdated(stream.Conn().RemotePeer(), &status)
	}

	if err := c.reqResp.WriteResponse(ctx, stream, &status, nil); err != nil {
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

	// Metadata requests have no content per the Ethereum consensus spec
	// The request opens and negotiates the stream without sending any request content
	// We immediately respond with our local metadata
	logCtx.Debug("Received metadata request")

	resp := c.metadata

	if err := c.reqResp.WriteResponse(ctx, stream, resp, nil); err != nil {
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
