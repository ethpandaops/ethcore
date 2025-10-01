package gossipsub

import (
	ethtypes "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/p2p/events"
)

// TraceEventAttestation represents an attestation event.
type TraceEventAttestation struct {
	events.TraceEventPayloadMetaData
	Attestation *ethtypes.Attestation
}

// TraceEventAttestationElectra represents an Electra attestation event.
type TraceEventAttestationElectra struct {
	events.TraceEventPayloadMetaData
	AttestationElectra *ethtypes.AttestationElectra
}

// TraceEventSingleAttestation represents a single attestation event.
type TraceEventSingleAttestation struct {
	events.TraceEventPayloadMetaData
	SingleAttestation *ethtypes.SingleAttestation
}

// TraceEventSignedAggregateAttestationAndProof represents a signed aggregate attestation and proof event.
type TraceEventSignedAggregateAttestationAndProof struct {
	events.TraceEventPayloadMetaData
	SignedAggregateAttestationAndProof *ethtypes.SignedAggregateAttestationAndProof
}

// TraceEventSignedAggregateAttestationAndProofElectra represents an Electra signed aggregate attestation and proof event.
type TraceEventSignedAggregateAttestationAndProofElectra struct {
	events.TraceEventPayloadMetaData
	SignedAggregateAttestationAndProofElectra *ethtypes.SignedAggregateAttestationAndProofElectra
}

// TraceEventSignedContributionAndProof represents a signed contribution and proof event.
type TraceEventSignedContributionAndProof struct {
	events.TraceEventPayloadMetaData
	SignedContributionAndProof *ethtypes.SignedContributionAndProof
}

// TraceEventVoluntaryExit represents a voluntary exit event.
type TraceEventVoluntaryExit struct {
	events.TraceEventPayloadMetaData
	VoluntaryExit *ethtypes.VoluntaryExit
}

// TraceEventSyncCommitteeMessage represents a sync committee message event.
type TraceEventSyncCommitteeMessage struct {
	events.TraceEventPayloadMetaData
	SyncCommitteeMessage *ethtypes.SyncCommitteeMessage //nolint:staticcheck // SA1019 gRPC API deprecated but still supported until v8 (2026)
}

// TraceEventBLSToExecutionChange represents a BLS to execution change event.
type TraceEventBLSToExecutionChange struct {
	events.TraceEventPayloadMetaData
	BLSToExecutionChange *ethtypes.BLSToExecutionChange
}

// TraceEventProposerSlashing represents a proposer slashing event.
type TraceEventProposerSlashing struct {
	events.TraceEventPayloadMetaData
	ProposerSlashing *ethtypes.ProposerSlashing
}

// TraceEventAttesterSlashing represents an attester slashing event.
type TraceEventAttesterSlashing struct {
	events.TraceEventPayloadMetaData
	AttesterSlashing *ethtypes.AttesterSlashing
}
