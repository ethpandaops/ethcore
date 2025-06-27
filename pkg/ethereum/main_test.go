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
	// Initialize logger for setup/teardown
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Start with default configuration
	defaultConfig := kurtosis.DefaultNetworkConfig()
	// Override name for ethereum tests
	defaultConfig.Name = "ethcore-ethereum-test"

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

	// Cleanup is handled by individual tests through testing.T.Cleanup()
	if config.KeepAlive {
		logger.Info("TestMain: Keeping test network alive (KeepAlive=true)")
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
