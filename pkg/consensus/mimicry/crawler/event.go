package crawler

import (
	"fmt"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

const (
	ModuleName = "crawler"
)

// Event names used for broker communication.
var (
	OnUpdatedPeerStatus = fmt.Sprintf("%s:peer:status:updated", ModuleName)
	OnMetadataReceived  = fmt.Sprintf("%s:metadata:updated", ModuleName)
	OnSuccessfulCrawl   = fmt.Sprintf("%s:crawl:success", ModuleName)
	OnFailedCrawl       = fmt.Sprintf("%s:crawl:failed", ModuleName)
)

type OnPeerStatusUpdatedCallback func(peerID peer.ID, status *common.Status)
type OnMetadataReceivedCallback func(peerID peer.ID, metadata *common.MetaData)
type OnSuccessfulCrawlCallback func(peerID peer.ID, enr *enode.Node, status *common.Status, metadata *common.MetaData)
type OnFailedCrawlCallback func(peerID peer.ID, err CrawlError)

// OnPeerStatusUpdated subscribes to the peer status updated event.
func (n *Crawler) OnPeerStatusUpdated(callback OnPeerStatusUpdatedCallback) {
	n.broker.On(OnUpdatedPeerStatus, callback)
}

// OnMetadataReceived subscribes to the metadata received event.
func (n *Crawler) OnMetadataReceived(callback OnMetadataReceivedCallback) {
	n.broker.On(OnMetadataReceived, callback)
}

// OnSuccessfulCrawl subscribes to the successful crawl event.
func (n *Crawler) OnSuccessfulCrawl(callback OnSuccessfulCrawlCallback) {
	n.broker.On(OnSuccessfulCrawl, callback)
}

// OnFailedCrawl subscribes to the failed crawl event.
func (n *Crawler) OnFailedCrawl(callback OnFailedCrawlCallback) {
	n.broker.On(OnFailedCrawl, callback)
}

func (n *Crawler) emitPeerStatusUpdated(peerID peer.ID, status *common.Status) {
	n.broker.Emit(OnUpdatedPeerStatus, peerID, status)
}

func (n *Crawler) emitMetadataReceived(peerID peer.ID, metadata *common.MetaData) {
	n.broker.Emit(OnMetadataReceived, peerID, metadata)
}

func (n *Crawler) emitSuccessfulCrawl(peerID peer.ID, status *common.Status, metadata *common.MetaData) {
	n.broker.Emit(OnSuccessfulCrawl, peerID, n.GetPeerENR(peerID), status, metadata)
}

func (n *Crawler) emitFailedCrawl(peerID peer.ID, err CrawlError) {
	n.broker.Emit(OnFailedCrawl, peerID, err)
}
