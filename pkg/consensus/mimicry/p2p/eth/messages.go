package eth

import (
	"context"

	pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Typed message handlers for Ethereum consensus layer messages
type (
	// BeaconBlockHandler processes beacon block messages
	BeaconBlockHandler func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error

	// BeaconAggregateAndProofHandler processes aggregate attestation messages
	BeaconAggregateAndProofHandler func(ctx context.Context, msg *pb.AggregateAttestationAndProof, from peer.ID) error

	// AttestationHandler processes attestation messages for a specific subnet
	AttestationHandler func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error

	// SyncCommitteeHandler processes sync committee messages for a specific subnet
	SyncCommitteeHandler func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64, from peer.ID) error //nolint:staticcheck // deprecated but still functional

	// SyncContributionAndProofHandler processes sync committee contribution messages
	SyncContributionAndProofHandler func(ctx context.Context, msg *pb.SignedContributionAndProof, from peer.ID) error

	// VoluntaryExitHandler processes voluntary exit messages
	VoluntaryExitHandler func(ctx context.Context, msg *pb.SignedVoluntaryExit, from peer.ID) error

	// ProposerSlashingHandler processes proposer slashing messages
	ProposerSlashingHandler func(ctx context.Context, msg *pb.ProposerSlashing, from peer.ID) error

	// AttesterSlashingHandler processes attester slashing messages
	AttesterSlashingHandler func(ctx context.Context, msg *pb.AttesterSlashing, from peer.ID) error

	// BlsToExecutionChangeHandler processes BLS to execution change messages
	BlsToExecutionChangeHandler func(ctx context.Context, msg *pb.SignedBLSToExecutionChange, from peer.ID) error
)

// Typed validators for Ethereum consensus layer messages
type (
	// BeaconBlockValidator validates beacon block messages
	BeaconBlockValidator func(block *pb.SignedBeaconBlock) error

	// AttestationValidator validates attestation messages for a specific subnet
	AttestationValidator func(att *pb.Attestation, subnet uint64) error

	// SyncCommitteeValidator validates sync committee messages for a specific subnet
	SyncCommitteeValidator func(msg *pb.SyncCommitteeMessage, subnet uint64) error //nolint:staticcheck // deprecated but still functional

	// AggregateAndProofValidator validates aggregate and proof messages
	AggregateAndProofValidator func(msg *pb.AggregateAttestationAndProof) error

	// VoluntaryExitValidator validates voluntary exit messages
	VoluntaryExitValidator func(msg *pb.SignedVoluntaryExit) error

	// ProposerSlashingValidator validates proposer slashing messages
	ProposerSlashingValidator func(msg *pb.ProposerSlashing) error

	// AttesterSlashingValidator validates attester slashing messages
	AttesterSlashingValidator func(msg *pb.AttesterSlashing) error
)

// Handler collection for bulk subscription to global topics
type GlobalTopicHandlers struct {
	BeaconBlock              BeaconBlockHandler
	BeaconAggregateAndProof  BeaconAggregateAndProofHandler
	VoluntaryExit            VoluntaryExitHandler
	ProposerSlashing         ProposerSlashingHandler
	AttesterSlashing         AttesterSlashingHandler
	SyncContributionAndProof SyncContributionAndProofHandler
	BlsToExecutionChange     BlsToExecutionChangeHandler
}

// Validator collection for bulk registration of global topic validators
type GlobalTopicValidators struct {
	BeaconBlock              BeaconBlockValidator
	BeaconAggregateAndProof  AggregateAndProofValidator
	VoluntaryExit            VoluntaryExitValidator
	ProposerSlashing         ProposerSlashingValidator
	AttesterSlashing         AttesterSlashingValidator
	SyncContributionAndProof func(*pb.SignedContributionAndProof) error
	BlsToExecutionChange     func(*pb.SignedBLSToExecutionChange) error
}

// Event wrapper functions that convert generic events to typed events

// WrapMessageReceivedAsBeaconBlock wraps a generic message received callback as a beacon block callback
func WrapMessageReceivedAsBeaconBlock(callback pubsub.MessageReceivedCallback, handler BeaconBlockHandler) pubsub.MessageReceivedCallback {
	return func(topic string, from peer.ID) {
		callback(topic, from)
	}
}

// WrapMessageReceivedAsAttestation wraps a generic message received callback as an attestation callback
func WrapMessageReceivedAsAttestation(callback pubsub.MessageReceivedCallback, handler AttestationHandler) pubsub.MessageReceivedCallback {
	return func(topic string, from peer.ID) {
		callback(topic, from)
	}
}

// WrapValidationFailedAsBeaconBlock wraps a generic validation failed callback for beacon blocks
func WrapValidationFailedAsBeaconBlock(callback pubsub.ValidationFailedCallback) func(block *pb.SignedBeaconBlock, from peer.ID, err error) {
	return func(block *pb.SignedBeaconBlock, from peer.ID, err error) {
		// Extract topic from block context if needed
		topic := BeaconBlockTopicName
		callback(topic, from, err)
	}
}

// WrapValidationFailedAsAttestation wraps a generic validation failed callback for attestations
func WrapValidationFailedAsAttestation(callback pubsub.ValidationFailedCallback, subnet uint64) func(att *pb.Attestation, from peer.ID, err error) {
	return func(att *pb.Attestation, from peer.ID, err error) {
		// Extract topic from attestation context
		topic := AttestationSubnetTopicTemplate + string(rune(subnet))
		callback(topic, from, err)
	}
}

// Message type detection helpers

// IsBeaconBlockTopic checks if a topic is a beacon block topic
func IsBeaconBlockTopic(topicName string) bool {
	return topicName == BeaconBlockTopicName
}

// IsAggregateAndProofTopic checks if a topic is an aggregate and proof topic
func IsAggregateAndProofTopic(topicName string) bool {
	return topicName == BeaconAggregateAndProofTopicName
}

// IsVoluntaryExitTopic checks if a topic is a voluntary exit topic
func IsVoluntaryExitTopic(topicName string) bool {
	return topicName == VoluntaryExitTopicName
}

// IsProposerSlashingTopic checks if a topic is a proposer slashing topic
func IsProposerSlashingTopic(topicName string) bool {
	return topicName == ProposerSlashingTopicName
}

// IsAttesterSlashingTopic checks if a topic is an attester slashing topic
func IsAttesterSlashingTopic(topicName string) bool {
	return topicName == AttesterSlashingTopicName
}

// IsSyncContributionAndProofTopic checks if a topic is a sync contribution and proof topic
func IsSyncContributionAndProofTopic(topicName string) bool {
	return topicName == SyncContributionAndProofTopicName
}

// IsBlsToExecutionChangeTopic checks if a topic is a BLS to execution change topic
func IsBlsToExecutionChangeTopic(topicName string) bool {
	return topicName == BlsToExecutionChangeTopicName
}

// GetMessageType returns the expected message type for a given topic name
func GetMessageType(topicName string) string {
	switch topicName {
	case BeaconBlockTopicName:
		return "SignedBeaconBlock"
	case BeaconAggregateAndProofTopicName:
		return "SignedAggregateAndProof"
	case VoluntaryExitTopicName:
		return "SignedVoluntaryExit"
	case ProposerSlashingTopicName:
		return "ProposerSlashing"
	case AttesterSlashingTopicName:
		return "AttesterSlashing"
	case SyncContributionAndProofTopicName:
		return "SignedContributionAndProof"
	case BlsToExecutionChangeTopicName:
		return "SignedBLSToExecutionChange"
	default:
		if isAttestation, _ := IsAttestationTopic(topicName); isAttestation {
			return "Attestation"
		}
		if isSyncCommittee, _ := IsSyncCommitteeTopic(topicName); isSyncCommittee {
			return "SyncCommitteeMessage"
		}
		return "Unknown"
	}
}
