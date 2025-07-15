package ethereum

import (
	"crypto/sha256"

	"github.com/attestantio/go-eth2-client/spec/phase0"
)

// BlobScheduleEntry represents a blob parameter configuration for a specific epoch.
type BlobScheduleEntry struct {
	Epoch            uint64
	MaxBlobsPerBlock uint64
}

func ComputeForkDigest(genesisValidatorsRoot phase0.Root, forkVersion phase0.Version, blobParams *BlobScheduleEntry) phase0.ForkDigest {
	forkData := phase0.ForkData{
		CurrentVersion:        forkVersion,
		GenesisValidatorsRoot: genesisValidatorsRoot,
	}

	forkDataRoot, _ := forkData.HashTreeRoot()

	// For Fulu fork and later, modify the fork digest with blob parameters
	if blobParams != nil {
		// serialize epoch and max_blobs_per_block as uint64 little-endian
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

		// xor baseDigest with first 4 bytes of blobParamHash
		forkDigest := make([]byte, 4)
		for i := 0; i < 4; i++ {
			forkDigest[i] = forkDataRoot[i] ^ blobParamHash[i]
		}

		return phase0.ForkDigest(forkDigest)
	}

	return phase0.ForkDigest(forkDataRoot[:4])
}
