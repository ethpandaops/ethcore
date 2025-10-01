package libp2p

// Protocol-level trace events (ADD_PEER, RECV_RPC, etc.)

const (
	// Event type constants matching the original hermes implementation.
	EventTypeAddPeer              = "ADD_PEER"
	EventTypeRemovePeer           = "REMOVE_PEER"
	EventTypeJoin                 = "JOIN"
	EventTypeLeave                = "LEAVE"
	EventTypeGraft                = "GRAFT"
	EventTypePrune                = "PRUNE"
	EventTypeValidateMessage      = "VALIDATE_MESSAGE"
	EventTypeDeliverMessage       = "DELIVER_MESSAGE"
	EventTypeRejectMessage        = "REJECT_MESSAGE"
	EventTypeDuplicateMessage     = "DUPLICATE_MESSAGE"
	EventTypeThrottlePeer         = "THROTTLE_PEER"
	EventTypeRecvRPC              = "RECV_RPC"
	EventTypeSendRPC              = "SEND_RPC"
	EventTypeDropRPC              = "DROP_RPC"
	EventTypeUndeliverableMessage = "UNDELIVERABLE_MESSAGE"
	EventTypePublishMessage       = "PUBLISH_MESSAGE"
)
