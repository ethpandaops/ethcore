//nolint:tagliatelle // upstream json tag expectations.
package rpc

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// RpcMeta represents RPC metadata.
type RpcMeta struct {
	PeerID        peer.ID
	Subscriptions []RpcMetaSub    `json:"Subs,omitempty"`
	Messages      []RpcMetaMsg    `json:"Msgs,omitempty"`
	Control       *RpcMetaControl `json:"Control,omitempty"`
}

// RpcMetaSub represents RPC subscription metadata.
type RpcMetaSub struct {
	Subscribe bool
	TopicID   string
}

// RpcMetaMsg represents RPC message metadata.
type RpcMetaMsg struct {
	MsgID string `json:"MsgID,omitempty"`
	Topic string `json:"Topic,omitempty"`
}

// RpcMetaControl represents RPC control message metadata.
type RpcMetaControl struct {
	IHave     []RpcControlIHave     `json:"IHave,omitempty"`
	IWant     []RpcControlIWant     `json:"IWant,omitempty"`
	Graft     []RpcControlGraft     `json:"Graft,omitempty"`
	Prune     []RpcControlPrune     `json:"Prune,omitempty"`
	Idontwant []RpcControlIdontWant `json:"Idontwant,omitempty"`
}

// RpcControlIHave represents an IHAVE control message.
type RpcControlIHave struct {
	TopicID string
	MsgIDs  []string
}

// RpcControlIWant represents an IWANT control message.
type RpcControlIWant struct {
	MsgIDs []string
}

// RpcControlGraft represents a GRAFT control message.
type RpcControlGraft struct {
	TopicID string
}

// RpcControlPrune represents a PRUNE control message.
type RpcControlPrune struct {
	TopicID string
	PeerIDs []peer.ID
}

// RpcControlIdontWant represents an IDONTWANT control message.
type RpcControlIdontWant struct {
	MsgIDs []string
}
