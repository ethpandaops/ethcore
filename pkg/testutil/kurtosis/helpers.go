package kurtosis

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon"
	ethcoreEthereum "github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/ethpandaops/ethereum-package-go/pkg/network"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// GetNATExitIP returns the appropriate IP address for containers to reach the host.
// On macOS/Windows with Docker Desktop, this is 127.0.0.1 due to automatic port forwarding.
// On Linux, we need to use the docker bridge gateway IP (typically 172.17.0.1).
func GetNATExitIP() string {
	// On macOS/Windows, Docker Desktop forwards localhost
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return "127.0.0.1"
	}

	// On Linux, try to get the docker0 bridge IP
	// First, try to get the default gateway from inside perspective
	// This is typically 172.17.0.1 for Docker's default bridge
	iface, err := net.InterfaceByName("docker0")
	if err == nil {
		addrs, err := iface.Addrs()
		if err == nil && len(addrs) > 0 {
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
	}

	// Fallback: use Docker's default bridge gateway
	// This is almost always 172.17.0.1 unless Docker is configured differently
	return "172.17.0.1"
}

// WaitForBeaconNode waits for a beacon node to be healthy with context and timeout.
// It creates a temporary beacon node client to monitor the health of the specified
// beacon node URL. The function returns when the node becomes healthy or when the
// context is cancelled/times out.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - logger: Logger instance for diagnostic output
//   - beaconURL: The URL of the beacon node to monitor
//   - name: A descriptive name for the beacon node (used in logs)
//   - timeout: Maximum duration to wait for the node to become healthy
//
// Returns an error if the node doesn't become healthy within the timeout period
// or if the context is cancelled.
func WaitForBeaconNode(ctx context.Context, logger logrus.FieldLogger, beaconURL, name string, timeout time.Duration) error {
	if logger == nil {
		logger = logrus.New()
	}

	logger = logger.WithFields(logrus.Fields{
		"beacon_url": beaconURL,
		"name":       name,
		"timeout":    timeout,
	})

	logger.Info("Waiting for beacon node to be healthy")

	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)

	defer cancel()

	// Create beacon configuration
	beaconConfig := &ethcoreEthereum.Config{
		BeaconNodeAddress: beaconURL,
		BeaconNodeHeaders: make(map[string]string),
	}

	// Create beacon options with shorter health check interval
	opts := beacon.DefaultOptions()
	opts.HealthCheck.Interval.Duration = 250 * time.Millisecond

	// Create the beacon node
	beaconNode, err := ethcoreEthereum.NewBeaconNode(
		logger,
		"testing",
		beaconConfig,
		*opts,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to create beacon node client for %s", name)
	}

	// Start the beacon node in a separate goroutine
	nodeCtx, nodeCancel := context.WithCancel(timeoutCtx)
	defer nodeCancel()

	startErr := make(chan error, 1)

	go func() {
		if err := beaconNode.Start(nodeCtx); err != nil {
			startErr <- err
		}
	}()

	// Wait for the node to become healthy
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-timeoutCtx.Done():
			return errors.Wrapf(timeoutCtx.Err(), "timeout waiting for beacon node %s to be healthy", name)
		case err := <-startErr:
			return errors.Wrapf(err, "beacon node %s failed to start", name)
		case <-ticker.C:
			if beaconNode.IsHealthy() {
				logger.WithField("duration", time.Since(startTime)).Info("Beacon node is healthy")

				return nil
			}

			logger.Debug("Beacon node not yet healthy, continuing to wait...")
		}
	}
}

// GetRandomBeaconClient returns a random beacon client from the foundation.
// This is useful for tests that need to interact with any beacon node without
// preference for a specific client type.
//
// Parameters:
//   - foundation: The test foundation containing beacon clients
//
// Returns the name of a randomly selected beacon client, or an error if no
// beacon clients are available.
func GetRandomBeaconClient(foundation *TestFoundation) (string, error) {
	if foundation == nil {
		return "", errors.New("foundation is nil")
	}

	foundation.mu.Lock()
	defer foundation.mu.Unlock()

	if len(foundation.BeaconClients) == 0 {
		return "", errors.New("no beacon clients available")
	}

	// Use math/rand for test randomness (not cryptographic)
	//nolint:gosec // G404: This is test code selecting a random beacon client, not security-critical
	index := rand.Intn(len(foundation.BeaconClients))

	return foundation.BeaconClients[index], nil
}

// AssertNetworkHealth asserts all beacon nodes in the network are healthy.
// This function is useful in tests to ensure the network is in a good state
// before proceeding with test operations.
//
// Parameters:
//   - t: Testing instance for assertions
//   - ctx: Context for cancellation
//   - foundation: The test foundation containing network information
//   - epgNetwork: The ethereum package network instance
//
// The function will fail the test if any beacon node is not healthy within
// the specified timeout period.
func AssertNetworkHealth(t *testing.T, ctx context.Context, foundation *TestFoundation, epgNetwork network.Network) {
	t.Helper()

	require.NotNil(t, foundation, "foundation must not be nil")
	require.NotNil(t, epgNetwork, "network must not be nil")

	// Get all consensus clients
	consensusClients := epgNetwork.ConsensusClients().All()
	require.NotEmpty(t, consensusClients, "no consensus clients found in network")

	// Create a logger if foundation doesn't have one
	logger := foundation.Logger
	if logger == nil {
		logger = logrus.New()
	}

	logger.WithField("client_count", len(consensusClients)).Info("Checking health of all beacon nodes")

	// Check each client's health in parallel
	type healthResult struct {
		name string
		err  error
	}

	results := make(chan healthResult, len(consensusClients))
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

	defer cancel()

	// Launch health checks concurrently
	for _, consensusClient := range consensusClients {
		go func(c interface {
			Name() string
			BeaconAPIURL() string
		}) {
			err := WaitForBeaconNode(
				checkCtx,
				logger.WithField("client", c.Name()),
				c.BeaconAPIURL(),
				c.Name(),
				20*time.Second,
			)
			results <- healthResult{name: c.Name(), err: err}
		}(consensusClient)
	}

	// Collect results
	var unhealthyNodes []string

	for i := 0; i < len(consensusClients); i++ {
		result := <-results
		if result.err != nil {
			unhealthyNodes = append(unhealthyNodes, fmt.Sprintf("%s: %v", result.name, result.err))
		}
	}

	// Assert all nodes are healthy
	if len(unhealthyNodes) > 0 {
		t.Fatalf("The following beacon nodes are not healthy:\n%s", strings.Join(unhealthyNodes, "\n"))
	}

	logger.Info("All beacon nodes are healthy")
}

// GetBeaconClientByType finds a beacon client by its type (e.g., "lighthouse", "teku").
// This is useful when tests need to interact with a specific client implementation.
//
// Parameters:
//   - foundation: The test foundation containing network information
//   - epgNetwork: The ethereum package network instance
//   - clientType: The type of client to find (case-insensitive)
//
// Returns the name of the first beacon client matching the specified type,
// or an error if no matching client is found.
func GetBeaconClientByType(foundation *TestFoundation, epgNetwork network.Network, clientType string) (string, error) {
	if foundation == nil {
		return "", errors.New("foundation is nil")
	}

	if epgNetwork == nil {
		return "", errors.New("network is nil")
	}

	if clientType == "" {
		return "", errors.New("clientType cannot be empty")
	}

	// Normalize the client type to lowercase for comparison
	targetType := strings.ToLower(clientType)

	// Get all consensus clients
	consensusClients := epgNetwork.ConsensusClients().All()
	if len(consensusClients) == 0 {
		return "", errors.New("no consensus clients available in network")
	}

	// Search for a client matching the requested type
	for _, consensusClient := range consensusClients {
		// Get the client type as a string
		ct := fmt.Sprintf("%v", consensusClient.Type())

		if strings.ToLower(ct) == targetType {
			// Verify this client is in the foundation's list
			foundation.mu.Lock()
			for _, beaconClient := range foundation.BeaconClients {
				if beaconClient == consensusClient.Name() {
					foundation.mu.Unlock()

					return consensusClient.Name(), nil
				}
			}

			foundation.mu.Unlock()
		}
	}

	return "", errors.Errorf("no beacon client found with type %q", clientType)
}

// GetHostIPForContainers is a deprecated alias for GetNATExitIP.
// This function is provided for backwards compatibility with existing tests.
//
// Deprecated: Use GetNATExitIP instead.
func GetHostIPForContainers() string {
	return GetNATExitIP()
}

// GetEPGNetwork returns the ethereum-package-go network from the foundation.
// It performs a type assertion to convert from interface{} to network.Network.
// Returns nil if the foundation or network is not properly initialized.
func GetEPGNetwork(foundation *TestFoundation) network.Network {
	if foundation == nil || foundation.EPGNetwork == nil {
		return nil
	}

	// Type assert to network.Network
	if epgNet, ok := foundation.EPGNetwork.(network.Network); ok {
		return epgNet
	}

	return nil
}
