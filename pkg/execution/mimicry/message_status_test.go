package mimicry

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCode(t *testing.T) {
	expectedCode := RLPXOffset + eth.StatusMsg

	status68 := &Status68{}
	assert.Equal(t, expectedCode, status68.Code())

	status69 := &Status69{}
	assert.Equal(t, expectedCode, status69.Code())

	assert.Equal(t, StatusCode, expectedCode)
}

func TestStatus68Interface(t *testing.T) {
	genesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	head := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	forkIDHash := [4]byte{0xfc, 0x64, 0xec, 0x04}

	status := &Status68{
		StatusPacket68: eth.StatusPacket68{
			ProtocolVersion: 68,
			NetworkID:       1,
			TD:              nil,
			Head:            head,
			Genesis:         genesis,
			ForkID: forkid.ID{
				Hash: forkIDHash,
				Next: 1000,
			},
		},
	}

	// Test interface compliance
	var _ Status = status

	// Test Code
	assert.Equal(t, StatusCode, status.Code())

	// Test ReqID
	assert.Equal(t, uint64(0), status.ReqID())

	// Test GetGenesis
	assert.Equal(t, genesis[:], status.GetGenesis())

	// Test GetHead
	assert.Equal(t, head[:], status.GetHead())

	// Test GetNetworkID
	assert.Equal(t, uint64(1), status.GetNetworkID())

	// Test GetForkIDHash
	assert.Equal(t, forkIDHash[:], status.GetForkIDHash())

	// Test GetForkIDNext
	assert.Equal(t, uint64(1000), status.GetForkIDNext())
}

func TestStatus69Interface(t *testing.T) {
	genesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	latestBlockHash := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	forkIDHash := [4]byte{0xfc, 0x64, 0xec, 0x04}

	status := &Status69{
		StatusPacket69: eth.StatusPacket69{
			ProtocolVersion: 69,
			NetworkID:       1,
			Genesis:         genesis,
			ForkID: forkid.ID{
				Hash: forkIDHash,
				Next: 2000,
			},
			EarliestBlock:   100,
			LatestBlock:     500,
			LatestBlockHash: latestBlockHash,
		},
	}

	// Test interface compliance
	var _ Status = status

	// Test Code
	assert.Equal(t, StatusCode, status.Code())

	// Test ReqID
	assert.Equal(t, uint64(0), status.ReqID())

	// Test GetGenesis
	assert.Equal(t, genesis[:], status.GetGenesis())

	// Test GetHead - for Status69, this returns LatestBlockHash
	assert.Equal(t, latestBlockHash[:], status.GetHead())

	// Test GetNetworkID
	assert.Equal(t, uint64(1), status.GetNetworkID())

	// Test GetForkIDHash
	assert.Equal(t, forkIDHash[:], status.GetForkIDHash())

	// Test GetForkIDNext
	assert.Equal(t, uint64(2000), status.GetForkIDNext())
}

func TestStatusRLPEncoding68(t *testing.T) {
	genesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	head := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	forkIDHash := [4]byte{0xfc, 0x64, 0xec, 0x04}

	original := eth.StatusPacket68{
		ProtocolVersion: 68,
		NetworkID:       1,
		Head:            head,
		Genesis:         genesis,
		ForkID: forkid.ID{
			Hash: forkIDHash,
			Next: 1000,
		},
	}

	// Encode
	encoded, err := rlp.EncodeToBytes(&original)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// Decode
	var decoded eth.StatusPacket68
	err = rlp.DecodeBytes(encoded, &decoded)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, original.ProtocolVersion, decoded.ProtocolVersion)
	assert.Equal(t, original.NetworkID, decoded.NetworkID)
	assert.Equal(t, original.Head, decoded.Head)
	assert.Equal(t, original.Genesis, decoded.Genesis)
	assert.Equal(t, original.ForkID.Hash, decoded.ForkID.Hash)
	assert.Equal(t, original.ForkID.Next, decoded.ForkID.Next)
}

func TestStatusRLPEncoding69(t *testing.T) {
	genesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	latestBlockHash := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	forkIDHash := [4]byte{0xfc, 0x64, 0xec, 0x04}

	original := eth.StatusPacket69{
		ProtocolVersion: 69,
		NetworkID:       1,
		Genesis:         genesis,
		ForkID: forkid.ID{
			Hash: forkIDHash,
			Next: 2000,
		},
		EarliestBlock:   100,
		LatestBlock:     500,
		LatestBlockHash: latestBlockHash,
	}

	// Encode
	encoded, err := rlp.EncodeToBytes(&original)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// Decode
	var decoded eth.StatusPacket69
	err = rlp.DecodeBytes(encoded, &decoded)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, original.ProtocolVersion, decoded.ProtocolVersion)
	assert.Equal(t, original.NetworkID, decoded.NetworkID)
	assert.Equal(t, original.Genesis, decoded.Genesis)
	assert.Equal(t, original.ForkID.Hash, decoded.ForkID.Hash)
	assert.Equal(t, original.ForkID.Next, decoded.ForkID.Next)
	assert.Equal(t, original.EarliestBlock, decoded.EarliestBlock)
	assert.Equal(t, original.LatestBlock, decoded.LatestBlock)
	assert.Equal(t, original.LatestBlockHash, decoded.LatestBlockHash)
}

func TestStatusInterfaceCompliance(t *testing.T) {
	// Ensure both Status68 and Status69 implement the Status interface
	var _ Status = (*Status68)(nil)
	var _ Status = (*Status69)(nil)
}

func TestStatusDifferentNetworks(t *testing.T) {
	tests := []struct {
		name      string
		networkID uint64
	}{
		{"mainnet", 1},
		{"sepolia", 11155111},
		{"holesky", 17000},
		{"goerli", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status68 := &Status68{
				StatusPacket68: eth.StatusPacket68{
					NetworkID: tt.networkID,
				},
			}
			assert.Equal(t, tt.networkID, status68.GetNetworkID())

			status69 := &Status69{
				StatusPacket69: eth.StatusPacket69{
					NetworkID: tt.networkID,
				},
			}
			assert.Equal(t, tt.networkID, status69.GetNetworkID())
		})
	}
}
