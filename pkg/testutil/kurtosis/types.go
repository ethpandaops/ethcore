package kurtosis

import (
	"sync"
	"testing"
	"time"

	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/enclaves"
	"github.com/sirupsen/logrus"
)

// TestFoundation represents the foundational structure for Kurtosis-based tests.
// It provides the core components needed to set up and manage test networks,
// including configuration, network management, beacon clients, NAT settings,
// and thread-safe access to shared resources.
type TestFoundation struct {
	// Config holds the network configuration for the test
	Config *NetworkConfig

	// Network represents the active Kurtosis network instance
	Network *enclaves.EnclaveContext

	// EPGNetwork is the ethereum-package-go network instance.
	// This is stored as interface{} to avoid circular dependencies,
	// but should be cast to network.Network when used.
	EPGNetwork interface{}

	// BeaconClients contains the list of beacon client identifiers used in the test
	BeaconClients []string

	// NAT specifies whether NAT (Network Address Translation) is enabled for the test network
	NAT bool

	// Logger provides structured logging for test operations
	Logger *logrus.Logger

	// mu ensures thread-safe access to the TestFoundation fields
	mu sync.Mutex
}

// NetworkConfig defines the configuration parameters for a Kurtosis test network.
// It specifies how the network should be created, managed, and torn down during testing.
type NetworkConfig struct {
	// Name is the unique identifier for the test network
	Name string

	// Timeout specifies the maximum duration for network operations
	Timeout time.Duration

	// KurtosisEnclave represents the Kurtosis enclave configuration
	KurtosisEnclave string

	// KeepAlive determines whether the network should persist after the test completes
	KeepAlive bool

	// TestNet indicates whether this is a test network configuration
	TestNet bool

	// ElChain specifies the execution layer chain configuration
	ElChain string

	// participants stores the number of participants in the network
	participants int

	// PortOffset specifies the port offset for avoiding conflicts between test packages
	// For example, if PortOffset is 1000, EL ports start at 33000 and CL ports at 34000
	PortOffset int
}

// NetworkManager defines the interface for managing Kurtosis test networks.
// It provides methods to acquire and release network resources for testing.
type NetworkManager interface {
	// GetNetwork retrieves or creates a Kurtosis network based on the provided configuration.
	// It returns an enclave context that can be used to interact with the network,
	// or an error if the network cannot be obtained.
	GetNetwork(t *testing.T, config *NetworkConfig) (*enclaves.EnclaveContext, error)

	// ReleaseNetwork releases the resources associated with a test network.
	// It should be called when the test is complete to ensure proper cleanup.
	// Returns an error if the network cannot be properly released.
	ReleaseNetwork(t *testing.T, config *NetworkConfig) error
}
