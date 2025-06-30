package p2p_test

import (
	"testing"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestErrorFunctionality(t *testing.T) {
	// Test basic error
	err := &p2p.RequestError{Msg: "test error", Type: "test error"}
	assert.Equal(t, "test error", err.Error())
	assert.Equal(t, "test error", err.Type)

	// Test Add method
	err2 := err.Add("additional info")
	assert.Equal(t, "test error: additional info", err2.Error())
	assert.Equal(t, "test error", err2.Type) // Type should remain the same

	// Test Unwrap
	unwrapped := err.Unwrap()
	assert.Equal(t, "test error", unwrapped.Error())
}

func TestPredefinedErrorsExist(t *testing.T) {
	// Test that predefined errors are properly initialized
	assert.NotNil(t, p2p.ErrInvalidResponse)
	assert.Equal(t, "received invalid response", p2p.ErrInvalidResponse.Error())

	assert.NotNil(t, p2p.ErrFailedToCreateStream)
	assert.Equal(t, "failed to create stream", p2p.ErrFailedToCreateStream.Error())

	assert.NotNil(t, p2p.ErrFailedToEncodeRequest)
	assert.Equal(t, "failed to encode request", p2p.ErrFailedToEncodeRequest.Error())

	assert.NotNil(t, p2p.ErrFailedToReadResponse)
	assert.Equal(t, "failed to read response", p2p.ErrFailedToReadResponse.Error())

	assert.NotNil(t, p2p.ErrFailedToDecodeResponse)
	assert.Equal(t, "failed to decode response", p2p.ErrFailedToDecodeResponse.Error())

	assert.NotNil(t, p2p.ErrFailedToCloseWriteStream)
	assert.Equal(t, "failed to close write stream", p2p.ErrFailedToCloseWriteStream.Error())
}

func TestCreateReqResp(t *testing.T) {
	// Since we can't access unexported fields from external tests,
	// we'll test ReqResp creation through the public constructor
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
	// Test that it starts with no protocols
	protocols := r.SupportedProtocols()
	assert.Empty(t, protocols)
}

func TestRequestStruct(t *testing.T) {
	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)

	req := &p2p.Request{
		ProtocolID: "/test/1",
		PeerID:     peerID,
		Payload:    nil, // We'll test with nil payload since we don't have access to internal mock
		Timeout:    5 * time.Second,
	}

	assert.Equal(t, protocol.ID("/test/1"), req.ProtocolID)
	assert.Equal(t, peerID, req.PeerID)
	assert.Equal(t, 5*time.Second, req.Timeout)
}

func TestRequestErrorMethods(t *testing.T) {
	err := &p2p.RequestError{
		Msg:  "original error",
		Type: "original error",
	}

	// Test Error() method
	assert.Equal(t, "original error", err.Error())

	// Test Add() method
	newErr := err.Add("additional context")
	assert.Equal(t, "original error: additional context", newErr.Error())
	assert.Equal(t, "original error", newErr.Type)

	// Test Unwrap() method
	unwrapped := err.Unwrap()
	assert.Equal(t, "original error", unwrapped.Error())
}

func TestNewRequestError(t *testing.T) {
	msg := "test message"
	err := &p2p.RequestError{Msg: msg, Type: msg}

	assert.Equal(t, msg, err.Msg)
	assert.Equal(t, msg, err.Type)
	assert.Equal(t, msg, err.Error())
}
