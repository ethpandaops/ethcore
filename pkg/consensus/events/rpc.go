package events

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// RpcMeta represents RPC metadata.
type RpcMeta struct {
	PeerID        peer.ID         `json:"peerId"`
	Subscriptions []RpcMetaSub    `json:"subs,omitempty"`
	Messages      []RpcMetaMsg    `json:"msgs,omitempty"`
	Control       *RpcMetaControl `json:"control,omitempty"`
}

// RpcMetaSub represents subscription metadata.
type RpcMetaSub struct {
	Subscribe bool   `json:"subscribe"`
	TopicID   string `json:"topicId"`
}

// RpcMetaMsg represents message metadata.
type RpcMetaMsg struct {
	MsgID string `json:"msgId,omitempty"`
	Topic string `json:"topic,omitempty"`
}

// RpcMetaControl represents control message metadata.
type RpcMetaControl struct {
	IHave     []RpcControlIHave     `json:"ihave,omitempty"`
	IWant     []RpcControlIWant     `json:"iwant,omitempty"`
	Graft     []RpcControlGraft     `json:"graft,omitempty"`
	Prune     []RpcControlPrune     `json:"prune,omitempty"`
	Idontwant []RpcControlIdontWant `json:"idontwant,omitempty"`
}

// RpcControlIHave represents IHAVE control message.
type RpcControlIHave struct {
	TopicID    string   `json:"topicId"`
	MessageIDs []string `json:"messageIds"`
}

// RpcControlIWant represents IWANT control message.
type RpcControlIWant struct {
	MessageIDs []string `json:"messageIds"`
}

// RpcControlGraft represents GRAFT control message.
type RpcControlGraft struct {
	TopicID string `json:"topicId"`
}

// RpcControlPrune represents PRUNE control message.
type RpcControlPrune struct {
	TopicID string    `json:"topicId"`
	Peers   []peer.ID `json:"peers,omitempty"`
	Backoff uint64    `json:"backoff,omitempty"`
}

// RpcControlIdontWant represents IDONTWANT control message.
type RpcControlIdontWant struct {
	MessageIDs []string `json:"messageIds"`
}

// RecvRPCData represents receiving an RPC.
type RecvRPCData struct {
	ReceivedFrom peer.ID  `json:"receivedFrom"`
	Meta         *RpcMeta `json:"meta"`
}

// SendRPCData represents sending an RPC.
type SendRPCData struct {
	SendTo peer.ID  `json:"sendTo"`
	Meta   *RpcMeta `json:"meta"`
}

// DropRPCData represents dropping an RPC.
type DropRPCData struct {
	SendTo peer.ID  `json:"sendTo"`
	Meta   *RpcMeta `json:"meta"`
}
