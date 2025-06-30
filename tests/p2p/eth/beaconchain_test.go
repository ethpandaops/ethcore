package eth_test

import (
	"testing"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewBeaconchainGossipsub(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	enc := encoder.SszNetworkEncoder{}
	log := logrus.New()

	bg := eth.NewBeaconchainGossipsub(
		nil, // Will be set in integration tests
		forkDigest,
		enc,
		log,
	)

	assert.NotNil(t, bg)
	// Since fields are unexported, we can only test that creation succeeded
	// The internal implementation is tested through integration tests
}

// TestProcessorTopics tests that topic functions work correctly
// Since processor types are unexported, we test the public topic functions directly.
func TestProcessorTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	t.Run("BeaconBlockTopic", func(t *testing.T) {
		topic := eth.BeaconBlockTopic(forkDigest)
		assert.Contains(t, topic, "beacon_block")
		assert.Contains(t, topic, "01020304")
	})

	t.Run("BeaconAggregateAndProofTopic", func(t *testing.T) {
		topic := eth.BeaconAggregateAndProofTopic(forkDigest)
		assert.Contains(t, topic, "beacon_aggregate_and_proof")
		assert.Contains(t, topic, "01020304")
	})

	t.Run("AttestationSubnetTopics", func(t *testing.T) {
		// Test a few attestation subnet topics
		for i := uint64(0); i < 3; i++ {
			topic := eth.AttestationSubnetTopic(forkDigest, i)
			assert.Contains(t, topic, "beacon_attestation")
			assert.Contains(t, topic, "01020304")
		}
	})

	t.Run("SyncCommitteeSubnetTopics", func(t *testing.T) {
		// Test a few sync committee subnet topics
		for i := uint64(0); i < 2; i++ {
			topic := eth.SyncCommitteeSubnetTopic(forkDigest, i)
			assert.Contains(t, topic, "sync_committee")
			assert.Contains(t, topic, "01020304")
		}
	})
}

// TestTopicGeneration verifies that topics are generated correctly.
func TestTopicGeneration(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	t.Run("BeaconBlockTopic", func(t *testing.T) {
		topic := eth.BeaconBlockTopic(forkDigest)
		assert.Contains(t, topic, "/eth2/01020304/beacon_block/ssz_snappy")
	})

	t.Run("BeaconAggregateAndProofTopic", func(t *testing.T) {
		topic := eth.BeaconAggregateAndProofTopic(forkDigest)
		assert.Contains(t, topic, "/eth2/01020304/beacon_aggregate_and_proof/ssz_snappy")
	})

	t.Run("AttestationSubnetTopic", func(t *testing.T) {
		topic := eth.AttestationSubnetTopic(forkDigest, 5)
		assert.Contains(t, topic, "/eth2/01020304/beacon_attestation_5/ssz_snappy")
	})

	t.Run("SyncCommitteeSubnetTopic", func(t *testing.T) {
		topic := eth.SyncCommitteeSubnetTopic(forkDigest, 2)
		assert.Contains(t, topic, "/eth2/01020304/sync_committee_2/ssz_snappy")
	})

	t.Run("VoluntaryExitTopic", func(t *testing.T) {
		topic := eth.VoluntaryExitTopic(forkDigest[:])
		assert.Contains(t, topic, "/eth2/01020304/voluntary_exit/ssz_snappy")
	})

	t.Run("ProposerSlashingTopic", func(t *testing.T) {
		topic := eth.ProposerSlashingTopic(forkDigest[:])
		assert.Contains(t, topic, "/eth2/01020304/proposer_slashing/ssz_snappy")
	})

	t.Run("AttesterSlashingTopic", func(t *testing.T) {
		topic := eth.AttesterSlashingTopic(forkDigest)
		assert.Contains(t, topic, "/eth2/01020304/attester_slashing/ssz_snappy")
	})

	t.Run("SyncContributionAndProofTopic", func(t *testing.T) {
		topic := eth.SyncContributionAndProofTopic(forkDigest)
		assert.Contains(t, topic, "/eth2/01020304/sync_committee_contribution_and_proof/ssz_snappy")
	})

	t.Run("BlsToExecutionChangeTopic", func(t *testing.T) {
		topic := eth.BlsToExecutionChangeTopic(forkDigest)
		assert.Contains(t, topic, "/eth2/01020304/bls_to_execution_change/ssz_snappy")
	})
}
