package beacon_test

import (
	"testing"

	"github.com/ethpandaops/ethcore/pkg/ethereum/beacon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateBeaconTraceID(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid_http_address",
			address: "http://beacon-node:5052",
			wantErr: false,
		},
		{
			name:    "valid_https_address",
			address: "https://beacon-node.example.com:5052",
			wantErr: false,
		},
		{
			name:    "valid_localhost_address",
			address: "http://localhost:5052",
			wantErr: false,
		},
		{
			name:    "valid_ip_address",
			address: "http://192.168.1.100:5052",
			wantErr: false,
		},
		{
			name:    "empty_address",
			address: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceID, err := beacon.GenerateBeaconTraceID(tt.address)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, traceID)
				assert.Len(t, traceID, 8)
			}
		})
	}
}

func TestGenerateBeaconTraceID_Consistency(t *testing.T) {
	address := "http://beacon-node:5052"

	traceID1, err := beacon.GenerateBeaconTraceID(address)
	require.NoError(t, err)

	traceID2, err := beacon.GenerateBeaconTraceID(address)
	require.NoError(t, err)

	assert.Equal(t, traceID1, traceID2, "same address should generate same trace ID")
}

func TestGenerateBeaconTraceIDs(t *testing.T) {
	tests := []struct {
		name        string
		addresses   []string
		wantErr     bool
		errContains string
		wantLen     int
	}{
		{
			name: "multiple_unique_addresses",
			addresses: []string{
				"http://beacon1:5052",
				"http://beacon2:5052",
				"http://beacon3:5052",
			},
			wantErr: false,
			wantLen: 3,
		},
		{
			name: "duplicate_addresses",
			addresses: []string{
				"http://beacon:5052",
				"http://beacon:5052",
				"http://beacon:5052",
			},
			wantErr: false,
			wantLen: 3,
		},
		{
			name:        "empty_addresses_list",
			addresses:   []string{},
			wantErr:     true,
			errContains: "no addresses provided",
		},
		{
			name:        "nil_addresses_list",
			addresses:   nil,
			wantErr:     true,
			errContains: "no addresses provided",
		},
		{
			name: "single_address",
			addresses: []string{
				"http://beacon:5052",
			},
			wantErr: false,
			wantLen: 1,
		},
		{
			name: "mixed_protocols",
			addresses: []string{
				"http://beacon1:5052",
				"https://beacon2:5053",
				"ws://beacon3:5054",
			},
			wantErr: false,
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceIDs, err := beacon.GenerateBeaconTraceIDs(tt.addresses)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, traceIDs, tt.wantLen)

				for i, id := range traceIDs {
					assert.NotEmpty(t, id, "trace ID at index %d should not be empty", i)
				}
			}
		})
	}
}

func TestGenerateBeaconTraceIDs_Uniqueness(t *testing.T) {
	t.Run("all_unique_addresses", func(t *testing.T) {
		addresses := []string{
			"http://beacon1:5052",
			"http://beacon2:5052",
			"http://beacon3:5052",
			"http://beacon4:5052",
			"http://beacon5:5052",
		}

		traceIDs, err := beacon.GenerateBeaconTraceIDs(addresses)
		require.NoError(t, err)

		uniqueIDs := make(map[string]bool)
		for _, id := range traceIDs {
			assert.False(t, uniqueIDs[id], "duplicate trace ID found: %s", id)
			uniqueIDs[id] = true
		}
	})

	t.Run("duplicate_addresses_get_unique_ids", func(t *testing.T) {
		addresses := []string{
			"http://beacon:5052",
			"http://beacon:5052",
			"http://beacon:5052",
			"http://different:5052",
		}

		traceIDs, err := beacon.GenerateBeaconTraceIDs(addresses)
		require.NoError(t, err)
		require.Len(t, traceIDs, 4)

		assert.NotEqual(t, traceIDs[0], traceIDs[1], "duplicate addresses should get unique IDs")
		assert.NotEqual(t, traceIDs[0], traceIDs[2], "duplicate addresses should get unique IDs")
		assert.NotEqual(t, traceIDs[1], traceIDs[2], "duplicate addresses should get unique IDs")

		assert.True(t, traceIDs[1] == traceIDs[0]+"-1" || traceIDs[1] == traceIDs[0]+"-2",
			"second duplicate should have counter suffix")
		assert.True(t, traceIDs[2] == traceIDs[0]+"-1" || traceIDs[2] == traceIDs[0]+"-2",
			"third duplicate should have counter suffix")
	})
}

func TestGenerateBeaconTraceIDs_LargeScale(t *testing.T) {
	const numAddresses = 1000
	addresses := make([]string, numAddresses)

	for i := 0; i < numAddresses/2; i++ {
		addresses[i] = "http://same-beacon:5052"
	}

	for i := numAddresses / 2; i < numAddresses; i++ {
		addresses[i] = "http://beacon-" + string(rune(i)) + ":5052"
	}

	traceIDs, err := beacon.GenerateBeaconTraceIDs(addresses)
	require.NoError(t, err)
	require.Len(t, traceIDs, numAddresses)

	uniqueIDs := make(map[string]bool)
	for i, id := range traceIDs {
		assert.False(t, uniqueIDs[id], "duplicate trace ID found at index %d: %s", i, id)
		uniqueIDs[id] = true
	}
}

func TestGenerateBeaconTraceID_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		addresses []string
	}{
		{
			name: "very_long_address",
			addresses: []string{
				"http://this-is-a-very-long-beacon-node-address-that-should-still-generate-a-valid-trace-id.example.com:5052/api/v1/beacon/very/long/path",
			},
		},
		{
			name: "special_characters",
			addresses: []string{
				"http://beacon-node!@#$%^&*():5052",
				"http://beacon-node[brackets]:5052",
				"http://beacon-node{braces}:5052",
			},
		},
		{
			name: "unicode_addresses",
			addresses: []string{
				"http://beacon-node-🚀:5052",
				"http://beacon-node-测试:5052",
				"http://beacon-node-тест:5052",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceIDs, err := beacon.GenerateBeaconTraceIDs(tt.addresses)
			require.NoError(t, err)
			require.Len(t, traceIDs, len(tt.addresses))

			for i, id := range traceIDs {
				assert.NotEmpty(t, id, "trace ID at index %d should not be empty", i)
				assert.GreaterOrEqual(t, len(id), 8, "trace ID should be at least 8 characters")
			}
		})
	}
}

func BenchmarkGenerateBeaconTraceID(b *testing.B) {
	address := "http://beacon-node:5052"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := beacon.GenerateBeaconTraceID(address)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateBeaconTraceIDs(b *testing.B) {
	addresses := []string{
		"http://beacon1:5052",
		"http://beacon2:5052",
		"http://beacon3:5052",
		"http://beacon4:5052",
		"http://beacon5:5052",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := beacon.GenerateBeaconTraceIDs(addresses)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateBeaconTraceIDs_Duplicates(b *testing.B) {
	addresses := make([]string, 10)
	for i := range addresses {
		addresses[i] = "http://beacon:5052"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := beacon.GenerateBeaconTraceIDs(addresses)
		if err != nil {
			b.Fatal(err)
		}
	}
}
