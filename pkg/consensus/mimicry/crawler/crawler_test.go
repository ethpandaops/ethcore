package crawler_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/beacon/pkg/beacon/api/types"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/crawler"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/host"
	"github.com/ethpandaops/ethcore/pkg/discovery"
	"github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/ethpandaops/ethcore/pkg/testutil/kurtosis"
	"github.com/ethpandaops/ethereum-package-go/pkg/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// NodeInfo holds beacon node and its identity information.
type NodeInfo struct {
	Node     *ethereum.BeaconNode
	Identity *types.Identity
}

// TestCrawlerConfig holds configuration for the crawler tests.
type TestCrawlerConfig struct {
	// CrawlerTimeout is the timeout for crawler operations.
	CrawlerTimeout time.Duration
	// DialConcurrency is the number of concurrent dials.
	DialConcurrency int
	// CooloffDuration is the duration to wait between dials.
	CooloffDuration time.Duration
	// DialTimeout is the timeout for dialing peers.
	DialTimeout time.Duration
}

// DefaultTestCrawlerConfig returns a default configuration for the crawler tests.
func DefaultTestCrawlerConfig() *TestCrawlerConfig {
	return &TestCrawlerConfig{
		CrawlerTimeout:  60 * time.Second,
		DialConcurrency: 10,
		CooloffDuration: 1 * time.Second,
		DialTimeout:     30 * time.Second,
	}
}

// Test_RunKurtosisTests runs the network-based tests.
func Test_RunKurtosisTests(t *testing.T) {
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

	// Initialize test config for crawler-specific settings
	crawlerConfig := DefaultTestCrawlerConfig()

	// Get the EPG network from foundation
	epgNetwork := kurtosis.GetEPGNetwork(foundation)
	require.NotNil(t, epgNetwork, "EPG network must be available")

	t.Run("discover-crawlable-nodes", func(t *testing.T) {
		allDiscoverableNodes(t, logger, epgNetwork, crawlerConfig)
	})
}

// allDiscoverableNodes tests that all nodes are discoverable.
func allDiscoverableNodes(
	t *testing.T,
	logger *logrus.Logger,
	network network.Network,
	config *TestCrawlerConfig,
) {
	t.Helper()

	// Setup beacon nodes and get their identities.
	nodeInfos, successful := setupBeaconNodes(t, logger, network)
	defer cleanupBeaconNodes(logger, nodeInfos)

	// Create mutex for synchronizing access to the successful map
	mu := &sync.Mutex{}

	// Create our discovery instance, which we'll use to manually add peers
	manual := &discovery.Manual{}

	// Setup and start the crawler
	cr, crawlerBeaconName := setupCrawler(t, logger, network, manual, config)

	// Remove the crawler's beacon node from the nodes to discover
	// This prevents any potential conflicts
	if crawlerBeaconName != "" {
		logger.Infof("Excluding crawler's beacon node '%s' from discovery targets", crawlerBeaconName)
		delete(nodeInfos, crawlerBeaconName)
		delete(successful, crawlerBeaconName)
	}

	// Extract identities from nodeInfos for event handlers
	identities := make(map[string]*types.Identity)
	for name, info := range nodeInfos {
		identities[name] = info.Identity
	}

	// Create a sink of the crawler's events
	setupCrawlerEventHandlers(t, logger, cr, identities, successful, mu)

	// Wait until the crawler is ready
	// The crawler needs to wait for HeadSlot > 0 which can take some time
	logger.Info("Waiting for crawler to be ready (needs HeadSlot > 0)...")
	select {
	case <-cr.OnReady:
		logger.Info("Crawler is ready")
	case <-time.After(config.CrawlerTimeout):
		t.Fatal("Timed out waiting for crawler to be ready")
	}

	// Wait a bit to ensure all nodes have their P2P stacks fully initialized
	// This is important because the identify protocol requires both sides to be ready
	logger.Info("Waiting for nodes to fully initialize their P2P stacks...")
	time.Sleep(5 * time.Second)

	// Feed ENRs to the crawler and wait for results
	feedENRsToCrawler(t, logger, nodeInfos, manual, successful, mu, config.CrawlerTimeout)
}

// setupBeaconNodes creates and starts beacon nodes for all consensus clients.
func setupBeaconNodes(
	t *testing.T,
	logger *logrus.Logger,
	network network.Network,
) (map[string]*NodeInfo, map[string]bool) {
	t.Helper()

	var (
		nodeInfos  = make(map[string]*NodeInfo)
		successful = make(map[string]bool)
		mu         sync.Mutex
		wg         sync.WaitGroup
	)

	// Get all consensus clients
	consensusClients := network.ConsensusClients().All()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	for i, client := range consensusClients {
		clientName := client.Name()
		nodeLogger := logger.WithField("client", clientName)

		// Configure the beacon node
		config := &ethereum.Config{
			BeaconNodeAddress: client.BeaconAPIURL(),
		}

		// Set up beacon options
		opts := &ethereum.Options{Options: beacon.DefaultOptions()}
		opts.DisablePrometheusMetrics()
		opts.HealthCheck.Interval.Duration = time.Millisecond * 250
		opts.HealthCheck.SuccessfulResponses = 1
		opts.BeaconSubscription.Enabled = true
		opts.BeaconSubscription.Topics = []string{"head"}

		// Create the beacon node using ethereum.NewBeaconNode
		beaconNode, err := ethereum.NewBeaconNode(
			nodeLogger,
			fmt.Sprintf("crawler-test-%s-%v", clientName, i),
			config,
			opts,
		)
		require.NoError(t, err, "Failed to create beacon node for %s", clientName)

		wg.Add(1)

		// Start the node in a goroutine
		go func(bn *ethereum.BeaconNode, name string) {
			defer wg.Done() // Always call Done when this goroutine exits

			// Register callback for when node is ready
			readyHandled := false
			beaconNode.OnReady(func(ctx context.Context) error {
				if readyHandled {
					return nil // Prevent double execution
				}
				readyHandled = true

				// Fetch node identity
				identity, err := beaconNode.Node().FetchNodeIdentity(ctx)
				if err != nil {
					return fmt.Errorf("failed to fetch node identity for %s: %w", clientName, err)
				}

				mu.Lock()
				nodeInfos[clientName] = &NodeInfo{
					Node:     beaconNode,
					Identity: identity,
				}

				successful[clientName] = false
				mu.Unlock()

				nodeLogger.Infof("Identified peer ID for %s: %s", clientName, identity.PeerID)

				return nil
			})

			if err := bn.Start(ctx); err != nil {
				nodeLogger.WithError(err).Errorf("Failed to start beacon node %s", name)
			}
		}(beaconNode, clientName)
	}

	// Wait for all nodes to be ready and fetch their identities
	done := make(chan struct{})
	go func() {
		wg.Wait()

		close(done)
	}()

	select {
	case <-done:
		logger.Info("All beacon nodes are ready and identities fetched")
	case <-ctx.Done():
		t.Fatal("Timeout waiting for beacon nodes to be ready")
	}

	return nodeInfos, successful
}

// setupCrawler creates and starts a crawler instance.
func setupCrawler(
	t *testing.T,
	logger *logrus.Logger,
	network network.Network,
	manual *discovery.Manual,
	config *TestCrawlerConfig,
) (*crawler.Crawler, string) {
	t.Helper()

	// Get the first consensus client
	consensusClients := network.ConsensusClients().All()
	require.NotEmpty(t, consensusClients, "No consensus clients found")

	// Use a separate dedicated beacon node for the crawler to avoid conflicts
	// This ensures the crawler isn't trying to discover the same node it's connected to
	firstClient := consensusClients[0]
	crawlerBeaconName := firstClient.Name()
	logger.Infof("Creating dedicated beacon node for crawler based on: %s", crawlerBeaconName)

	// Create a dedicated beacon node for the crawler
	beaconConfig := &ethereum.Config{
		BeaconNodeAddress: firstClient.BeaconAPIURL(),
		NetworkOverride:   "kurtosis",
	}

	opts := &ethereum.Options{Options: beacon.DefaultOptions()}
	opts.DisablePrometheusMetrics()
	opts.HealthCheck.Interval.Duration = time.Millisecond * 250
	opts.HealthCheck.SuccessfulResponses = 1
	opts.BeaconSubscription.Enabled = true
	opts.BeaconSubscription.Topics = []string{"head"}

	crawlerBeaconNode, err := ethereum.NewBeaconNode(
		logger.WithField("component", "crawler-beacon"),
		"crawler-dedicated-beacon",
		beaconConfig,
		opts,
	)
	require.NoError(t, err, "Failed to create crawler beacon node")

	// Start the dedicated beacon node and wait for it to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	beaconReady := make(chan struct{})
	crawlerBeaconNode.OnReady(func(ctx context.Context) error {
		close(beaconReady)

		return nil
	})

	go func() {
		if err := crawlerBeaconNode.Start(ctx); err != nil {
			logger.WithError(err).Error("Failed to start crawler beacon node")
		}
	}()

	select {
	case <-beaconReady:
		logger.Info("Crawler beacon node is ready")
	case <-ctx.Done():
		t.Fatal("Timeout waiting for crawler beacon node to be ready")
	}

	// Ensure cleanup of the beacon node
	t.Cleanup(func() {
		if err := crawlerBeaconNode.Stop(context.Background()); err != nil {
			logger.WithError(err).Error("Failed to stop crawler beacon node")
		}
	})

	// Create and start the crawler
	cr := crawler.New(logger, &crawler.Config{
		DialConcurrency: config.DialConcurrency,
		CooloffDuration: config.CooloffDuration,
		DialTimeout:     config.DialTimeout,
		Namespace:       "crawler-test",
		UserAgent:       "ethcore/crawler-test",
		Node: &host.Config{
			IPAddr: net.ParseIP("127.0.0.1"),
		},
		MaxRetryAttempts: 3,
		RetryBackoff:     2 * time.Second,
		Beacon:           beaconConfig,
	}, manual)

	// Start the crawler in a goroutine
	go func() {
		if err := cr.Start(context.Background()); err != nil {
			logger.Errorf("Crawler failed: %v", err)
		}
	}()

	return cr, crawlerBeaconName
}

// setupCrawlerEventHandlers sets up event handlers for the crawler.
func setupCrawlerEventHandlers(
	t *testing.T,
	logger *logrus.Logger,
	cr *crawler.Crawler,
	identities map[string]*types.Identity,
	successful map[string]bool,
	mu *sync.Mutex,
) {
	t.Helper()

	cr.OnSuccessfulCrawl(func(peerID peer.ID, enr *enode.Node, status *common.Status, metadata *common.MetaData) {
		// Get the service name
		found := false

		for name, identity := range identities {
			if identity.PeerID == peerID.String() {
				logger.Infof("Successfully crawled %s (status received and metadata exchanged)", name)

				mu.Lock()
				successful[name] = true
				mu.Unlock()

				found = true

				break
			}
		}

		if !found {
			logger.Warnf("Received successful crawl for unknown peer ID: %s", peerID)
		}
	})

	cr.OnFailedCrawl(func(peerID peer.ID, err crawler.CrawlError) {
		// Check if this peer is one we're tracking
		peerName := "unknown"
		for name, identity := range identities {
			if identity.PeerID == peerID.String() {
				peerName = name

				break
			}
		}

		// Log detailed error information to help debug connection issues
		logger.WithFields(logrus.Fields{
			"peer_name": peerName,
			"peer_id":   peerID,
			"error":     err.Error(),
			"err_type":  fmt.Sprintf("%T", err),
		}).Error("Failed to crawl peer")
	})
}

// feedENRsToCrawler feeds ENRs to the crawler and waits for results.
func feedENRsToCrawler(
	t *testing.T,
	logger *logrus.Logger,
	nodeInfos map[string]*NodeInfo,
	manual *discovery.Manual,
	successful map[string]bool,
	mu *sync.Mutex,
	timeout time.Duration,
) {
	t.Helper()

	// Create a context with timeout for crawler operations
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Start feeding in the ENR's
	go func() {
		// Process each node
		for clientName, nodeInfo := range nodeInfos {
			nodeLogger := logger.WithField("client", clientName)

			// Check if we already have this client's data
			mu.Lock()
			if complete, ok := successful[clientName]; ok && complete {
				mu.Unlock()

				nodeLogger.Infof("Already have status/metadata for participant: %s", clientName)

				continue
			}
			mu.Unlock()

			nodeLogger.Infof("Processing node %s (PeerID: %s)", clientName, nodeInfo.Identity.PeerID)
			nodeLogger.Infof("Adding node %s's ENR to discovery pool (%s)", clientName, nodeInfo.Identity.ENR)

			en, err := discovery.ENRToEnode(nodeInfo.Identity.ENR)
			if err != nil {
				nodeLogger.WithError(err).Errorf("Failed to convert ENR to enode")

				continue
			}

			if err := manual.AddNode(ctx, en); err != nil {
				nodeLogger.Errorf("Failed to add node: %v", err)
			}
		}

		logger.Info("Finished feeding ENRs to crawler")
	}()

	// Wait until we've discovered all the nodes or timeout.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for all nodes to be discovered: %v", ctx.Err())
		case <-ticker.C:
			mu.Lock()
			okPeers := 0
			for _, complete := range successful {
				if complete {
					okPeers++
				}
			}

			logger.Infof("Discovered %d/%d peers", okPeers, len(successful))

			if okPeers == len(successful) {
				mu.Unlock()
				// Test complete!
				return
			} else {
				missingPeers := []string{}
				for name, complete := range successful {
					if !complete {
						missingPeers = append(missingPeers, name)
					}
				}
				mu.Unlock()
				logger.Infof("Missing peers: %v", missingPeers)
			}
		}
	}
}

// cleanupBeaconNodes stops all beacon nodes.
func cleanupBeaconNodes(logger *logrus.Logger, nodeInfos map[string]*NodeInfo) {
	var wg sync.WaitGroup
	ctx := context.Background()

	for name, info := range nodeInfos {
		wg.Add(1)

		go func(nodeName string, nodeInfo *NodeInfo) {
			defer wg.Done()

			if err := nodeInfo.Node.Stop(ctx); err != nil {
				logger.WithError(err).Errorf("Failed to stop beacon node %s", nodeName)
			}
		}(name, info)
	}

	wg.Wait()

	logger.Info("All beacon nodes cleaned up")
}
