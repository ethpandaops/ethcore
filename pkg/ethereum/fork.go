package ethereum

import (
	"github.com/OffchainLabs/prysm/v6/beacon-chain/core/signing"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/pkg/errors"
)

func ComputeForkDigest(genesisValidatorsRoot phase0.Root, forkVersion [4]byte) (phase0.ForkDigest, error) {
	if len(genesisValidatorsRoot) != 32 {
		return phase0.ForkDigest{}, errors.New("invalid genesis validators root")
	}

	digest, err := signing.ComputeForkDigest(forkVersion[:], genesisValidatorsRoot[:])
	if err != nil {
		return phase0.ForkDigest{}, errors.Wrap(err, "failed to compute fork digest")
	}

	return phase0.ForkDigest(digest[:]), nil
}
