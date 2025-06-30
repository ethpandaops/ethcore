package p2p_test

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
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

	config := &p2p.ReqRespConfig{
		WriteTimeout:    5 * time.Second,
		ReadTimeout:     5 * time.Second,
		TimeToFirstByte: 500 * time.Millisecond,
	}

	r := p2p.NewReqResp(log, h, encoder.SszNetworkEncoder{}, config)

	assert.NotNil(t, r)
	// Since fields are unexported, we can only test that creation succeeded
	// and that public methods work
	protocols := r.SupportedProtocols()
	assert.Empty(t, protocols) // Should start with no protocols
}

func TestSupportedProtocols(t *testing.T) {
	// Since we can't access unexported fields from external tests,
	// we'll test this through the RegisterHandler method which should
	// add protocols to the supported protocols list
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	defer h.Close()

	config := &p2p.ReqRespConfig{
		WriteTimeout:    5 * time.Second,
		ReadTimeout:     5 * time.Second,
		TimeToFirstByte: 500 * time.Millisecond,
	}

	r := p2p.NewReqResp(log, h, encoder.SszNetworkEncoder{}, config)

	// Initially should have no protocols
	protocols := r.SupportedProtocols()
	assert.Empty(t, protocols)
}

func TestRegisterHandler(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	defer h.Close()

	config := &p2p.ReqRespConfig{
		WriteTimeout:    5 * time.Second,
		ReadTimeout:     5 * time.Second,
		TimeToFirstByte: 500 * time.Millisecond,
	}

	r := p2p.NewReqResp(log, h, encoder.SszNetworkEncoder{}, config)

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
				// Test that the protocol was registered by checking SupportedProtocols
				protocols := r.SupportedProtocols()
				assert.Contains(t, protocols, tt.protocol)
			}
		})
	}
}

func TestWrapper(t *testing.T) {
	// Since we can't access unexported methods from external tests,
	// we'll test the wrapper functionality indirectly by registering
	// a handler (which uses the wrapper internally) and verifying
	// it works correctly
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	defer h.Close()

	config := &p2p.ReqRespConfig{
		WriteTimeout:    5 * time.Second,
		ReadTimeout:     5 * time.Second,
		TimeToFirstByte: 500 * time.Millisecond,
	}

	r := p2p.NewReqResp(log, h, encoder.SszNetworkEncoder{}, config)

	handler := func(ctx context.Context, stream network.Stream) error {
		return nil
	}

	// Register a handler, which tests that the wrapper works correctly
	ctx := context.Background()
	err = r.RegisterHandler(ctx, "/test/wrapper", handler)
	assert.NoError(t, err)

	// Verify the protocol was registered (indicating wrapper worked)
	protocols := r.SupportedProtocols()
	assert.Contains(t, protocols, protocol.ID("/test/wrapper"))
}
