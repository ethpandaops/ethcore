package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/golang/snappy"
	"github.com/libp2p/go-libp2p/core/network"
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

var (
	_ error = &RequestError{}
)

type RequestError struct {
	Msg  string
	Type string
}

func (e *RequestError) Error() string {
	return e.Msg
}

func newRequestError(msg string) *RequestError {
	return &RequestError{Msg: msg, Type: msg}
}

func (e *RequestError) Add(msg string) *RequestError {
	return &RequestError{
		Msg:  fmt.Sprintf("%s: %s", e.Msg, msg),
		Type: e.Type,
	}
}

func (e *RequestError) Unwrap() error {
	return errors.New(e.Msg)
}

var (
	ErrInvalidResponse          = newRequestError("received invalid response")
	ErrFailedToCreateStream     = newRequestError("failed to create stream")
	ErrFailedToEncodeRequest    = newRequestError("failed to encode request")
	ErrFailedToReadResponse     = newRequestError("failed to read response")
	ErrFailedToDecodeResponse   = newRequestError("failed to decode response")
	ErrFailedToCloseWriteStream = newRequestError("failed to close write stream")
)

func (r *ReqResp) ReadRequest(ctx context.Context, stream network.Stream, payload common.SSZObj) error {
	if err := stream.SetReadDeadline(time.Now().Add(r.config.ReadTimeout)); err != nil {
		return fmt.Errorf("failed to set read deadline on stream: %w", err)
	}

	if err := r.encoder.DecodeWithMaxLength(stream, WrapSSZObject(payload)); err != nil {
		return fmt.Errorf("failed to decode request payload: %w", err)
	}

	if err := stream.CloseRead(); err != nil {
		return fmt.Errorf("failed to close read stream: %w", err)
	}

	return nil
}

func (r *ReqResp) WriteRequest(ctx context.Context, stream network.Stream, payload common.SSZObj) error {
	if err := stream.SetWriteDeadline(time.Now().Add(r.config.WriteTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline on stream: %w", err)
	}

	if _, err := r.encoder.EncodeWithMaxLength(stream, WrapSSZObject(payload)); err != nil {
		return fmt.Errorf("failed to encode request payload: %w", err)
	}

	if err := stream.CloseWrite(); err != nil {
		return fmt.Errorf("failed to close write stream: %w", err)
	}

	return nil
}

// WriteRequestBytes writes a request to the stream using raw bytes payload
func (r *ReqResp) WriteRequestBytes(ctx context.Context, stream network.Stream, payloadBytes []byte) error {
	if err := stream.SetWriteDeadline(time.Now().Add(r.config.WriteTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline on stream: %w", err)
	}

	if payloadBytes != nil {
		// Compress the payload bytes with snappy
		compressedData := snappy.Encode(nil, payloadBytes)

		// Write varint length prefix
		length := uint64(len(compressedData))
		varintBuf := make([]byte, 0, 10)
		for {
			if length < 0x80 {
				varintBuf = append(varintBuf, byte(length))
				break
			}
			varintBuf = append(varintBuf, byte(length&0x7F|0x80))
			length >>= 7
		}

		if _, err := stream.Write(varintBuf); err != nil {
			return fmt.Errorf("failed to write varint length: %w", err)
		}

		// Write the compressed data
		if _, err := stream.Write(compressedData); err != nil {
			return fmt.Errorf("failed to write compressed request: %w", err)
		}
	}

	if err := stream.CloseWrite(); err != nil {
		return fmt.Errorf("failed to close write stream: %w", err)
	}

	return nil
}

func (r *ReqResp) SendRequest(ctx context.Context, req *Request, rsp common.SSZObj) error {
	// Open the stream
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	logCtx := r.log.WithFields(logrus.Fields{
		"peer":        req.PeerID,
		"protocol_id": req.ProtocolID,
		"direction":   "outgoing",
	})

	logCtx.Debug("Sending request")

	stream, err := r.host.NewStream(ctx, req.PeerID, req.ProtocolID)
	if err != nil {
		return ErrFailedToCreateStream.Add(err.Error())
	}

	defer func() {
		if err := stream.Close(); err != nil {
			logCtx.WithError(err).Debug("Failed to close stream")
		}
	}()

	// Send the request payload only if one exists
	// IMPORTANT: Some requests (like metadata) have no payload per the Ethereum consensus spec.
	// Sending a payload when none is expected will cause the request to be rejected by some clients.
	writeStreamClosed := false

	if req.Payload != nil {
		if _, err := r.encoder.EncodeWithMaxLength(stream, WrapSSZObject(req.Payload)); err != nil {
			return ErrFailedToEncodeRequest.Add(err.Error())
		}
	} else {
		// For requests with no payload, we need to close the write side
		// immediately to signal to the remote that we're done sending.
		// This prevents the remote from waiting for data that will never come.
		if err := stream.CloseWrite(); err != nil {
			return ErrFailedToCloseWriteStream.Add(err.Error())
		}

		writeStreamClosed = true
	}

	// Wait for the response
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stream, buf); err != nil {
		return ErrFailedToReadResponse.Add(err.Error())
	}

	// Code 0x00 is a success
	// Anything else is an error
	if buf[0] != 0 {
		return ErrInvalidResponse.Add(fmt.Sprintf("received invalid response code: %#x", buf[0]))
	}

	// Read the response
	if err := r.encoder.DecodeWithMaxLength(stream, WrapSSZObject(rsp)); err != nil {
		return ErrFailedToDecodeResponse.Add(fmt.Sprintf("failed to decode response: %v", err))
	}

	// Close the write stream if not already closed.
	if !writeStreamClosed {
		if err := stream.CloseWrite(); err != nil {
			return ErrFailedToCloseWriteStream.Add(err.Error())
		}
	}

	return nil
}
