//nolint:tagliatelle // upstream json tag expectations.
package rpc

// RPC event handler types for processing RPC messages

// HandleStatusEvent represents a status message handler event.
type HandleStatusEvent struct {
	PeerID string `json:"PeerID"`
}

// HandleGoodbyeEvent represents a goodbye message handler event.
type HandleGoodbyeEvent struct {
	PeerID string `json:"PeerID"`
	Reason uint64 `json:"Reason"`
}

// HandlePingEvent represents a ping message handler event.
type HandlePingEvent struct {
	PeerID string `json:"PeerID"`
	SeqNum uint64 `json:"SeqNum"`
}

// HandleMetadataEvent represents a metadata message handler event.
type HandleMetadataEvent struct {
	PeerID string `json:"PeerID"`
}

// HandleBlocksByRangeEvent represents a blocks by range request handler event.
type HandleBlocksByRangeEvent struct {
	PeerID string `json:"PeerID"`
	Start  uint64 `json:"Start"`
	Count  uint64 `json:"Count"`
}

// HandleBlocksByRootEvent represents a blocks by root request handler event.
type HandleBlocksByRootEvent struct {
	PeerID string   `json:"PeerID"`
	Roots  []string `json:"Roots"`
}
