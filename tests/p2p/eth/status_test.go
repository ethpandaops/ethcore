package eth_test

import (
	"testing"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeerStatus(t *testing.T) {
	// Create a test peer ID
	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	// Create a test status
	status := &common.Status{
		ForkDigest:     [4]byte{0x01, 0x02, 0x03, 0x04},
		FinalizedRoot:  [32]byte{},
		FinalizedEpoch: 100,
		HeadRoot:       [32]byte{},
		HeadSlot:       3200,
	}

	// Create eth.PeerStatus
	peerStatus := eth.PeerStatus{
		PeerID: peerID,
		Status: status,
	}

	// Test fields
	assert.Equal(t, peerID, peerStatus.PeerID)
	assert.Equal(t, status, peerStatus.Status)
	assert.Equal(t, common.ForkDigest{0x01, 0x02, 0x03, 0x04}, peerStatus.Status.ForkDigest)
	assert.Equal(t, common.Epoch(100), peerStatus.Status.FinalizedEpoch)
	assert.Equal(t, common.Slot(3200), peerStatus.Status.HeadSlot)
}

func TestPeerStatusNil(t *testing.T) {
	// Test with nil status
	peerStatus := eth.PeerStatus{
		PeerID: "",
		Status: nil,
	}

	assert.Empty(t, peerStatus.PeerID)
	assert.Nil(t, peerStatus.Status)
}

func TestPeerStatusMultiple(t *testing.T) {
	// Test creating multiple peer statuses
	peers := []struct {
		id    string
		epoch common.Epoch
		slot  common.Slot
	}{
		{"12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf", 100, 3200},
		{"12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN", 101, 3232},
		{"12D3KooWHCJbJKGDfCgHSoCuK9q4STyRnVveqLoXAPBbXHTZx9Cv", 102, 3264},
	}

	statuses := make([]eth.PeerStatus, len(peers))
	for i, p := range peers {
		peerID, err := peer.Decode(p.id)
		require.NoError(t, err)

		statuses[i] = eth.PeerStatus{
			PeerID: peerID,
			Status: &common.Status{
				ForkDigest:     [4]byte{0x01, 0x02, 0x03, 0x04},
				FinalizedRoot:  [32]byte{},
				FinalizedEpoch: p.epoch,
				HeadRoot:       [32]byte{},
				HeadSlot:       p.slot,
			},
		}
	}

	// Verify all statuses
	for i, status := range statuses {
		assert.Equal(t, peers[i].epoch, status.Status.FinalizedEpoch)
		assert.Equal(t, peers[i].slot, status.Status.HeadSlot)
	}
}
