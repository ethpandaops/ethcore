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

func TestParseExecutionClientVersion(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		implementation string
		version        string
		versionMajor   string
		versionMinor   string
		versionPatch   string
	}{
		// Real-world examples from docstring
		{
			name:           "geth standard format",
			input:          "Geth/v1.16.4-stable-41714b49/linux-amd64/go1.24.7",
			implementation: "Geth",
			version:        "1.16.4-stable-41714b49",
			versionMajor:   "1",
			versionMinor:   "16",
			versionPatch:   "4",
		},
		{
			name:           "erigon lowercase no v prefix",
			input:          "erigon/3.0.14/linux-amd64/go1.23.11",
			implementation: "erigon",
			version:        "3.0.14",
			versionMajor:   "3",
			versionMinor:   "0",
			versionPatch:   "14",
		},
		{
			name:           "nethermind plus separator",
			input:          "Nethermind/v1.32.4+1c4c7c0a/linux-x64/dotnet9.0.7",
			implementation: "Nethermind",
			version:        "1.32.4+1c4c7c0a",
			versionMajor:   "1",
			versionMinor:   "32",
			versionPatch:   "4",
		},
		{
			name:           "besu lowercase",
			input:          "besu/v25.7.0/linux-x86_64/openjdk-java-21",
			implementation: "besu",
			version:        "25.7.0",
			versionMajor:   "25",
			versionMinor:   "7",
			versionPatch:   "0",
		},
		{
			name:           "reth dash separator for commit",
			input:          "reth/v1.8.2-9c30bf7/x86_64-unknown-linux-gnu",
			implementation: "reth",
			version:        "1.8.2-9c30bf7",
			versionMajor:   "1",
			versionMinor:   "8",
			versionPatch:   "2",
		},
		// Edge cases
		{
			name:           "empty string",
			input:          "",
			implementation: "",
			version:        "",
			versionMajor:   "",
			versionMinor:   "",
			versionPatch:   "",
		},
		{
			name:           "implementation only",
			input:          "Geth",
			implementation: "Geth",
			version:        "",
			versionMajor:   "",
			versionMinor:   "",
			versionPatch:   "",
		},
		{
			name:           "implementation with trailing slash",
			input:          "Geth/",
			implementation: "Geth",
			version:        "",
			versionMajor:   "",
			versionMinor:   "",
			versionPatch:   "",
		},
		{
			name:           "major only",
			input:          "Geth/v1",
			implementation: "Geth",
			version:        "1",
			versionMajor:   "1",
			versionMinor:   "",
			versionPatch:   "",
		},
		{
			name:           "major.minor only",
			input:          "Geth/v1.16",
			implementation: "Geth",
			version:        "1.16",
			versionMajor:   "1",
			versionMinor:   "16",
			versionPatch:   "",
		},
		{
			name:           "version without v prefix",
			input:          "client/1.2.3",
			implementation: "client",
			version:        "1.2.3",
			versionMajor:   "1",
			versionMinor:   "2",
			versionPatch:   "3",
		},
		{
			name:           "alpha version suffix",
			input:          "reth/v0.1.0-alpha.13",
			implementation: "reth",
			version:        "0.1.0-alpha.13",
			versionMajor:   "0",
			versionMinor:   "1",
			versionPatch:   "0",
		},
		{
			name:           "complex suffix",
			input:          "Client/v2.3.4-beta+build.567",
			implementation: "Client",
			version:        "2.3.4-beta+build.567",
			versionMajor:   "2",
			versionMinor:   "3",
			versionPatch:   "4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impl, ver, major, minor, patch := ParseExecutionClientVersion(tt.input)

			if impl != tt.implementation {
				t.Errorf("implementation = %q, want %q", impl, tt.implementation)
			}

			if ver != tt.version {
				t.Errorf("version = %q, want %q", ver, tt.version)
			}

			if major != tt.versionMajor {
				t.Errorf("versionMajor = %q, want %q", major, tt.versionMajor)
			}

			if minor != tt.versionMinor {
				t.Errorf("versionMinor = %q, want %q", minor, tt.versionMinor)
			}

			if patch != tt.versionPatch {
				t.Errorf("versionPatch = %q, want %q", patch, tt.versionPatch)
			}
		})
	}
}

func TestSplitVersionString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      string
		expected []string
	}{
		{
			name:     "simple split by slash",
			input:    "a/b/c",
			sep:      "/",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "split by dot",
			input:    "1.2.3",
			sep:      ".",
			expected: []string{"1", "2", "3"},
		},
		{
			name:     "empty string",
			input:    "",
			sep:      "/",
			expected: nil,
		},
		{
			name:     "no separator found",
			input:    "abc",
			sep:      "/",
			expected: []string{"abc"},
		},
		{
			name:     "consecutive separators",
			input:    "a//b",
			sep:      "/",
			expected: []string{"a", "b"},
		},
		{
			name:     "trailing separator",
			input:    "a/b/",
			sep:      "/",
			expected: []string{"a", "b"},
		},
		{
			name:     "leading separator",
			input:    "/a/b",
			sep:      "/",
			expected: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitVersionString(tt.input, tt.sep)

			if len(result) != len(tt.expected) {
				t.Errorf("len(result) = %d, want %d", len(result), len(tt.expected))

				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("result[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}
