package clients

import (
	"testing"
)

func TestClientFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Client
	}{
		// Exact matches (case-insensitive)
		{
			name:     "lighthouse exact match",
			input:    "lighthouse",
			expected: ClientLighthouse,
		},
		{
			name:     "lighthouse uppercase",
			input:    "LIGHTHOUSE",
			expected: ClientLighthouse,
		},
		{
			name:     "lighthouse mixed case",
			input:    "LightHouse",
			expected: ClientLighthouse,
		},
		{
			name:     "nimbus exact match",
			input:    "nimbus",
			expected: ClientNimbus,
		},
		{
			name:     "teku exact match",
			input:    "teku",
			expected: ClientTeku,
		},
		{
			name:     "prysm exact match",
			input:    "prysm",
			expected: ClientPrysm,
		},
		{
			name:     "lodestar exact match",
			input:    "lodestar",
			expected: ClientLodestar,
		},
		{
			name:     "grandine exact match",
			input:    "grandine",
			expected: ClientGrandine,
		},
		{
			name:     "caplin exact match",
			input:    "caplin",
			expected: ClientCaplin,
		},
		{
			name:     "geth exact match",
			input:    "geth",
			expected: ClientGeth,
		},
		{
			name:     "besu exact match",
			input:    "besu",
			expected: ClientBesu,
		},
		{
			name:     "nethermind exact match",
			input:    "nethermind",
			expected: ClientNethermind,
		},
		{
			name:     "erigon exact match",
			input:    "erigon",
			expected: ClientErigon,
		},
		{
			name:     "reth exact match",
			input:    "reth",
			expected: ClientReth,
		},
		{
			name:     "ethereumjs exact match",
			input:    "ethereumjs",
			expected: ClientEthereumJS,
		},
		// Partial matches (contains)
		{
			name:     "lighthouse in user agent",
			input:    "Lighthouse/v4.5.0-1234567/x86_64-linux",
			expected: ClientLighthouse,
		},
		{
			name:     "nimbus with version",
			input:    "nimbus-eth2/v23.10.0",
			expected: ClientNimbus,
		},
		{
			name:     "teku with full path",
			input:    "teku/v23.10.1/linux-x86_64/-eclipseadoptium-openjdk64bitservervm-java-17",
			expected: ClientTeku,
		},
		{
			name:     "prysm with metadata",
			input:    "Prysm/v4.0.8/b1234567",
			expected: ClientPrysm,
		},
		{
			name:     "geth in full string",
			input:    "Geth/v1.13.5-stable-1234567/linux-amd64/go1.21.4",
			expected: ClientGeth,
		},
		{
			name:     "besu with version",
			input:    "besu/v23.10.0",
			expected: ClientBesu,
		},
		{
			name:     "nethermind in string",
			input:    "Nethermind/v1.21.0+123456789",
			expected: ClientNethermind,
		},
		{
			name:     "erigon with metadata",
			input:    "erigon/2.54.0/linux-amd64/go1.21.4",
			expected: ClientErigon,
		},
		{
			name:     "reth with version",
			input:    "reth/v0.1.0-alpha.13",
			expected: ClientReth,
		},
		// Unknown cases
		{
			name:     "unknown client",
			input:    "someunknownclient",
			expected: ClientUnknown,
		},
		{
			name:     "empty string",
			input:    "",
			expected: ClientUnknown,
		},
		{
			name:     "random string",
			input:    "Mozilla/5.0 Firefox/120.0",
			expected: ClientUnknown,
		},
		// Edge cases
		{
			name:     "client name as substring",
			input:    "mylighthouseclient",
			expected: ClientLighthouse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClientFromString(tt.input)
			if result != tt.expected {
				t.Errorf("ClientFromString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestClientConstants(t *testing.T) {
	expectedConstants := map[Client]string{
		ClientUnknown:    "unknown",
		ClientLighthouse: "lighthouse",
		ClientNimbus:     "nimbus",
		ClientTeku:       "teku",
		ClientPrysm:      "prysm",
		ClientLodestar:   "lodestar",
		ClientGrandine:   "grandine",
		ClientCaplin:     "caplin",
		ClientGeth:       "geth",
		ClientBesu:       "besu",
		ClientNethermind: "nethermind",
		ClientErigon:     "erigon",
		ClientReth:       "reth",
		ClientEthereumJS: "ethereumjs",
	}

	for client, expected := range expectedConstants {
		if string(client) != expected {
			t.Errorf("Client constant %v = %v, want %v", client, string(client), expected)
		}
	}
}

func TestAllConsensusClients(t *testing.T) {
	expectedConsensusClients := []Client{
		ClientLighthouse,
		ClientNimbus,
		ClientTeku,
		ClientPrysm,
		ClientLodestar,
		ClientGrandine,
		ClientCaplin,
	}

	if len(AllConsensusClients) != len(expectedConsensusClients) {
		t.Errorf("AllConsensusClients length = %d, want %d", len(AllConsensusClients), len(expectedConsensusClients))
	}

	for i, client := range expectedConsensusClients {
		if i < len(AllConsensusClients) && AllConsensusClients[i] != client {
			t.Errorf("AllConsensusClients[%d] = %v, want %v", i, AllConsensusClients[i], client)
		}
	}
}

func TestAllExecutionClients(t *testing.T) {
	expectedExecutionClients := []Client{
		ClientGeth,
		ClientBesu,
		ClientNethermind,
		ClientErigon,
		ClientReth,
		ClientEthereumJS,
	}

	if len(AllExecutionClients) != len(expectedExecutionClients) {
		t.Errorf("AllExecutionClients length = %d, want %d", len(AllExecutionClients), len(expectedExecutionClients))
	}

	for i, client := range expectedExecutionClients {
		if i < len(AllExecutionClients) && AllExecutionClients[i] != client {
			t.Errorf("AllExecutionClients[%d] = %v, want %v", i, AllExecutionClients[i], client)
		}
	}
}

func TestAllClients(t *testing.T) {
	// Check that AllClients contains all unique clients
	clientMap := make(map[Client]bool)
	for _, client := range AllClients {
		clientMap[client] = true
	}

	// Verify it contains ClientUnknown
	if !clientMap[ClientUnknown] {
		t.Error("AllClients should contain ClientUnknown")
	}

	// Verify it contains all consensus clients (excluding duplicates from execution)
	consensusOnlyClients := []Client{
		ClientLighthouse,
		ClientNimbus,
		ClientTeku,
		ClientPrysm,
		ClientLodestar,
		ClientGrandine,
		ClientCaplin,
	}

	for _, client := range consensusOnlyClients {
		if !clientMap[client] {
			t.Errorf("AllClients should contain consensus client %v", client)
		}
	}

	// Verify it contains all execution clients
	for _, client := range AllExecutionClients {
		if !clientMap[client] {
			t.Errorf("AllClients should contain execution client %v", client)
		}
	}

	// Check expected total count (1 unknown + 7 consensus-only + 6 execution = 14)
	expectedCount := 14
	if len(AllClients) != expectedCount {
		t.Errorf("AllClients length = %d, want %d", len(AllClients), expectedCount)
	}
}

func TestClientIdentifiersConsistency(t *testing.T) {
	// Verify that clientIdentifiers map is consistent with Client constants
	for identifier, client := range clientIdentifiers {
		// The identifier should match the client string value
		if identifier != string(client) {
			t.Errorf("Inconsistent mapping: identifier %q maps to client %q", identifier, string(client))
		}

		// ClientFromString should return the correct client for the identifier
		result := ClientFromString(identifier)
		if result != client {
			t.Errorf("ClientFromString(%q) = %v, but clientIdentifiers maps to %v", identifier, result, client)
		}
	}

	// Verify all clients (except unknown) are in clientIdentifiers
	allDefinedClients := []Client{
		ClientLighthouse,
		ClientNimbus,
		ClientTeku,
		ClientPrysm,
		ClientLodestar,
		ClientGrandine,
		ClientCaplin,
		ClientGeth,
		ClientBesu,
		ClientNethermind,
		ClientErigon,
		ClientReth,
		ClientEthereumJS,
	}

	for _, client := range allDefinedClients {
		found := false
		for _, mappedClient := range clientIdentifiers {
			if mappedClient == client {
				found = true

				break
			}
		}
		if !found {
			t.Errorf("Client %v is not present in clientIdentifiers map", client)
		}
	}
}
