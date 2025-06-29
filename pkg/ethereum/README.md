# Ethereum Pkg

Utilities and implementations for interacting with Ethereum networks.

## Beacon Node

This package provides a Go client for connecting to and interacting with Ethereum beacon nodes. It includes support for metadata services, network detection, and health monitoring.

### Installation

```bash
go get github.com/ethpandaops/ethcore/pkg/ethereum
```

### Usage

#### Single Node Example

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/ethpandaops/beacon/pkg/beacon"
    "github.com/ethpandaops/ethcore/pkg/ethereum"
    "github.com/sirupsen/logrus"
)

func main() {
    ctx := context.Background()
    logger := logrus.New()

    // Configure the beacon node
    config := &ethereum.Config{
        BeaconNodeAddress: "http://localhost:5052",
        BeaconNodeHeaders: map[string]string{
            "Authorization": "Bearer your-token",
        },
    }

    // Set up beacon options
    opts := beacon.DefaultOptions()
    opts.HealthCheck.Interval.Duration = time.Second * 3
    opts.HealthCheck.SuccessfulResponses = 1

    // Create the beacon node
    beaconNode, err := ethereum.NewBeaconNode(
        logger,
        "my-app",
        config,
        *opts,
    )
    if err != nil {
        log.Fatal("Failed to create beacon node:", err)
    }

    // Register a callback to be executed when the node is ready
    beaconNode.OnReady(ctx, func(ctx context.Context) error {
        logger.Info("Beacon node is ready! Starting application logic...")

        // Your application logic here
        metadata := beaconNode.Metadata()
        network := metadata.Network
        logger.WithFields(logrus.Fields{
            "network": network.Name,
            "network_id": network.ID,
        }).Info("Connected to network")

        return nil
    })

    // Start the beacon node
    if err := beaconNode.Start(ctx); err != nil {
        log.Fatal("Failed to start beacon node:", err)
    }

    // Keep the application running
    select {}
}
```

#### Multiple Nodes Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/ethpandaops/beacon/pkg/beacon"
    "github.com/ethpandaops/ethcore/pkg/ethereum"
    "github.com/sirupsen/logrus"
)

type BeaconNodeManager struct {
    nodes []*ethereum.BeaconNode
    mu    sync.Mutex
    wg    sync.WaitGroup
}

func main() {
    ctx := context.Background()
    logger := logrus.New()

    // Define multiple beacon node endpoints
    endpoints := []string{
        "http://beacon1:5052",
        "http://beacon2:5052",
        "http://beacon3:5052",
    }

    manager := &BeaconNodeManager{
        nodes: make([]*ethereum.BeaconNode, 0, len(endpoints)),
    }

    // Create and start all beacon nodes
    for i, endpoint := range endpoints {
        nodeLogger := logger.WithField("node", fmt.Sprintf("beacon-%d", i))

        config := &ethereum.Config{
            BeaconNodeAddress: endpoint,
        }

        opts := beacon.DefaultOptions()
        opts.HealthCheck.Interval.Duration = time.Second * 3

        // Create the beacon node
        node, err := ethereum.NewBeaconNode(
            nodeLogger,
            fmt.Sprintf("node-%d", i),
            config,
            *opts,
        )
        if err != nil {
            log.Printf("Failed to create beacon node %d: %v", i, err)
            continue
        }

        // Track this node
        manager.mu.Lock()
        manager.nodes = append(manager.nodes, node)
        manager.mu.Unlock()

        // Add to wait group for this node
        manager.wg.Add(1)

        // Register callback for when node is ready
        nodeIndex := i
        node.OnReady(ctx, func(ctx context.Context) error {
            defer manager.wg.Done()

            nodeLogger.Info("Node is ready")

            // Log network information
            metadata := node.Metadata()
            nodeLogger.WithFields(logrus.Fields{
                "network": metadata.Network.Name,
                "node_index": nodeIndex,
            }).Info("Node connected to network")

            return nil
        })

        // Start the node in a goroutine
        go func(n *ethereum.BeaconNode, idx int) {
            if err := n.Start(ctx); err != nil {
                nodeLogger.WithError(err).Error("Failed to start beacon node")
                manager.wg.Done() // Ensure we decrease the counter even on error
            }
        }(node, i)
    }

    // Wait for all nodes to be ready
    logger.Info("Waiting for all nodes to be ready...")
    manager.wg.Wait()
    logger.Info("All nodes are ready!")

    // Now all nodes are ready, you can use them
    manager.mu.Lock()
    for i, node := range manager.nodes {
        if node.IsHealthy() {
            logger.WithField("node", i).Info("Node is healthy")
        }
    }
    manager.mu.Unlock()

    // Keep the application running
    select {}
}
```
