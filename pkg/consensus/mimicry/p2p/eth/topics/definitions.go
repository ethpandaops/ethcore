package topics

import (
	"fmt"

	eth "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
)

// Topic name constants for Ethereum consensus layer gossipsub topics.
const (
	// Regular topics (non-subnet).
	BeaconBlockTopicName          = "beacon_block"
	BeaconAggregateAndProofName   = "beacon_aggregate_and_proof"
	VoluntaryExitTopicName        = "voluntary_exit"
	ProposerSlashingTopicName     = "proposer_slashing"
	AttesterSlashingTopicName     = "attester_slashing"
	BlsToExecutionChangeTopicName = "bls_to_execution_change"

	// Subnet topic patterns.
	BeaconAttestationTopicPattern        = "beacon_attestation_%d"
	SyncCommitteeTopicPattern            = "sync_committee_%d"
	SyncContributionAndProofTopicPattern = "sync_committee_contribution_and_proof"

	// Maximum number of subnets.
	AttestationSubnetCount   = 64
	SyncCommitteeSubnetCount = 4
)

// Topic definitions for regular (non-subnet) topics.
var (
	// BeaconBlock represents signed beacon blocks broadcasted to all peers.
	BeaconBlock = mustCreateTopic[*eth.SignedBeaconBlock](
		BeaconBlockTopicName,
		nil, // Encoder will be set by the application
	)

	// BeaconAggregateAndProof represents aggregated attestations.
	BeaconAggregateAndProof = mustCreateTopic[*eth.SignedAggregateAttestationAndProof](
		BeaconAggregateAndProofName,
		nil, // Encoder will be set by the application
	)

	// VoluntaryExit represents voluntary validator exits.
	VoluntaryExit = mustCreateTopic[*eth.SignedVoluntaryExit](
		VoluntaryExitTopicName,
		nil, // Encoder will be set by the application
	)

	// ProposerSlashing represents proposer slashing events.
	ProposerSlashing = mustCreateTopic[*eth.ProposerSlashing](
		ProposerSlashingTopicName,
		nil, // Encoder will be set by the application
	)

	// AttesterSlashing represents attester slashing events.
	AttesterSlashing = mustCreateTopic[*eth.AttesterSlashing](
		AttesterSlashingTopicName,
		nil, // Encoder will be set by the application
	)

	// BlsToExecutionChange represents BLS to execution address change messages.
	BlsToExecutionChange = mustCreateTopic[*eth.SignedBLSToExecutionChange](
		BlsToExecutionChangeTopicName,
		nil, // Encoder will be set by the application
	)

	// SyncContributionAndProof represents sync committee contribution and proof messages.
	SyncContributionAndProof = mustCreateTopic[*eth.SignedContributionAndProof](
		SyncContributionAndProofTopicPattern,
		nil, // Encoder will be set by the application
	)
)

// Subnet topic definitions.
var (
	// Attestation represents attestation subnet topics.
	Attestation = mustCreateSubnetTopic[*eth.Attestation](
		BeaconAttestationTopicPattern,
		AttestationSubnetCount,
		nil, // Encoder will be set by the application
	)

	// SyncCommittee represents sync committee subnet topics.
	SyncCommittee = mustCreateSubnetTopic[*eth.SyncCommitteeMessage](
		SyncCommitteeTopicPattern,
		SyncCommitteeSubnetCount,
		nil, // Encoder will be set by the application
	)
)

// WithFork is a helper function to add fork digest to a topic.
// It wraps the v1 Topic's WithForkDigest method for convenience.
func WithFork[T any](topic *v1.Topic[T], forkDigest [4]byte) *v1.Topic[T] {
	return topic.WithForkDigest(forkDigest)
}

// mustCreateTopic creates a new topic and panics if there's an error.
// This is used for compile-time topic definitions where we control the inputs.
func mustCreateTopic[T any](name string, encoder v1.Encoder[T]) *v1.Topic[T] {
	// During initialization, encoder can be nil as it will be set later by the application
	if encoder == nil {
		// Create a placeholder encoder that will be replaced
		encoder = &placeholderEncoder[T]{}
	}

	topic, err := v1.NewTopic[T](name, encoder)
	if err != nil {
		panic(fmt.Sprintf("failed to create topic %s: %v", name, err))
	}
	return topic
}

// mustCreateSubnetTopic creates a new subnet topic and panics if there's an error.
// This is used for compile-time topic definitions where we control the inputs.
func mustCreateSubnetTopic[T any](pattern string, maxSubnets uint64, encoder v1.Encoder[T]) *v1.SubnetTopic[T] {
	// During initialization, encoder can be nil as it will be set later by the application
	if encoder == nil {
		// Create a placeholder encoder that will be replaced
		encoder = &placeholderEncoder[T]{}
	}

	topic, err := v1.NewSubnetTopic[T](pattern, maxSubnets, encoder)
	if err != nil {
		panic(fmt.Sprintf("failed to create subnet topic %s: %v", pattern, err))
	}
	return topic
}

// placeholderEncoder is a temporary encoder used during topic initialization.
// It should be replaced with a real encoder before use.
type placeholderEncoder[T any] struct{}

func (p *placeholderEncoder[T]) Encode(msg T) ([]byte, error) {
	return nil, fmt.Errorf("placeholder encoder: real encoder must be set before use")
}

func (p *placeholderEncoder[T]) Decode(data []byte) (T, error) {
	var zero T
	return zero, fmt.Errorf("placeholder encoder: real encoder must be set before use")
}

// CreateTopicWithEncoder creates a new topic with the specified encoder.
// This is useful when you need to create topics dynamically with specific encoders.
func CreateTopicWithEncoder[T any](baseTopic *v1.Topic[T], encoder v1.Encoder[T]) (*v1.Topic[T], error) {
	return v1.NewTopic[T](baseTopic.Name(), encoder)
}

// CreateSubnetTopicWithEncoder creates a new subnet topic with the specified encoder.
// This is useful when you need to create subnet topics dynamically with specific encoders.
func CreateSubnetTopicWithEncoder[T any](pattern string, maxSubnets uint64, encoder v1.Encoder[T]) (*v1.SubnetTopic[T], error) {
	return v1.NewSubnetTopic[T](pattern, maxSubnets, encoder)
}
