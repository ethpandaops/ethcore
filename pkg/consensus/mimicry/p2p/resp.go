package p2p

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/golang/snappy"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

func (r *ReqResp) ReadResponse(ctx context.Context, stream network.Stream, payload common.SSZObj, errMsg error) error {
	if err := stream.SetReadDeadline(time.Now().Add(r.config.ReadTimeout)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}

	code := make([]byte, 1)
	if _, err := io.ReadFull(stream, code); err != nil {
		return fmt.Errorf("failed to read response code: %w", err)
	}

	// 0 -> success, otherwise -> error
	if int(code[0]) != 0 {
		errData, err := io.ReadAll(stream)
		if err != nil {
			return fmt.Errorf("failed to read error data (code %d): %w", int(code[0]), err)
		}

		return fmt.Errorf("got error response (%d): %s", int(code[0]), string(errData))
	}

	if err := r.encoder.DecodeWithMaxLength(stream, WrapSSZObject(payload)); err != nil {
		return fmt.Errorf("failed to decode response payload: %w", err)
	}

	if err := stream.CloseRead(); err != nil {
		return fmt.Errorf("failed to close read stream: %w", err)
	}

	return nil
}

func (r *ReqResp) WriteResponse(ctx context.Context, stream network.Stream, payload common.SSZObj, errMsg error) error {
	if err := stream.SetWriteDeadline(time.Now().Add(r.config.WriteTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	if errMsg != nil {
		if _, err := stream.Write([]byte{0x01}); err != nil {
			return fmt.Errorf("failed to write error response code: %w", err)
		}
	} else {
		if _, err := stream.Write([]byte{0x00}); err != nil {
			return fmt.Errorf("failed to write success response code: %w", err)
		}
	}

	if payload != nil {
		if _, err := r.encoder.EncodeWithMaxLength(stream, WrapSSZObject(payload)); err != nil {
			return fmt.Errorf("failed to write response: %w", err)
		}
	}

	if err := stream.CloseWrite(); err != nil {
		return fmt.Errorf("failed to close writing side of stream: %w", err)
	}

	return nil
}

// ReadResponseBytes reads a response from the stream and returns the raw bytes payload.
// This allows the caller to handle marshalling themselves instead of depending on common.SSZObj.
func (r *ReqResp) ReadResponseBytes(ctx context.Context, stream network.Stream) ([]byte, error) {
	if err := stream.SetReadDeadline(time.Now().Add(r.config.ReadTimeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	code := make([]byte, 1)
	if _, err := io.ReadFull(stream, code); err != nil {
		return nil, fmt.Errorf("failed to read response code: %w", err)
	}

	// 0 -> success, otherwise -> error
	if int(code[0]) != 0 {
		errData, err := io.ReadAll(stream)
		if err != nil {
			return nil, fmt.Errorf("failed to read error data (code %d): %w", int(code[0]), err)
		}

		return nil, fmt.Errorf("got error response (%d): %s", int(code[0]), string(errData))
	}

	// Read the varint length prefix
	var length uint64
	buf := make([]byte, 1)
	shift := uint(0)
	for {
		if _, err := io.ReadFull(stream, buf); err != nil {
			return nil, fmt.Errorf("failed to read varint: %w", err)
		}

		length |= uint64(buf[0]&0x7F) << shift
		if buf[0]&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			return nil, fmt.Errorf("varint overflows uint64")
		}
	}

	// Sanity check the length
	if length == 0 {
		return nil, fmt.Errorf("zero length response")
	}
	if length > 10_000_000 { // 10MB max
		return nil, fmt.Errorf("response too large: %d bytes", length)
	}

	// Read the exact amount of snappy-compressed data
	compressedData := make([]byte, length)
	if _, err := io.ReadFull(stream, compressedData); err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	// Decompress the snappy data using frame decoder
	decompressedData, err := snappy.Decode(nil, compressedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress snappy data: %w", err)
	}

	if err := stream.CloseRead(); err != nil {
		return nil, fmt.Errorf("failed to close read stream: %w", err)
	}

	return decompressedData, nil
}

// WriteResponseBytes writes a response to the stream using raw bytes payload.
// This allows the caller to handle marshalling themselves instead of depending on common.SSZObj.
func (r *ReqResp) WriteResponseBytes(ctx context.Context, stream network.Stream, payloadBytes []byte, errMsg error) error {
	if err := stream.SetWriteDeadline(time.Now().Add(r.config.WriteTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	if errMsg != nil {
		if _, err := stream.Write([]byte{0x01}); err != nil {
			return fmt.Errorf("failed to write error response code: %w", err)
		}
	} else {
		if _, err := stream.Write([]byte{0x00}); err != nil {
			return fmt.Errorf("failed to write success response code: %w", err)
		}
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
			return fmt.Errorf("failed to write compressed response: %w", err)
		}
	}

	if err := stream.CloseWrite(); err != nil {
		return fmt.Errorf("failed to close writing side of stream: %w", err)
	}

	return nil
}
