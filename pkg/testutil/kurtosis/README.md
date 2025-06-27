# Kurtosis Test Utilities

This package provides shared test utilities for managing Kurtosis networks across the ethcore test suite.

## Overview

The `kurtosis` package centralizes Kurtosis network management to:
- Reduce test execution time by sharing networks within test packages
- Eliminate code duplication across test suites
- Provide consistent test infrastructure
- Enable easier addition of new integration tests

## Usage

### Basic Setup with TestMain

Each test package that needs Kurtosis should implement a `TestMain` function:

```go
//go:build integration
// +build integration

package mytest

import (
    "os"
    "testing"
    "github.com/ethpandaops/ethcore/pkg/testutil/kurtosis"
)

var testFoundation *kurtosis.TestFoundation

func TestMain(m *testing.M) {
    config := kurtosis.DefaultNetworkConfig()
    config.Name = "my-test-network"
    
    foundation, err := kurtosis.GetNetwork(nil, config)
    if err != nil {
        panic(err)
    }
    testFoundation = foundation
    
    code := m.Run()
    
    if !config.KeepAlive {
        foundation.Cleanup()
    }
    
    os.Exit(code)
}
```

### Writing Tests

Individual tests can access the shared network:

```go
func TestMyFeature(t *testing.T) {
    require.NotNil(t, testFoundation, "TestMain must run first")
    
    // Access beacon clients
    for _, clientID := range testFoundation.BeaconClients {
        // Use the client ID to interact with beacon nodes
    }
    
    // Use helper functions
    client := kurtosis.GetRandomBeaconClient(testFoundation)
    kurtosis.AssertNetworkHealth(t, testFoundation)
}
```

## Configuration

### Environment Variables

The following environment variables control test behavior:

| Variable | Description | Default |
|----------|-------------|---------|
| `KURTOSIS_ENCLAVE` | Kurtosis enclave name | `"kurtosis"` |
| `KEEP_ENCLAVE` | Keep network after tests (`true`/`false`) | `false` |
| `TEST_TIMEOUT` | Network operation timeout (e.g., `5m`) | `2m` |
| `TEST_NET` | Use testnet configuration (`true`/`false`) | `false` |
| `EL_CHAIN` | Execution layer chain | `"mainnet"` |

### Custom Configuration

```go
config := kurtosis.NetworkConfig{
    Name:            "custom-test",
    Timeout:         5 * time.Minute,
    KurtosisEnclave: "my-enclave",
    KeepAlive:       true,
    TestNet:         false,
    ElChain:         "mainnet",
}
config.SetParticipants(8)

// Merge with environment
envConfig := kurtosis.ConfigFromEnvironment()
config = kurtosis.MergeConfigs(config, envConfig)
```

## Helper Functions

### Network Management
- `GetNetwork(t *testing.T, config NetworkConfig)` - Get or create a network
- `GetNATExitIP()` - Get appropriate NAT IP for Docker containers

### Beacon Node Operations
- `WaitForBeaconNode(ctx, node, timeout)` - Wait for node to be healthy
- `GetRandomBeaconClient(foundation)` - Get random beacon client
- `GetBeaconClientByType(foundation, clientType)` - Get specific client type
- `AssertNetworkHealth(t, foundation)` - Assert all nodes are healthy

## Best Practices

### 1. Use Build Tags

Always use the `integration` build tag for tests requiring Kurtosis:

```go
//go:build integration
// +build integration
```

### 2. Run Tests Correctly

```bash
# Run unit tests only
make test-unit

# Run integration tests in parallel
make test-integration

# Run integration tests serially (for debugging)
make test-integration-serial
```

### 3. Network Reuse

Networks are automatically reused within a test package. To reuse across runs:

```bash
KEEP_ENCLAVE=true go test -tags=integration ./...
```

### 4. Debugging

Enable debug logging:
```bash
export KURTOSIS_LOG_LEVEL=debug
```

Keep the network alive for inspection:
```bash
export KEEP_ENCLAVE=true
```

## Troubleshooting

### Tests Timeout

Increase the timeout:
```bash
export TEST_TIMEOUT=10m
```

### Network Already Exists

The package automatically handles existing networks. If issues persist:
```bash
kurtosis enclave rm ethcore-test
```

### Docker Issues

Ensure Docker is running and has sufficient resources:
- Memory: At least 8GB
- CPU: At least 4 cores
- Disk: At least 20GB free

### Port Conflicts

Check for conflicting services:
```bash
lsof -i :8545  # Execution client
lsof -i :5052  # Beacon API
```

## Performance Considerations

### Parallel vs Serial Execution

- **Parallel** (`make test-integration`): Faster but requires more resources
- **Serial** (`make test-integration-serial`): Slower but more reliable on constrained systems

### Resource Usage

Each test package spins up its own network with:
- Multiple beacon nodes (typically 5-7)
- Multiple execution clients
- Supporting infrastructure

Plan resources accordingly.

### Network Startup Time

Initial network creation takes 30-60 seconds. Subsequent tests in the same package reuse the network, making them much faster.

## Examples

### Testing Beacon Node APIs

```go
func TestBeaconAPIs(t *testing.T) {
    require.NotNil(t, testFoundation)
    
    // Get a specific client type
    lighthouse := kurtosis.GetBeaconClientByType(testFoundation, "lighthouse")
    require.NotNil(t, lighthouse)
    
    // Create beacon node instance
    node := ethereum.NewBeaconNode(logger, &ethereum.Config{
        Name: lighthouse,
        Addr: fmt.Sprintf("http://cl-%s:5052", lighthouse),
    })
    
    // Wait for it to be ready
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    err := kurtosis.WaitForBeaconNode(ctx, node, 30*time.Second)
    require.NoError(t, err)
    
    // Use the node...
}
```

### Testing Network Conditions

```go
func TestNetworkPartition(t *testing.T) {
    require.NotNil(t, testFoundation)
    
    // Ensure network is healthy first
    kurtosis.AssertNetworkHealth(t, testFoundation)
    
    // Test your network partition logic...
}
```

## Migration Guide

If you're migrating from inline Kurtosis setup:

1. Remove `SetupKurtosisEnvironment()` function
2. Remove local `TestFoundation` and `NetworkConfig` types
3. Add `//go:build integration` tag
4. Implement `TestMain` as shown above
5. Update imports to use `kurtosis` package
6. Replace `foundation := SetupKurtosisEnvironment(t)` with `require.NotNil(t, testFoundation)`

## Contributing

When adding new helper functions:
1. Follow ethpandaops Go coding standards
2. Add comprehensive documentation
3. Include error handling with wrapped errors
4. Add unit tests where possible
5. Update this README with usage examples