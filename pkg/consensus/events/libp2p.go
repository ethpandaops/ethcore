package events

import (
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// ConnectedData represents a peer connection event.
type ConnectedData struct {
	RemotePeer   peer.ID           `json:"remotePeer"`
	RemoteMaddrs ma.Multiaddr      `json:"remoteMaddrs"`
	AgentVersion string            `json:"agentVersion"`
	Direction    network.Direction `json:"direction"`
	Opened       time.Time         `json:"opened"`
	Transient    bool              `json:"transient"`
}

// DisconnectedData represents a peer disconnection event.
type DisconnectedData struct {
	RemotePeer   peer.ID           `json:"remotePeer"`
	RemoteMaddrs ma.Multiaddr      `json:"remoteMaddrs"`
	AgentVersion string            `json:"agentVersion"`
	Direction    network.Direction `json:"direction"`
	Opened       time.Time         `json:"opened"`
	Transient    bool              `json:"transient"`
}

// SyntheticHeartbeatData represents periodic peer status.
type SyntheticHeartbeatData struct {
	RemotePeer      string       `json:"remotePeer"`
	RemoteMaddrs    ma.Multiaddr `json:"remoteMaddrs"`
	LatencyMs       int64        `json:"latencyMs"`
	AgentVersion    string       `json:"agentVersion"`
	Direction       uint32       `json:"direction"`
	Protocols       []string     `json:"protocols"`
	ConnectionAgeNs int64        `json:"connectionAgeNs"`
}

// PeerScoreEvent represents peer scoring metrics.
type PeerScoreEvent struct {
	PeerID             string       `json:"peerId"`
	Score              float64      `json:"score"`
	AppSpecificScore   float64      `json:"appSpecificScore"`
	IPColocationFactor float64      `json:"ipColocationFactor"`
	BehaviourPenalty   float64      `json:"behaviourPenalty"`
	Topics             []TopicScore `json:"topics"`
}

// TopicScore represents scoring metrics for a specific topic.
type TopicScore struct {
	Topic                    string        `json:"topic"`
	TimeInMesh               time.Duration `json:"timeInMesh"`
	FirstMessageDeliveries   float64       `json:"firstMessageDeliveries"`
	MeshMessageDeliveries    float64       `json:"meshMessageDeliveries"`
	InvalidMessageDeliveries float64       `json:"invalidMessageDeliveries"`
}
