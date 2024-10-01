package host

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	// Import the metrics package if not already imported
)

// Event names used for broker communication
var (
	// Peer disconnect events
	BeforePeerDisconnectEvent = "peer:before:disconnect"
	AfterPeerDisconnectEvent  = "peer:after:disconnect"
	// Peer connect events
	BeforePeerConnectEvent = "peer:before:connect"
	AfterPeerConnectEvent  = "peer:after:connect"
)

type BeforePeerConnectCallback func(peerID peer.ID)
type AfterPeerConnectCallback func(net network.Network, conn network.Conn)
type BeforePeerDisconnectCallback func(peerID peer.ID)
type AfterPeerDisconnectCallback func(net network.Network, conn network.Conn)

// Subscribers
func (n *Node) BeforePeerConnect(callback BeforePeerConnectCallback) {
	n.broker.On(BeforePeerConnectEvent, callback)
}

func (n *Node) AfterPeerConnect(callback AfterPeerConnectCallback) {
	n.broker.On(AfterPeerConnectEvent, callback)
}

// New subscriber methods for lifecycle events
func (n *Node) BeforePeerDisconnect(callback BeforePeerDisconnectCallback) {
	n.broker.On(BeforePeerDisconnectEvent, callback)
}

func (n *Node) AfterPeerDisconnect(callback AfterPeerDisconnectCallback) {
	n.broker.On(AfterPeerDisconnectEvent, callback)
}

// Emitters
func (n *Node) emitBeforePeerConnect(peerID peer.ID) {
	n.broker.Emit(BeforePeerConnectEvent, peerID)
}

func (n *Node) emitAfterPeerConnect(net network.Network, conn network.Conn) {
	n.broker.Emit(AfterPeerConnectEvent, net, conn)
}

func (n *Node) emitBeforePeerDisconnect(peerID peer.ID) {
	n.broker.Emit(BeforePeerDisconnectEvent, peerID)
}

func (n *Node) emitAfterPeerDisconnect(net network.Network, conn network.Conn) {
	n.broker.Emit(AfterPeerDisconnectEvent, net, conn)
}
