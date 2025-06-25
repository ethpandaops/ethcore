package ethereum

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			config: Config{
				BeaconNodeAddress: "http://localhost:5052",
				Network:           "mainnet",
				BeaconNodeHeaders: map[string]string{
					"Authorization": "Bearer token",
					"X-Custom":      "value",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with minimal fields",
			config: Config{
				BeaconNodeAddress: "http://beacon.example.com",
			},
			wantErr: false,
		},
		{
			name: "invalid config - missing beacon node address",
			config: Config{
				Network: "mainnet",
			},
			wantErr: true,
			errMsg:  "beaconNodeAddress is required",
		},
		{
			name: "valid config with empty network",
			config: Config{
				BeaconNodeAddress: "http://localhost:5052",
				Network:           "",
			},
			wantErr: false,
		},
		{
			name: "valid config with nil headers",
			config: Config{
				BeaconNodeAddress: "http://localhost:5052",
				BeaconNodeHeaders: nil,
			},
			wantErr: false,
		},
		{
			name: "valid config with empty headers map",
			config: Config{
				BeaconNodeAddress: "http://localhost:5052",
				BeaconNodeHeaders: map[string]string{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
