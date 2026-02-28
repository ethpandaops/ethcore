package mimicry

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStatus implements the Status interface for testing.
type mockStatus struct {
	networkID  uint64
	forkIDHash []byte
	forkIDNext uint64
	genesis    []byte
	head       []byte
}

func (m *mockStatus) Code() int             { return StatusCode }
func (m *mockStatus) ReqID() uint64         { return 0 }
func (m *mockStatus) GetNetworkID() uint64  { return m.networkID }
func (m *mockStatus) GetForkIDHash() []byte { return m.forkIDHash }
func (m *mockStatus) GetForkIDNext() uint64 { return m.forkIDNext }
func (m *mockStatus) GetGenesis() []byte    { return m.genesis }
func (m *mockStatus) GetHead() []byte       { return m.head }

func ptr[T any](v T) *T {
	return &v
}

func TestNetworkMatches(t *testing.T) {
	mainnetGenesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	sepoliaGenesis := common.HexToHash("0x25a5cc106eea7138acab33231d7160d69cb777ee0c2c553fcddf5138993e6dd9")
	dencunForkHash := []byte{0x9f, 0x3d, 0x22, 0x54}

	tests := []struct {
		name    string
		network Network
		status  Status
		want    bool
	}{
		{
			name:    "empty filter matches any network",
			network: Network{},
			status: &mockStatus{
				networkID:  1,
				genesis:    mainnetGenesis[:],
				forkIDHash: dencunForkHash,
				forkIDNext: 0,
			},
			want: true,
		},
		{
			name: "network ID filter - match",
			network: Network{
				NetworkID: ptr(uint64(1)),
			},
			status: &mockStatus{
				networkID: 1,
				genesis:   mainnetGenesis[:],
			},
			want: true,
		},
		{
			name: "network ID filter - no match",
			network: Network{
				NetworkID: ptr(uint64(1)),
			},
			status: &mockStatus{
				networkID: 11155111, // sepolia
				genesis:   sepoliaGenesis[:],
			},
			want: false,
		},
		{
			name: "genesis filter - match",
			network: Network{
				Genesis: mainnetGenesis[:],
			},
			status: &mockStatus{
				networkID: 1,
				genesis:   mainnetGenesis[:],
			},
			want: true,
		},
		{
			name: "genesis filter - no match",
			network: Network{
				Genesis: mainnetGenesis[:],
			},
			status: &mockStatus{
				networkID: 11155111,
				genesis:   sepoliaGenesis[:],
			},
			want: false,
		},
		{
			name: "fork ID hash filter - match",
			network: Network{
				ForkIDHash: dencunForkHash,
			},
			status: &mockStatus{
				networkID:  1,
				forkIDHash: dencunForkHash,
			},
			want: true,
		},
		{
			name: "fork ID hash filter - no match",
			network: Network{
				ForkIDHash: dencunForkHash,
			},
			status: &mockStatus{
				networkID:  1,
				forkIDHash: []byte{0x00, 0x00, 0x00, 0x00},
			},
			want: false,
		},
		{
			name: "fork ID next filter - match",
			network: Network{
				ForkIDNext: ptr(uint64(1000)),
			},
			status: &mockStatus{
				networkID:  1,
				forkIDNext: 1000,
			},
			want: true,
		},
		{
			name: "fork ID next filter - no match",
			network: Network{
				ForkIDNext: ptr(uint64(1000)),
			},
			status: &mockStatus{
				networkID:  1,
				forkIDNext: 2000,
			},
			want: false,
		},
		{
			name: "multiple filters - all match",
			network: Network{
				NetworkID:  ptr(uint64(1)),
				Genesis:    mainnetGenesis[:],
				ForkIDHash: dencunForkHash,
			},
			status: &mockStatus{
				networkID:  1,
				genesis:    mainnetGenesis[:],
				forkIDHash: dencunForkHash,
			},
			want: true,
		},
		{
			name: "multiple filters - one doesn't match",
			network: Network{
				NetworkID:  ptr(uint64(1)),
				Genesis:    mainnetGenesis[:],
				ForkIDHash: dencunForkHash,
			},
			status: &mockStatus{
				networkID:  1,
				genesis:    sepoliaGenesis[:], // doesn't match
				forkIDHash: dencunForkHash,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.network.Matches(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	require.NotNil(t, config)
	assert.Equal(t, 5*time.Second, config.HandshakeTimeout)
	assert.Equal(t, 30*time.Second, config.ReadTimeout)
	assert.Empty(t, config.ListenAddr)
	assert.Empty(t, config.Name)
	assert.Nil(t, config.PrivateKey)
	assert.Nil(t, config.StatusProvider)
	assert.Empty(t, config.AllowedNetworks)
	assert.Equal(t, 0, config.MaxPeers)
}
