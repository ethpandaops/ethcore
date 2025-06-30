package eth_test

import (
	"testing"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/stretchr/testify/assert"
)

func TestGoodbyeReasonConstants(t *testing.T) {
	// Test that reason codes have expected values
	assert.Equal(t, eth.GoodbyeReason(0), eth.GoodbyeReasonClientShutdown)
	assert.Equal(t, eth.GoodbyeReason(1), eth.GoodbyeReasonIrrelevantNetwork)
	assert.Equal(t, eth.GoodbyeReason(2), eth.GoodbyeReasonFaultError)
}

func TestGoodbyeReasonValues(t *testing.T) {
	// Test that we can use the constants
	tests := []struct {
		name   string
		reason eth.GoodbyeReason
		value  uint8
	}{
		{
			name:   "client shutdown",
			reason: eth.GoodbyeReasonClientShutdown,
			value:  0,
		},
		{
			name:   "irrelevant network",
			reason: eth.GoodbyeReasonIrrelevantNetwork,
			value:  1,
		},
		{
			name:   "fault error",
			reason: eth.GoodbyeReasonFaultError,
			value:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.value, uint8(tt.reason))
		})
	}
}

func TestGoodbyeReasonType(t *testing.T) {
	// Test that GoodbyeReason is uint8
	reason := eth.GoodbyeReasonClientShutdown
	assert.IsType(t, eth.GoodbyeReason(0), reason)

	// Test conversions
	u8 := uint8(reason)
	assert.Equal(t, uint8(0), u8)

	// Test that we can create custom reasons
	customReason := eth.GoodbyeReason(255)
	assert.Equal(t, uint8(255), uint8(customReason))
}
