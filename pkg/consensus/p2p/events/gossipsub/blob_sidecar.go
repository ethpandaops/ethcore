package gossipsub

import (
	ethtypes "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/p2p/events"
)

// TraceEventBlobSidecar represents a blob sidecar event.
type TraceEventBlobSidecar struct {
	events.TraceEventPayloadMetaData
	BlobSidecar *ethtypes.BlobSidecar
}
