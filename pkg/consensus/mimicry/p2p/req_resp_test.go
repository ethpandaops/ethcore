package p2p

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
		"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReqResp(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	defer h.Close()
	
	config := &ReqRespConfig{
		WriteTimeout:    5 * time.Second,
		ReadTimeout:     5 * time.Second,
		TimeToFirstByte: 500 * time.Millisecond,
	}
	
	r := NewReqResp(log, h, encoder.SszNetworkEncoder{}, config)
	
	assert.NotNil(t, r)
	assert.Equal(t, h, r.host)
	assert.NotNil(t, r.encoder)
	assert.Equal(t, config, r.config)
	assert.Empty(t, r.protocols)
}

func TestSupportedProtocols(t *testing.T) {
	r := &ReqResp{
		protocols: []protocol.ID{"/test/1", "/test/2"},
	}
	
	protocols := r.SupportedProtocols()
	assert.Equal(t, []protocol.ID{"/test/1", "/test/2"}, protocols)
}

func TestRegisterHandler(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	defer h.Close()
	
	config := &ReqRespConfig{
		WriteTimeout:    5 * time.Second,
		ReadTimeout:     5 * time.Second,
		TimeToFirstByte: 500 * time.Millisecond,
	}
	
	r := NewReqResp(log, h, encoder.SszNetworkEncoder{}, config)
	
	tests := []struct {
		name        string
		protocol    protocol.ID
		handler     func(context.Context, network.Stream) error
		expectError bool
		errorMsg    string
	}{
		{
			name:     "successful registration",
			protocol: "/test/1",
			handler: func(ctx context.Context, stream network.Stream) error {
				return nil
			},
			expectError: false,
		},
		{
			name:     "duplicate protocol",
			protocol: "/test/1",
			handler: func(ctx context.Context, stream network.Stream) error {
				return nil
			},
			expectError: true,
			errorMsg:    "protocol already registered: /test/1",
		},
		{
			name:     "another successful registration",
			protocol: "/test/2",
			handler: func(ctx context.Context, stream network.Stream) error {
				return nil
			},
			expectError: false,
		},
	}
	
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.RegisterHandler(ctx, tt.protocol, tt.handler)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, r.protocols, tt.protocol)
			}
		})
	}
}

// mockHost implements a minimal host.Host for testing
type mockHost struct {
	host.Host
	streamHandlers map[protocol.ID]network.StreamHandler
}

func newMockHost() *mockHost {
	return &mockHost{
		streamHandlers: make(map[protocol.ID]network.StreamHandler),
	}
}

func (m *mockHost) SetStreamHandler(pid protocol.ID, handler network.StreamHandler) {
	m.streamHandlers[pid] = handler
}

func (m *mockHost) NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
	return nil, errors.New("not implemented")
}

func TestWrapper(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	
	h := newMockHost()
	
	config := &ReqRespConfig{
		WriteTimeout:    5 * time.Second,
		ReadTimeout:     5 * time.Second,
		TimeToFirstByte: 500 * time.Millisecond,
	}
	
	r := &ReqResp{
		log:    log,
		host:   h,
		config: config,
	}
	
	handler := func(ctx context.Context, stream network.Stream) error {
		return nil
	}
	
	wrapped := r.wrapper(context.Background(), handler)
	
	// Test wrapper functionality by ensuring it doesn't panic
	// The wrapper function should return a valid StreamHandler
	assert.NotNil(t, wrapped)
}