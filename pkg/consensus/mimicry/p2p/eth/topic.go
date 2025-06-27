package eth

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
)

const (
	ProtocolSuffix    = encoder.ProtocolSuffixSSZSnappy
	ProtocolVersionV1 = "1"
	ProtocolVersionV2 = "2"
)

// Request-Response Protocol IDs
const (
	StatusV1ProtocolID  = "/eth2/beacon_chain/req/status/" + ProtocolVersionV1 + "/" + ProtocolSuffix
	GoodbyeV1ProtocolID = "/eth2/beacon_chain/req/goodbye/" + ProtocolVersionV1 + "/" + ProtocolSuffix
	PingV1ProtocolID    = "/eth2/beacon_chain/req/ping/" + ProtocolVersionV1 + "/" + ProtocolSuffix

	MetaDataV1ProtocolID = "/eth2/beacon_chain/req/metadata/" + ProtocolVersionV1 + "/" + ProtocolSuffix
	MetaDataV2ProtocolID = "/eth2/beacon_chain/req/metadata/" + ProtocolVersionV2 + "/" + ProtocolSuffix

	BeaconBlocksByRangeV1ProtocolID = "/eth2/beacon_chain/req/beacon_blocks_by_range/" + ProtocolVersionV1 + "/" + ProtocolSuffix
	BeaconBlocksByRangeV2ProtocolID = "/eth2/beacon_chain/req/beacon_blocks_by_range/" + ProtocolVersionV2 + "/" + ProtocolSuffix
	BeaconBlocksByRootV1ProtocolID  = "/eth2/beacon_chain/req/beacon_blocks_by_root/" + ProtocolVersionV1 + "/" + ProtocolSuffix
	BeaconBlocksByRootV2ProtocolID  = "/eth2/beacon_chain/req/beacon_blocks_by_root/" + ProtocolVersionV2 + "/" + ProtocolSuffix

	BlobSidecarsByRangeV1ProtocolID = "/eth2/beacon_chain/req/blob_sidecars_by_range/" + ProtocolVersionV1 + "/" + ProtocolSuffix
	BlobSidecarsByRootV1ProtocolID  = "/eth2/beacon_chain/req/blob_sidecars_by_root/" + ProtocolVersionV1 + "/" + ProtocolSuffix
)

// Gossipsub Topic Definitions
const (
	// Topic format template for Ethereum gossipsub topics
	GossipsubTopicFormat = "/eth2/%x/%s/ssz_snappy"

	// Global topic names (no subnet ID)
	BeaconBlockTopicName              = "beacon_block"
	BeaconAggregateAndProofTopicName  = "beacon_aggregate_and_proof"
	VoluntaryExitTopicName            = "voluntary_exit"
	ProposerSlashingTopicName         = "proposer_slashing"
	AttesterSlashingTopicName         = "attester_slashing"
	SyncContributionAndProofTopicName = "sync_committee_contribution_and_proof"
	BlsToExecutionChangeTopicName     = "bls_to_execution_change"

	// Subnet topic templates (require subnet ID)
	AttestationSubnetTopicTemplate   = "beacon_attestation_%d"
	SyncCommitteeSubnetTopicTemplate = "sync_committee_%d"

	// Network constants
	AttestationSubnetCount   = 64
	SyncCommitteeSubnetCount = 4
)

// Topic construction helpers for global topics
func BeaconBlockTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BeaconBlockTopicName)
}

func BeaconAggregateAndProofTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BeaconAggregateAndProofTopicName)
}

func VoluntaryExitTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, VoluntaryExitTopicName)
}

func ProposerSlashingTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, ProposerSlashingTopicName)
}

func AttesterSlashingTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, AttesterSlashingTopicName)
}

func SyncContributionAndProofTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, SyncContributionAndProofTopicName)
}

func BlsToExecutionChangeTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BlsToExecutionChangeTopicName)
}

// Topic construction helpers for subnet topics
func AttestationSubnetTopic(forkDigest [4]byte, subnet uint64) string {
	name := fmt.Sprintf(AttestationSubnetTopicTemplate, subnet)
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, name)
}

func SyncCommitteeSubnetTopic(forkDigest [4]byte, subnet uint64) string {
	name := fmt.Sprintf(SyncCommitteeSubnetTopicTemplate, subnet)
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, name)
}

// Topic parsing helpers
func ParseGossipsubTopic(topic string) (forkDigest [4]byte, name string, err error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 5 {
		return forkDigest, "", fmt.Errorf("invalid topic format: %s", topic)
	}

	if parts[0] != "" || parts[1] != "eth2" || parts[4] != "ssz_snappy" {
		return forkDigest, "", fmt.Errorf("invalid topic format: %s", topic)
	}

	// Parse fork digest (hex string without 0x prefix)
	forkDigestStr := parts[2]
	if len(forkDigestStr) != 8 {
		return forkDigest, "", fmt.Errorf("invalid fork digest length: %s", forkDigestStr)
	}

	for i := 0; i < 4; i++ {
		val, err := strconv.ParseUint(forkDigestStr[i*2:i*2+2], 16, 8)
		if err != nil {
			return forkDigest, "", fmt.Errorf("invalid fork digest: %s", forkDigestStr)
		}
		forkDigest[i] = byte(val)
	}

	name = parts[3]
	return forkDigest, name, nil
}

// IsAttestationTopic checks if a topic name is an attestation subnet topic and returns the subnet ID
func IsAttestationTopic(topicName string) (bool, uint64) {
	if !strings.HasPrefix(topicName, "beacon_attestation_") {
		return false, 0
	}

	subnetStr := strings.TrimPrefix(topicName, "beacon_attestation_")
	subnet, err := strconv.ParseUint(subnetStr, 10, 64)
	if err != nil {
		return false, 0
	}

	return subnet < AttestationSubnetCount, subnet
}

// IsSyncCommitteeTopic checks if a topic name is a sync committee subnet topic and returns the subnet ID
func IsSyncCommitteeTopic(topicName string) (bool, uint64) {
	if !strings.HasPrefix(topicName, "sync_committee_") {
		return false, 0
	}

	subnetStr := strings.TrimPrefix(topicName, "sync_committee_")
	subnet, err := strconv.ParseUint(subnetStr, 10, 64)
	if err != nil {
		return false, 0
	}

	return subnet < SyncCommitteeSubnetCount, subnet
}

// AllGlobalTopics returns all global gossipsub topics for a given fork digest
func AllGlobalTopics(forkDigest [4]byte) []string {
	return []string{
		BeaconBlockTopic(forkDigest),
		BeaconAggregateAndProofTopic(forkDigest),
		VoluntaryExitTopic(forkDigest),
		ProposerSlashingTopic(forkDigest),
		AttesterSlashingTopic(forkDigest),
		SyncContributionAndProofTopic(forkDigest),
		BlsToExecutionChangeTopic(forkDigest),
	}
}

// AllAttestationTopics returns all attestation subnet topics for a given fork digest
func AllAttestationTopics(forkDigest [4]byte) []string {
	topics := make([]string, AttestationSubnetCount)
	for i := uint64(0); i < AttestationSubnetCount; i++ {
		topics[i] = AttestationSubnetTopic(forkDigest, i)
	}
	return topics
}

// AllSyncCommitteeTopics returns all sync committee subnet topics for a given fork digest
func AllSyncCommitteeTopics(forkDigest [4]byte) []string {
	topics := make([]string, SyncCommitteeSubnetCount)
	for i := uint64(0); i < SyncCommitteeSubnetCount; i++ {
		topics[i] = SyncCommitteeSubnetTopic(forkDigest, i)
	}
	return topics
}

// AllTopics returns all gossipsub topics (global + all subnets) for a given fork digest
func AllTopics(forkDigest [4]byte) []string {
	global := AllGlobalTopics(forkDigest)
	attestation := AllAttestationTopics(forkDigest)
	syncCommittee := AllSyncCommitteeTopics(forkDigest)

	all := make([]string, 0, len(global)+len(attestation)+len(syncCommittee))
	all = append(all, global...)
	all = append(all, attestation...)
	all = append(all, syncCommittee...)

	return all
}
