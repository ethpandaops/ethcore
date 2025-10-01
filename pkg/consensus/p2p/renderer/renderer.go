package renderer

import (
	"encoding/hex"
	"fmt"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ssz "github.com/prysmaticlabs/fastssz"

	"github.com/ethpandaops/ethcore/pkg/consensus/p2p/events"
)

// DataStreamRenderer is an interface to support rendering a data-stream message into a destination.
type DataStreamRenderer interface {
	RenderPayload(evt *events.TraceEvent, msg *pubsub.Message, dst ssz.Unmarshaler) (*events.TraceEvent, error)
}

// FullOutput is a renderer for full output.
type FullOutput struct {
	encoder encoder.NetworkEncoding
}

// NewFullOutput creates a new instance of FullOutput.
func NewFullOutput(enc encoder.NetworkEncoding) DataStreamRenderer {
	return &FullOutput{encoder: enc}
}

// RenderPayload renders message into the destination.
func (t *FullOutput) RenderPayload(evt *events.TraceEvent, msg *pubsub.Message, dst ssz.Unmarshaler) (*events.TraceEvent, error) {
	if t.encoder == nil {
		return nil, fmt.Errorf("no network encoding provided to raw output renderer")
	}

	if err := t.encoder.DecodeGossip(msg.Data, dst); err != nil {
		return nil, fmt.Errorf("decode gossip message: %w", err)
	}

	// The actual rendering logic would go here, converting the decoded message
	// into the appropriate event type based on the destination type.
	// For now, we just update the event metadata.
	evt.Payload = map[string]any{
		"PeerID":  msg.ReceivedFrom.String(),
		"Topic":   msg.GetTopic(),
		"Seq":     hex.EncodeToString(msg.GetSeqno()),
		"MsgID":   hex.EncodeToString([]byte(msg.ID)),
		"MsgSize": len(msg.Data),
		"Data":    dst,
	}

	return evt, nil
}
