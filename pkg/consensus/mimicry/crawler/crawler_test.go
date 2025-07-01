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
		DialTimeout:     30 * time.Second, // Generous timeout for Kurtosis environment
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
		allDiscoverableNodes(t, epgNetwork, crawlerConfig, logger)
	})
}

// allDiscoverableNodes tests that all nodes are discoverable.
func allDiscoverableNodes(
	t *testing.T,
	network network.Network,
	config *TestCrawlerConfig,
	logger *logrus.Logger,
) {
	t.Helper()

	// Setup beacon nodes and get their identities.
	nodeInfos, successful := setupBeaconNodes(t, network, logger)
	defer cleanupBeaconNodes(nodeInfos, logger)

	// Create mutex for synchronizing access to the successful map
	mu := &sync.Mutex{}

	// Create our discovery instance which we'll use to manually add peers
	manual := &discovery.Manual{}

	// Setup and start the crawler
	cr := setupCrawler(t, network, logger, manual, config)

	// Extract identities from nodeInfos for event handlers
	identities := make(map[string]*types.Identity)
	for name, info := range nodeInfos {
		identities[name] = info.Identity
	}

	// Create a sink of the crawler's events
	setupCrawlerEventHandlers(t, cr, logger, identities, successful, mu)

	// Wait until the crawler is ready
	select {
	case <-cr.OnReady:
		logger.Info("Crawler is ready")
	case <-time.After(config.CrawlerTimeout):
		t.Fatal("Timed out waiting for crawler to be ready")
	}

	// Feed ENRs to the crawler and wait for results
	feedENRsToCrawler(t, nodeInfos, logger, manual, successful, mu, config.CrawlerTimeout)
}

// setupBeaconNodes creates and starts beacon nodes for all consensus clients.
func setupBeaconNodes(t *testing.T, network network.Network, logger *logrus.Logger) (map[string]*NodeInfo, map[string]bool) {
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
		opts := beacon.DefaultOptions()
		opts = opts.DisablePrometheusMetrics()
		opts.HealthCheck.Interval.Duration = 250 * time.Millisecond
		opts.HealthCheck.SuccessfulResponses = 1

		// Create the beacon node using ethereum.NewBeaconNode
		beaconNode, err := ethereum.NewBeaconNode(
			nodeLogger,
			fmt.Sprintf("crawler-test-%s-%v", clientName, i),
			config,
			*opts,
		)
		require.NoError(t, err, "Failed to create beacon node for %s", clientName)

		wg.Add(1)

		// Register callback for when node is ready
		beaconNode.OnReady(ctx, func(ctx context.Context) error {
			defer wg.Done()

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

		// Start the node in a goroutine
		go func(bn *ethereum.BeaconNode, name string) {
			if err := bn.Start(ctx); err != nil {
				nodeLogger.WithError(err).Errorf("Failed to start beacon node %s", name)

				wg.Done() // Ensure we decrease the counter even on error
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
func setupCrawler(t *testing.T, network network.Network, logger *logrus.Logger, manual *discovery.Manual, config *TestCrawlerConfig) *crawler.Crawler {
	t.Helper()

	// Get the first consensus client
	consensusClients := network.ConsensusClients().All()
	require.NotEmpty(t, consensusClients, "No consensus clients found")

	firstClient := consensusClients[0]
	logger.Infof("Using beacon node: %s", firstClient.Name())

	// Create and start the crawler
	cr := crawler.New(logger, &crawler.Config{
		DialConcurrency: config.DialConcurrency,
		CooloffDuration: config.CooloffDuration,
		DialTimeout:     config.DialTimeout,
		UserAgent:       "ethcore/crawler-test",
		Node: &host.Config{
			IPAddr: net.ParseIP("127.0.0.1"),
		},
		Beacon: &ethereum.Config{
			BeaconNodeAddress: firstClient.BeaconAPIURL(),
			NetworkOverride:   "kurtosis",
		},
	}, manual)

	// Start the crawler in a goroutine
	go func() {
		if err := cr.Start(context.Background()); err != nil {
			logger.Errorf("Crawler failed: %v", err)
		}
	}()

	return cr
}

// setupCrawlerEventHandlers sets up event handlers for the crawler.
func setupCrawlerEventHandlers(
	t *testing.T,
	cr *crawler.Crawler,
	logger *logrus.Logger,
	identities map[string]*types.Identity,
	successful map[string]bool,
	mu *sync.Mutex,
) {
	t.Helper()

	cr.OnSuccessfulCrawl(func(peerID peer.ID, enr *enode.Node, status *common.Status, metadata *common.MetaData) {
		logger.Infof("Got status/metadata: %s", peerID)

		// Get the service name
		found := false

		for name, identity := range identities {
			if identity.PeerID == peerID.String() {
				logger.Infof("Got a successful crawl for %s", name)

				mu.Lock()
				successful[name] = true
				mu.Unlock()

				found = true

				break
			}
		}

		if !found {
			logger.Errorf("Failed to find peer ID in our list of identities. Peer ID: %s", peerID)
		}
	})

	cr.OnFailedCrawl(func(peerID peer.ID, err crawler.CrawlError) {
		logger.Errorf("Failed to crawl peer %s: %v", peerID, err)
	})
}

// feedENRsToCrawler feeds ENRs to the crawler and waits for results.
func feedENRsToCrawler(
	t *testing.T,
	nodeInfos map[string]*NodeInfo,
	logger *logrus.Logger,
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

				continue
			}
			mu.Unlock()

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
func cleanupBeaconNodes(nodeInfos map[string]*NodeInfo, logger *logrus.Logger) {
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
