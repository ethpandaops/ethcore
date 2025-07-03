package ethereum

import "github.com/ethpandaops/beacon/pkg/beacon"

// Options configures beacon node behavior.
type Options struct {
	*beacon.Options

	FetchBeaconCommittees bool
	FetchProposerDuties   bool
}
