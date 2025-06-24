package networks

import (
	"testing"
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

func TestNetworkConstants(t *testing.T) {
	expectedConstants := []NetworkName{
		NetworkNameNone,
		NetworkNameUnknown,
		NetworkNameMainnet,
		NetworkNameGoerli,
		NetworkNameSepolia,
		NetworkNameHolesky,
		NetworkNameHoodi,
	}

	expectedValues := []string{
		"none",
		"unknown",
		"mainnet",
		"goerli",
		"sepolia",
		"holesky",
		"hoodi",
	}

	for i, constant := range expectedConstants {
		if string(constant) != expectedValues[i] {
			t.Errorf("Network constant %v = %v, want %v", constant, string(constant), expectedValues[i])
		}
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

		if name, ok := NetworkIds[id]; ok {
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
