package eth

import (
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/p2p/encoder"
)

const (
	StatusProtocolID   = "/eth2/beacon_chain/req/status/1/" + encoder.ProtocolSuffixSSZSnappy
	GoodbyeProtocolID  = "/eth2/beacon_chain/req/goodbye/1/" + encoder.ProtocolSuffixSSZSnappy
	PingProtocolID     = "/eth2/beacon_chain/req/ping/1/" + encoder.ProtocolSuffixSSZSnappy
	MetaDataProtocolID = "/eth2/beacon_chain/req/metadata/2/" + encoder.ProtocolSuffixSSZSnappy
)
