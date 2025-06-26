package discovery

import (
	"context"
	"net"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManual_Start(t *testing.T) {
	manual := &Manual{}
	ctx := context.Background()

	err := manual.Start(ctx)
	assert.NoError(t, err)
}

func TestManual_Stop(t *testing.T) {
	manual := &Manual{}
	ctx := context.Background()

	err := manual.Stop(ctx)
	assert.NoError(t, err)
}

func TestManual_AddNode(t *testing.T) {
	ctx := context.Background()
	manual := &Manual{}

	// Test adding node without handler
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	db, err := enode.OpenDB("")
	require.NoError(t, err)
	defer db.Close()

	ln := enode.NewLocalNode(db, privKey)
	ln.Set(enr.IP(net.ParseIP("127.0.0.1")))
	ln.Set(enr.TCP(30303))
	testNode := ln.Node()

	// Should handle nil handler gracefully
	err = manual.AddNode(ctx, testNode)
	assert.Error(t, err) // Should error because handler is nil

	// Set up handler
	receivedNodes := make([]*enode.Node, 0)
	manual.OnNodeRecord(ctx, func(ctx context.Context, node *enode.Node) error {
		receivedNodes = append(receivedNodes, node)

		return nil
	})

	// Add node with handler
	err = manual.AddNode(ctx, testNode)
	assert.NoError(t, err)
	assert.Len(t, receivedNodes, 1)
	assert.Equal(t, testNode.ID(), receivedNodes[0].ID())
}

func TestManual_OnNodeRecord(t *testing.T) {
	ctx := context.Background()
	manual := &Manual{}

	// Test setting handler
	handlerCalled := false
	manual.OnNodeRecord(ctx, func(ctx context.Context, node *enode.Node) error {
		handlerCalled = true

		return nil
	})

	// Create test node
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	db, err := enode.OpenDB("")
	require.NoError(t, err)
	defer db.Close()

	ln := enode.NewLocalNode(db, privKey)
	ln.Set(enr.IP(net.ParseIP("127.0.0.1")))
	ln.Set(enr.TCP(30303))
	testNode := ln.Node()

	// Add node should trigger handler
	err = manual.AddNode(ctx, testNode)
	assert.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestManual_MultipleNodes(t *testing.T) {
	ctx := context.Background()
	manual := &Manual{}

	receivedNodes := make([]*enode.Node, 0)
	manual.OnNodeRecord(ctx, func(ctx context.Context, node *enode.Node) error {
		receivedNodes = append(receivedNodes, node)

		return nil
	})

	// Add multiple nodes
	numNodes := 5
	for i := 0; i < numNodes; i++ {
		privKey, err := crypto.GenerateKey()
		require.NoError(t, err)

		db, err := enode.OpenDB("")
		require.NoError(t, err)
		defer db.Close()

		ln := enode.NewLocalNode(db, privKey)
		ln.Set(enr.IP(net.ParseIP("127.0.0.1")))
		ln.Set(enr.TCP(30303 + i))

		err = manual.AddNode(ctx, ln.Node())
		assert.NoError(t, err)
	}

	assert.Len(t, receivedNodes, numNodes)
}

func TestManual_HandlerError(t *testing.T) {
	ctx := context.Background()
	manual := &Manual{}

	// Set handler that returns error
	manual.OnNodeRecord(ctx, func(ctx context.Context, node *enode.Node) error {
		return assert.AnError
	})

	// Create test node
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	db, err := enode.OpenDB("")
	require.NoError(t, err)
	defer db.Close()

	ln := enode.NewLocalNode(db, privKey)
	ln.Set(enr.IP(net.ParseIP("127.0.0.1")))
	ln.Set(enr.TCP(30303))

	// Error should be propagated
	err = manual.AddNode(ctx, ln.Node())
	assert.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

func TestManual_InterfaceCompliance(t *testing.T) {
	// Ensure Manual implements NodeFinder interface
	var _ NodeFinder = &Manual{}
}
