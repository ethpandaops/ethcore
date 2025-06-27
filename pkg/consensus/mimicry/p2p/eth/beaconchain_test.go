package eth

import (
	"testing"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewBeaconchainGossipsub(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	enc := encoder.SszNetworkEncoder{}
	log := logrus.New()

	bg := NewBeaconchainGossipsub(
		nil, // Will be set in integration tests
		forkDigest,
		enc,
		log,
	)

	assert.NotNil(t, bg)
	assert.Equal(t, forkDigest, bg.forkDigest)
	assert.Equal(t, enc, bg.encoder)
	assert.NotNil(t, bg.log)
}

// TestProcessorCreation tests that processors can be created with proper configurations
func TestProcessorCreation(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	enc := encoder.SszNetworkEncoder{}
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	t.Run("BeaconBlockProcessor", func(t *testing.T) {
		processor := &beaconBlockProcessor{
			forkDigest: forkDigest,
			encoder:    enc,
			log:        log,
		}

		assert.Equal(t, BeaconBlockTopic(forkDigest), processor.Topic())
		assert.NotEmpty(t, processor.AllPossibleTopics())
	})

	t.Run("AggregateProcessor", func(t *testing.T) {
		processor := &aggregateProcessor{
			forkDigest: forkDigest,
			encoder:    enc,
			log:        log,
		}

		assert.Equal(t, BeaconAggregateAndProofTopic(forkDigest), processor.Topic())
		assert.NotEmpty(t, processor.AllPossibleTopics())
	})

	t.Run("AttestationProcessor", func(t *testing.T) {
		subnets := []uint64{0, 1, 2}
		processor := &attestationProcessor{
			forkDigest: forkDigest,
			encoder:    enc,
			subnets:    subnets,
			log:        log,
		}

		// Should return all possible attestation subnet topics
		topics := processor.AllPossibleTopics()
		assert.Equal(t, 64, len(topics)) // 64 attestation subnets
	})

	t.Run("SyncCommitteeProcessor", func(t *testing.T) {
		subnets := []uint64{0, 1}
		processor := &syncCommitteeProcessor{
			forkDigest: forkDigest,
			encoder:    enc,
			subnets:    subnets,
			log:        log,
		}

		// Should return all possible sync committee subnet topics
		topics := processor.AllPossibleTopics()
		assert.Equal(t, 4, len(topics)) // 4 sync committee subnets
	})
}

// TestTopicGeneration verifies that topics are generated correctly
func TestTopicGeneration(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	t.Run("BeaconBlockTopic", func(t *testing.T) {
		topic := BeaconBlockTopic(forkDigest)
		assert.Contains(t, topic, "/eth2/01020304/beacon_block/ssz_snappy")
	})

	t.Run("BeaconAggregateAndProofTopic", func(t *testing.T) {
		topic := BeaconAggregateAndProofTopic(forkDigest)
		assert.Contains(t, topic, "/eth2/01020304/beacon_aggregate_and_proof/ssz_snappy")
	})

	t.Run("AttestationSubnetTopic", func(t *testing.T) {
		topic := AttestationSubnetTopic(forkDigest, 5)
		assert.Contains(t, topic, "/eth2/01020304/beacon_attestation_5/ssz_snappy")
	})

	t.Run("SyncCommitteeSubnetTopic", func(t *testing.T) {
		topic := SyncCommitteeSubnetTopic(forkDigest, 2)
		assert.Contains(t, topic, "/eth2/01020304/sync_committee_2/ssz_snappy")
	})

	t.Run("VoluntaryExitTopic", func(t *testing.T) {
		topic := VoluntaryExitTopic(forkDigest[:])
		assert.Contains(t, topic, "/eth2/01020304/voluntary_exit/ssz_snappy")
	})

	t.Run("ProposerSlashingTopic", func(t *testing.T) {
		topic := ProposerSlashingTopic(forkDigest[:])
		assert.Contains(t, topic, "/eth2/01020304/proposer_slashing/ssz_snappy")
	})

	t.Run("AttesterSlashingTopic", func(t *testing.T) {
		topic := AttesterSlashingTopic(forkDigest)
		assert.Contains(t, topic, "/eth2/01020304/attester_slashing/ssz_snappy")
	})

	t.Run("SyncContributionAndProofTopic", func(t *testing.T) {
		topic := SyncContributionAndProofTopic(forkDigest)
		assert.Contains(t, topic, "/eth2/01020304/sync_committee_contribution_and_proof/ssz_snappy")
	})

	t.Run("BlsToExecutionChangeTopic", func(t *testing.T) {
		topic := BlsToExecutionChangeTopic(forkDigest)
		assert.Contains(t, topic, "/eth2/01020304/bls_to_execution_change/ssz_snappy")
	})
}
