package host

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Event names used for broker communication.
var (
	// Peer disconnect events.
	BeforePeerDisconnectEvent = "peer:before:disconnect"
	AfterPeerDisconnectEvent  = "peer:after:disconnect"
	// Peer connect events.
	BeforePeerConnectEvent = "peer:before:connect"
	AfterPeerConnectEvent  = "peer:after:connect"
)

type BeforePeerConnectCallback func(peerID peer.ID)
type AfterPeerConnectCallback func(net network.Network, conn network.Conn)
type BeforePeerDisconnectCallback func(peerID peer.ID)
type AfterPeerDisconnectCallback func(net network.Network, conn network.Conn)

// BeforePeerConnect subscribes to the before peer connect event.
func (n *Node) BeforePeerConnect(callback BeforePeerConnectCallback) {
	n.broker.On(BeforePeerConnectEvent, callback)
}

// AfterPeerConnect subscribes to the after peer connect event.
func (n *Node) AfterPeerConnect(callback AfterPeerConnectCallback) {
	n.broker.On(AfterPeerConnectEvent, callback)
}

// BeforePeerDisconnect subscribes to the before peer disconnect event.
func (n *Node) BeforePeerDisconnect(callback BeforePeerDisconnectCallback) {
	n.broker.On(BeforePeerDisconnectEvent, callback)
}

// AfterPeerDisconnect subscribes to the after peer disconnect event.
func (n *Node) AfterPeerDisconnect(callback AfterPeerDisconnectCallback) {
	n.broker.On(AfterPeerDisconnectEvent, callback)
}

// emit runs the broker emit on a bounded background goroutine. Emits happen
// from libp2p notifiees, which run synchronously on the swarm's connection
// setup path: blocking there stalls inbound stream handling on the new
// connection, so slow event handlers must never block the notifiee.
func (n *Node) emit(event string, arguments ...any) {
	n.emitSem <- struct{}{}

	go func() {
		defer func() { <-n.emitSem }()

		n.broker.Emit(event, arguments...)
	}()
}

func (n *Node) emitBeforePeerConnect(peerID peer.ID) {
	n.emit(BeforePeerConnectEvent, peerID)
}

func (n *Node) emitAfterPeerConnect(net network.Network, conn network.Conn) {
	n.emit(AfterPeerConnectEvent, net, conn)
}

func (n *Node) emitBeforePeerDisconnect(peerID peer.ID) {
	n.emit(BeforePeerDisconnectEvent, peerID)
}

func (n *Node) emitAfterPeerDisconnect(net network.Network, conn network.Conn) {
	n.emit(AfterPeerDisconnectEvent, net, conn)
}
