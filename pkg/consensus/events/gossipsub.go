package events

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// AddPeerData represents adding a peer to a topic.
type AddPeerData struct {
	PeerID peer.ID `json:"peerId"`
	Topic  string  `json:"topic"`
}

// RemovePeerData represents removing a peer from a topic.
type RemovePeerData struct {
	PeerID peer.ID `json:"peerId"`
	Topic  string  `json:"topic"`
}

// GraftData represents grafting a peer to mesh.
type GraftData struct {
	PeerID peer.ID `json:"peerId"`
	Topic  string  `json:"topic"`
}

// PruneData represents pruning a peer from mesh.
type PruneData struct {
	PeerID peer.ID `json:"peerId"`
	Topic  string  `json:"topic"`
}

// JoinData represents joining a topic.
type JoinData struct {
	Topic string `json:"topic"`
}

// LeaveData represents leaving a topic.
type LeaveData struct {
	Topic string `json:"topic"`
}

// DeliverMessageData represents message delivery.
type DeliverMessageData struct {
	PeerID      peer.ID `json:"peerId"`
	Topic       string  `json:"topic"`
	MessageID   string  `json:"messageId"`
	MessageSize int     `json:"messageSize"`
	LocalPeer   peer.ID `json:"localPeer"`
}

// ValidateMessageData represents message validation.
type ValidateMessageData struct {
	PeerID      peer.ID `json:"peerId"`
	Topic       string  `json:"topic"`
	MessageID   string  `json:"messageId"`
	MessageSize int     `json:"messageSize"`
	LocalPeer   peer.ID `json:"localPeer"`
	Result      string  `json:"result"`
}

// PublishMessageData represents publishing a message.
type PublishMessageData struct {
	Topic       string  `json:"topic"`
	MessageID   string  `json:"messageId"`
	MessageSize int     `json:"messageSize"`
	LocalPeer   peer.ID `json:"localPeer"`
}
