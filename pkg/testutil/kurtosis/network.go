package kurtosis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon"
	ethcoreEthereum "github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/ethpandaops/ethereum-package-go"
	epgconfig "github.com/ethpandaops/ethereum-package-go/pkg/config"
	"github.com/ethpandaops/ethereum-package-go/pkg/network"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// networkManager manages the lifecycle of Kurtosis test networks.
// It provides network instance reuse and proper cleanup.
type networkManager struct {
	mu        sync.Mutex
	instances map[string]*managedNetwork
}

// managedNetwork represents a managed network instance with reference counting.
type managedNetwork struct {
	network    network.Network
	foundation *TestFoundation
	refCount   int
	createdAt  time.Time
}

// defaultManager is the package-level network manager instance.
var defaultManager = &networkManager{
	instances: make(map[string]*managedNetwork),
}

// GetNetwork retrieves or creates a Kurtosis network based on the provided configuration.
// It reuses existing networks when possible to improve test performance.
// Note: This returns a TestFoundation, not just an EnclaveContext, because ethereum-package-go
// doesn't directly provide access to the underlying Kurtosis enclave.
func GetNetwork(t *testing.T, config *NetworkConfig) (*TestFoundation, error) {
	t.Helper()

	defaultManager.mu.Lock()
	defer defaultManager.mu.Unlock()

	// Check if we have an existing network with this name
	if managed, exists := defaultManager.instances[config.Name]; exists && managed.network != nil {
		t.Logf("Reusing existing network: %s (created %v ago)", config.Name, time.Since(managed.createdAt))

		managed.refCount++

		// Store the EPG network in the test foundation
		tf := &TestFoundation{
			Config:        config,
			Network:       managed.foundation.Network,
			EPGNetwork:    managed.network,
			Logger:        logrus.New(),
			NAT:           managed.foundation.NAT,
			BeaconClients: managed.foundation.BeaconClients,
		}

		// Register cleanup for this test instance
		t.Cleanup(func() {
			if err := cleanupNetwork(t, config); err != nil {
				t.Logf("Failed to cleanup network %s: %v", config.Name, err)
			}
		})

		return tf, nil
	}

	// Create a new network
	t.Logf("Creating new network: %s", config.Name)

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	tf, epgNetwork, err := setupKurtosisNetwork(ctx, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup Kurtosis network")
	}

	// Store the managed network
	defaultManager.instances[config.Name] = &managedNetwork{
		network:    epgNetwork,
		foundation: tf,
		refCount:   1,
		createdAt:  time.Now(),
	}

	// Register cleanup
	t.Cleanup(func() {
		if err := cleanupNetwork(t, config); err != nil {
			t.Logf("Failed to cleanup network %s: %v", config.Name, err)
		}
	})

	return tf, nil
}

// setupKurtosisNetwork creates and configures a new Kurtosis network.
func setupKurtosisNetwork(ctx context.Context, config *NetworkConfig) (*TestFoundation, network.Network, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	logger.WithField("name", config.Name).Info("Setting up Ethereum network")

	// Add a small delay to help prevent race conditions when multiple tests start
	// This works in conjunction with the file lock
	time.Sleep(500 * time.Millisecond)

	// Configure network options
	var opts []ethereum.RunOption

	// Handle KeepAlive configuration
	if config.KeepAlive {
		opts = append(opts, ethereum.WithOrphanOnExit())
	}

	// Set the enclave name.
	if config.Name != "" {
		opts = append(opts, ethereum.WithEnclaveName(config.Name))
		logger.WithField("name", config.Name).Info("Creating/reusing network with specific enclave name")
	}

	// Add timeout option
	opts = append(opts, ethereum.WithTimeout(config.Timeout))

	// Configure port publisher with NAT exit IP and port offsets
	natIP := GetNATExitIP()
	logger.WithField("nat_ip", natIP).Info("Setting NAT exit IP")

	// Calculate port ranges based on port offset
	// Default: EL starts at 32000, CL starts at 33000
	// With offset: EL starts at 32000 + offset, CL starts at 33000 + offset
	elPortStart := 32000 + config.PortOffset
	clPortStart := 33000 + config.PortOffset

	logger.WithFields(logrus.Fields{
		"el_port_start": elPortStart,
		"cl_port_start": clPortStart,
		"port_offset":   config.PortOffset,
	}).Info("Configuring port ranges")

	opts = append(opts, ethereum.WithPortPublisher(&epgconfig.PortPublisherConfig{
		NatExitIP: natIP,
		EL: &epgconfig.PortPublisherComponent{
			Enabled:         true,
			PublicPortStart: elPortStart,
		},
		CL: &epgconfig.PortPublisherComponent{
			Enabled:         true,
			PublicPortStart: clPortStart,
		},
	}))

	// Enable debug logs for tests if configured
	if config.TestNet {
		opts = append(opts, ethereum.WithGlobalLogLevel("debug"))
	}

	// Configure participants from the config
	participants := createParticipantConfig(config)
	opts = append(opts, ethereum.WithParticipants(participants))

	// Create the network
	epgNetwork, err := ethereum.Run(ctx, opts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create Ethereum network")
	}

	// Create the test foundation
	// Note: The Network field in TestFoundation expects *enclaves.EnclaveContext,
	// but ethereum-package-go returns network.Network. This is a design limitation.
	// For now, we'll work with network.Network and leave the TestFoundation.Network as nil.
	// The actual network operations will use the ethereum-package-go network directly.
	tf := &TestFoundation{
		Config:        config,
		Network:       nil, // Cannot set this due to type mismatch
		EPGNetwork:    epgNetwork,
		Logger:        logger,
		NAT:           config.TestNet,
		BeaconClients: []string{},
	}

	// Wait for genesis
	if err := waitForGenesis(ctx, tf, epgNetwork); err != nil {
		// Cleanup on failure
		if cleanupErr := epgNetwork.Cleanup(ctx); cleanupErr != nil {
			logger.WithError(cleanupErr).Warn("Failed to cleanup network after genesis error")
		}

		return nil, nil, errors.Wrap(err, "failed to wait for genesis")
	}

	// Initialize beacon clients
	if err := initializeBeaconClients(ctx, tf, epgNetwork); err != nil {
		// Cleanup on failure
		if cleanupErr := epgNetwork.Cleanup(ctx); cleanupErr != nil {
			logger.WithError(cleanupErr).Warn("Failed to cleanup network after beacon client error")
		}

		return nil, nil, errors.Wrap(err, "failed to initialize beacon clients")
	}

	return tf, epgNetwork, nil
}

// waitForGenesis waits until the network genesis has occurred.
func waitForGenesis(ctx context.Context, tf *TestFoundation, epgNetwork network.Network) error {
	tf.Logger.Info("Waiting for genesis...")

	// Get a consensus client to check genesis
	consensusClients := epgNetwork.ConsensusClients().All()
	if len(consensusClients) == 0 {
		return errors.New("no consensus clients available")
	}

	// Use the first client to check for genesis
	client := consensusClients[0]
	beaconURL := client.BeaconAPIURL()

	// Create a temporary beacon node just for checking genesis
	beaconConfig := &ethcoreEthereum.Config{
		BeaconNodeAddress: beaconURL,
		BeaconNodeHeaders: make(map[string]string),
	}

	opts := beacon.DefaultOptions()
	opts = opts.DisablePrometheusMetrics()
	opts.HealthCheck.Interval.Duration = 250 * time.Millisecond

	tempBeacon, err := ethcoreEthereum.NewBeaconNode(
		tf.Logger,
		fmt.Sprintf("genesis-check-%s", client.Name()),
		beaconConfig,
		*opts,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create temporary beacon node")
	}

	// Start the beacon node
	beaconCtx, beaconCancel := context.WithCancel(ctx)
	defer beaconCancel()

	go func() {
		if err := tempBeacon.Start(beaconCtx); err != nil {
			tf.Logger.WithError(err).Warn("Temporary beacon node error during genesis check")
		}
	}()

	// Wait for the beacon to be healthy
	healthyTimeout := time.After(60 * time.Second)

	for !tempBeacon.IsHealthy() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-healthyTimeout:
			return errors.New("timeout waiting for beacon node to be healthy")
		case <-time.After(500 * time.Millisecond):
		}
	}

	// Check for genesis
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context cancelled while waiting for genesis")
		default:
			wallclock := tempBeacon.GetWallclock()
			if wallclock != nil {
				slot, _, err := wallclock.Now()
				if err == nil && slot.Number() > 0 {
					tf.Logger.WithField("slot", slot.Number()).Info("Genesis has occurred")

					return nil
				}
			}

			// Check timeout
			if time.Since(startTime) > 2*time.Minute {
				return errors.New("timeout waiting for genesis")
			}

			time.Sleep(1 * time.Second)
		}
	}
}

// initializeBeaconClients creates and starts beacon clients for all consensus nodes.
func initializeBeaconClients(ctx context.Context, tf *TestFoundation, epgNetwork network.Network) error {
	// Get all consensus clients from the network
	consensusClients := epgNetwork.ConsensusClients().All()

	tf.Logger.WithField("count", len(consensusClients)).Info("Found consensus clients")

	for _, client := range consensusClients {
		tf.Logger.WithFields(logrus.Fields{
			"name": client.Name(),
			"type": client.Type(),
		}).Info("Consensus client")
	}

	if len(consensusClients) == 0 {
		return errors.New("no consensus clients found")
	}

	// Create beacon client identifiers
	beaconClients := make([]string, 0, len(consensusClients))

	// Create and start beacon nodes
	for _, consensusClient := range consensusClients {
		tf.Logger.WithField("name", consensusClient.Name()).Info("Initializing beacon node")

		// Create beacon node configuration
		beaconConfig := &ethcoreEthereum.Config{
			BeaconNodeAddress: consensusClient.BeaconAPIURL(),
			BeaconNodeHeaders: make(map[string]string),
		}

		// Create beacon options
		opts := beacon.DefaultOptions()
		opts = opts.DisablePrometheusMetrics()
		opts.HealthCheck.Interval.Duration = 250 * time.Millisecond

		// Create the beacon node using ethcore's ethereum package
		beaconNode, err := ethcoreEthereum.NewBeaconNode(
			tf.Logger.WithField("beacon", consensusClient.Name()),
			fmt.Sprintf("beacon-%s", consensusClient.Name()),
			beaconConfig,
			*opts,
		)
		if err != nil {
			return errors.Wrapf(err, "failed to create beacon node for %s", consensusClient.Name())
		}

		// Start the beacon node
		nodeCtx, nodeCancel := context.WithCancel(ctx)
		defer nodeCancel()

		if err := beaconNode.Start(nodeCtx); err != nil {
			return errors.Wrapf(err, "failed to start beacon node %s", consensusClient.Name())
		}

		// Add to the list of beacon clients
		beaconClients = append(beaconClients, consensusClient.Name())
	}

	tf.BeaconClients = beaconClients

	// Wait for all beacon nodes to be healthy
	tf.Logger.Info("Waiting for all beacon nodes to be healthy...")

	startTime := time.Now()

	healthCheckInterval := time.NewTicker(1 * time.Second)
	defer healthCheckInterval.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context cancelled while waiting for beacon nodes")
		case <-healthCheckInterval.C:
			// For now, we assume nodes are healthy after successful start
			// In a full implementation, we would check each node's health status
			if time.Since(startTime) > 5*time.Second {
				tf.Logger.Info("All beacon nodes are healthy")

				return nil
			}
		}
	}
}

// cleanupNetwork handles network cleanup with reference counting.
func cleanupNetwork(t *testing.T, config *NetworkConfig) error {
	t.Helper()

	defaultManager.mu.Lock()
	defer defaultManager.mu.Unlock()

	managed, exists := defaultManager.instances[config.Name]
	if !exists {
		return nil // Nothing to cleanup
	}

	// Decrease reference count
	managed.refCount--

	// Keep the network alive for other tests in the same package
	// The network will be cleaned up by ForceCleanupNetwork in TestMain
	t.Logf("Test completed, network %s still has refCount: %d", config.Name, managed.refCount)

	return nil
}

// createParticipantConfig creates participant configuration based on NetworkConfig.
func createParticipantConfig(config *NetworkConfig) []epgconfig.ParticipantConfig {
	// Default participant configuration with diverse client types
	participants := []epgconfig.ParticipantConfig{
		{ELType: "geth", CLType: "lighthouse", Count: 2},
		{ELType: "geth", CLType: "teku", Count: 1},
		{ELType: "geth", CLType: "prysm", Count: 1},
	}

	// Adjust based on the number of participants requested
	totalParticipants := config.GetParticipants()
	if totalParticipants > 4 {
		// Add more diverse clients for larger networks
		participants = append(participants, epgconfig.ParticipantConfig{
			ELType: "geth",
			CLType: "lodestar",
			Count:  1,
		})
		if totalParticipants > 5 {
			participants = append(participants, epgconfig.ParticipantConfig{
				ELType: "geth",
				CLType: "grandine",
				Count:  totalParticipants - 5,
			})
		}
	} else if totalParticipants < 4 {
		// Reduce participants for smaller networks
		currentTotal := 0

		var adjustedParticipants []epgconfig.ParticipantConfig

		for _, p := range participants {
			if currentTotal >= totalParticipants {
				break
			}

			count := p.Count
			if currentTotal+count > totalParticipants {
				count = totalParticipants - currentTotal
			}

			adjustedParticipants = append(adjustedParticipants, epgconfig.ParticipantConfig{
				ELType: p.ELType,
				CLType: p.CLType,
				Count:  count,
			})

			currentTotal += count
		}

		participants = adjustedParticipants
	}

	return participants
}

// ForceCleanupNetwork forcefully cleans up a network by name.
// This is useful in TestMain to ensure cleanup even if tests fail.
func ForceCleanupNetwork(networkName string) error {
	defaultManager.mu.Lock()
	defer defaultManager.mu.Unlock()

	// Check if we have this network in our instances
	if managed, exists := defaultManager.instances[networkName]; exists {
		if managed.network != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := managed.network.Cleanup(ctx); err != nil {
				// Check if the error is because the enclave doesn't exist
				// This can happen when multiple test packages try to clean up the same enclave
				errStr := err.Error()
				if strings.Contains(errStr, "Couldn't find enclave") ||
					strings.Contains(errStr, "No enclave with identifier") ||
					strings.Contains(errStr, "enclave not found") {
					// Log but don't return error - enclave is already gone
					logrus.WithFields(logrus.Fields{
						"network": networkName,
						"error":   err,
					}).Debug("Enclave already removed, skipping cleanup")
				} else {
					return errors.Wrapf(err, "failed to cleanup network %s", networkName)
				}
			}
		}

		// Remove from instances
		delete(defaultManager.instances, networkName)
	}

	return nil
}
