package mimicry

import (
	"context"
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel) // Suppress logs during tests

	return log
}

func testStatusProvider(hello *Hello) (Status, error) {
	return &Status69{
		StatusPacket69: eth.StatusPacket69{
			NetworkID:       1,
			Genesis:         common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3"),
			LatestBlockHash: common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
	}, nil
}

func TestNewServer(t *testing.T) {
	log := testLogger()

	tests := []struct {
		name    string
		config  *ServerConfig
		wantErr bool
		errMsg  string
		checks  func(t *testing.T, server *Server)
	}{
		{
			name: "valid config creates server",
			config: &ServerConfig{
				Name:           "test-server",
				ListenAddr:     ":0", // Use any available port
				StatusProvider: testStatusProvider,
			},
			wantErr: false,
			checks: func(t *testing.T, server *Server) {
				t.Helper()
				assert.NotNil(t, server.broker)
				assert.NotNil(t, server.privateKey)
				assert.NotNil(t, server.peers)
				assert.NotNil(t, server.done)
				assert.Equal(t, 5*time.Second, server.config.HandshakeTimeout)
				assert.Equal(t, 30*time.Second, server.config.ReadTimeout)
			},
		},
		{
			name: "valid config with custom private key",
			config: func() *ServerConfig {
				key, _ := crypto.GenerateKey()

				return &ServerConfig{
					Name:           "test-server",
					ListenAddr:     ":0",
					StatusProvider: testStatusProvider,
					PrivateKey:     key,
				}
			}(),
			wantErr: false,
			checks: func(t *testing.T, server *Server) {
				t.Helper()
				assert.NotNil(t, server.privateKey)
			},
		},
		{
			name: "valid config with custom timeouts",
			config: &ServerConfig{
				Name:             "test-server",
				ListenAddr:       ":0",
				StatusProvider:   testStatusProvider,
				HandshakeTimeout: 10 * time.Second,
				ReadTimeout:      60 * time.Second,
			},
			wantErr: false,
			checks: func(t *testing.T, server *Server) {
				t.Helper()
				assert.Equal(t, 10*time.Second, server.config.HandshakeTimeout)
				assert.Equal(t, 60*time.Second, server.config.ReadTimeout)
			},
		},
		{
			name: "valid config with max peers",
			config: &ServerConfig{
				Name:           "test-server",
				ListenAddr:     ":0",
				StatusProvider: testStatusProvider,
				MaxPeers:       100,
			},
			wantErr: false,
			checks: func(t *testing.T, server *Server) {
				t.Helper()
				assert.Equal(t, 100, server.config.MaxPeers)
			},
		},
		{
			name: "valid config with allowed networks",
			config: &ServerConfig{
				Name:           "test-server",
				ListenAddr:     ":0",
				StatusProvider: testStatusProvider,
				AllowedNetworks: []Network{
					{NetworkID: ptr(uint64(1))},
					{NetworkID: ptr(uint64(11155111))},
				},
			},
			wantErr: false,
			checks: func(t *testing.T, server *Server) {
				t.Helper()
				assert.Len(t, server.config.AllowedNetworks, 2)
			},
		},
		{
			name: "missing StatusProvider returns error",
			config: &ServerConfig{
				Name:       "test-server",
				ListenAddr: ":0",
			},
			wantErr: true,
			errMsg:  "StatusProvider is required",
		},
		{
			name: "missing ListenAddr returns error",
			config: &ServerConfig{
				Name:           "test-server",
				StatusProvider: testStatusProvider,
			},
			wantErr: true,
			errMsg:  "ListenAddr is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(log, tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, server)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, server)

			if tt.checks != nil {
				tt.checks(t, server)
			}
		})
	}
}

func TestServerStartStop(t *testing.T) {
	log := testLogger()

	config := &ServerConfig{
		Name:           "test-server",
		ListenAddr:     "127.0.0.1:0", // Use any available port on localhost
		StatusProvider: testStatusProvider,
	}

	server, err := NewServer(log, config)
	require.NoError(t, err)

	ctx := context.Background()

	// Start server
	err = server.Start(ctx)
	require.NoError(t, err)

	// Verify server is listening
	assert.NotNil(t, server.listener)
	assert.False(t, server.closed.Load())

	// Get the actual address
	addr := server.listener.Addr()
	assert.NotNil(t, addr)

	// Stop server
	err = server.Stop(ctx)
	require.NoError(t, err)

	// Verify server is stopped
	assert.True(t, server.closed.Load())
}

func TestServerDoubleStop(t *testing.T) {
	log := testLogger()

	config := &ServerConfig{
		Name:           "test-server",
		ListenAddr:     "127.0.0.1:0",
		StatusProvider: testStatusProvider,
	}

	server, err := NewServer(log, config)
	require.NoError(t, err)

	ctx := context.Background()

	err = server.Start(ctx)
	require.NoError(t, err)

	// Stop twice - should not error
	err = server.Stop(ctx)
	require.NoError(t, err)

	err = server.Stop(ctx)
	require.NoError(t, err) // Second stop should be idempotent
}

func TestServerPeersAndCount(t *testing.T) {
	log := testLogger()

	config := &ServerConfig{
		Name:           "test-server",
		ListenAddr:     "127.0.0.1:0",
		StatusProvider: testStatusProvider,
	}

	server, err := NewServer(log, config)
	require.NoError(t, err)

	ctx := context.Background()

	err = server.Start(ctx)
	require.NoError(t, err)

	defer func() {
		_ = server.Stop(ctx)
	}()

	// Initially no peers
	assert.Equal(t, 0, server.PeerCount())
	assert.Empty(t, server.Peers())

	// Broker should be accessible
	assert.NotNil(t, server.Broker())
}

func TestServerIsNetworkAllowed(t *testing.T) {
	log := testLogger()
	mainnetGenesis := common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	sepoliaGenesis := common.HexToHash("0x25a5cc106eea7138acab33231d7160d69cb777ee0c2c553fcddf5138993e6dd9")

	tests := []struct {
		name            string
		allowedNetworks []Network
		status          Status
		want            bool
	}{
		{
			name:            "no filter allows all networks",
			allowedNetworks: nil,
			status: &mockStatus{
				networkID: 1,
				genesis:   mainnetGenesis[:],
			},
			want: true,
		},
		{
			name: "filter allows matching network",
			allowedNetworks: []Network{
				{NetworkID: ptr(uint64(1))},
			},
			status: &mockStatus{
				networkID: 1,
				genesis:   mainnetGenesis[:],
			},
			want: true,
		},
		{
			name: "filter rejects non-matching network",
			allowedNetworks: []Network{
				{NetworkID: ptr(uint64(1))},
			},
			status: &mockStatus{
				networkID: 11155111,
				genesis:   sepoliaGenesis[:],
			},
			want: false,
		},
		{
			name: "multiple filters - matches one",
			allowedNetworks: []Network{
				{NetworkID: ptr(uint64(1))},
				{NetworkID: ptr(uint64(11155111))},
			},
			status: &mockStatus{
				networkID: 11155111,
				genesis:   sepoliaGenesis[:],
			},
			want: true,
		},
		{
			name: "multiple filters - matches none",
			allowedNetworks: []Network{
				{NetworkID: ptr(uint64(1))},
				{NetworkID: ptr(uint64(11155111))},
			},
			status: &mockStatus{
				networkID: 5, // goerli
				genesis:   common.Hash{}.Bytes(),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ServerConfig{
				Name:            "test-server",
				ListenAddr:      ":0",
				StatusProvider:  testStatusProvider,
				AllowedNetworks: tt.allowedNetworks,
			}

			server, err := NewServer(log, config)
			require.NoError(t, err)

			got := server.isNetworkAllowed(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestServerPeerMethods(t *testing.T) {
	// Test ServerPeer accessor methods
	peer := &ServerPeer{
		remoteID: [32]byte{1, 2, 3, 4},
		remotePubkey: func() *ecdsa.PublicKey {
			key, _ := crypto.GenerateKey()

			return &key.PublicKey
		}(),
	}

	assert.Equal(t, [32]byte{1, 2, 3, 4}, [32]byte(peer.RemoteID()))
	assert.NotNil(t, peer.RemotePubkey())
}
