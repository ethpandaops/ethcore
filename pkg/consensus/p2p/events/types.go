//nolint:tagliatelle // upstream json tag expectations.
package events

import (
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TraceEvent represents a P2P trace event from the consensus layer.
type TraceEvent struct {
	Type      string
	Topic     string
	PeerID    peer.ID
	Timestamp time.Time
	Payload   any `json:"Data"` // cannot use field "Data" because of gk.Record method
}

// TraceEventPayloadMetaData represents metadata for P2P message payloads.
type TraceEventPayloadMetaData struct {
	PeerID  string `json:"PeerID"`
	Topic   string `json:"Topic"`
	Seq     []byte `json:"Seq"`
	MsgID   string `json:"MsgID"`
	MsgSize int    `json:"MsgSize"`
}

// EventTypeFromBeaconChainProtocol returns the event type for a given protocol string.
func EventTypeFromBeaconChainProtocol(protocol string) string {
	// Usual protocol string: /eth2/beacon_chain/req/metadata/2/ssz_snappy
	parts := strings.Split(protocol, "/")
	if len(parts) > 4 {
		return "HANDLE_" + strings.ToUpper(parts[4])
	}

	return "UNKNOWN"
}
