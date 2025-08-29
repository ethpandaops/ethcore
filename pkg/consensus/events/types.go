package events

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TraceEvent represents a generic libp2p trace event.
type TraceEvent struct {
	Type      string    `json:"type"`
	Topic     string    `json:"topic,omitempty"`
	PeerID    peer.ID   `json:"peerId,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"data"`
}

// TraceEventPayloadMetaData provides common metadata for messages.
type TraceEventPayloadMetaData struct {
	PeerID  string `json:"peerId"`
	Topic   string `json:"topic"`
	Seq     []byte `json:"seq"`
	MsgID   string `json:"msgId"`
	MsgSize int    `json:"msgSize"`
}

// EventType represents the type of P2P event.
type EventType int8

const (
	EventTypeUnknown EventType = iota
	EventTypeGenericEvent
	// Gossip-mesh.
	EventTypeAddRemovePeer
	EventTypeGraftPrune
	// PeerExchange.
	EventTypeGossipPx
	// Gossip RPCs.
	EventTypeControlRPC
	EventTypeIhave
	EventTypeIwant
	EventTypeIdontwant
	EventTypeSentMsg
	// Gossip Message arrivals.
	EventTypeMsgArrivals
	// Gossip Join/Leave Topic.
	EventTypeJoinLeaveTopic
	// Libp2p Event.
	EventTypeConnectDisconnectPeer
)

// EventSubType provides granular event classification.
type EventSubType int8

const (
	EventSubTypeNone EventSubType = iota
	// Add / Remove peers.
	EventSubTypeAddPeer
	EventSubTypeRemovePeer
	// Graft / Prunes.
	EventSubTypeGraft
	EventSubTypePrune
	// Msg arrivals.
	EventSubTypeDeliverMsg
	EventSubTypeValidateMsg
	EventSubTypeHandleMsg
	EventSubTypeDuplicatedMsg
	EventSubTypeRejectMsg
	EventSubTypeUndeliverableMsg
	// Join/Leave Topic.
	EventSubTypeJoinTopic
	EventSubTypeLeaveTopic
	// Libp2p.
	EventSubTypeConnectPeer
	EventSubTypeDisconnectPeer
	// Peer score.
	EventSubTypeThrottlePeer
)
