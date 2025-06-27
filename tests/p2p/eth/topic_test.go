package eth_test

import (
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGossipsubTopicFormat(t *testing.T) {
	// Test the format string
	assert.Equal(t, "/eth2/%x/%s/ssz_snappy", eth.GossipsubTopicFormat)
}

func TestBeaconBlockTopic(t *testing.T) {
	forkDigest := [4]byte{0x00, 0x00, 0x00, 0x01}
	expected := "/eth2/00000001/beacon_block/ssz_snappy"

	result := eth.BeaconBlockTopic(forkDigest)
	assert.Equal(t, expected, result)
}

func TestBeaconAggregateAndProofTopic(t *testing.T) {
	forkDigest := [4]byte{0x00, 0x00, 0x00, 0x02}
	expected := "/eth2/00000002/beacon_aggregate_and_proof/ssz_snappy"

	result := eth.BeaconAggregateAndProofTopic(forkDigest)
	assert.Equal(t, expected, result)
}

func TestVoluntaryExitTopic(t *testing.T) {
	forkDigest := [4]byte{0x00, 0x00, 0x00, 0x03}
	expected := "/eth2/00000003/voluntary_exit/ssz_snappy"

	result := VoluntaryExitTopic(forkDigest[:])
	assert.Equal(t, expected, result)
}

func TestProposerSlashingTopic(t *testing.T) {
	forkDigest := [4]byte{0x00, 0x00, 0x00, 0x04}
	expected := "/eth2/00000004/proposer_slashing/ssz_snappy"

	result := ProposerSlashingTopic(forkDigest[:])
	assert.Equal(t, expected, result)
}

func TestAttesterSlashingTopic(t *testing.T) {
	forkDigest := [4]byte{0x00, 0x00, 0x00, 0x05}
	expected := "/eth2/00000005/attester_slashing/ssz_snappy"

	result := AttesterSlashingTopic(forkDigest)
	assert.Equal(t, expected, result)
}

func TestSyncContributionAndProofTopic(t *testing.T) {
	forkDigest := [4]byte{0x00, 0x00, 0x00, 0x06}
	expected := "/eth2/00000006/sync_committee_contribution_and_proof/ssz_snappy"

	result := SyncContributionAndProofTopic(forkDigest)
	assert.Equal(t, expected, result)
}

func TestBlsToExecutionChangeTopic(t *testing.T) {
	forkDigest := [4]byte{0x00, 0x00, 0x00, 0x07}
	expected := "/eth2/00000007/bls_to_execution_change/ssz_snappy"

	result := BlsToExecutionChangeTopic(forkDigest)
	assert.Equal(t, expected, result)
}

func TestAttestationSubnetTopic(t *testing.T) {
	forkDigest := [4]byte{0x00, 0x00, 0x00, 0x08}

	tests := []struct {
		subnet   uint64
		expected string
	}{
		{0, "/eth2/00000008/beacon_attestation_0/ssz_snappy"},
		{1, "/eth2/00000008/beacon_attestation_1/ssz_snappy"},
		{63, "/eth2/00000008/beacon_attestation_63/ssz_snappy"},
		{100, "/eth2/00000008/beacon_attestation_100/ssz_snappy"}, // Out of range but should still format
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("subnet_%d", tt.subnet), func(t *testing.T) {
			result := AttestationSubnetTopic(forkDigest, tt.subnet)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncCommitteeSubnetTopic(t *testing.T) {
	forkDigest := [4]byte{0x00, 0x00, 0x00, 0x09}

	tests := []struct {
		subnet   uint64
		expected string
	}{
		{0, "/eth2/00000009/sync_committee_0/ssz_snappy"},
		{1, "/eth2/00000009/sync_committee_1/ssz_snappy"},
		{3, "/eth2/00000009/sync_committee_3/ssz_snappy"},
		{10, "/eth2/00000009/sync_committee_10/ssz_snappy"}, // Out of range but should still format
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("subnet_%d", tt.subnet), func(t *testing.T) {
			result := SyncCommitteeSubnetTopic(forkDigest, tt.subnet)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubnetConstants(t *testing.T) {
	// Test that constants have expected values
	assert.Equal(t, 64, AttestationSubnetCount)
	assert.Equal(t, 4, SyncCommitteeSubnetCount)
}

func TestTopicTemplates(t *testing.T) {
	// Test that template strings are correct
	assert.Equal(t, "beacon_attestation_%d", AttestationSubnetTopicTemplate)
	assert.Equal(t, "sync_committee_%d", SyncCommitteeSubnetTopicTemplate)
}

func TestForkDigestFormatting(t *testing.T) {
	tests := []struct {
		name       string
		forkDigest [4]byte
		expected   string
	}{
		{
			name:       "all zeros",
			forkDigest: [4]byte{0x00, 0x00, 0x00, 0x00},
			expected:   "00000000",
		},
		{
			name:       "all ones",
			forkDigest: [4]byte{0xff, 0xff, 0xff, 0xff},
			expected:   "ffffffff",
		},
		{
			name:       "mixed values",
			forkDigest: [4]byte{0xab, 0xcd, 0xef, 0x12},
			expected:   "abcdef12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with a known topic function
			result := BeaconBlockTopic(tt.forkDigest)
			expected := fmt.Sprintf("/eth2/%s/beacon_block/ssz_snappy", tt.expected)
			assert.Equal(t, expected, result)
		})
	}
}

func TestAllPossibleTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	// Test that we can generate all possible topics without panic
	topics := []string{
		BeaconBlockTopic(forkDigest),
		eth.BeaconAggregateAndProofTopic(forkDigest),
		VoluntaryExitTopic(forkDigest[:]),
		ProposerSlashingTopic(forkDigest[:]),
		AttesterSlashingTopic(forkDigest),
		SyncContributionAndProofTopic(forkDigest),
		BlsToExecutionChangeTopic(forkDigest),
	}

	// Add all attestation subnet topics
	for i := uint64(0); i < AttestationSubnetCount; i++ {
		topics = append(topics, AttestationSubnetTopic(forkDigest, i))
	}

	// Add all sync committee subnet topics
	for i := uint64(0); i < SyncCommitteeSubnetCount; i++ {
		topics = append(topics, SyncCommitteeSubnetTopic(forkDigest, i))
	}

	// Verify all topics are unique
	seen := make(map[string]bool)
	for _, topic := range topics {
		assert.False(t, seen[topic], "Duplicate topic: %s", topic)
		seen[topic] = true

		// Verify topic format
		assert.Regexp(t, "^/eth2/[0-9a-f]{8}/[a-z_0-9]+/ssz_snappy$", topic)
	}

	// Expected total: 7 single topics + 64 attestation subnets + 4 sync committee subnets
	assert.Equal(t, 7+64+4, len(topics))
}
