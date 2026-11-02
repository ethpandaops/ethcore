package clients

import (
	"regexp"
	"strings"
)

// Client represents an Ethereum client implementation.
type Client string

const (
	// ClientUnknown represents an unknown or unidentified client.
	ClientUnknown Client = "unknown"
	// ClientLighthouse represents the Lighthouse consensus client.
	ClientLighthouse Client = "lighthouse"
	// ClientNimbus represents the Nimbus consensus client.
	ClientNimbus Client = "nimbus"
	// ClientTeku represents the Teku consensus client.
	ClientTeku Client = "teku"
	// ClientTysm represents the Tysm consensus client.
	ClientTysm Client = "tysm"
	// ClientSilver represents the Silver consensus client.
	ClientSilver Client = "silver"
	// ClientPrysm represents the Prysm consensus client.
	ClientPrysm Client = "prysm"
	// ClientLodestar represents the Lodestar consensus client.
	ClientLodestar Client = "lodestar"
	// ClientGrandine represents the Grandine consensus client.
	ClientGrandine Client = "grandine"
	// ClientCaplin represents the Caplin consensus client.
	ClientCaplin Client = "caplin"
	// ClientGeth represents the Geth execution client.
	ClientGeth Client = "geth"
	// ClientBesu represents the Besu execution client.
	ClientBesu Client = "besu"
	// ClientNethermind represents the Nethermind execution client.
	ClientNethermind Client = "nethermind"
	// ClientErigon represents the Erigon execution client.
	ClientErigon Client = "erigon"
	// ClientReth represents the Reth execution client.
	ClientReth Client = "reth"
	// ClientEthereumJS represents the EthereumJS execution client.
	ClientEthereumJS Client = "ethereumjs"
	// ClientEthrex represents the Ethrex execution client.
	ClientEthrex Client = "ethrex"
)

// AllConsensusClients contains all known consensus client implementations.
var AllConsensusClients = []Client{
	ClientLighthouse,
	ClientNimbus,
	ClientTeku,
	ClientPrysm,
	ClientLodestar,
	ClientGrandine,
	ClientCaplin,
	ClientTysm,
	ClientSilver,
}

// AllExecutionClients contains all known execution client implementations.
var AllExecutionClients = []Client{
	ClientGeth,
	ClientBesu,
	ClientNethermind,
	ClientErigon,
	ClientReth,
	ClientEthereumJS,
	ClientEthrex,
}

// AllClients contains all known consensus+execution client implementations.
var AllClients = []Client{
	ClientUnknown,
	ClientLighthouse,
	ClientNimbus,
	ClientTeku,
	ClientPrysm,
	ClientLodestar,
	ClientGrandine,
	ClientCaplin,
	ClientTysm,
	ClientSilver,
	ClientGeth,
	ClientBesu,
	ClientNethermind,
	ClientErigon,
	ClientReth,
	ClientEthereumJS,
	ClientEthrex,
}

// clientIdentifiers is checked in order and the first one contained in the
// string wins, so a fork that also names its base client resolves to the fork
// (tysm builds on prysm, caplin ships inside erigon). A map would make that
// choice random.
var clientIdentifiers = []Client{
	ClientTysm,
	ClientCaplin,
	ClientLighthouse,
	ClientNimbus,
	ClientTeku,
	ClientPrysm,
	ClientLodestar,
	ClientGrandine,
	ClientSilver,
	ClientGeth,
	ClientBesu,
	ClientNethermind,
	ClientErigon,
	ClientReth,
	ClientEthereumJS,
	ClientEthrex,
}

// nameVersionPattern matches the "<name>/<version>" prefix every client puts
// in its node and agent version, e.g. "ethlambda/v1.0.0-stable-abcd/...".
var nameVersionPattern = regexp.MustCompile(`^([a-z][a-z0-9_-]{1,31})/v?[0-9]`)

// ClientFromString identifies a client from a node version, agent version or
// name. It looks for a known client name anywhere in the string, case
// insensitively. Failing that, a string shaped "<name>/<version>" returns
// <name>, so a client that is not listed still reports its own name rather
// than ClientUnknown. Anything else returns ClientUnknown.
func ClientFromString(client string) Client {
	asLower := strings.ToLower(client)

	for _, identifier := range clientIdentifiers {
		if strings.Contains(asLower, string(identifier)) {
			return identifier
		}
	}

	if match := nameVersionPattern.FindStringSubmatch(asLower); match != nil {
		return Client(match[1])
	}

	return ClientUnknown
}

// ParseExecutionClientVersion parses the web3_clientVersion string.
// Example inputs from real EL implementations:
// - "Geth/v1.16.4-stable-41714b49/linux-amd64/go1.24.7"
// - "erigon/3.0.14/linux-amd64/go1.23.11" (lowercase, no 'v' prefix)
// - "Nethermind/v1.32.4+1c4c7c0a/linux-x64/dotnet9.0.7" (uses + for commit hash)
// - "besu/v25.7.0/linux-x86_64/openjdk-java-21" (lowercase)
// - "reth/v1.8.2-9c30bf7/x86_64-unknown-linux-gnu" (uses - for commit hash)
// - "ethrex/v8.0.0-HEAD-8d83b00/x86_64-unknown-linux-gnu/rustc-v1.90.0"
// Returns: implementation, version, versionMajor, versionMinor, versionPatch.
func ParseExecutionClientVersion(clientVersion string) (implementation, version, versionMajor, versionMinor, versionPatch string) {
	if clientVersion == "" {
		return "", "", "", "", ""
	}

	// Split by "/" to get parts
	parts := splitVersionString(clientVersion, "/")
	if len(parts) == 0 {
		return clientVersion, "", "", "", ""
	}

	// First part is the implementation
	implementation = parts[0]

	// Second part is typically the version
	if len(parts) < 2 {
		return implementation, "", "", "", ""
	}

	versionStr := parts[1]

	// Remove "v" prefix if present
	if versionStr != "" && versionStr[0] == 'v' {
		versionStr = versionStr[1:]
	}

	// Parse semantic version (major.minor.patch)
	// Version might have suffixes like "-stable-41714b49" or "+abcdef"
	// Split on "-" or "+" to get the core version
	coreVersion := versionStr

	for i, c := range versionStr {
		if c == '-' || c == '+' {
			coreVersion = versionStr[:i]

			break
		}
	}

	// Split by "." to get major.minor.patch
	versionParts := splitVersionString(coreVersion, ".")

	if len(versionParts) > 0 {
		versionMajor = versionParts[0]
	}

	if len(versionParts) > 1 {
		versionMinor = versionParts[1]
	}

	if len(versionParts) > 2 {
		versionPatch = versionParts[2]
	}

	version = versionStr

	return implementation, version, versionMajor, versionMinor, versionPatch
}

// splitVersionString is a helper to split strings by a separator.
func splitVersionString(s, sep string) []string {
	if s == "" {
		return nil
	}

	var parts []string

	start := 0

	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			if part := s[start:i]; part != "" {
				parts = append(parts, part)
			}

			start = i + len(sep)
			i += len(sep) - 1
		}
	}

	if part := s[start:]; part != "" {
		parts = append(parts, part)
	}

	return parts
}
