// Package kurtosis provides shared test utilities for managing Kurtosis networks
// across the ethcore test suite.
//
// Usage:
//
//	func TestMain(m *testing.M) {
//	    config := kurtosis.DefaultNetworkConfig()
//	    foundation, err := kurtosis.GetNetwork(nil, config)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer foundation.Cleanup()
//
//	    os.Exit(m.Run())
//	}
//
// Individual tests can then access the shared network through the foundation.
package kurtosis
