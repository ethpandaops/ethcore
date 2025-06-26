package p2p

import (
	"context"
	"fmt"
	"io"
	"time"

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
