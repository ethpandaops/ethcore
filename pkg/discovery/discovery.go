package discovery

import (
	"context"

	"github.com/ethereum/go-ethereum/p2p/enode"
)

type NodeFinder interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	OnNodeRecord(ctx context.Context, handler func(ctx context.Context, node *enode.Node) error)
}

var _ NodeFinder = &DiscV5{}
