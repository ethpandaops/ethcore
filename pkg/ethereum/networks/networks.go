package networks

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethpandaops/beacon/pkg/beacon/state"
)

// NetworkName represents the name of an Ethereum network.
type NetworkName string

// Network represents an Ethereum network.
type Network struct {
	Name                   NetworkName
	ID                     uint64
	DepositContractAddress string
	DepositChainID         uint64
}

// Define known networks.
var (
	NetworkNameNone    NetworkName = "none"
	NetworkNameUnknown NetworkName = "unknown"
	NetworkNameMainnet NetworkName = "mainnet"
	NetworkNameGoerli  NetworkName = "goerli"
	NetworkNameSepolia NetworkName = "sepolia"
	NetworkNameHolesky NetworkName = "holesky"
	NetworkNameHoodi   NetworkName = "hoodi"
)

// NetworkGenesisRoots maps genesis roots to chain IDs.
var NetworkGenesisRoots = map[string]uint64{
	"0x4b363db94e286120d76eb905340fdd4e54bfe9f06bf33ff6cf5ad27f511bfe95": 1,
	"0x043db0d9a83813551ee2f33450d23797757d430911a9320530ad8a0eabc43efb": 5,
	"0xd8ea171f3c94aea21ebc42a1ed61052acf3f9209c00e4efbaaddac09ed9b8078": 11155111,
	"0x9143aa7c615a7f7115e2b6aac319c03529df8242ae705fba9df39b79c59fa8b1": 17000,
	"0x212f13fc4df078b6cb7db228f1c8307566dcecf900867401a92023d7ba99cb5f": 560048,
}

// NetworkIDs maps chain IDs to network names.
var NetworkIDs = map[uint64]NetworkName{
	1:        NetworkNameMainnet,
	5:        NetworkNameGoerli,
	11155111: NetworkNameSepolia,
	17000:    NetworkNameHolesky,
	560048:   NetworkNameHoodi,
}

var (
	ErrNetworkNotFound = errors.New("network not found")
	KnownNetworks      = []Network{
		{
			Name:                   NetworkNameMainnet,
			ID:                     1,
			DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
			DepositChainID:         1,
		},
		{
			Name:                   NetworkNameGoerli,
			ID:                     5,
			DepositContractAddress: "0xff50ed3d0ec03aC01D4C79aAd74928BFF48a7b2b",
			DepositChainID:         5,
		},
		{
			Name:                   NetworkNameSepolia,
			ID:                     11155111,
			DepositContractAddress: "0x7f02c3e3c98b133055b8b348b2ac625669ed295d",
			DepositChainID:         11155111,
		},
		{
			Name:                   NetworkNameHolesky,
			ID:                     17000,
			DepositContractAddress: "0x4242424242424242424242424242424242424242",
			DepositChainID:         17000,
		},
		{
			Name:                   NetworkNameHoodi,
			ID:                     560048,
			DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
			DepositChainID:         560048,
		},
	}
)

// DeriveFromGenesisRoot derives a network from a genesis root.
func DeriveFromGenesisRoot(genesisRoot string) *Network {
	if id, ok := NetworkGenesisRoots[genesisRoot]; ok {
		network := &Network{Name: NetworkNameUnknown, ID: id}
		if name, ok := NetworkIDs[id]; ok {
			network.Name = name
		}

		return network
	}

	return &Network{Name: NetworkNameUnknown, ID: 0}
}

// DeriveFromID derives a network from a chain ID.
func DeriveFromID(id uint64) *Network {
	network := &Network{Name: NetworkNameUnknown, ID: id}
	if name, ok := NetworkIDs[id]; ok {
		network.Name = name
	}

	return network
}

// DeriveFromSpec derives a network from a spec.
// If the deposit contract address and chain ID aren't known by the consumer, then
// it attempts to derive the network name from the CONFIG_NAME.
// If this CONFIG_NAME is one of our known networks, then we return an error.
// This ensures that we:
// - We have safety garauntees for our defined networks
// - We still support networks that are not in our list of known networks.
func DeriveFromSpec(spec *state.Spec) (*Network, error) {
	for _, network := range KnownNetworks {
		if strings.EqualFold(network.DepositContractAddress, spec.DepositContractAddress) &&
			network.DepositChainID == spec.DepositChainID {
			return &network, nil
		}
	}

	// Attempt to support networks that are not in our list of known networks
	// by using the spec config name.
	if spec.ConfigName != "" {
		// Check if the spec config name is one of our known networks
		if _, err := FindByName(NetworkName(spec.ConfigName)); err == nil {
			// We've somehow found a network that is not in our list of known networks
			// but the CONFIG_NAME matches one of our known networks
			// We'll return an error here to ensure that we don't send incorrect network information.
			// Realistically, this should never happen.
			return nil, fmt.Errorf("incorrect network detected: %s", spec.ConfigName)
		}

		// The spec config name is not one of our known networks, so we'll return the network
		// with the given deposit contract address and chain ID.
		return &Network{
			Name:                   NetworkName(spec.ConfigName),
			ID:                     spec.DepositChainID,
			DepositContractAddress: spec.DepositContractAddress,
			DepositChainID:         spec.DepositChainID,
		}, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrNetworkNotFound, spec.ConfigName)
}

// FindByName returns a network with the given name or an error if not found.
func FindByName(name NetworkName) (*Network, error) {
	for _, network := range KnownNetworks {
		if network.Name == name {
			return &network, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrNetworkNotFound, name)
}
