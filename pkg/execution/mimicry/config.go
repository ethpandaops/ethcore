package mimicry

import (
	"bytes"
	"crypto/ecdsa"
	"time"
)

// StatusProvider is a callback function that returns a Status response
// for an incoming connection. It receives the remote Hello for context.
type StatusProvider func(remoteHello *Hello) (Status, error)

// Network defines filter criteria for allowed networks.
// All non-nil/non-zero fields must match for a peer to be accepted.
// If all fields are nil/zero, matches any network.
type Network struct {
	NetworkID  *uint64 // nil = don't filter on network ID
	ForkIDHash []byte  // nil = don't filter on fork ID hash
	ForkIDNext *uint64 // nil = don't filter on fork ID next
	Genesis    []byte  // nil = don't filter on genesis hash
}

// Matches returns true if the given Status matches this Network filter.
func (n *Network) Matches(status Status) bool {
	if n.NetworkID != nil && *n.NetworkID != status.GetNetworkID() {
		return false
	}

	if n.ForkIDHash != nil && !bytes.Equal(n.ForkIDHash, status.GetForkIDHash()) {
		return false
	}

	if n.ForkIDNext != nil && *n.ForkIDNext != status.GetForkIDNext() {
		return false
	}

	if n.Genesis != nil && !bytes.Equal(n.Genesis, status.GetGenesis()) {
		return false
	}

	return true
}

// ServerConfig configures the Server for accepting incoming connections.
type ServerConfig struct {
	// Name is the name to advertise in Hello messages.
	Name string

	// PrivateKey is the ECDSA private key for the server.
	// If nil, a new key will be generated.
	PrivateKey *ecdsa.PrivateKey

	// ListenAddr is the address to listen on (e.g., ":30303").
	ListenAddr string

	// MaxPeers is the maximum number of concurrent connections.
	// 0 means unlimited.
	MaxPeers int

	// HandshakeTimeout is the timeout for RLPx handshake.
	// Default: 5s
	HandshakeTimeout time.Duration

	// ReadTimeout is the timeout for reading messages.
	// Default: 30s
	ReadTimeout time.Duration

	// StatusProvider is a required callback that returns the Status
	// to send in response to incoming peers.
	StatusProvider StatusProvider

	// AllowedNetworks filters incoming peers by network.
	// If empty/nil, all networks are allowed.
	// If set, peer must match at least ONE of the networks.
	AllowedNetworks []Network
}

// DefaultServerConfig returns a ServerConfig with sensible defaults.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		HandshakeTimeout: 5 * time.Second,
		ReadTimeout:      30 * time.Second,
	}
}
