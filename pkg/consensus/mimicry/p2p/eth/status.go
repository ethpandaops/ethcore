package eth

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

type PeerStatus struct {
	PeerID peer.ID
	Status *common.Status
}
