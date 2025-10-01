package renderer

import (
	"encoding/hex"
	"fmt"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	ethtypes "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ssz "github.com/prysmaticlabs/fastssz"

	"github.com/ethpandaops/ethcore/pkg/consensus/p2p/events"
	"github.com/ethpandaops/ethcore/pkg/consensus/p2p/events/gossipsub"
)

// MessageRenderer is an interface for rendering pubsub messages into typed consensus events.
type MessageRenderer interface {
	RenderPayload(evt *events.TraceEvent, msg *pubsub.Message, dst ssz.Unmarshaler) (*events.TraceEvent, error)
}

// FullOutput is a renderer for full output.
type FullOutput struct {
	encoder encoder.NetworkEncoding
}

// NewFullOutput creates a new instance of FullOutput.
func NewFullOutput(enc encoder.NetworkEncoding) MessageRenderer {
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

	var (
		err     error
		payload any
	)

	switch d := dst.(type) {
	case *ethtypes.SignedBeaconBlock:
		payload, err = t.renderPhase0Block(msg, d)
	case *ethtypes.SignedBeaconBlockAltair:
		payload, err = t.renderAltairBlock(msg, d)
	case *ethtypes.SignedBeaconBlockBellatrix:
		payload, err = t.renderBellatrixBlock(msg, d)
	case *ethtypes.SignedBeaconBlockCapella:
		payload, err = t.renderCapellaBlock(msg, d)
	case *ethtypes.SignedBeaconBlockDeneb:
		payload, err = t.renderDenebBlock(msg, d)
	case *ethtypes.SignedBeaconBlockElectra:
		payload, err = t.renderElectraBlock(msg, d)
	case *ethtypes.SignedBeaconBlockFulu:
		payload, err = t.renderFuluBlock(msg, d)
	case *ethtypes.Attestation:
		payload, err = t.renderAttestation(msg, d)
	case *ethtypes.AttestationElectra:
		payload, err = t.renderAttestationElectra(msg, d)
	case *ethtypes.SingleAttestation:
		payload, err = t.renderSingleAttestation(msg, d)
	case *ethtypes.SignedAggregateAttestationAndProof:
		payload, err = t.renderAggregateAttestationAndProof(msg, d)
	case *ethtypes.SignedAggregateAttestationAndProofElectra:
		payload, err = t.renderAggregateAttestationAndProofElectra(msg, d)
	case *ethtypes.SignedContributionAndProof:
		payload, err = t.renderContributionAndProof(msg, d)
	case *ethtypes.VoluntaryExit:
		payload, err = t.renderVoluntaryExit(msg, d)
	case *ethtypes.SyncCommitteeMessage: //nolint:staticcheck // SA1019 gRPC API deprecated but still supported until v8 (2026)
		payload, err = t.renderSyncCommitteeMessage(msg, d)
	case *ethtypes.BLSToExecutionChange:
		payload, err = t.renderBLSToExecutionChange(msg, d)
	case *ethtypes.BlobSidecar:
		payload, err = t.renderBlobSidecar(msg, d)
	case *ethtypes.DataColumnSidecar:
		payload, err = t.renderDataColumnSidecar(msg, d)
	case *ethtypes.ProposerSlashing:
		payload, err = t.renderProposerSlashing(msg, d)
	case *ethtypes.AttesterSlashing:
		payload, err = t.renderAttesterSlashing(msg, d)
	default:
		return nil, fmt.Errorf("unsupported message type: %T", dst)
	}

	if err != nil {
		return nil, err
	}

	evt.Payload = payload

	return evt, nil
}

func (t *FullOutput) renderPhase0Block(
	msg *pubsub.Message,
	block *ethtypes.SignedBeaconBlock,
) (*gossipsub.TraceEventPhase0Block, error) {
	return &gossipsub.TraceEventPhase0Block{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		Block: block,
	}, nil
}

func (t *FullOutput) renderAltairBlock(
	msg *pubsub.Message,
	block *ethtypes.SignedBeaconBlockAltair,
) (*gossipsub.TraceEventAltairBlock, error) {
	return &gossipsub.TraceEventAltairBlock{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		Block: block,
	}, nil
}

func (t *FullOutput) renderBellatrixBlock(
	msg *pubsub.Message,
	block *ethtypes.SignedBeaconBlockBellatrix,
) (*gossipsub.TraceEventBellatrixBlock, error) {
	return &gossipsub.TraceEventBellatrixBlock{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		Block: block,
	}, nil
}

func (t *FullOutput) renderCapellaBlock(
	msg *pubsub.Message,
	block *ethtypes.SignedBeaconBlockCapella,
) (*gossipsub.TraceEventCapellaBlock, error) {
	return &gossipsub.TraceEventCapellaBlock{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		Block: block,
	}, nil
}

func (t *FullOutput) renderDenebBlock(
	msg *pubsub.Message,
	block *ethtypes.SignedBeaconBlockDeneb,
) (*gossipsub.TraceEventDenebBlock, error) {
	return &gossipsub.TraceEventDenebBlock{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		Block: block,
	}, nil
}

func (t *FullOutput) renderElectraBlock(
	msg *pubsub.Message,
	block *ethtypes.SignedBeaconBlockElectra,
) (*gossipsub.TraceEventElectraBlock, error) {
	return &gossipsub.TraceEventElectraBlock{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		Block: block,
	}, nil
}

func (t *FullOutput) renderFuluBlock(
	msg *pubsub.Message,
	block *ethtypes.SignedBeaconBlockFulu,
) (*gossipsub.TraceEventFuluBlock, error) {
	return &gossipsub.TraceEventFuluBlock{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		Block: block,
	}, nil
}

func (t *FullOutput) renderAttestation(
	msg *pubsub.Message,
	attestation *ethtypes.Attestation,
) (*gossipsub.TraceEventAttestation, error) {
	return &gossipsub.TraceEventAttestation{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		Attestation: attestation,
	}, nil
}

func (t *FullOutput) renderAttestationElectra(
	msg *pubsub.Message,
	attestation *ethtypes.AttestationElectra,
) (*gossipsub.TraceEventAttestationElectra, error) {
	return &gossipsub.TraceEventAttestationElectra{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		AttestationElectra: attestation,
	}, nil
}

func (t *FullOutput) renderSingleAttestation(
	msg *pubsub.Message,
	attestation *ethtypes.SingleAttestation,
) (*gossipsub.TraceEventSingleAttestation, error) {
	return &gossipsub.TraceEventSingleAttestation{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		SingleAttestation: attestation,
	}, nil
}

func (t *FullOutput) renderAggregateAttestationAndProof(
	msg *pubsub.Message,
	agg *ethtypes.SignedAggregateAttestationAndProof,
) (*gossipsub.TraceEventSignedAggregateAttestationAndProof, error) {
	return &gossipsub.TraceEventSignedAggregateAttestationAndProof{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		SignedAggregateAttestationAndProof: agg,
	}, nil
}

func (t *FullOutput) renderAggregateAttestationAndProofElectra(
	msg *pubsub.Message,
	agg *ethtypes.SignedAggregateAttestationAndProofElectra,
) (*gossipsub.TraceEventSignedAggregateAttestationAndProofElectra, error) {
	return &gossipsub.TraceEventSignedAggregateAttestationAndProofElectra{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		SignedAggregateAttestationAndProofElectra: agg,
	}, nil
}

func (t *FullOutput) renderContributionAndProof(
	msg *pubsub.Message,
	cp *ethtypes.SignedContributionAndProof,
) (*gossipsub.TraceEventSignedContributionAndProof, error) {
	return &gossipsub.TraceEventSignedContributionAndProof{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		SignedContributionAndProof: cp,
	}, nil
}

func (t *FullOutput) renderVoluntaryExit(
	msg *pubsub.Message,
	ve *ethtypes.VoluntaryExit,
) (*gossipsub.TraceEventVoluntaryExit, error) {
	return &gossipsub.TraceEventVoluntaryExit{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		VoluntaryExit: ve,
	}, nil
}

func (t *FullOutput) renderSyncCommitteeMessage(
	msg *pubsub.Message,
	sc *ethtypes.SyncCommitteeMessage, //nolint:staticcheck // SA1019 gRPC API deprecated but still supported until v8 (2026)
) (*gossipsub.TraceEventSyncCommitteeMessage, error) {
	return &gossipsub.TraceEventSyncCommitteeMessage{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		SyncCommitteeMessage: sc,
	}, nil
}

func (t *FullOutput) renderBLSToExecutionChange(
	msg *pubsub.Message,
	blsec *ethtypes.BLSToExecutionChange,
) (*gossipsub.TraceEventBLSToExecutionChange, error) {
	return &gossipsub.TraceEventBLSToExecutionChange{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		BLSToExecutionChange: blsec,
	}, nil
}

func (t *FullOutput) renderBlobSidecar(
	msg *pubsub.Message,
	blob *ethtypes.BlobSidecar,
) (*gossipsub.TraceEventBlobSidecar, error) {
	return &gossipsub.TraceEventBlobSidecar{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		BlobSidecar: blob,
	}, nil
}

func (t *FullOutput) renderDataColumnSidecar(
	msg *pubsub.Message,
	sidecar *ethtypes.DataColumnSidecar,
) (*gossipsub.TraceEventDataColumnSidecar, error) {
	return &gossipsub.TraceEventDataColumnSidecar{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		DataColumnSidecar: sidecar,
	}, nil
}

func (t *FullOutput) renderProposerSlashing(
	msg *pubsub.Message,
	ps *ethtypes.ProposerSlashing,
) (*gossipsub.TraceEventProposerSlashing, error) {
	return &gossipsub.TraceEventProposerSlashing{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		ProposerSlashing: ps,
	}, nil
}

func (t *FullOutput) renderAttesterSlashing(
	msg *pubsub.Message,
	as *ethtypes.AttesterSlashing,
) (*gossipsub.TraceEventAttesterSlashing, error) {
	return &gossipsub.TraceEventAttesterSlashing{
		TraceEventPayloadMetaData: events.TraceEventPayloadMetaData{
			PeerID:  msg.ReceivedFrom.String(),
			Topic:   msg.GetTopic(),
			Seq:     msg.GetSeqno(),
			MsgID:   hex.EncodeToString([]byte(msg.ID)),
			MsgSize: len(msg.Data),
		},
		AttesterSlashing: as,
	}, nil
}
