package topics_test

import (
	"testing"

	eth "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/topics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicDefinitions(t *testing.T) {
	t.Run("regular topics", func(t *testing.T) {
		// Test that all regular topics are properly defined
		assert.Equal(t, "beacon_block", topics.BeaconBlock.Name())
		assert.Equal(t, "beacon_aggregate_and_proof", topics.BeaconAggregateAndProof.Name())
		assert.Equal(t, "voluntary_exit", topics.VoluntaryExit.Name())
		assert.Equal(t, "proposer_slashing", topics.ProposerSlashing.Name())
		assert.Equal(t, "attester_slashing", topics.AttesterSlashing.Name())
		assert.Equal(t, "bls_to_execution_change", topics.BlsToExecutionChange.Name())
		assert.Equal(t, "sync_committee_contribution_and_proof", topics.SyncContributionAndProof.Name())
	})

	t.Run("subnet topics", func(t *testing.T) {
		// Test attestation subnet topic
		assert.Equal(t, uint64(64), topics.Attestation.MaxSubnets())

		// Test sync committee subnet topic
		assert.Equal(t, uint64(4), topics.SyncCommittee.MaxSubnets())
	})
}

func TestWithFork(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	t.Run("regular topic with fork", func(t *testing.T) {
		topicWithFork := topics.WithFork(topics.BeaconBlock, forkDigest)
		assert.Equal(t, "/eth2/01020304/beacon_block/ssz_snappy", topicWithFork.Name())
	})

	t.Run("subnet topic with fork", func(t *testing.T) {
		// Get a specific subnet topic
		subnet0, err := topics.Attestation.TopicForSubnet(0, forkDigest)
		require.NoError(t, err)
		assert.Equal(t, "/eth2/01020304/beacon_attestation_0/ssz_snappy", subnet0.Name())

		subnet63, err := topics.Attestation.TopicForSubnet(63, forkDigest)
		require.NoError(t, err)
		assert.Equal(t, "/eth2/01020304/beacon_attestation_63/ssz_snappy", subnet63.Name())
	})
}

func TestSubnetTopicParsing(t *testing.T) {
	forkDigest := [4]byte{0xaa, 0xbb, 0xcc, 0xdd}

	t.Run("parse attestation subnet", func(t *testing.T) {
		// Create a topic for subnet 42
		topic, err := topics.Attestation.TopicForSubnet(42, forkDigest)
		require.NoError(t, err)

		// Parse the subnet from the full topic name
		subnet, err := topics.Attestation.ParseSubnet(topic.Name())
		require.NoError(t, err)
		assert.Equal(t, uint64(42), subnet)

		// Parse from base topic name without fork digest
		subnet, err = topics.Attestation.ParseSubnet("beacon_attestation_15")
		require.NoError(t, err)
		assert.Equal(t, uint64(15), subnet)
	})

	t.Run("parse sync committee subnet", func(t *testing.T) {
		// Create a topic for subnet 2
		topic, err := topics.SyncCommittee.TopicForSubnet(2, forkDigest)
		require.NoError(t, err)

		// Parse the subnet from the full topic name
		subnet, err := topics.SyncCommittee.ParseSubnet(topic.Name())
		require.NoError(t, err)
		assert.Equal(t, uint64(2), subnet)
	})

	t.Run("invalid subnet parsing", func(t *testing.T) {
		// Try to parse a non-subnet topic
		_, err := topics.Attestation.ParseSubnet("beacon_block")
		assert.Error(t, err)

		// Try to parse invalid subnet number
		_, err = topics.Attestation.ParseSubnet("beacon_attestation_invalid")
		assert.Error(t, err)

		// Try to parse out-of-range subnet
		_, err = topics.Attestation.ParseSubnet("beacon_attestation_100")
		assert.Error(t, err)
	})
}

func TestEncoderCompatibility(t *testing.T) {
	t.Run("verify SSZ encoder creation", func(t *testing.T) {
		// Verify we can create encoders for handler configuration
		blockEncoder := topics.NewSSZSnappyEncoder[*eth.SignedBeaconBlock]()
		assert.NotNil(t, blockEncoder)

		attestationEncoder := topics.NewSSZSnappyEncoder[*eth.Attestation]()
		assert.NotNil(t, attestationEncoder)

		// Verify topics exist without encoders
		assert.Equal(t, "beacon_block", topics.BeaconBlock.Name())
		assert.Equal(t, uint64(64), topics.Attestation.MaxSubnets())
	})

	t.Run("verify encoder with max length", func(t *testing.T) {
		// Create encoder with max length for handler configuration
		encoder := topics.NewSSZSnappyEncoderWithMaxLen[*eth.SignedBeaconBlock](10 * 1024 * 1024)
		assert.NotNil(t, encoder)
	})
}

func TestTopicConstants(t *testing.T) {
	// Verify all topic name constants match expected values
	assert.Equal(t, "beacon_block", topics.BeaconBlockTopicName)
	assert.Equal(t, "beacon_aggregate_and_proof", topics.BeaconAggregateAndProofName)
	assert.Equal(t, "voluntary_exit", topics.VoluntaryExitTopicName)
	assert.Equal(t, "proposer_slashing", topics.ProposerSlashingTopicName)
	assert.Equal(t, "attester_slashing", topics.AttesterSlashingTopicName)
	assert.Equal(t, "bls_to_execution_change", topics.BlsToExecutionChangeTopicName)
	assert.Equal(t, "beacon_attestation_%d", topics.BeaconAttestationTopicPattern)
	assert.Equal(t, "sync_committee_%d", topics.SyncCommitteeTopicPattern)
	assert.Equal(t, "sync_committee_contribution_and_proof", topics.SyncContributionAndProofTopicPattern)
	assert.Equal(t, 64, topics.AttestationSubnetCount)
	assert.Equal(t, 4, topics.SyncCommitteeSubnetCount)
}
