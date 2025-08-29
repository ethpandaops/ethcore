package events

import (
	eth1alpha1 "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
)

// Phase0 Events.

// TraceEventPhase0Block represents a Phase0 beacon block event.
type TraceEventPhase0Block struct {
	Slot      uint64                        `json:"slot"`
	Block     *eth1alpha1.SignedBeaconBlock `json:"block"`
	Signature string                        `json:"signature"`
}

// Altair Events.

// TraceEventAltairBlock represents an Altair beacon block event.
type TraceEventAltairBlock struct {
	Slot      uint64                              `json:"slot"`
	Block     *eth1alpha1.SignedBeaconBlockAltair `json:"block"`
	Signature string                              `json:"signature"`
}

// Bellatrix Events.

// TraceEventBellatrixBlock represents a Bellatrix beacon block event.
type TraceEventBellatrixBlock struct {
	Slot      uint64                                 `json:"slot"`
	Block     *eth1alpha1.SignedBeaconBlockBellatrix `json:"block"`
	Signature string                                 `json:"signature"`
}

// Capella Events.

// TraceEventCapellaBlock represents a Capella beacon block event.
type TraceEventCapellaBlock struct {
	Slot      uint64                               `json:"slot"`
	Block     *eth1alpha1.SignedBeaconBlockCapella `json:"block"`
	Signature string                               `json:"signature"`
}

// Deneb Events.

// TraceEventDenebBlock represents a Deneb beacon block event.
type TraceEventDenebBlock struct {
	Slot      uint64                             `json:"slot"`
	Block     *eth1alpha1.SignedBeaconBlockDeneb `json:"block"`
	Signature string                             `json:"signature"`
}

// Electra Events.

// TraceEventElectraBlock represents an Electra beacon block event.
type TraceEventElectraBlock struct {
	Slot      uint64                               `json:"slot"`
	Block     *eth1alpha1.SignedBeaconBlockElectra `json:"block"`
	Signature string                               `json:"signature"`
}

// Fulu Events.

// TraceEventFuluBlock represents a Fulu beacon block event.
type TraceEventFuluBlock struct {
	Slot      uint64                            `json:"slot"`
	Block     *eth1alpha1.SignedBeaconBlockFulu `json:"block"`
	Signature string                            `json:"signature"`
}

// Attestation Events.

// TraceEventAttestation represents a beacon attestation event.
type TraceEventAttestation struct {
	AggregationBits string                      `json:"aggregationBits"`
	Data            *eth1alpha1.AttestationData `json:"data"`
	Signature       string                      `json:"signature"`
}

// TraceEventAttestationElectra represents an Electra attestation event.
type TraceEventAttestationElectra struct {
	AggregationBits string                      `json:"aggregationBits"`
	Data            *eth1alpha1.AttestationData `json:"data"`
	Signature       string                      `json:"signature"`
	CommitteeBits   string                      `json:"committeeBits"`
}

// TraceEventSingleAttestation represents a single attestation event.
type TraceEventSingleAttestation struct {
	CommitteeID  uint64                      `json:"committeeId"`
	AttesterSlot uint64                      `json:"attesterSlot"`
	Data         *eth1alpha1.AttestationData `json:"data"`
	Signature    string                      `json:"signature"`
}

// Aggregate Attestation Events.

// TraceEventSignedAggregateAttestationAndProof represents a signed aggregate attestation event.
type TraceEventSignedAggregateAttestationAndProof struct {
	Message   *eth1alpha1.AggregateAttestationAndProof `json:"message"`
	Signature string                                   `json:"signature"`
}

// TraceEventSignedAggregateAttestationAndProofElectra represents an Electra signed aggregate attestation event.
type TraceEventSignedAggregateAttestationAndProofElectra struct {
	Message   *eth1alpha1.AggregateAttestationAndProofElectra `json:"message"`
	Signature string                                          `json:"signature"`
}

// Sync Committee Events.

// TraceEventSyncCommitteeMessage represents a sync committee message event.
type TraceEventSyncCommitteeMessage struct {
	Slot           uint64 `json:"slot"`
	BlockRoot      string `json:"blockRoot"`
	ValidatorIndex uint64 `json:"validatorIndex"`
	Signature      string `json:"signature"`
}

// TraceEventSignedContributionAndProof represents a signed sync committee contribution event.
type TraceEventSignedContributionAndProof struct {
	Message   *eth1alpha1.ContributionAndProof `json:"message"`
	Signature string                           `json:"signature"`
}

// Slashing Events.

// TraceEventProposerSlashing represents a proposer slashing event.
type TraceEventProposerSlashing struct {
	SignedHeader1 *eth1alpha1.SignedBeaconBlockHeader `json:"signedHeader1"`
	SignedHeader2 *eth1alpha1.SignedBeaconBlockHeader `json:"signedHeader2"`
}

// TraceEventAttesterSlashing represents an attester slashing event.
type TraceEventAttesterSlashing struct {
	Attestation1 *eth1alpha1.IndexedAttestation `json:"attestation1"`
	Attestation2 *eth1alpha1.IndexedAttestation `json:"attestation2"`
}

// TraceEventAttesterSlashingElectra represents an Electra attester slashing event.
type TraceEventAttesterSlashingElectra struct {
	Attestation1 *eth1alpha1.IndexedAttestationElectra `json:"attestation1"`
	Attestation2 *eth1alpha1.IndexedAttestationElectra `json:"attestation2"`
}

// Exit Events.

// TraceEventVoluntaryExit represents a voluntary exit event.
type TraceEventVoluntaryExit struct {
	Message   *eth1alpha1.VoluntaryExit `json:"message"`
	Signature string                    `json:"signature"`
}

// BLS Events.

// TraceEventBLSToExecutionChange represents a BLS to execution change event.
type TraceEventBLSToExecutionChange struct {
	Message   *eth1alpha1.BLSToExecutionChange `json:"message"`
	Signature string                           `json:"signature"`
}

// Blob Events.

// TraceEventBlobSidecar represents a blob sidecar event.
type TraceEventBlobSidecar struct {
	BlockRoot       string `json:"blockRoot"`
	Index           uint64 `json:"index"`
	Slot            uint64 `json:"slot"`
	BlockParentRoot string `json:"blockParentRoot"`
	ProposerIndex   uint64 `json:"proposerIndex"`
	Blob            []byte `json:"blob"`
	KzgCommitment   string `json:"kzgCommitment"`
	KzgProof        string `json:"kzgProof"`
}

// Data Column Events.

// TraceEventDataColumnSidecar represents a data column sidecar event.
type TraceEventDataColumnSidecar struct {
	ColumnIndex       uint64                              `json:"columnIndex"`
	DataColumn        [][]byte                            `json:"dataColumn"`
	KzgCommitments    []string                            `json:"kzgCommitments"`
	KzgProof          []string                            `json:"kzgProof"`
	SignedBlockHeader *eth1alpha1.SignedBeaconBlockHeader `json:"signedBlockHeader"`
}
