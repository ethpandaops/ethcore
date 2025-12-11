package mimicry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPingCode(t *testing.T) {
	// PingCode should be 0x02 per RLPx spec
	assert.Equal(t, 0x02, PingCode)
}

func TestPongCode(t *testing.T) {
	// PongCode should be 0x03 per RLPx spec
	assert.Equal(t, 0x03, PongCode)
}

func TestPingStruct(t *testing.T) {
	// Ping struct should exist and be empty
	ping := Ping{}
	_ = ping
}

func TestPongStruct(t *testing.T) {
	// Pong struct should exist and be empty
	pong := Pong{}
	_ = pong
}

func TestPingPongCodeValues(t *testing.T) {
	// RLPx base protocol codes
	// https://github.com/ethereum/devp2p/blob/master/rlpx.md
	tests := []struct {
		name     string
		code     int
		expected int
	}{
		{"HelloCode", HelloCode, 0x00},
		{"DisconnectCode", DisconnectCode, 0x01},
		{"PingCode", PingCode, 0x02},
		{"PongCode", PongCode, 0x03},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code)
		})
	}
}
