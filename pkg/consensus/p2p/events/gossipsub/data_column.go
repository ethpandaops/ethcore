package gossipsub

import (
	ethtypes "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/p2p/events"
)

// TraceEventDataColumnSidecar represents a data column sidecar event.
type TraceEventDataColumnSidecar struct {
	events.TraceEventPayloadMetaData
	DataColumnSidecar *ethtypes.DataColumnSidecar
}
