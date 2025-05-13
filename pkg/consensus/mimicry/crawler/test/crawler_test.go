package crawler_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon/api/types"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/crawler"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/host"
	"github.com/ethpandaops/ethcore/pkg/discovery"
	"github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/services"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	// Default timeout for crawler operations.
	defaultCrawlerTimeout = 30 * time.Second
)

// TestConfig holds configuration for the crawler tests.
type TestConfig struct {
	// KurtosisConfig is the configuration for the Kurtosis environment.
	KurtosisConfig *KurtosisConfig
	// CrawlerTimeout is the timeout for crawler operations.
	CrawlerTimeout time.Duration
	// DialConcurrency is the number of concurrent dials.
	DialConcurrency int
	// CooloffDuration is the duration to wait between dials.
	CooloffDuration time.Duration
}

// DefaultTestConfig returns a default configuration for the crawler tests.
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		KurtosisConfig:  DefaultKurtosisConfig(),
		CrawlerTimeout:  defaultCrawlerTimeout,
		DialConcurrency: 10,
		CooloffDuration: 1 * time.Second,
	}
}

// TestOptions holds options for the crawler tests.
type TestOptions struct {
	// Config is the test configuration
	Config *TestConfig
	// Logger is the logger for the tests
	Logger *logrus.Logger
}

// DefaultTestOptions returns default options for the crawler tests.
func DefaultTestOptions() *TestOptions {
	return &TestOptions{
		Config: DefaultTestConfig(),
		Logger: nil, // Will be initialized in the test
	}
}

// validateEnclaveName validates that the enclave name follows Kurtosis's naming rules.
func validateEnclaveName(name string) error {
	if !enclaveNameRegex.MatchString(name) {
		return fmt.Errorf("enclave name '%s' doesn't match allowed enclave name regex '^[-A-Za-z0-9]{1,60}$'", name)
	}

	return nil
}

// Test_RunKurtosisTests runs the Kurtosis-based tests.
func Test_RunKurtosisTests(t *testing.T) {
	// Initialize test options
	options := DefaultTestOptions()

	// Override from environment variables if provided.
	if envEnclave := os.Getenv("KURTOSIS_ENCLAVE"); envEnclave != "" {
		// Validate the enclave name.
		if err := validateEnclaveName(envEnclave); err != nil {
			t.Fatalf("Invalid enclave name: %v", err)
		}
		options.Config.KurtosisConfig.EnclaveName = envEnclave
	}

	if os.Getenv("KEEP_ENCLAVE") == "true" {
		options.Config.KurtosisConfig.KeepEnclave = true
	}

	// Initialize logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	options.Logger = logger

	// Setup the Kurtosis environment
	testFoundation := SetupKurtosisEnvironment(t, options.Config.KurtosisConfig, logger)

	// Run all the kurtosis tests in parallel
	t.Run("ListParticipants", func(t *testing.T) {
		ListParticipants(t, testFoundation)
	})
	t.Run("AllDiscoverableNodes", func(t *testing.T) {
		AllDiscoverableNodes(t, testFoundation, options)
	})
}

// ListParticipants lists all participants in the Kurtosis environment.
func ListParticipants(t *testing.T, tf *TestFoundation) {
	t.Helper()

	services, err := tf.EnclaveCtx.GetServices()
	require.NoError(t, err, "Failed to get services")

	t.Logf("Found %d services in enclave %s:", len(services), tf.EnclaveCtx.GetEnclaveName())

	for name := range services {
		t.Logf("- %s", name)
	}
}

// AllDiscoverableNodes tests that all nodes are discoverable.
func AllDiscoverableNodes(t *testing.T, tf *TestFoundation, options *TestOptions) {
	t.Helper()

	logger := options.Logger

	// Wait until genesis has happened
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err := tf.WaitForGenesis(ctx)
	require.NoError(t, err, "Failed waiting for genesis")

	// Get all our peer IDs
	identities, successful := setupNodeTracking(t, tf, logger)

	// Create our discovery instance which we'll use to manually add peers
	manual := &discovery.Manual{}

	// Setup and start the crawler
	cr := setupCrawler(t, tf, logger, manual, options.Config)

	// Create a sink of the crawler's events
	setupCrawlerEventHandlers(t, cr, logger, identities, successful)

	// Wait until the crawler is ready
	select {
	case <-cr.OnReady:
		logger.Info("Crawler is ready")
	case <-time.After(options.Config.CrawlerTimeout):
		t.Fatal("Timed out waiting for crawler to be ready")
	}

	// Feed ENRs to the crawler and wait for results
	feedENRsToCrawler(t, tf, logger, manual, successful, options.Config.CrawlerTimeout)
}

// setupNodeTracking sets up tracking of node identities and crawl status.
func setupNodeTracking(t *testing.T, tf *TestFoundation, logger *logrus.Logger) (map[services.ServiceName]*types.Identity, map[services.ServiceName]bool) {
	t.Helper()

	var (
		identities = make(map[services.ServiceName]*types.Identity)
		successful = make(map[services.ServiceName]bool)
	)

	for _, bn := range tf.BeaconNodes {
		identity, err := bn.Node.FetchNodeIdentity(context.Background())
		require.NoError(t, err, "Failed to fetch node identity")

		identities[bn.Service.GetServiceName()] = identity
		successful[bn.Service.GetServiceName()] = false

		logger.Infof("Identified peer ID for %s: %s", bn.Service.GetServiceName(), identity.PeerID)
	}

	return identities, successful
}

// setupCrawler creates and starts a crawler instance.
func setupCrawler(t *testing.T, tf *TestFoundation, logger *logrus.Logger, manual *discovery.Manual, config *TestConfig) *crawler.Crawler {
	t.Helper()

	// Get the first beacon node
	clServiceNames, err := GetCLServices(tf.EnclaveCtx)
	require.NoError(t, err, "Failed to get CL services")
	require.NotEmpty(t, clServiceNames, "No CL services found")

	clServiceName := clServiceNames[0]
	logger.Infof("Using CL service: %s", clServiceName)

	clService, err := tf.EnclaveCtx.GetServiceContext(clServiceName)
	require.NoError(t, err, "Failed to get service context")

	beaconPort := clService.GetPublicPorts()["http"]
	require.NotNil(t, beaconPort, "Beacon port not found")

	// Create and start the crawler
	cr := crawler.New(logger, &crawler.Config{
		DialConcurrency: config.DialConcurrency,
		CooloffDuration: config.CooloffDuration,
		Node: &host.Config{
			IPAddr: net.ParseIP("127.0.0.1"),
		},
		Beacon: &ethereum.Config{
			BeaconNodeAddress: "http://" + net.JoinHostPort(clService.GetMaybePublicIPAddress(), fmt.Sprintf("%d", beaconPort.GetNumber())),
			Network:           "kurtosis",
		},
	}, "mimicry/crawler", "mimicry", manual)

	// Start the crawler in a goroutine
	go func() {
		if err := cr.Start(context.Background()); err != nil {
			logger.Errorf("Crawler failed: %v", err)
		}
	}()

	return cr
}

// setupCrawlerEventHandlers sets up event handlers for the crawler.
func setupCrawlerEventHandlers(t *testing.T, cr *crawler.Crawler, logger *logrus.Logger, identities map[services.ServiceName]*types.Identity, successful map[services.ServiceName]bool) {
	t.Helper()

	cr.OnSuccessfulCrawl(func(peerID peer.ID, status *common.Status, metadata *common.MetaData) {
		logger.Infof("Got status/metadata: %s", peerID)

		// Get the service name
		found := false

		for name, identity := range identities {
			if identity.PeerID == peerID.String() {
				logger.Infof("Got a successful crawl for %s", name)

				successful[name] = true
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
func feedENRsToCrawler(t *testing.T, tf *TestFoundation, logger *logrus.Logger, manual *discovery.Manual, successful map[services.ServiceName]bool, timeout time.Duration) {
	t.Helper()

	// Create a context with timeout for crawler operations
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Start feeding in the ENR's
	go func() {
		// Get all our enrs
		for _, bn := range tf.BeaconNodes {
			identity, err := bn.Node.FetchNodeIdentity(context.Background())
			require.NoError(t, err, "Failed to fetch node identity")

			if complete, ok := successful[bn.Service.GetServiceName()]; ok && complete {
				logger.Infof("Already have status/metadata for participant: %s", bn.Service.GetServiceName())

				continue
			}

			logger.Infof("Adding node %s's ENR to discovery pool (%s)", bn.Service.GetServiceName(), identity.ENR)

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
			okPeers := 0
			for _, complete := range successful {
				if complete {
					okPeers++
				}
			}

			logger.Infof("Discovered %d/%d peers", okPeers, len(successful))

			if okPeers == len(successful) {
				// Test complete!
				return
			} else {
				missingPeers := []services.ServiceName{}
				for name, complete := range successful {
					if !complete {
						missingPeers = append(missingPeers, name)
					}
				}
				logger.Infof("Missing peers: %v", missingPeers)
			}
		}
	}
}
