package p2p

import (
	"testing"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/libp2p/go-libp2p"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReqRespStructure(t *testing.T) {
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

	r := &ReqResp{
		log:     log,
		host:    h,
		encoder: encoder.SszNetworkEncoder{},
		config:  config,
	}

	// Test basic structure
	assert.NotNil(t, r.log)
	assert.NotNil(t, r.host)
	assert.NotNil(t, r.config)
	assert.Equal(t, config, r.config)
	assert.Empty(t, r.protocols)
}

func TestReqRespConfig(t *testing.T) {
	config := &ReqRespConfig{
		WriteTimeout:    1 * time.Second,
		ReadTimeout:     2 * time.Second,
		TimeToFirstByte: 500 * time.Millisecond,
	}

	assert.Equal(t, 1*time.Second, config.WriteTimeout)
	assert.Equal(t, 2*time.Second, config.ReadTimeout)
	assert.Equal(t, 500*time.Millisecond, config.TimeToFirstByte)
}

func TestEncoderPresent(t *testing.T) {
	// Just test that the encoder can be instantiated
	enc := encoder.SszNetworkEncoder{}
	assert.NotNil(t, enc)
}
