package networks

import (
	"testing"

	"github.com/ethpandaops/beacon/pkg/beacon/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveFromGenesisRoot(t *testing.T) {
	tests := []struct {
		name         string
		genesisRoot  string
		expectedName NetworkName
		expectedID   uint64
	}{
		{
			name:         "mainnet genesis root",
			genesisRoot:  "0x4b363db94e286120d76eb905340fdd4e54bfe9f06bf33ff6cf5ad27f511bfe95",
			expectedName: NetworkNameMainnet,
			expectedID:   1,
		},
		{
			name:         "goerli genesis root",
			genesisRoot:  "0x043db0d9a83813551ee2f33450d23797757d430911a9320530ad8a0eabc43efb",
			expectedName: NetworkNameGoerli,
			expectedID:   5,
		},
		{
			name:         "sepolia genesis root",
			genesisRoot:  "0xd8ea171f3c94aea21ebc42a1ed61052acf3f9209c00e4efbaaddac09ed9b8078",
			expectedName: NetworkNameSepolia,
			expectedID:   11155111,
		},
		{
			name:         "holesky genesis root",
			genesisRoot:  "0x9143aa7c615a7f7115e2b6aac319c03529df8242ae705fba9df39b79c59fa8b1",
			expectedName: NetworkNameHolesky,
			expectedID:   17000,
		},
		{
			name:         "hoodi genesis root",
			genesisRoot:  "0x212f13fc4df078b6cb7db228f1c8307566dcecf900867401a92023d7ba99cb5f",
			expectedName: NetworkNameHoodi,
			expectedID:   560048,
		},
		{
			name:         "unknown genesis root",
			genesisRoot:  "0x0000000000000000000000000000000000000000000000000000000000000000",
			expectedName: NetworkNameUnknown,
			expectedID:   0,
		},
		{
			name:         "empty genesis root",
			genesisRoot:  "",
			expectedName: NetworkNameUnknown,
			expectedID:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network := DeriveFromGenesisRoot(tt.genesisRoot)
			if network.Name != tt.expectedName {
				t.Errorf("DeriveFromGenesisRoot() name = %v, want %v", network.Name, tt.expectedName)
			}
			if network.ID != tt.expectedID {
				t.Errorf("DeriveFromGenesisRoot() ID = %v, want %v", network.ID, tt.expectedID)
			}
		})
	}
}

func TestDeriveFromID(t *testing.T) {
	tests := []struct {
		name         string
		id           uint64
		expectedName NetworkName
	}{
		{
			name:         "mainnet ID",
			id:           1,
			expectedName: NetworkNameMainnet,
		},
		{
			name:         "goerli ID",
			id:           5,
			expectedName: NetworkNameGoerli,
		},
		{
			name:         "sepolia ID",
			id:           11155111,
			expectedName: NetworkNameSepolia,
		},
		{
			name:         "holesky ID",
			id:           17000,
			expectedName: NetworkNameHolesky,
		},
		{
			name:         "hoodi ID",
			id:           560048,
			expectedName: NetworkNameHoodi,
		},
		{
			name:         "unknown ID",
			id:           999999,
			expectedName: NetworkNameUnknown,
		},
		{
			name:         "zero ID",
			id:           0,
			expectedName: NetworkNameUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network := DeriveFromID(tt.id)
			if network.Name != tt.expectedName {
				t.Errorf("DeriveFromID() name = %v, want %v", network.Name, tt.expectedName)
			}
			if network.ID != tt.id {
				t.Errorf("DeriveFromID() ID = %v, want %v", network.ID, tt.id)
			}
		})
	}
}

func TestNetworkMapsConsistency(t *testing.T) {
	for genesisRoot, id := range NetworkGenesisRoots {
		network := DeriveFromGenesisRoot(genesisRoot)
		if network.ID != id {
			t.Errorf(
				"Inconsistent mapping for genesis root %s: map has ID %d, DeriveFromGenesisRoot returns %d",
				genesisRoot,
				id,
				network.ID,
			)
		}

		if name, ok := NetworkIDs[id]; ok {
			if network.Name != name {
				t.Errorf(
					"Inconsistent network name for ID %d: NetworkIds has %s, DeriveFromGenesisRoot returns %s",
					id,
					name,
					network.Name,
				)
			}
		}
	}
}

func TestNetworkConstants(t *testing.T) {
	// Verify network name constants
	assert.Equal(t, NetworkName("unknown"), NetworkNameUnknown)
	assert.Equal(t, NetworkName("mainnet"), NetworkNameMainnet)
	assert.Equal(t, NetworkName("sepolia"), NetworkNameSepolia)
	assert.Equal(t, NetworkName("holesky"), NetworkNameHolesky)
	assert.Equal(t, NetworkName("hoodi"), NetworkNameHoodi)
}

func TestKnownNetworks(t *testing.T) {
	// Verify KnownNetworks has the expected number of entries
	assert.Len(t, KnownNetworks, 5)

	// Verify specific network properties
	mainnetFound := false
	holeskiFound := false

	for _, network := range KnownNetworks {
		if network.Name == NetworkNameMainnet {
			mainnetFound = true
			assert.Equal(t, uint64(1), network.ID)
			assert.Equal(t, "0x00000000219ab540356cBB839Cbe05303d7705Fa", network.DepositContractAddress)
			assert.Equal(t, uint64(1), network.DepositChainID)
		}

		if network.Name == NetworkNameHolesky {
			holeskiFound = true
			assert.Equal(t, uint64(17000), network.ID)
			assert.Equal(t, "0x4242424242424242424242424242424242424242", network.DepositContractAddress)
			assert.Equal(t, uint64(17000), network.DepositChainID)
		}
	}

	assert.True(t, mainnetFound, "Mainnet network not found in KnownNetworks")
	assert.True(t, holeskiFound, "Holesky network not found in KnownNetworks")
}

func TestDeriveFromSpec(t *testing.T) {
	tests := []struct {
		name                  string
		spec                  *state.Spec
		expectedNetwork       *Network
		expectedErrorContains string
	}{
		{
			name: "mainnet",
			spec: createMockSpec("0x00000000219ab540356cBB839Cbe05303d7705Fa", 1, "mainnet"),
			expectedNetwork: &Network{
				Name:                   NetworkNameMainnet,
				ID:                     1,
				DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
				DepositChainID:         1,
			},
		},
		{
			name: "mainnet with lowercase address",
			spec: createMockSpec("0x00000000219ab540356cbb839cbe05303d7705fa", 1, "mainnet"),
			expectedNetwork: &Network{
				Name:                   NetworkNameMainnet,
				ID:                     1,
				DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
				DepositChainID:         1,
			},
		},
		{
			name: "sepolia",
			spec: createMockSpec("0x7f02c3e3c98b133055b8b348b2ac625669ed295d", 11155111, "sepolia"),
			expectedNetwork: &Network{
				Name:                   NetworkNameSepolia,
				ID:                     11155111,
				DepositContractAddress: "0x7f02c3e3c98b133055b8b348b2ac625669ed295d",
				DepositChainID:         11155111,
			},
		},
		{
			name: "holesky",
			spec: createMockSpec("0x4242424242424242424242424242424242424242", 17000, "holesky"),
			expectedNetwork: &Network{
				Name:                   NetworkNameHolesky,
				ID:                     17000,
				DepositContractAddress: "0x4242424242424242424242424242424242424242",
				DepositChainID:         17000,
			},
		},
		{
			name: "hoodi",
			spec: createMockSpec("0x00000000219ab540356cBB839Cbe05303d7705Fa", 560048, "hoodi"),
			expectedNetwork: &Network{
				Name:                   NetworkNameHoodi,
				ID:                     560048,
				DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
				DepositChainID:         560048,
			},
		},
		{
			name:                  "unknown network with no config name",
			spec:                  createMockSpec("0x1111111111111111111111111111111111111111", 123456, ""),
			expectedErrorContains: "network not found",
		},
		{
			name: "unknown network with custom config name",
			spec: createMockSpec("0x1111111111111111111111111111111111111111", 123456, "custom-testnet"),
			expectedNetwork: &Network{
				Name:                   "custom-testnet",
				ID:                     123456,
				DepositContractAddress: "0x1111111111111111111111111111111111111111",
				DepositChainID:         123456,
			},
		},
		{
			name:                  "conflicting config name",
			spec:                  createMockSpec("0x1111111111111111111111111111111111111111", 123456, "mainnet"),
			expectedErrorContains: "incorrect network detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, err := DeriveFromSpec(tt.spec)

			if tt.expectedErrorContains != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
				assert.Nil(t, network)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedNetwork.Name, network.Name)
				assert.Equal(t, tt.expectedNetwork.ID, network.ID)
				assert.Equal(t, tt.expectedNetwork.DepositContractAddress, network.DepositContractAddress)
				assert.Equal(t, tt.expectedNetwork.DepositChainID, network.DepositChainID)
			}
		})
	}
}

func TestDeriveNetworkFromSpecWithOverride(t *testing.T) {
	tests := []struct {
		name            string
		spec            *state.Spec
		networkOverride string
		expectedNetwork string
		expectError     bool
	}{
		{
			name:            "mainnet network is not overridden",
			spec:            createMockSpec("0x00000000219ab540356cBB839Cbe05303d7705Fa", 1, "mainnet"),
			networkOverride: "custom-network",
			expectedNetwork: "mainnet",
			expectError:     false,
		},
		{
			name:            "testnet allows override",
			spec:            createMockSpec("0x1111111111111111111111111111111111111111", 12345, "testnet"),
			networkOverride: "pectra-devnet-6",
			expectedNetwork: "pectra-devnet-6",
			expectError:     false,
		},
		{
			name:            "custom network spec is not overridden",
			spec:            createMockSpec("0x1111111111111111111111111111111111111111", 12345, "custom-network-from-spec"),
			networkOverride: "override-attempt",
			expectedNetwork: "custom-network-from-spec",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First derive the network normally
			network, err := DeriveFromSpec(tt.spec)
			if tt.expectError {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)

			// Apply network override when network name is "testnet"
			if tt.networkOverride != "" && (network.Name == "testnet") {
				network = &Network{
					Name:                   NetworkName(tt.networkOverride),
					ID:                     network.ID,
					DepositContractAddress: network.DepositContractAddress,
					DepositChainID:         network.DepositChainID,
				}
			}

			// Verify results
			assert.Equal(t, NetworkName(tt.expectedNetwork), network.Name)
		})
	}
}

func TestFindByName(t *testing.T) {
	tests := []struct {
		name                  string
		networkName           NetworkName
		expectedNetwork       *Network
		expectedErrorContains string
	}{
		{
			name:        "mainnet",
			networkName: NetworkNameMainnet,
			expectedNetwork: &Network{
				Name:                   NetworkNameMainnet,
				ID:                     1,
				DepositContractAddress: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
				DepositChainID:         1,
			},
		},
		{
			name:        "sepolia",
			networkName: NetworkNameSepolia,
			expectedNetwork: &Network{
				Name:                   NetworkNameSepolia,
				ID:                     11155111,
				DepositContractAddress: "0x7f02c3e3c98b133055b8b348b2ac625669ed295d",
				DepositChainID:         11155111,
			},
		},
		{
			name:                  "unknown network",
			networkName:           "invalid-network",
			expectedErrorContains: "network not found",
		},
		{
			name:                  "unknown network with value",
			networkName:           NetworkNameUnknown,
			expectedErrorContains: "network not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, err := FindByName(tt.networkName)

			if tt.expectedErrorContains != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
				assert.Nil(t, network)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedNetwork.Name, network.Name)
				assert.Equal(t, tt.expectedNetwork.ID, network.ID)
				assert.Equal(t, tt.expectedNetwork.DepositContractAddress, network.DepositContractAddress)
				assert.Equal(t, tt.expectedNetwork.DepositChainID, network.DepositChainID)
			}
		})
	}
}

// Mock state.Spec for testing.
func createMockSpec(depositContractAddress string, depositChainID uint64, configName string) *state.Spec {
	return &state.Spec{
		DepositContractAddress: depositContractAddress,
		DepositChainID:         depositChainID,
		ConfigName:             configName,
	}
}
