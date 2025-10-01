package host

import (
	"context"

	"github.com/ethpandaops/ethcore/pkg/consensus/p2p/events"
)

// EventCallback is a function type for handling P2P trace events.
// This is used by components that need to process consensus layer P2P events.
type EventCallback func(ctx context.Context, event *events.TraceEvent)
