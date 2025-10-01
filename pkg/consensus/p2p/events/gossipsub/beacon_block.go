package gossipsub

import (
	ethtypes "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/p2p/events"
)

// TraceEventPhase0Block represents a Phase0 beacon block event.
type TraceEventPhase0Block struct {
	events.TraceEventPayloadMetaData
	Block *ethtypes.SignedBeaconBlock
}

// TraceEventAltairBlock represents an Altair beacon block event.
type TraceEventAltairBlock struct {
	events.TraceEventPayloadMetaData
	Block *ethtypes.SignedBeaconBlockAltair
}

// TraceEventBellatrixBlock represents a Bellatrix beacon block event.
type TraceEventBellatrixBlock struct {
	events.TraceEventPayloadMetaData
	Block *ethtypes.SignedBeaconBlockBellatrix
}

// TraceEventCapellaBlock represents a Capella beacon block event.
type TraceEventCapellaBlock struct {
	events.TraceEventPayloadMetaData
	Block *ethtypes.SignedBeaconBlockCapella
}

// TraceEventDenebBlock represents a Deneb beacon block event.
type TraceEventDenebBlock struct {
	events.TraceEventPayloadMetaData
	Block *ethtypes.SignedBeaconBlockDeneb
}

// TraceEventElectraBlock represents an Electra beacon block event.
type TraceEventElectraBlock struct {
	events.TraceEventPayloadMetaData
	Block *ethtypes.SignedBeaconBlockElectra
}

// TraceEventFuluBlock represents a Fulu beacon block event.
type TraceEventFuluBlock struct {
	events.TraceEventPayloadMetaData
	Block *ethtypes.SignedBeaconBlockFulu
}
