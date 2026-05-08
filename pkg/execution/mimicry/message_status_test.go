package mimicry

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCode(t *testing.T) {
	expectedCode := RLPXOffset + eth.StatusMsg

	status68 := &Status68{}
	status69 := &Status69{}
	assert.Equal(t, expectedCode, status68.Code())
	assert.Equal(t, expectedCode, status69.Code())

	assert.Equal(t, StatusCode, expectedCode)
}

func TestStatus68Interface(t *testing.T) {
	genesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	head := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	forkIDHash := [4]byte{0xfc, 0x64, 0xec, 0x04}

	status := &Status68{
		Status68Packet: Status68Packet{
			ProtocolVersion: 68,
			NetworkID:       1,
			TD:              big.NewInt(0),
			Head:            head,
			Genesis:         genesis,
			ForkID: forkid.ID{
				Hash: forkIDHash,
				Next: 2000,
			},
		},
	}

	var _ Status = status

	assert.Equal(t, StatusCode, status.Code())
	assert.Equal(t, uint64(0), status.ReqID())
	assert.Equal(t, uint32(68), status.GetProtocolVersion())
	assert.Equal(t, genesis[:], status.GetGenesis())
	assert.Equal(t, head[:], status.GetHead())
	assert.Equal(t, uint64(1), status.GetNetworkID())
	assert.Equal(t, forkIDHash[:], status.GetForkIDHash())
	assert.Equal(t, uint64(2000), status.GetForkIDNext())
}

func TestStatus69Interface(t *testing.T) {
	genesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	latestBlockHash := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	forkIDHash := [4]byte{0xfc, 0x64, 0xec, 0x04}

	status := &Status69{
		StatusPacket: eth.StatusPacket{
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

	var _ Status = status

	assert.Equal(t, StatusCode, status.Code())
	assert.Equal(t, uint64(0), status.ReqID())
	assert.Equal(t, uint32(69), status.GetProtocolVersion())
	assert.Equal(t, genesis[:], status.GetGenesis())
	assert.Equal(t, latestBlockHash[:], status.GetHead())
	assert.Equal(t, uint64(1), status.GetNetworkID())
	assert.Equal(t, forkIDHash[:], status.GetForkIDHash())
	assert.Equal(t, uint64(2000), status.GetForkIDNext())
}

func TestStatusRLPEncoding68(t *testing.T) {
	genesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	head := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	forkIDHash := [4]byte{0xfc, 0x64, 0xec, 0x04}

	original := Status68Packet{
		ProtocolVersion: 68,
		NetworkID:       1,
		TD:              big.NewInt(0),
		Head:            head,
		Genesis:         genesis,
		ForkID: forkid.ID{
			Hash: forkIDHash,
			Next: 2000,
		},
	}

	encoded, err := rlp.EncodeToBytes(&original)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	var decoded Status68Packet
	err = rlp.DecodeBytes(encoded, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.ProtocolVersion, decoded.ProtocolVersion)
	assert.Equal(t, original.NetworkID, decoded.NetworkID)
	assert.Equal(t, original.TD, decoded.TD)
	assert.Equal(t, original.Head, decoded.Head)
	assert.Equal(t, original.Genesis, decoded.Genesis)
	assert.Equal(t, original.ForkID.Hash, decoded.ForkID.Hash)
	assert.Equal(t, original.ForkID.Next, decoded.ForkID.Next)
}

func TestStatusRLPEncoding69(t *testing.T) {
	genesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	latestBlockHash := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	forkIDHash := [4]byte{0xfc, 0x64, 0xec, 0x04}

	original := eth.StatusPacket{
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

	encoded, err := rlp.EncodeToBytes(&original)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	var decoded eth.StatusPacket
	err = rlp.DecodeBytes(encoded, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.ProtocolVersion, decoded.ProtocolVersion)
	assert.Equal(t, original.NetworkID, decoded.NetworkID)
	assert.Equal(t, original.Genesis, decoded.Genesis)
	assert.Equal(t, original.ForkID.Hash, decoded.ForkID.Hash)
	assert.Equal(t, original.ForkID.Next, decoded.ForkID.Next)
	assert.Equal(t, original.EarliestBlock, decoded.EarliestBlock)
	assert.Equal(t, original.LatestBlock, decoded.LatestBlock)
	assert.Equal(t, original.LatestBlockHash, decoded.LatestBlockHash)
}

func TestReceiveStatus70UsesRangeStatusPacket(t *testing.T) {
	genesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	latestBlockHash := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	forkIDHash := [4]byte{0xfc, 0x64, 0xec, 0x04}

	original := eth.StatusPacket{
		ProtocolVersion: 70,
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

	encoded, err := rlp.EncodeToBytes(&original)
	require.NoError(t, err)

	client := &Client{ethCapVersion: 70}
	status, err := client.receiveStatus(context.Background(), encoded)
	require.NoError(t, err)

	assert.Equal(t, uint32(70), status.GetProtocolVersion())
	assert.Equal(t, genesis[:], status.GetGenesis())
	assert.Equal(t, latestBlockHash[:], status.GetHead())
	assert.Equal(t, forkIDHash[:], status.GetForkIDHash())
	assert.Equal(t, uint64(2000), status.GetForkIDNext())
}

func TestStatusInterfaceCompliance(t *testing.T) {
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
			status69 := &Status69{
				StatusPacket: eth.StatusPacket{
					NetworkID: tt.networkID,
				},
			}
			assert.Equal(t, tt.networkID, status69.GetNetworkID())
		})
	}
}

func TestHandleStatusReturnsStatusProviderError(t *testing.T) {
	status := &Status69{
		StatusPacket: eth.StatusPacket{
			ProtocolVersion: 69,
			NetworkID:       56,
		},
	}
	encoded, err := rlp.EncodeToBytes(&status.StatusPacket)
	require.NoError(t, err)

	expected := errors.New("wrong network")
	client := &Client{
		ethCapVersion: 69,
		log:           logrus.New(),
		statusProvider: func(ctx context.Context, protocolVersion uint, peerStatus Status) (Status, error) {
			return nil, expected
		},
	}

	err = client.handleStatus(context.Background(), StatusCode, encoded)
	require.ErrorIs(t, err, expected)
}
