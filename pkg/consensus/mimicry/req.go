package mimicry

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/sirupsen/logrus"
)

type Request struct {
	ProtocolID protocol.ID
	PeerID     peer.ID
	Payload    common.SSZObj
	Timeout    time.Duration
}

func (c *Client) sendRequest(ctx context.Context, r *Request, rsp common.SSZObj) error {
	// Open the stream
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	logCtx := c.log.WithFields(logrus.Fields{
		"peer":        r.PeerID,
		"protocol_id": r.ProtocolID,
	})

	logCtx.Debug("Sending request")

	stream, err := c.host.NewStream(ctx, r.PeerID, r.ProtocolID)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	defer func() {
		if err := stream.Close(); err != nil {
			c.log.WithError(err).Error("Failed to close stream")
		}
	}()

	// Send the request
	if _, err := sszNetworkEncoder.EncodeWithMaxLength(stream, WrapSSZObject(r.Payload)); err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	// Wait for the response
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stream, buf); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Code 0x00 is a success
	// Anything else is an error
	if buf[0] != 0 {
		return fmt.Errorf("received invalid response")
	}

	// Read the response
	response := WrapSSZObject(r.Payload)
	if err := sszNetworkEncoder.DecodeWithMaxLength(stream, response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Close the stream
	if err := stream.CloseWrite(); err != nil {
		return fmt.Errorf("failed to close write stream: %w", err)
	}

	return nil
}

func (c *Client) RequestStatusFromPeer(ctx context.Context, peerID peer.ID) (*common.Status, error) {
	status := c.GetStatus()

	req := &Request{
		ProtocolID: StatusProtocolID,
		PeerID:     peerID,
		Payload:    &status,
		Timeout:    time.Second * 30,
	}

	rsp := &common.Status{}

	if err := c.sendRequest(ctx, req, rsp); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return rsp, nil
}

func (c *Client) RequestMetadataFromPeer(ctx context.Context, peerID peer.ID) (*common.MetaData, error) {
	metadata := c.metadata

	req := &Request{
		ProtocolID: MetaDataProtocolID,
		PeerID:     peerID,
		Payload:    metadata,
		Timeout:    time.Second * 30,
	}

	rsp := &common.MetaData{}

	if err := c.sendRequest(ctx, req, rsp); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return rsp, nil
}
