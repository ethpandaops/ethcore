package crawler

import (
	"net"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethpandaops/ethcore/pkg/discovery"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrawler_GetPeerENR(t *testing.T) {
	// Create a simple crawler instance with just the ENR storage initialized
	c := &Crawler{
		peerENRs: make(map[peer.ID]*enode.Node),
	}

	// Create a test ENR
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	record := &enr.Record{}
	record.Set(enr.IP(net.ParseIP("192.168.1.1")))
	record.Set(enr.TCP(30303))
	require.NoError(t, enode.SignV4(record, privKey))

	node, err := enode.New(enode.ValidSchemes, record)
	require.NoError(t, err)

	// Derive peer info
	connectablePeer, err := discovery.DeriveDetailsFromNode(node)
	require.NoError(t, err)

	peerID := connectablePeer.AddrInfo.ID

	// Test 1: ENR should be nil for unknown peer
	assert.Nil(t, c.GetPeerENR(peerID))

	// Test 2: Store ENR
	c.peerENRsMu.Lock()
	c.peerENRs[peerID] = node
	c.peerENRsMu.Unlock()

	// Test 3: Retrieved ENR should match stored ENR
	retrievedENR := c.GetPeerENR(peerID)
	assert.NotNil(t, retrievedENR)
	assert.Equal(t, node.ID().String(), retrievedENR.ID().String())

	// Test 4: Delete ENR
	c.peerENRsMu.Lock()
	delete(c.peerENRs, peerID)
	c.peerENRsMu.Unlock()

	// Test 5: ENR should be nil after deletion
	assert.Nil(t, c.GetPeerENR(peerID))
}

func TestCrawler_ENRConcurrency(t *testing.T) {
	// Test concurrent access to ENR storage
	c := &Crawler{
		peerENRs: make(map[peer.ID]*enode.Node),
	}

	// Create multiple test ENRs
	numPeers := 10
	peers := make([]peer.ID, numPeers)
	nodes := make([]*enode.Node, numPeers)

	for i := 0; i < numPeers; i++ {
		privKey, err := crypto.GenerateKey()
		require.NoError(t, err)

		record := &enr.Record{}
		record.Set(enr.IP(net.ParseIP("192.168.1.1")))
		record.Set(enr.TCP(uint16(30303 + i)))
		require.NoError(t, enode.SignV4(record, privKey))

		node, err := enode.New(enode.ValidSchemes, record)
		require.NoError(t, err)

		connectablePeer, err := discovery.DeriveDetailsFromNode(node)
		require.NoError(t, err)

		peers[i] = connectablePeer.AddrInfo.ID
		nodes[i] = node
	}

	// Concurrently store and retrieve ENRs
	done := make(chan struct{})

	// Writer goroutine
	go func() {
		for i := 0; i < numPeers; i++ {
			c.peerENRsMu.Lock()
			c.peerENRs[peers[i]] = nodes[i]
			c.peerENRsMu.Unlock()
		}
		close(done)
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			for j := 0; j < numPeers; j++ {
				_ = c.GetPeerENR(peers[j])
			}
		}
	}()

	// Wait for writer to complete
	<-done

	// Verify all ENRs were stored correctly
	for i := 0; i < numPeers; i++ {
		enr := c.GetPeerENR(peers[i])
		assert.NotNil(t, enr)
		assert.Equal(t, nodes[i].ID().String(), enr.ID().String())
	}
}
