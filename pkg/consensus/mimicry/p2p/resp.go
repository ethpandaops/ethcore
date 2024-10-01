package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

func (r *ReqResp) sendResponse(ctx context.Context, stream network.Stream, payload common.SSZObj, errMsg error) error {
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
