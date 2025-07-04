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
	BeaconBlock = mustCreateTopic[*eth.SignedBeaconBlock](BeaconBlockTopicName)

	// BeaconAggregateAndProof represents aggregated attestations.
	BeaconAggregateAndProof = mustCreateTopic[*eth.SignedAggregateAttestationAndProof](BeaconAggregateAndProofName)

	// VoluntaryExit represents voluntary validator exits.
	VoluntaryExit = mustCreateTopic[*eth.SignedVoluntaryExit](VoluntaryExitTopicName)

	// ProposerSlashing represents proposer slashing events.
	ProposerSlashing = mustCreateTopic[*eth.ProposerSlashing](ProposerSlashingTopicName)

	// AttesterSlashing represents attester slashing events.
	AttesterSlashing = mustCreateTopic[*eth.AttesterSlashing](AttesterSlashingTopicName)

	// BlsToExecutionChange represents BLS to execution address change messages.
	BlsToExecutionChange = mustCreateTopic[*eth.SignedBLSToExecutionChange](BlsToExecutionChangeTopicName)

	// SyncContributionAndProof represents sync committee contribution and proof messages.
	SyncContributionAndProof = mustCreateTopic[*eth.SignedContributionAndProof](SyncContributionAndProofTopicPattern)
)

// Subnet topic definitions.
var (
	// Attestation represents attestation subnet topics.
	Attestation = mustCreateSubnetTopic[*eth.Attestation](
		BeaconAttestationTopicPattern,
		AttestationSubnetCount,
	)

	// SyncCommittee represents sync committee subnet topics.
	SyncCommittee = mustCreateSubnetTopic[*eth.SyncCommitteeMessage]( //nolint:staticcheck // Using deprecated prysm type for compatibility
		SyncCommitteeTopicPattern,
		SyncCommitteeSubnetCount,
	)
)

// WithFork is a helper function to add fork digest to a topic.
// It wraps the v1 Topic's WithForkDigest method for convenience.
func WithFork[T any](topic *v1.Topic[T], forkDigest [4]byte) *v1.Topic[T] {
	return topic.WithForkDigest(forkDigest)
}

// mustCreateTopic creates a new topic and panics if there's an error.
// This is used for compile-time topic definitions where we control the inputs.
func mustCreateTopic[T any](name string) *v1.Topic[T] {
	topic, err := v1.NewTopic[T](name)
	if err != nil {
		panic(fmt.Sprintf("failed to create topic %s: %v", name, err))
	}

	return topic
}

// mustCreateSubnetTopic creates a new subnet topic and panics if there's an error.
// This is used for compile-time topic definitions where we control the inputs.
func mustCreateSubnetTopic[T any](pattern string, maxSubnets uint64) *v1.SubnetTopic[T] {
	topic, err := v1.NewSubnetTopic[T](pattern, maxSubnets)
	if err != nil {
		panic(fmt.Sprintf("failed to create subnet topic %s: %v", pattern, err))
	}

	return topic
}
