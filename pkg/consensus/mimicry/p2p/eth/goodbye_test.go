package eth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoodbyeReasonConstants(t *testing.T) {
	// Test that reason codes have expected values
	assert.Equal(t, GoodbyeReason(0), GoodbyeReasonClientShutdown)
	assert.Equal(t, GoodbyeReason(1), GoodbyeReasonIrrelevantNetwork)
	assert.Equal(t, GoodbyeReason(2), GoodbyeReasonFaultError)
}

func TestGoodbyeReasonValues(t *testing.T) {
	// Test that we can use the constants
	tests := []struct {
		name   string
		reason GoodbyeReason
		value  uint8
	}{
		{
			name:   "client shutdown",
			reason: GoodbyeReasonClientShutdown,
			value:  0,
		},
		{
			name:   "irrelevant network",
			reason: GoodbyeReasonIrrelevantNetwork,
			value:  1,
		},
		{
			name:   "fault error",
			reason: GoodbyeReasonFaultError,
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
	var reason GoodbyeReason = GoodbyeReasonClientShutdown
	assert.IsType(t, GoodbyeReason(0), reason)
	
	// Test conversions
	var u8 uint8 = uint8(reason)
	assert.Equal(t, uint8(0), u8)
	
	// Test that we can create custom reasons
	customReason := GoodbyeReason(255)
	assert.Equal(t, uint8(255), uint8(customReason))
}