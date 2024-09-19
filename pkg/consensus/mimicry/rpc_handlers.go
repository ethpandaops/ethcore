package mimicry

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/sirupsen/logrus"
)

func (c *Client) registerHandlers() error {
	if err := c.registerHandler(StatusProtocolID, c.handleStatus); err != nil {
		return fmt.Errorf("failed to register status handler: %w", err)
	}

	if err := c.registerHandler(GoodbyeProtocolID, c.handleGoodbye); err != nil {
		return fmt.Errorf("failed to register goodbye handler: %w", err)
	}

	if err := c.registerHandler(PingProtocolID, c.handlePing); err != nil {
		return fmt.Errorf("failed to register ping handler: %w", err)
	}

	return nil
}

func (c *Client) registerHandler(proto protocol.ID, handler func(stream network.Stream)) error {
	c.log.WithField("protocol", proto).Debug("Registering protocol handler")

	c.host.SetStreamHandler(proto, handler)

	return nil
}

func (c *Client) handleStatus(stream network.Stream) {
	start := time.Now()
	defer func() {
		c.log.WithField("duration", time.Since(start)).Debug("Handled status message")

		if err := stream.Close(); err != nil {
			c.log.WithError(err).Error("Failed to close stream")
		}
	}()

	logCtx := c.log.WithFields(logrus.Fields{
		"peer":     stream.Conn().RemotePeer().String(),
		"protocol": stream.Protocol(),
	})

	var theirStatus common.Status
	if err := sszNetworkEncoder.DecodeWithMaxLength(stream, WrapSSZObject(&theirStatus)); err != nil {
		logCtx.WithError(err).Error("Failed to decode status message")

		return
	}

	logCtx.WithFields(logrus.Fields{
		"fork_version":    theirStatus.ForkDigest,
		"finalized_epoch": theirStatus.FinalizedEpoch,
		"finalized_root":  theirStatus.FinalizedRoot,
		"head_slot":       theirStatus.HeadSlot,
		"head_root":       theirStatus.HeadRoot,
	}).Debug("Received status message")

	// Write our response
	if _, err := stream.Write([]byte{0x00}); err != nil {
		logCtx.WithError(err).Error("Failed to write response to status message")

		return
	}

	status := c.GetStatus()

	// Write our status
	if _, err := sszNetworkEncoder.EncodeWithMaxLength(stream, WrapSSZObject(&status)); err != nil {
		logCtx.WithError(err).Error("Failed to write status to peer")

		return
	}

	if err := stream.CloseWrite(); err != nil {
		logCtx.WithError(err).Error("Failed to close write stream")

		return
	}
}

func (c *Client) handleGoodbye(stream network.Stream) {
	start := time.Now()
	defer func() {
		c.log.WithField("duration", time.Since(start)).Debug("Handled goodbye message")

		if err := stream.Close(); err != nil {
			c.log.WithError(err).Error("Failed to close stream")
		}
	}()

	logCtx := c.log.WithFields(logrus.Fields{
		"peer":     stream.Conn().RemotePeer().String(),
		"protocol": stream.Protocol(),
	})

	if _, err := stream.Write([]byte{0x00}); err != nil {
		logCtx.WithError(err).Error("Failed to write response to goodbye")

		return
	}

	var theirGoodbye common.Goodbye
	if err := sszNetworkEncoder.DecodeWithMaxLength(stream, WrapSSZObject(&theirGoodbye)); err != nil {
		logCtx.WithError(err).Error("Failed to decode goodbye message")

		return
	}

	logCtx.WithFields(logrus.Fields{
		"goodbye": theirGoodbye,
	}).Debug("Received goodbye message")

	var resp common.Goodbye
	if _, err := sszNetworkEncoder.EncodeWithMaxLength(stream, WrapSSZObject(&resp)); err != nil {
		logCtx.WithError(err).Error("Failed to write goodbye to peer")

		return
	}

	if err := stream.CloseWrite(); err != nil {
		logCtx.WithError(err).Error("Failed to close write stream")
	}
}

func (c *Client) handlePing(stream network.Stream) {
	start := time.Now()
	defer func() {
		c.log.WithField("duration", time.Since(start)).Debug("Handled ping message")

		if err := stream.Close(); err != nil {
			c.log.WithError(err).Error("Failed to close stream")
		}
	}()

	logCtx := c.log.WithFields(logrus.Fields{
		"peer":     stream.Conn().RemotePeer().String(),
		"protocol": stream.Protocol(),
	})

	var theirPing common.Ping
	if err := sszNetworkEncoder.DecodeWithMaxLength(stream, WrapSSZObject(&theirPing)); err != nil {
		logCtx.WithError(err).Error("Failed to decode ping message")

		return
	}

	logCtx.WithFields(logrus.Fields{
		"ping": fmt.Sprintf("%d", theirPing),
	}).Debug("Received ping message")

	ping := common.Ping(c.metadata.SeqNumber)

	// Send the response
	if _, err := stream.Write([]byte{0x00}); err != nil {
		logCtx.WithError(err).Error("Failed to write response to ping")

		return
	}

	// Write the ping
	if _, err := sszNetworkEncoder.EncodeWithMaxLength(stream, WrapSSZObject(&ping)); err != nil {
		logCtx.WithError(err).Error("Failed to write ping to peer")

		return
	}

	if err := stream.CloseWrite(); err != nil {
		logCtx.WithError(err).Error("Failed to close write stream")
	}
}

func (c *Client) handleMetadata(stream network.Stream) {
	start := time.Now()
	defer func() {
		c.log.WithField("duration", time.Since(start)).Debug("Handled metadata message")

		if err := stream.Close(); err != nil {
			c.log.WithError(err).Error("Failed to close stream")
		}
	}()

	logCtx := c.log.WithFields(logrus.Fields{
		"peer":     stream.Conn().RemotePeer().String(),
		"protocol": stream.Protocol(),
	})

	var theirMetadata common.MetaData
	if err := sszNetworkEncoder.DecodeWithMaxLength(stream, WrapSSZObject(&theirMetadata)); err != nil {
		logCtx.WithError(err).Error("Failed to decode metadata message")

		return
	}

	logCtx.WithFields(logrus.Fields{
		"metadata": theirMetadata,
	}).Debug("Received metadata message")

	// Write the response
	if _, err := stream.Write([]byte{0x00}); err != nil {
		logCtx.WithError(err).Error("Failed to write response to metadata")

		return
	}

	resp := c.metadata
	if _, err := sszNetworkEncoder.EncodeWithMaxLength(stream, WrapSSZObject(resp)); err != nil {
		logCtx.WithError(err).Error("Failed to write metadata to peer")

		return
	}

	if err := stream.CloseWrite(); err != nil {
		logCtx.WithError(err).Error("Failed to close write stream")
	}
}
