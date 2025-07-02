package discovery

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test the filterPeer function with various node configurations.
func TestDiscV5_FilterPeer(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	disc := NewDiscV5(ctx, 30*time.Second, logger)

	// Create a mock listener with a local node for testing
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	db, err := enode.OpenDB("")
	require.NoError(t, err)
	defer db.Close()

	localNode := enode.NewLocalNode(db, privKey)
	localNode.Set(enr.IP(net.ParseIP("192.168.1.1")))
	localNode.Set(enr.TCP(30303))
	localNode.Set(enr.UDP(30303))

	disc.listener = &ListenerV5{
		localNode: localNode,
	}

	tests := []struct {
		name     string
		node     func() *enode.Node
		expected bool
	}{
		{
			name: "nil node",
			node: func() *enode.Node {
				return nil
			},
			expected: false,
		},
		{
			name: "node without IP",
			node: func() *enode.Node {
				privKey, _ := crypto.GenerateKey()

				db, _ := enode.OpenDB("")
				defer db.Close()

				ln := enode.NewLocalNode(db, privKey)

				// Don't set IP
				ln.Set(enr.TCP(30303))

				return ln.Node()
			},
			expected: false,
		},
		{
			name: "self node",
			node: func() *enode.Node {
				return localNode.Node()
			},
			expected: false,
		},
		{
			name: "node without TCP port",
			node: func() *enode.Node {
				privKey, _ := crypto.GenerateKey()
				db, _ := enode.OpenDB("")

				defer db.Close()

				ln := enode.NewLocalNode(db, privKey)
				ln.Set(enr.IP(net.ParseIP("8.8.8.8")))
				ln.Set(enr.UDP(30303))

				// Don't set TCP
				return ln.Node()
			},
			expected: false,
		},
		{
			name: "private IP node",
			node: func() *enode.Node {
				privKey, _ := crypto.GenerateKey()
				db, _ := enode.OpenDB("")

				defer db.Close()

				ln := enode.NewLocalNode(db, privKey)
				ln.Set(enr.IP(net.ParseIP("192.168.1.100")))
				ln.Set(enr.TCP(30303))
				ln.Set(enr.UDP(30303))

				return ln.Node()
			},
			expected: false,
		},
		{
			name: "valid public node",
			node: func() *enode.Node {
				privKey, _ := crypto.GenerateKey()
				db, _ := enode.OpenDB("")

				defer db.Close()

				ln := enode.NewLocalNode(db, privKey)
				ln.Set(enr.IP(net.ParseIP("8.8.8.8")))
				ln.Set(enr.TCP(30303))
				ln.Set(enr.UDP(30303))

				return ln.Node()
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := tt.node()
			result := disc.filterPeer(node)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test UpdateBootNodes functionality.
func TestDiscV5_UpdateBootNodes(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	disc := NewDiscV5(ctx, 30*time.Second, logger)

	tests := []struct {
		name      string
		bootNodes []string
		wantErr   bool
		expected  int
	}{
		{
			name:      "empty boot nodes",
			bootNodes: []string{},
			wantErr:   false,
			expected:  0,
		},
		{
			name: "valid enode URLs",
			bootNodes: []string{
				"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.0.0.1:30303",
				"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.0.0.2:30303",
			},
			wantErr:  false,
			expected: 2,
		},
		{
			name: "valid ENR",
			bootNodes: []string{
				"enr:-IS4QHCYrYZbAKWCBRlAy5zzaDZXJBGkcnh4MHcBFZntXNFrdvJjX04jRzjzCBOonrkTfj499SZuOh8R33Ls8RRcy5wBgmlkgnY0gmlwhH8AAAGJc2VjcDI1NmsxoQPKY0yuDUmstAHYpMa2_oxVtw0RW_QAdpzBQA8yWM0xOIN1ZHCCdl8",
			},
			wantErr:  false,
			expected: 1,
		},
		{
			name: "invalid boot node",
			bootNodes: []string{
				"invalid-boot-node",
			},
			wantErr: true,
		},
		{
			name: "mixed valid and invalid",
			bootNodes: []string{
				"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.0.0.1:30303",
				"invalid-node",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := disc.UpdateBootNodes(tt.bootNodes)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, len(disc.bootNodes))
			}
		})
	}
}

// Integration test for basic discovery lifecycle.
func TestDiscV5_Lifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	disc := NewDiscV5(ctx, 5*time.Second, logger)

	// Test starting
	err := disc.Start(ctx)
	require.NoError(t, err)
	assert.True(t, disc.started)

	// Test double start (should be idempotent)
	err = disc.Start(ctx)
	require.NoError(t, err)

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	// Test stopping
	err = disc.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, disc.started)
}

// Test OnNodeRecord subscription.
func TestDiscV5_OnNodeRecord(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	disc := NewDiscV5(ctx, 30*time.Second, logger)

	// Set up a handler to capture emitted nodes
	receivedNodes := make([]*enode.Node, 0)
	disc.OnNodeRecord(ctx, func(ctx context.Context, node *enode.Node) error {
		receivedNodes = append(receivedNodes, node)

		return nil
	})

	// Create a test node
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	db, err := enode.OpenDB("")
	require.NoError(t, err)
	defer db.Close()

	ln := enode.NewLocalNode(db, privKey)
	ln.Set(enr.IP(net.ParseIP("8.8.8.8")))
	ln.Set(enr.TCP(30303))
	testNode := ln.Node()

	// Emit a node record directly
	disc.publishNodeRecord(ctx, testNode)

	// Give the event system time to process
	time.Sleep(10 * time.Millisecond)

	// Check that we received the node
	assert.Len(t, receivedNodes, 1)
	assert.Equal(t, testNode.ID(), receivedNodes[0].ID())
}

// Test error handling in OnNodeRecord.
func TestDiscV5_OnNodeRecord_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress error logs during test

	disc := NewDiscV5(ctx, 30*time.Second, logger)

	// Track if error handler was called
	errorHandled := false

	// Set up a handler that returns an error
	disc.OnNodeRecord(ctx, func(ctx context.Context, node *enode.Node) error {
		errorHandled = true

		return assert.AnError
	})

	// Create and emit a test node
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	db, err := enode.OpenDB("")
	require.NoError(t, err)
	defer db.Close()

	ln := enode.NewLocalNode(db, privKey)
	ln.Set(enr.IP(net.ParseIP("8.8.8.8")))
	ln.Set(enr.TCP(30303))
	testNode := ln.Node()

	disc.publishNodeRecord(ctx, testNode)

	// Give the event system time to process
	time.Sleep(10 * time.Millisecond)

	// Verify that the error handler was called
	assert.True(t, errorHandled, "Error handler should have been called")
}

// Test concurrent operations.
func TestDiscV5_ConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce log noise

	// Test concurrent UpdateBootNodes without Start/Stop
	t.Run("concurrent boot node updates", func(t *testing.T) {
		disc := NewDiscV5(ctx, 1*time.Hour, logger)
		var wg sync.WaitGroup
		wg.Add(3)

		// Multiple goroutines updating boot nodes
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				err := disc.UpdateBootNodes([]string{
					"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.0.0.1:30303",
				})
				assert.NoError(t, err)
			}
		}()

		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				err := disc.UpdateBootNodes([]string{
					"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.0.0.2:30303",
				})
				assert.NoError(t, err)
			}
		}()

		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				err := disc.UpdateBootNodes([]string{
					"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.0.0.3:30303",
					"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.0.0.4:30303",
				})
				assert.NoError(t, err)
			}
		}()

		wg.Wait()

		// Verify final state
		assert.True(t, len(disc.bootNodes) > 0)
	})

	// Test multiple instances don't interfere
	t.Run("multiple instances", func(t *testing.T) {
		instances := make([]*DiscV5, 3)
		for i := 0; i < 3; i++ {
			instances[i] = NewDiscV5(ctx, 1*time.Hour, logger)
		}

		var wg sync.WaitGroup
		wg.Add(len(instances))

		for idx, disc := range instances {
			go func(d *DiscV5, i int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					err := d.UpdateBootNodes([]string{
						fmt.Sprintf("enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.0.%d.1:30303", i),
					})
					assert.NoError(t, err)
				}
			}(disc, idx)
		}

		wg.Wait()
	})
}

// Test concurrent Start/Stop operations.
func TestDiscV5_ConcurrentStartStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	disc := NewDiscV5(ctx, 1*time.Hour, logger)

	// Use a mutex to ensure Start/Stop operations don't overlap
	var opMutex sync.Mutex
	var wg sync.WaitGroup
	wg.Add(3)

	// Start goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				opMutex.Lock()
				err := disc.Start(ctx)
				opMutex.Unlock()
				if err != nil {
					// It's OK if Start fails when already started
					t.Logf("Start error (iteration %d, expected): %v", i, err)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	// Stop goroutine
	go func() {
		defer wg.Done()
		// Wait a bit to ensure Start is called first
		time.Sleep(5 * time.Millisecond)
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				opMutex.Lock()
				err := disc.Stop(ctx)
				opMutex.Unlock()
				if err != nil {
					// It's OK if Stop fails when already stopped
					t.Logf("Stop error (iteration %d, expected): %v", i, err)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	// UpdateBootNodes goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				err := disc.UpdateBootNodes([]string{
					"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.0.0.1:30303",
				})
				if err != nil {
					// It's OK if UpdateBootNodes fails during Start/Stop
					t.Logf("UpdateBootNodes error (iteration %d, expected): %v", i, err)
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	wg.Wait()

	// Ensure we clean up properly
	opMutex.Lock()
	err := disc.Stop(ctx)
	opMutex.Unlock()

	require.NoError(t, err)
}
