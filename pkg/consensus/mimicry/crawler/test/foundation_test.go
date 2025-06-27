package crawler_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/ethereum-package-go"
	"github.com/ethpandaops/ethereum-package-go/pkg/client"
	epgconfig "github.com/ethpandaops/ethereum-package-go/pkg/config"
	"github.com/ethpandaops/ethereum-package-go/pkg/network"
	"github.com/sirupsen/logrus"
)

// NetworkConfig holds configuration for the test network environment.
type NetworkConfig struct {
	// NetworkName is the name of the network to use.
	NetworkName string
	// KeepNetwork determines whether to keep the network after tests.
	KeepNetwork bool
	// NetworkTimeout is the timeout for network operations.
	NetworkTimeout time.Duration
	// BeaconHealthCheckInterval is the interval for beacon node health checks.
	BeaconHealthCheckInterval time.Duration
}

// BeaconNode represents a beacon node in the test environment.
type BeaconNode struct {
	// Node is the beacon API client.
	Node beacon.Node
	// Client is the ethereum-package-go consensus client.
	Client client.ConsensusClient
}

// TestFoundation provides the foundation for ethereum-package-go based tests.
type TestFoundation struct {
	// BeaconNodes are the beacon nodes in the test environment.
	BeaconNodes []*BeaconNode
	// Network is the ethereum-package-go network.
	Network network.Network
	// Config is the network configuration.
	Config *NetworkConfig
	// Logger is the logger for the test foundation.
	Logger *logrus.Logger
}

// DefaultNetworkConfig returns a default configuration for tests.
func DefaultNetworkConfig() *NetworkConfig {
	return &NetworkConfig{
		KeepNetwork:               false,
		NetworkName:               DefaultEnclaveName,
		NetworkTimeout:            DefaultNetworkTimeout,
		BeaconHealthCheckInterval: BeaconHealthCheckInterval,
	}
}

// SetupKurtosisEnvironment sets up the network environment and returns a TestFoundation.
func SetupKurtosisEnvironment(t *testing.T, config *NetworkConfig, logger *logrus.Logger) *TestFoundation {
	t.Helper()

	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	if config == nil {
		config = DefaultNetworkConfig()
	}

	logger.Infof("Setting up Ethereum network with name: %s", config.NetworkName)

	// Create context with timeout for setup operations.
	ctx, cancel := context.WithTimeout(context.Background(), config.NetworkTimeout)
	defer cancel()

	// Configure network options
	var opts []ethereum.RunOption

	// Handle KeepNetwork configuration
	if config.KeepNetwork {
		opts = append(opts, ethereum.WithOrphanOnExit())
	}

	// Always try to reuse existing network if available to avoid conflicts
	// This helps when previous test runs didn't clean up properly
	if config.NetworkName != "" {
		opts = append(opts, ethereum.WithReuse(config.NetworkName))
		logger.Infof("Attempting to reuse existing network: %s", config.NetworkName)
	}

	// Add timeout option
	opts = append(opts, ethereum.WithTimeout(config.NetworkTimeout))

	// Configure NAT exit IP based on environment
	// On macOS/Windows with Docker Desktop, localhost works due to port forwarding
	// On Linux, we need to use the docker bridge gateway IP
	natIP := getHostIPForContainers()
	logger.Infof("Setting NAT exit IP to %s", natIP)
	opts = append(opts, ethereum.WithNATExitIP(natIP))

	// Enable debug logs for tests.
	opts = append(opts, ethereum.WithGlobalLogLevel("debug"))

	// Load it up with some CL's.
	opts = append(opts, ethereum.WithParticipants([]epgconfig.ParticipantConfig{
		{ELType: client.Geth, CLType: client.Lighthouse, Count: 2},
		{ELType: client.Geth, CLType: client.Teku, Count: 1},
		{ELType: client.Geth, CLType: client.Prysm, Count: 2},
		{ELType: client.Geth, CLType: client.Lodestar, Count: 1},
		{ELType: client.Geth, CLType: client.Grandine, Count: 1},
	}))

	// Create the network
	network, err := ethereum.Run(ctx, opts...)
	if err != nil {
		t.Fatalf("Failed to create Ethereum network: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		if !config.KeepNetwork {
			logger.Info("Cleaning up network:", config.NetworkName)
			cleanupCtx, cancel := context.WithTimeout(context.Background(), config.NetworkTimeout)
			defer cancel()

			if cleanupErr := network.Cleanup(cleanupCtx); cleanupErr != nil {
				logger.Warnf("Failed to cleanup network %s: %v", config.NetworkName, cleanupErr)
			} else {
				logger.Infof("Successfully cleaned up network: %s", config.NetworkName)
			}
		} else {
			logger.Info("Skipping network cleanup because KeepNetwork=true")
		}
	})

	// Create and return the test foundation.
	tf := &TestFoundation{
		Network: network,
		Config:  config,
		Logger:  logger,
	}

	// Initialize the beacon nodes.
	if err := tf.initializeBeaconNodes(t); err != nil {
		t.Fatalf("Failed to initialize beacon nodes: %v", err)
	}

	return tf
}

// AllBeaconNodesHealthy returns true if all beacon nodes are healthy.
func (tf *TestFoundation) AllBeaconNodesHealthy() bool {
	for _, bn := range tf.BeaconNodes {
		if !bn.Node.Healthy() {
			return false
		}
	}

	return true
}

// RandomBeaconNode returns a random beacon node.
func (tf *TestFoundation) RandomBeaconNode() *BeaconNode {
	return tf.BeaconNodes[rand.IntN(len(tf.BeaconNodes))]
}

// GenesisHasHappened returns true if genesis has happened.
func (tf *TestFoundation) GenesisHasHappened() (bool, error) {
	bn := tf.RandomBeaconNode()

	wallclock := bn.Node.Wallclock()
	if wallclock == nil {
		return false, fmt.Errorf("wallclock not found")
	}

	slot, _, err := wallclock.Now()
	if err != nil {
		return false, fmt.Errorf("failed to get current slot: %w", err)
	}

	return slot.Number() > 0, nil
}

// WaitForGenesis waits until genesis has happened.
func (tf *TestFoundation) WaitForGenesis(ctx context.Context) error {
	tf.Logger.Info("Waiting for genesis...")

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for genesis: %w", ctx.Err())
		default:
			genesis, err := tf.GenesisHasHappened()
			if err != nil {
				return fmt.Errorf("failed to check if genesis has happened: %w", err)
			}

			if genesis {
				tf.Logger.Info("Genesis has happened")

				return nil
			}

			time.Sleep(1 * time.Second)
		}
	}
}

// initializeBeaconNodes initializes the beacon nodes in the test environment.
func (tf *TestFoundation) initializeBeaconNodes(t *testing.T) error {
	t.Helper()

	// Get all consensus clients from the network
	consensusClients := tf.Network.ConsensusClients().All()

	tf.Logger.Infof("Found %d consensus clients:", len(consensusClients))

	for _, client := range consensusClients {
		tf.Logger.Infof("- %s (%s)", client.Name(), client.Type())
	}

	if len(consensusClients) == 0 {
		t.Skip("No consensus clients found")
	}

	// Create all our beacon api instances.
	beaconNodes := []*BeaconNode{}

	for _, consensusClient := range consensusClients {
		tf.Logger.Infof("Initializing beacon node: %s", consensusClient.Name())

		opts := beacon.DefaultOptions()
		opts.HealthCheck.Interval.Duration = tf.Config.BeaconHealthCheckInterval

		b := beacon.NewNode(
			tf.Logger,
			&beacon.Config{
				Name: consensusClient.Name(),
				Addr: consensusClient.BeaconAPIURL(),
			},
			"testing",
			*opts,
		)

		beaconNodes = append(beaconNodes, &BeaconNode{
			Node:   b,
			Client: consensusClient,
		})
	}

	tf.BeaconNodes = beaconNodes

	// Start all beacon nodes.
	for _, bn := range tf.BeaconNodes {
		go func(bn *BeaconNode) {
			if err := bn.Node.Start(context.Background()); err != nil {
				tf.Logger.Errorf("Failed to start beacon node %s: %v", bn.Client.Name(), err)
			}
		}(bn)
	}

	// Wait for all the beacon nodes to be healthy.
	tf.Logger.Info("Waiting for all beacon nodes to be healthy...")
	startTime := time.Now()
	for !tf.AllBeaconNodesHealthy() {
		if time.Since(startTime) > GenesisWaitTimeout {
			return fmt.Errorf("timed out waiting for beacon nodes to be healthy")
		}
		time.Sleep(1 * time.Second)
	}
	tf.Logger.Info("All beacon nodes are healthy")

	return nil
}

// getHostIPForContainers returns the appropriate IP address for containers to reach the host.
// On macOS/Windows with Docker Desktop, this is 127.0.0.1 due to automatic port forwarding.
// On Linux, we need to use the docker bridge gateway IP (typically 172.17.0.1).
func getHostIPForContainers() string {
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
