//nolint:tagliatelle // upstream json tag expectations.
package libp2p

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// Core libp2p event types that don't come from pubsub

// AddPeerEvent represents a peer being added to the network.
type AddPeerEvent struct {
	PeerID   peer.ID     `json:"PeerID"`
	Protocol protocol.ID `json:"Protocol"`
}

// RemovePeerEvent represents a peer being removed from the network.
type RemovePeerEvent struct {
	PeerID peer.ID `json:"PeerID"`
}

// ConnectedEvent represents a peer connection event.
type ConnectedEvent struct {
	RemotePeer   peer.ID  `json:"RemotePeer"`
	RemoteMaddrs []string `json:"RemoteMaddrs"`
}

// DisconnectedEvent represents a peer disconnection event.
type DisconnectedEvent struct {
	RemotePeer peer.ID `json:"RemotePeer"`
}

// JoinEvent represents joining a pubsub topic.
type JoinEvent struct {
	Topic string `json:"Topic"`
}

// LeaveEvent represents leaving a pubsub topic.
type LeaveEvent struct {
	Topic string `json:"Topic"`
}

// GraftEvent represents grafting a peer to a topic.
type GraftEvent struct {
	PeerID peer.ID `json:"PeerID"`
	Topic  string  `json:"Topic"`
}

// PruneEvent represents pruning a peer from a topic.
type PruneEvent struct {
	PeerID peer.ID `json:"PeerID"`
	Topic  string  `json:"Topic"`
}
