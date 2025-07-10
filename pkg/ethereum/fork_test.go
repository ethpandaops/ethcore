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

			digest, err := ComputeForkDigest(tt.genesisValidatorsRoot, tt.forkVersion, nil)

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

	digest1, err1 := ComputeForkDigest(genesisValidatorsRoot, forkVersion, nil)
	require.NoError(t, err1)

	digest2, err2 := ComputeForkDigest(genesisValidatorsRoot, forkVersion, nil)
	require.NoError(t, err2)

	assert.Equal(t, digest1, digest2, "Fork digest should be deterministic")
}

func TestComputeForkDigest_DifferentInputs(t *testing.T) {
	// Test that different inputs produce different outputs
	genesisValidatorsRoot1 := phase0.Root{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20}
	genesisValidatorsRoot2 := phase0.Root{0x20, 0x1f, 0x1e, 0x1d, 0x1c, 0x1b, 0x1a, 0x19, 0x18, 0x17, 0x16, 0x15, 0x14, 0x13, 0x12, 0x11, 0x10, 0x0f, 0x0e, 0x0d, 0x0c, 0x0b, 0x0a, 0x09, 0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}
	forkVersion := [4]byte{0x00, 0x00, 0x00, 0x01}

	digest1, err1 := ComputeForkDigest(genesisValidatorsRoot1, forkVersion, nil)
	require.NoError(t, err1)

	digest2, err2 := ComputeForkDigest(genesisValidatorsRoot2, forkVersion, nil)
	require.NoError(t, err2)

	assert.NotEqual(t, digest1, digest2, "Different genesis validators roots should produce different digests")

	// Test with different fork versions
	forkVersion2 := [4]byte{0x00, 0x00, 0x00, 0x02}
	digest3, err3 := ComputeForkDigest(genesisValidatorsRoot1, forkVersion2, nil)
	require.NoError(t, err3)

	assert.NotEqual(t, digest1, digest3, "Different fork versions should produce different digests")
}

func TestComputeForkDigest_FusakaDevnet(t *testing.T) {
	// Test fork digest computation for fusaka-devnet-0
	// Genesis validators root: 0xe6f92d486d5d64c6d44342906b5674137733a48bae65a6e0d75dc4fb4dec030d
	genesisValidatorsRoot := phase0.Root{0xe6, 0xf9, 0x2d, 0x48, 0x6d, 0x5d, 0x64, 0xc6, 0xd4, 0x43, 0x42, 0x90, 0x6b, 0x56, 0x74, 0x13, 0x77, 0x33, 0xa4, 0x8b, 0xae, 0x65, 0xa6, 0xe0, 0xd7, 0x5d, 0xc4, 0xfb, 0x4d, 0xec, 0x03, 0x0d}

	// Test electra fork version: 0x60937544 (from fusaka-devnet config)
	electraForkVersion := [4]byte{0x60, 0x93, 0x75, 0x44}
	electraDigest, err := ComputeForkDigest(genesisValidatorsRoot, electraForkVersion, nil)
	require.NoError(t, err)
	t.Logf("Electra fork digest: 0x%x", electraDigest)

	// Test fulu fork version: 0x70937544 (from fusaka-devnet config)
	fuluForkVersion := [4]byte{0x70, 0x93, 0x75, 0x44}
	fuluDigest, err := ComputeForkDigest(genesisValidatorsRoot, fuluForkVersion, nil)
	require.NoError(t, err)
	t.Logf("Fulu fork digest: 0x%x", fuluDigest)

	// Expected values for fusaka-devnet config
	expectedElectraDigest := phase0.ForkDigest{0x72, 0xe1, 0x45, 0xaf}
	expectedFuluDigest := phase0.ForkDigest{0x57, 0x83, 0xa4, 0xb8}

	// Check if we're getting the expected values
	assert.Equal(t, expectedElectraDigest, electraDigest, "Electra fork digest should match expected value")
	assert.Equal(t, expectedFuluDigest, fuluDigest, "Fulu fork digest should match expected value")
}

func TestComputeForkDigest_WithBlobParams(t *testing.T) {
	// Test Fulu fork digest computation with blob parameters
	genesisValidatorsRoot := phase0.Root{0xe6, 0xf9, 0x2d, 0x48, 0x6d, 0x5d, 0x64, 0xc6, 0xd4, 0x43, 0x42, 0x90, 0x6b, 0x56, 0x74, 0x13, 0x77, 0x33, 0xa4, 0x8b, 0xae, 0x65, 0xa6, 0xe0, 0xd7, 0x5d, 0xc4, 0xfb, 0x4d, 0xec, 0x03, 0x0d}
	fuluForkVersion := [4]byte{0x70, 0x93, 0x75, 0x44}

	// Test without blob parameters (standard fork digest)
	digestWithoutBlobs, err := ComputeForkDigest(genesisValidatorsRoot, fuluForkVersion, nil)
	require.NoError(t, err)

	// Test with blob parameters (Fulu fork digest)
	blobParams := &BlobScheduleEntry{
		Epoch:            0,
		MaxBlobsPerBlock: 6,
	}
	digestWithBlobs, err := ComputeForkDigest(genesisValidatorsRoot, fuluForkVersion, blobParams)
	require.NoError(t, err)

	// The digests should be different when blob parameters are used
	assert.NotEqual(t, digestWithoutBlobs, digestWithBlobs, "Fork digest should be different with blob parameters")

	// Test with different blob parameters
	blobParams2 := &BlobScheduleEntry{
		Epoch:            10,
		MaxBlobsPerBlock: 8,
	}
	digestWithDifferentBlobs, err := ComputeForkDigest(genesisValidatorsRoot, fuluForkVersion, blobParams2)
	require.NoError(t, err)

	// Different blob parameters should produce different digests
	assert.NotEqual(t, digestWithBlobs, digestWithDifferentBlobs, "Different blob parameters should produce different digests")

	t.Logf("Digest without blobs: 0x%x", digestWithoutBlobs)
	t.Logf("Digest with blobs (epoch=0, max=6): 0x%x", digestWithBlobs)
	t.Logf("Digest with blobs (epoch=10, max=8): 0x%x", digestWithDifferentBlobs)
}
