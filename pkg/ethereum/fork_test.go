package ethereum

import (
	"testing"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeForkDigest(t *testing.T) {
	tests := []struct {
		name                  string
		genesisValidatorsRoot phase0.Root
		forkVersion           [4]byte
		wantErr               bool
		errMsg                string
	}{
		{
			name:                  "valid fork digest computation",
			genesisValidatorsRoot: phase0.Root{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f},
			forkVersion:           [4]byte{0x00, 0x00, 0x00, 0x01},
			wantErr:               false,
		},
		{
			name:                  "another valid fork digest",
			genesisValidatorsRoot: phase0.Root{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			forkVersion:           [4]byte{0x01, 0x02, 0x03, 0x04},
			wantErr:               false,
		},
		{
			name:                  "zero genesis validators root",
			genesisValidatorsRoot: phase0.Root{},
			forkVersion:           [4]byte{0x00, 0x00, 0x00, 0x00},
			wantErr:               false,
		},
		{
			name:                  "invalid genesis validators root length",
			genesisValidatorsRoot: phase0.Root{}, // This will be modified in the test
			forkVersion:           [4]byte{0x00, 0x00, 0x00, 0x01},
			wantErr:               true,
			errMsg:                "invalid genesis validators root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Special case for testing invalid length
			if tt.name == "invalid genesis validators root length" {
				// Since phase0.Root is a [32]byte, it's always 32 bytes
				// The function checks len(genesisValidatorsRoot) != 32
				// But phase0.Root is a fixed-size array type
				// This test case is not possible with the current implementation
				t.Skip("Cannot test invalid length with fixed-size array type")
			}

			digest, err := ComputeForkDigest(tt.genesisValidatorsRoot, tt.forkVersion)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				// Verify the digest is not empty
				assert.NotEqual(t, phase0.ForkDigest{}, digest)
			}
		})
	}
}

func TestComputeForkDigest_Deterministic(t *testing.T) {
	// Test that the same inputs produce the same output
	genesisValidatorsRoot := phase0.Root{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20}
	forkVersion := [4]byte{0x00, 0x00, 0x00, 0x01}

	digest1, err1 := ComputeForkDigest(genesisValidatorsRoot, forkVersion)
	require.NoError(t, err1)

	digest2, err2 := ComputeForkDigest(genesisValidatorsRoot, forkVersion)
	require.NoError(t, err2)

	assert.Equal(t, digest1, digest2, "Fork digest should be deterministic")
}

func TestComputeForkDigest_DifferentInputs(t *testing.T) {
	// Test that different inputs produce different outputs
	genesisValidatorsRoot1 := phase0.Root{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20}
	genesisValidatorsRoot2 := phase0.Root{0x20, 0x1f, 0x1e, 0x1d, 0x1c, 0x1b, 0x1a, 0x19, 0x18, 0x17, 0x16, 0x15, 0x14, 0x13, 0x12, 0x11, 0x10, 0x0f, 0x0e, 0x0d, 0x0c, 0x0b, 0x0a, 0x09, 0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}
	forkVersion := [4]byte{0x00, 0x00, 0x00, 0x01}

	digest1, err1 := ComputeForkDigest(genesisValidatorsRoot1, forkVersion)
	require.NoError(t, err1)

	digest2, err2 := ComputeForkDigest(genesisValidatorsRoot2, forkVersion)
	require.NoError(t, err2)

	assert.NotEqual(t, digest1, digest2, "Different genesis validators roots should produce different digests")

	// Test with different fork versions
	forkVersion2 := [4]byte{0x00, 0x00, 0x00, 0x02}
	digest3, err3 := ComputeForkDigest(genesisValidatorsRoot1, forkVersion2)
	require.NoError(t, err3)

	assert.NotEqual(t, digest1, digest3, "Different fork versions should produce different digests")
}
