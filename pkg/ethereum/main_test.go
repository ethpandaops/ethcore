package ethereum_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/ethpandaops/ethcore/pkg/testutil/kurtosis"
	"github.com/sirupsen/logrus"
)

// Package-level test foundation variable.
var testFoundation *kurtosis.TestFoundation

// TestMain is the entry point for all tests in this package.
// It sets up the shared Kurtosis network and ensures proper cleanup.
func TestMain(m *testing.M) {
	fmt.Printf("TestMain - Ethereum!!!")

	// Acquire global test lock to prevent concurrent Kurtosis networks
	kurtosis.AcquireTestLock()
	defer kurtosis.ReleaseTestLock()

	// Initialize logger for setup/teardown
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Start with default configuration
	defaultConfig := kurtosis.DefaultNetworkConfig()

	// Override name for ethereum tests
	defaultConfig.Name = "ethcore-ethereum-test"

	// Use port offset 0 for ethereum tests (ports 32000-32999, 33000-33999)
	defaultConfig.PortOffset = 0

	// Get configuration from environment
	envConfig, err := kurtosis.ConfigFromEnvironment()
	if err != nil {
		panic(fmt.Sprintf("failed to parse environment configuration: %v", err))
	}

	// Merge configurations (environment takes precedence)
	config := kurtosis.MergeConfigs(defaultConfig, envConfig)

	// Log configuration
	logger.WithFields(logrus.Fields{
		"name":      config.Name,
		"timeout":   config.Timeout,
		"keepAlive": config.KeepAlive,
		"testNet":   config.TestNet,
		"elChain":   config.ElChain,
	}).Info("TestMain: Setting up test network")

	// Since GetNetwork requires a real *testing.T, we'll create a minimal foundation
	// directly for TestMain. The actual tests will get the network through GetTestFoundation.
	foundation := &kurtosis.TestFoundation{
		Config: config,
		Logger: logger,
		NAT:    config.TestNet,
		// Network and BeaconClients will be initialized by the first test that calls GetNetwork
	}

	// Store the foundation in package variable
	testFoundation = foundation

	// Run tests
	exitCode := m.Run()

	// Cleanup the network if it was created
	logger.Info("TestMain: Cleaning up test network")
	if err := kurtosis.ForceCleanupNetwork(config.Name); err != nil {
		logger.WithError(err).Warn("Failed to cleanup network")
	}

	// Exit with test result code
	os.Exit(exitCode)
}

// GetTestFoundation returns the shared test foundation for use in tests.
// It panics if called before TestMain has initialized the foundation.
func GetTestFoundation() *kurtosis.TestFoundation {
	if testFoundation == nil {
		panic("test foundation not initialized - ensure TestMain has run")
	}

	return testFoundation
}
