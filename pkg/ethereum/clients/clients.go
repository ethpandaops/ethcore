package clients

import "strings"

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
}

// AllExecutionClients contains all known execution client implementations.
var AllExecutionClients = []Client{
	ClientGeth,
	ClientBesu,
	ClientNethermind,
	ClientErigon,
	ClientReth,
	ClientEthereumJS,
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
	ClientGeth,
	ClientBesu,
	ClientNethermind,
	ClientErigon,
	ClientReth,
	ClientEthereumJS,
}

// clientIdentifiers maps client-specific strings to their respective Client type.
var clientIdentifiers = map[string]Client{
	"lighthouse": ClientLighthouse,
	"nimbus":     ClientNimbus,
	"teku":       ClientTeku,
	"prysm":      ClientPrysm,
	"lodestar":   ClientLodestar,
	"grandine":   ClientGrandine,
	"caplin":     ClientCaplin,
	"geth":       ClientGeth,
	"besu":       ClientBesu,
	"nethermind": ClientNethermind,
	"erigon":     ClientErigon,
	"reth":       ClientReth,
	"ethereumjs": ClientEthereumJS,
}

// ClientFromString identifies a consensus client from a string identifier.
// It performs a case-insensitive search for known client names within the input string.
// Returns ClientUnknown if no known client is identified.
func ClientFromString(client string) Client {
	asLower := strings.ToLower(client)

	for identifier, clientType := range clientIdentifiers {
		if strings.Contains(asLower, identifier) {
			return clientType
		}
	}

	return ClientUnknown
}
