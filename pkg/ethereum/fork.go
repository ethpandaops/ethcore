package ethereum

import (
	"crypto/sha256"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/core/signing"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/pkg/errors"
)

// BlobScheduleEntry represents a blob parameter configuration for a specific epoch.
type BlobScheduleEntry struct {
	Epoch            uint64
	MaxBlobsPerBlock uint64
}

func ComputeForkDigest(genesisValidatorsRoot phase0.Root, forkVersion [4]byte, blobParams *BlobScheduleEntry) (phase0.ForkDigest, error) {
	if len(genesisValidatorsRoot) != 32 {
		return phase0.ForkDigest{}, errors.New("invalid genesis validators root")
	}

	// Compute the base fork digest
	baseDigest, err := signing.ComputeForkDigest(forkVersion[:], genesisValidatorsRoot[:])
	if err != nil {
		return phase0.ForkDigest{}, errors.Wrap(err, "failed to compute base fork digest")
	}

	// For Fulu fork and later, modify the fork digest with blob parameters
	if blobParams != nil {
		// Serialize epoch and max_blobs_per_block as uint64 little-endian
		epochBytes := make([]byte, 8)
		maxBlobsBytes := make([]byte, 8)

		for i := 0; i < 8; i++ {
			epochBytes[i] = byte((blobParams.Epoch >> (8 * i)) & 0xff)
			maxBlobsBytes[i] = byte((blobParams.MaxBlobsPerBlock >> (8 * i)) & 0xff)
		}

		blobParamBytes := append(epochBytes, maxBlobsBytes...)

		blobParamHash := [32]byte{}
		{
			h := sha256.New()
			h.Write(blobParamBytes)
			copy(blobParamHash[:], h.Sum(nil))
		}

		// XOR baseDigest with first 4 bytes of blobParamHash
		forkDigest := make([]byte, 4)
		for i := 0; i < 4; i++ {
			forkDigest[i] = baseDigest[i] ^ blobParamHash[i]
		}

		return phase0.ForkDigest(forkDigest), nil
	}

	return phase0.ForkDigest(baseDigest[:4]), nil
}
