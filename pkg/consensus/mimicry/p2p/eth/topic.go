package eth

import (
	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
)

const (
	ProtocolSuffix    = encoder.ProtocolSuffixSSZSnappy
	ProtocolVersionV1 = "1"
	ProtocolVersionV2 = "2"
)

// Request-Response Protocol IDs.
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

// Topic format template for Ethereum gossipsub topics.
var GossipsubTopicFormat = "/eth2/%x/%s/ssz_snappy"
