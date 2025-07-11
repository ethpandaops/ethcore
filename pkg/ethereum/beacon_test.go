package ethereum_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/ethpandaops/ethcore/pkg/testutil/kurtosis"
	"github.com/ethpandaops/ethereum-package-go/pkg/network"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBeaconNode tests that single and multiple beacon node(s) can be initialized and becomes ready.
func TestBeaconNode(t *testing.T) {
	// Get the shared test foundation config from TestMain
	testConfig := GetTestFoundation()
	require.NotNil(t, testConfig, "Test foundation config must be initialized")

	// Get or create the actual network using kurtosis.GetNetwork with real testing.T
	foundation, err := kurtosis.GetNetwork(t, testConfig.Config)
	require.NoError(t, err, "Failed to get network")

	// Initialize logger
	logger := foundation.Logger
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	// Get the EPG network from foundation
	epgNetwork := kurtosis.GetEPGNetwork(foundation)
	require.NotNil(t, epgNetwork, "EPG network must be available")

	t.Run("single_node_ready", func(t *testing.T) {
		singleNoteTest(t, logger, epgNetwork)
	})

	t.Run("multi_node_ready", func(t *testing.T) {
		multiNoteTest(t, logger, epgNetwork)
	})
}

func singleNoteTest(t *testing.T, logger *logrus.Logger, epgNetwork network.Network) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get the first beacon client from the network
	consensusClients := epgNetwork.ConsensusClients().All()
	require.NotEmpty(t, consensusClients, "No consensus clients found")

	beaconClient := consensusClients[0]
	beaconURL := beaconClient.BeaconAPIURL()

	// Configure the beacon node
	config := &ethereum.Config{
		BeaconNodeAddress: beaconURL,
	}

	opts := &ethereum.Options{Options: beacon.DefaultOptions()}
	opts.DisablePrometheusMetrics()
	opts.HealthCheck.Interval.Duration = time.Second * 3
	opts.HealthCheck.SuccessfulResponses = 1
	opts.BeaconSubscription.Enabled = true
	opts.BeaconSubscription.Topics = []string{"head"}

	// Create the beacon node
	beaconNode, err := ethereum.NewBeaconNode(
		logger.WithField("test", "single_node"),
		fmt.Sprintf("test-single-node-%s", beaconClient.Name()),
		config,
		opts,
	)
	require.NoError(t, err, "Failed to create beacon node")

	// Track readiness
	ready := make(chan struct{})

	// Register a callback to be executed when the node is ready
	beaconNode.OnReady(func(ctx context.Context) error {
		logger.Info("Single beacon node is ready!")

		// Verify we can access metadata
		metadata := beaconNode.Metadata()
		require.NotNil(t, metadata, "Metadata should not be nil")

		networkInfo := metadata.Network
		require.NotNil(t, networkInfo, "Network should not be nil")

		logger.WithFields(logrus.Fields{
			"network":    networkInfo.Name,
			"network_id": networkInfo.ID,
		}).Info("Connected to network")

		close(ready)

		return nil
	})

	// Start the beacon node
	err = beaconNode.Start(ctx)
	require.NoError(t, err, "Failed to start beacon node")

	// Wait for ready signal
	select {
	case <-ready:
		logger.Info("Single node test completed successfully")
	case <-ctx.Done():
		t.Fatal("Timeout waiting for beacon node to be ready")
	}

	// Verify node is healthy
	require.True(t, beaconNode.IsHealthy(), "Beacon node should be healthy")

	net := beaconNode.Metadata().GetNetwork()
	assert.NotNil(t, net, "Network should not be nil")

	spec := beaconNode.Metadata().GetSpec()
	assert.NotNil(t, spec, "Spec should not be nil")

	identity, err := beaconNode.Metadata().GetNodeIdentity()
	require.NoError(t, err, "Failed to get node identity")
	assert.NotNil(t, identity, "Node identity should not be nil")

	// Clean up
	err = beaconNode.Stop(context.Background())
	require.NoError(t, err, "Failed to stop beacon node")
}

func multiNoteTest(t *testing.T, logger *logrus.Logger, epgNetwork network.Network) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get all consensus clients from network
	consensusClients := epgNetwork.ConsensusClients().All()
	numNodes := len(consensusClients)
	if numNodes > 3 {
		numNodes = 3
	}

	var (
		nodes []*ethereum.BeaconNode
		mu    sync.Mutex
		wg    sync.WaitGroup
	)

	// Create and start all beacon nodes
	for i := 0; i < numNodes; i++ {
		beaconClient := consensusClients[i]
		nodeLogger := logger.WithField("node", fmt.Sprintf("beacon-%d", i))

		config := &ethereum.Config{
			BeaconNodeAddress: beaconClient.BeaconAPIURL(),
		}

		opts := &ethereum.Options{Options: beacon.DefaultOptions()}
		opts.DisablePrometheusMetrics()
		opts.HealthCheck.Interval.Duration = time.Second * 3
		opts.HealthCheck.SuccessfulResponses = 1
		opts.BeaconSubscription.Enabled = true
		opts.BeaconSubscription.Topics = []string{"head"}

		nodeLogger.WithField("beacon_url", beaconClient.BeaconAPIURL()).Info("Creating beacon node")

		// Create the beacon node
		node, err := ethereum.NewBeaconNode(
			nodeLogger,
			fmt.Sprintf("test-node-%d", i),
			config,
			opts,
		)
		require.NoError(t, err, "Failed to create beacon node %d", i)

		// Track this node
		mu.Lock()
		nodes = append(nodes, node)
		mu.Unlock()

		// Add to wait group for this node
		wg.Add(1)

		// Register callback for when node is ready
		nodeIndex := i
		node.OnReady(func(ctx context.Context) error {
			defer wg.Done()

			nodeLogger.Info("Node is ready")

			// Log network information
			metadata := node.Metadata()
			nodeLogger.WithFields(logrus.Fields{
				"network":    metadata.Network.Name,
				"node_index": nodeIndex,
			}).Info("Node connected to network")

			return nil
		})

		// Start the node in a goroutine
		go func(n *ethereum.BeaconNode, idx int) {
			nodeLogger.Info("Starting beacon node")
			if err := n.Start(ctx); err != nil {
				nodeLogger.WithError(err).Error("Failed to start beacon node")
				wg.Done() // Decrement counter since OnReady won't be called for failed nodes
			}
		}(node, i)
	}

	// Wait for all nodes to be ready
	logger.Info("Waiting for all nodes to be ready...")
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("All nodes are ready!")
	case <-ctx.Done():
		t.Fatal("Timeout waiting for all nodes to be ready")
	}

	// Verify all nodes are healthy
	mu.Lock()
	for i, node := range nodes {
		require.True(t, node.IsHealthy(), "Node %d should be healthy", i)

		// Verify we can call Synced without errors
		err := node.Synced(context.Background())
		require.NoError(t, err, "Node %d should be synced", i)
	}
	mu.Unlock()

	// Clean up all nodes
	var cleanupWg sync.WaitGroup
	for _, node := range nodes {
		cleanupWg.Add(1)
		go func(n *ethereum.BeaconNode) {
			defer cleanupWg.Done()
			if err := n.Stop(context.Background()); err != nil {
				logger.WithError(err).Error("Failed to stop beacon node")
			}
		}(node)
	}
	cleanupWg.Wait()

	logger.Info("Multiple nodes test completed successfully")
}
