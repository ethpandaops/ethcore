package crawler_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/services"
	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
	"github.com/sirupsen/logrus"
)

const (
	// Default enclave name.
	defaultEnclaveName = "ethcore-test-enclave"
	// Default enclave timeout.
	defaultEnclaveTimeout = 60 * time.Second
)

// Kurtosis enclave name validation regex.
var enclaveNameRegex = regexp.MustCompile(`^[-A-Za-z0-9]{1,60}$`)

// KurtosisConfig holds configuration for Kurtosis test environment.
type KurtosisConfig struct {
	// EnclaveName is the name of the Kurtosis enclave to use.
	EnclaveName string
	// KeepEnclave determines whether to keep the enclave after tests.
	KeepEnclave bool
	// EnclaveTimeout is the timeout for enclave operations.
	EnclaveTimeout time.Duration
	// BeaconHealthCheckInterval is the interval for beacon node health checks.
	BeaconHealthCheckInterval time.Duration
}

// DefaultKurtosisConfig returns a default configuration for Kurtosis tests.
func DefaultKurtosisConfig() *KurtosisConfig {
	return &KurtosisConfig{
		EnclaveName:               defaultEnclaveName,
		KeepEnclave:               false,
		EnclaveTimeout:            defaultEnclaveTimeout,
		BeaconHealthCheckInterval: 250 * time.Millisecond,
	}
}

// BeaconNode represents a beacon node in the test environment.
type BeaconNode struct {
	// Node is the beacon API client.
	Node beacon.Node
	// Service is the Kurtosis service context.
	Service *services.ServiceContext
}

// TestFoundation provides the foundation for Kurtosis-based tests.
type TestFoundation struct {
	// BeaconNodes are the beacon nodes in the test environment.
	BeaconNodes []*BeaconNode
	// KurtosisCtx is the Kurtosis context.
	KurtosisCtx *kurtosis_context.KurtosisContext
	// EnclaveCtx is the enclave context.
	EnclaveCtx *enclaves.EnclaveContext
	// Config is the Kurtosis configuration.
	Config *KurtosisConfig
	// Logger is the logger for the test foundation.
	Logger *logrus.Logger
}

// SetupKurtosisEnvironment sets up the Kurtosis environment and returns a TestFoundation.
func SetupKurtosisEnvironment(t *testing.T, config *KurtosisConfig, logger *logrus.Logger) *TestFoundation {
	t.Helper()

	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	if config == nil {
		config = DefaultKurtosisConfig()
	}

	logger.Infof("Setting up Kurtosis environment with enclave: %s", config.EnclaveName)

	// Create context with timeout for setup operations.
	ctx, cancel := context.WithTimeout(context.Background(), config.EnclaveTimeout)
	defer cancel()

	// Initialize Kurtosis context.
	kurtosisCtx, err := kurtosis_context.NewKurtosisContextFromLocalEngine()
	if err != nil {
		t.Fatalf("Failed to create Kurtosis context, check kurtosis is running: %v", err)
	}

	// Register cleanup to destroy the enclave when the test finishes.
	t.Cleanup(func() {
		// Skip cleanup if KeepEnclave is true.
		if config.KeepEnclave {
			logger.Info("Skipping enclave cleanup because KeepEnclave=true")

			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), config.EnclaveTimeout)
		defer cancel()

		logger.Info("Cleaning up enclave:", config.EnclaveName)

		if destroyErr := kurtosisCtx.DestroyEnclave(cleanupCtx, config.EnclaveName); destroyErr != nil {
			logger.Warnf("Failed to destroy enclave %s: %v", config.EnclaveName, destroyErr)
		} else {
			logger.Infof("Successfully destroyed enclave: %s", config.EnclaveName)
		}
	})

	// Check if the enclave exists.
	enclaves, err := kurtosisCtx.GetEnclaves(ctx)
	if err != nil {
		t.Fatalf("Failed to get enclaves: %v", err)
	}

	// Check if we have any enclaves with the matching name.
	enclaveExists := false
	if enclaveInfos, found := enclaves.GetEnclavesByName()[config.EnclaveName]; found && len(enclaveInfos) > 0 {
		enclaveExists = true
		logger.Infof("Using existing enclave: %s", config.EnclaveName)
	}

	// Create the enclave if it doesn't exist.
	if !enclaveExists {
		logger.Infof("Creating new enclave: %s", config.EnclaveName)

		if _, createErr := kurtosisCtx.CreateEnclave(ctx, config.EnclaveName); createErr != nil {
			t.Fatalf("Failed to create enclave: %v", createErr)
		}
	}

	// Get the enclave context.
	enclaveCtx, err := kurtosisCtx.GetEnclaveContext(ctx, config.EnclaveName)
	if err != nil {
		t.Fatalf("Failed to get enclave context: %v", err)
	}

	// Create and return the test foundation.
	tf := &TestFoundation{
		KurtosisCtx: kurtosisCtx,
		EnclaveCtx:  enclaveCtx,
		Config:      config,
		Logger:      logger,
	}

	// Initialize the beacon nodes.
	if err := tf.initializeBeaconNodes(t); err != nil {
		t.Fatalf("Failed to initialize beacon nodes: %v", err)
	}

	return tf
}

// initializeBeaconNodes initializes the beacon nodes in the test environment.
func (tf *TestFoundation) initializeBeaconNodes(t *testing.T) error {
	t.Helper()

	// Get all the CL services.
	services, err := tf.EnclaveCtx.GetServices()
	if err != nil {
		return fmt.Errorf("failed to get services: %w", err)
	}

	clServiceNames := []string{}

	for name := range services {
		if strings.HasPrefix(string(name), "cl-") {
			clServiceNames = append(clServiceNames, string(name))
		}
	}

	tf.Logger.Infof("Found %d CL services:", len(clServiceNames))

	for _, name := range clServiceNames {
		tf.Logger.Infof("- %s", name)
	}

	if len(clServiceNames) == 0 {
		t.Skip("No CL services found")
	}

	// Create all our beacon api instances.
	beaconNodes := []*BeaconNode{}

	for _, clServiceName := range clServiceNames {
		tf.Logger.Infof("Initializing beacon node: %s", clServiceName)
		b, err := tf.getBeaconAPIInstance(clServiceName)
		if err != nil {
			return fmt.Errorf("failed to get beacon API instance for %s: %w", clServiceName, err)
		}

		beaconNodes = append(beaconNodes, b)
	}

	tf.BeaconNodes = beaconNodes

	// Start all beacon nodes.
	for _, bn := range tf.BeaconNodes {
		go func(bn *BeaconNode) {
			if err := bn.Node.Start(context.Background()); err != nil {
				tf.Logger.Errorf("Failed to start beacon node %s: %v", bn.Service.GetServiceName(), err)
			}
		}(bn)
	}

	// Wait for all the beacon nodes to be healthy.
	tf.Logger.Info("Waiting for all beacon nodes to be healthy...")
	startTime := time.Now()
	for !tf.AllBeaconNodesHealthy() {
		if time.Since(startTime) > 2*time.Minute {
			return fmt.Errorf("timed out waiting for beacon nodes to be healthy")
		}
		time.Sleep(1 * time.Second)
	}
	tf.Logger.Info("All beacon nodes are healthy")

	return nil
}

// getBeaconAPIInstance creates a beacon API client for a given CL service.
func (tf *TestFoundation) getBeaconAPIInstance(clServiceName string) (*BeaconNode, error) {
	clService, err := tf.EnclaveCtx.GetServiceContext(clServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service context: %w", err)
	}

	beaconPort := clService.GetPublicPorts()["http"]
	if beaconPort == nil {
		return nil, fmt.Errorf("beacon port not found")
	}

	opts := beacon.DefaultOptions()
	opts.HealthCheck.Interval.Duration = tf.Config.BeaconHealthCheckInterval

	b := beacon.NewNode(
		tf.Logger,
		&beacon.Config{
			Name: clServiceName,
			Addr: "http://" + net.JoinHostPort(clService.GetMaybePublicIPAddress(), fmt.Sprintf("%d", beaconPort.GetNumber())),
		},
		"testing",
		*opts,
	)

	return &BeaconNode{
		Node:    b,
		Service: clService,
	}, nil
}

// GetCLServices returns the names of all CL services in the enclave.
func GetCLServices(enclaveCtx *enclaves.EnclaveContext) ([]string, error) {
	services, err := enclaveCtx.GetServices()
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}

	clServiceNames := []string{}

	for name := range services {
		if strings.HasPrefix(string(name), "cl-") {
			clServiceNames = append(clServiceNames, string(name))
		}
	}

	return clServiceNames, nil
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
