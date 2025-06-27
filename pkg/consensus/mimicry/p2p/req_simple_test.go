package p2p

import (
	"testing"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestRequestErrorFunctionality(t *testing.T) {
	// Test basic error
	err := &RequestError{Msg: "test error", Type: "test error"}
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
	assert.NotNil(t, ErrInvalidResponse)
	assert.Equal(t, "received invalid response", ErrInvalidResponse.Error())
	
	assert.NotNil(t, ErrFailedToCreateStream)
	assert.Equal(t, "failed to create stream", ErrFailedToCreateStream.Error())
	
	assert.NotNil(t, ErrFailedToEncodeRequest)
	assert.Equal(t, "failed to encode request", ErrFailedToEncodeRequest.Error())
	
	assert.NotNil(t, ErrFailedToReadResponse)
	assert.Equal(t, "failed to read response", ErrFailedToReadResponse.Error())
	
	assert.NotNil(t, ErrFailedToDecodeResponse)
	assert.Equal(t, "failed to decode response", ErrFailedToDecodeResponse.Error())
	
	assert.NotNil(t, ErrFailedToCloseWriteStream)
	assert.Equal(t, "failed to close write stream", ErrFailedToCloseWriteStream.Error())
}

func TestCreateReqResp(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	defer h.Close()
	
	r := &ReqResp{
		log:     log,
		host:    h,
		encoder: encoder.SszNetworkEncoder{},
		config: &ReqRespConfig{
			WriteTimeout:    5 * time.Second,
			ReadTimeout:     5 * time.Second,
			TimeToFirstByte: 500 * time.Millisecond,
		},
	}
	
	assert.NotNil(t, r)
	assert.Equal(t, h, r.host)
	assert.NotNil(t, r.encoder)
	assert.Empty(t, r.protocols)
}

func TestRequestStruct(t *testing.T) {
	peerID, err := peer.Decode("12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	require.NoError(t, err)
	
	req := &Request{
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
	err := &RequestError{
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
	err := &RequestError{Msg: msg, Type: msg}
	
	assert.Equal(t, msg, err.Msg)
	assert.Equal(t, msg, err.Type)
	assert.Equal(t, msg, err.Error())
}