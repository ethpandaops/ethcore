package discovery

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/p2p/enode"
)

// Manual is a NodeFinder that allows you to manually add nodes to the discovery
// pool.
type Manual struct {
	handler func(ctx context.Context, node *enode.Node) error
}

func (m *Manual) Start(ctx context.Context) error {
	return nil
}

func (m *Manual) Stop(ctx context.Context) error {
	return nil
}

func (m *Manual) OnNodeRecord(ctx context.Context, handler func(ctx context.Context, node *enode.Node) error) {
	m.handler = handler
}

func (m *Manual) AddNode(ctx context.Context, node *enode.Node) error {
	if m.handler == nil {
		return errors.New("no handler set")
	}
	return m.handler(ctx, node)
}
