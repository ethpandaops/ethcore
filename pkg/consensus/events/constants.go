package events

// Event type constants for TraceEvent.Type field.
const (
	// Gossipsub events.
	EventDeliverMessage       = "DELIVER_MESSAGE"
	EventValidateMessage      = "VALIDATE_MESSAGE"
	EventRejectMessage        = "REJECT_MESSAGE"
	EventDuplicateMessage     = "DUPLICATE_MESSAGE"
	EventUndeliverableMessage = "UNDELIVERABLE_MESSAGE"
	EventPublishMessage       = "PUBLISH_MESSAGE"
	EventHandleMessage        = "HANDLE_MESSAGE"

	// Mesh events.
	EventGraft = "GRAFT"
	EventPrune = "PRUNE"
	EventJoin  = "JOIN"
	EventLeave = "LEAVE"

	// Peer events.
	EventAddPeer      = "ADD_PEER"
	EventRemovePeer   = "REMOVE_PEER"
	EventThrottlePeer = "THROTTLE_PEER"

	// RPC events.
	EventRecvRPC = "RECV_RPC"
	EventSendRPC = "SEND_RPC"
	EventDropRPC = "DROP_RPC"

	// Connection events.
	EventConnected    = "CONNECTED"
	EventDisconnected = "DISCONNECTED"

	// Synthetic events.
	EventSyntheticHeartbeat = "SYNTHETIC_HEARTBEAT"

	// Peer score events.
	EventPeerScore = "PEERSCORE"

	// Metadata events.
	EventHandleMetadata = "HANDLE_METADATA"
	EventHandleStatus   = "HANDLE_STATUS"
)
