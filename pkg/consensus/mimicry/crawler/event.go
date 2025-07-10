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

type SuccessfulCrawl struct {
	PeerID       peer.ID
	NodeID       string
	AgentVersion string
	NetworkID    uint64
	ENR          *enode.Node
	Status       *common.Status
	Metadata     *common.MetaData
}

type FailedCrawl struct {
	PeerID peer.ID
	ENR    *enode.Node
	Error  CrawlError
}

type MetadataReceived struct {
	PeerID   peer.ID
	ENR      *enode.Node
	Metadata *common.MetaData
}

type PeerStatusUpdated struct {
	PeerID peer.ID
	ENR    *enode.Node
	Status *common.Status
}

type OnPeerStatusUpdatedCallback func(statusUpdated *PeerStatusUpdated)
type OnMetadataReceivedCallback func(mdReceived *MetadataReceived)
type OnSuccessfulCrawlCallback func(successfulCrawl *SuccessfulCrawl)
type OnFailedCrawlCallback func(failedCrawl *FailedCrawl)

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

func (n *Crawler) emitPeerStatusUpdated(statusUpdated *PeerStatusUpdated) {
	n.broker.Emit(OnUpdatedPeerStatus, statusUpdated)
}

func (n *Crawler) emitMetadataReceived(mdReceived *MetadataReceived) {
	n.broker.Emit(OnMetadataReceived, mdReceived)
}

func (n *Crawler) emitSuccessfulCrawl(successfulCrawl *SuccessfulCrawl) {
	n.broker.Emit(OnSuccessfulCrawl, successfulCrawl)
}

func (n *Crawler) emitFailedCrawl(failedCrawl *FailedCrawl) {
	n.broker.Emit(OnFailedCrawl, failedCrawl)
}
