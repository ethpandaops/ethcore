package kurtosis

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// DefaultNetworkConfig returns a NetworkConfig with sensible default values
// for testing Ethereum networks with Kurtosis.
func DefaultNetworkConfig() *NetworkConfig {
	return &NetworkConfig{
		Name:            "ethcore-test",
		Timeout:         2 * time.Minute,
		KurtosisEnclave: "kurtosis",
		KeepAlive:       false,
		TestNet:         false,
		ElChain:         "mainnet",
		participants:    4,
	}
}

// ConfigFromEnvironment reads configuration values from environment variables
// and returns a NetworkConfig. It returns an error if any environment variable
// contains an invalid value.
//
// The following environment variables are supported:
//   - KURTOSIS_ENCLAVE: The Kurtosis enclave name (default: "kurtosis")
//   - KEEP_ENCLAVE: Whether to keep the enclave alive after tests (default: false)
//   - TEST_TIMEOUT: Duration for test timeout (e.g., "5m", "30s") (default: 2m)
//   - TEST_NET: Whether this is a test network (default: false)
//   - EL_CHAIN: The execution layer chain configuration (default: "mainnet")
func ConfigFromEnvironment() (*NetworkConfig, error) {
	config := &NetworkConfig{}

	// Read KURTOSIS_ENCLAVE
	if enclave := os.Getenv("KURTOSIS_ENCLAVE"); enclave != "" {
		config.KurtosisEnclave = enclave
	}

	// Read KEEP_ENCLAVE
	if keepAlive := os.Getenv("KEEP_ENCLAVE"); keepAlive != "" {
		parsed, err := strconv.ParseBool(keepAlive)
		if err != nil {
			return nil, fmt.Errorf("invalid KEEP_ENCLAVE value %q: %w", keepAlive, err)
		}

		config.KeepAlive = parsed
	}

	// Read TEST_TIMEOUT
	if timeout := os.Getenv("TEST_TIMEOUT"); timeout != "" {
		duration, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid TEST_TIMEOUT value %q: %w", timeout, err)
		}

		config.Timeout = duration
	}

	// Read TEST_NET
	if testNet := os.Getenv("TEST_NET"); testNet != "" {
		parsed, err := strconv.ParseBool(testNet)
		if err != nil {
			return nil, fmt.Errorf("invalid TEST_NET value %q: %w", testNet, err)
		}

		config.TestNet = parsed
	}

	// Read EL_CHAIN
	if elChain := os.Getenv("EL_CHAIN"); elChain != "" {
		config.ElChain = elChain
	}

	return config, nil
}

// MergeConfigs merges two NetworkConfig instances, with values from the override
// configuration taking precedence over the base configuration for non-zero values.
// This allows for layering configurations, such as defaults with environment-specific overrides.
//
// The merge rules are:
//   - String fields: override value is used if not empty
//   - Duration fields: override value is used if not zero
//   - Boolean fields: override value is always used
//   - Private fields: override value is used if not zero
func MergeConfigs(base, override *NetworkConfig) *NetworkConfig {
	if base == nil && override == nil {
		return nil
	}

	if base == nil {
		// Create a copy of override to avoid modifying the original
		result := *override

		return &result
	}

	if override == nil {
		// Create a copy of base to avoid modifying the original
		result := *base

		return &result
	}

	// Create a new config starting with base values
	result := *base

	// Override string fields if not empty
	if override.Name != "" {
		result.Name = override.Name
	}

	if override.KurtosisEnclave != "" {
		result.KurtosisEnclave = override.KurtosisEnclave
	}

	if override.ElChain != "" {
		result.ElChain = override.ElChain
	}

	// Override duration if not zero
	if override.Timeout != 0 {
		result.Timeout = override.Timeout
	}

	// Boolean fields always take the override value
	// (we can't distinguish between explicitly set to false vs unset)
	result.KeepAlive = override.KeepAlive
	result.TestNet = override.TestNet

	// Override participants if not zero
	if override.participants != 0 {
		result.participants = override.participants
	}

	return &result
}

// GetParticipants returns the number of participants configured for the network.
func (nc *NetworkConfig) GetParticipants() int {
	return nc.participants
}

// SetParticipants sets the number of participants for the network.
// It returns an error if the participant count is invalid (less than 1).
func (nc *NetworkConfig) SetParticipants(count int) error {
	if count < 1 {
		return fmt.Errorf("participant count must be at least 1, got %d", count)
	}

	nc.participants = count

	return nil
}
