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
		allDiscoverableNodes(t, foundation, epgNetwork, crawlerConfig, logger)
	})
}

// allDiscoverableNodes tests that all nodes are discoverable.
func allDiscoverableNodes(t *testing.T, tf *kurtosis.TestFoundation, network network.Network, config *TestCrawlerConfig, logger *logrus.Logger) {
	t.Helper()

	// Get all our peer IDs
	identities, successful := setupNodeTracking(t, tf, network, logger)

	// Create mutex for synchronizing access to the successful map
	mu := &sync.Mutex{}

	// Create our discovery instance which we'll use to manually add peers
	manual := &discovery.Manual{}

	// Setup and start the crawler
	cr := setupCrawler(t, network, logger, manual, config)

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
	feedENRsToCrawler(t, tf, network, logger, manual, successful, mu, config.CrawlerTimeout)
}

// setupNodeTracking sets up tracking of node identities and crawl status.
func setupNodeTracking(t *testing.T, tf *kurtosis.TestFoundation, network network.Network, logger *logrus.Logger) (map[string]*types.Identity, map[string]bool) {
	t.Helper()

	var (
		identities = make(map[string]*types.Identity)
		successful = make(map[string]bool)
	)

	// Get all consensus clients
	consensusClients := network.ConsensusClients().All()

	for _, client := range consensusClients {
		// Create a beacon node client to fetch identity
		opts := beacon.DefaultOptions()
		opts = opts.DisablePrometheusMetrics()
		opts.HealthCheck.Interval.Duration = 250 * time.Millisecond

		beaconNode := beacon.NewNode(
			logger.WithField("client", client.Name()),
			&beacon.Config{
				Name: client.Name(),
				Addr: client.BeaconAPIURL(),
			},
			fmt.Sprintf("crawler-test-%s", client.Name()),
			*opts,
		)

		// Start the beacon node temporarily
		nodeCtx, nodeCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer nodeCancel()

		go func() {
			if err := beaconNode.Start(nodeCtx); err != nil {
				logger.WithError(err).Warnf("Failed to start beacon node %s", client.Name())
			}
		}()

		// Wait for it to be healthy
		for i := 0; i < 20; i++ {
			if beaconNode.Healthy() {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		identity, err := beaconNode.FetchNodeIdentity(context.Background())
		require.NoError(t, err, "Failed to fetch node identity for %s", client.Name())

		identities[client.Name()] = identity
		successful[client.Name()] = false

		logger.Infof("Identified peer ID for %s: %s", client.Name(), identity.PeerID)
	}

	return identities, successful
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
func setupCrawlerEventHandlers(t *testing.T, cr *crawler.Crawler, logger *logrus.Logger, identities map[string]*types.Identity, successful map[string]bool, mu *sync.Mutex) {
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
func feedENRsToCrawler(t *testing.T, tf *kurtosis.TestFoundation, network network.Network, logger *logrus.Logger, manual *discovery.Manual, successful map[string]bool, mu *sync.Mutex, timeout time.Duration) {
	t.Helper()

	// Create a context with timeout for crawler operations
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Get all consensus clients
	consensusClients := network.ConsensusClients().All()

	// Start feeding in the ENR's
	go func() {
		// Get all our enrs
		for _, client := range consensusClients {
			// Create a beacon node client to fetch identity
			opts := beacon.DefaultOptions()
			opts = opts.DisablePrometheusMetrics()
			opts.HealthCheck.Interval.Duration = 250 * time.Millisecond

			beaconNode := beacon.NewNode(
				logger.WithField("client", client.Name()),
				&beacon.Config{
					Name: client.Name(),
					Addr: client.BeaconAPIURL(),
				},
				fmt.Sprintf("crawler-test-%s", client.Name()),
				*opts,
			)

			// Start the beacon node temporarily
			nodeCtx, nodeCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer nodeCancel()

			go func() {
				if err := beaconNode.Start(nodeCtx); err != nil {
					logger.WithError(err).Warnf("Failed to start beacon node %s", client.Name())
				}
			}()

			// Wait for it to be healthy
			for i := 0; i < 20; i++ {
				if beaconNode.Healthy() {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}

			identity, err := beaconNode.FetchNodeIdentity(context.Background())
			require.NoError(t, err, "Failed to fetch node identity")

			if complete, ok := successful[client.Name()]; ok && complete {
				logger.Infof("Already have status/metadata for participant: %s", client.Name())

				continue
			}

			// Give nodes extra time to initialize their P2P stack
			// before they can properly handle identify protocol requests
			logger.Infof("Waiting for node %s to initialize P2P stack", client.Name())
			time.Sleep(5 * time.Second)

			logger.Infof("Adding node %s's ENR to discovery pool (%s)", client.Name(), identity.ENR)

			en, err := discovery.ENRToEnode(identity.ENR)
			require.NoError(t, err, "Failed to convert ENR to enode")

			if err := manual.AddNode(context.Background(), en); err != nil {
				logger.Errorf("Failed to add node: %v", err)
			}
		}
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
