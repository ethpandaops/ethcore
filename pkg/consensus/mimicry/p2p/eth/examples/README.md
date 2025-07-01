# Ethereum Consensus Layer P2P Examples

This directory contains production-ready examples demonstrating how to use the eth p2p library for Ethereum consensus layer communication.

## Overview

The examples show complete implementations of:
- **Beacon Block Handler**: Receiving, validating, and processing beacon blocks
- **Attestation Handler**: Managing dynamic attestation subnet subscriptions
- Message validation and processing patterns
- Event monitoring and metrics collection
- Database integration patterns
- Error handling and timeout management

## Examples

### 1. Beacon Block Handler (`beacon_block_handler.go`)

Demonstrates comprehensive beacon block handling including:

- **Setup**: Creating gossipsub instance with production settings
- **Validation**: Implementing block validation with proper error handling
- **Processing**: Saving blocks to database with transaction management
- **Publishing**: Publishing blocks to the network
- **Monitoring**: Event system integration for observability

**Key Features:**
- Timeout-based validation and processing
- Database transaction management
- Comprehensive logging with structured fields
- Metrics collection for monitoring
- Event-based debugging and observability
- Proper error handling without panicking

**Usage:**
```go
// See ExampleBeaconBlockSetup function for complete setup
handler := &BeaconBlockHandler{
    db:      database,
    log:     logger,
    metrics: &BlockMetrics{},
}

// The example shows how to:
// 1. Create gossipsub with production settings
// 2. Set up beacon block topic with fork digest
// 3. Configure validation and processing handlers
// 4. Subscribe to receive blocks
// 5. Monitor events for debugging
```

### 2. Attestation Handler (`attestation_handler.go`)

Demonstrates advanced attestation subnet management including:

- **Dynamic Subscriptions**: Managing subnet subscriptions based on validator duties
- **Subnet Validation**: Ensuring attestations belong to correct subnets
- **Pool Management**: Efficient attestation storage and retrieval
- **Metrics Tracking**: Per-subnet performance monitoring
- **Duty Rotation**: Simulating validator duty changes

**Key Features:**
- 64 subnet topic management
- Dynamic subscription changes
- Subnet-specific validation logic
- Attestation pool with size limits
- Per-subnet metrics tracking
- Concurrent-safe operations

**Usage:**
```go
// See ExampleAttestationSetup function for complete setup
handler := &AttestationHandler{
    pool:          attestationPool,
    activeSubnets: make(map[uint64]*v1.Subscription),
    metrics:       &AttestationMetrics{},
}

// The example shows how to:
// 1. Subscribe to specific attestation subnets
// 2. Validate attestations against subnet assignment
// 3. Manage dynamic subnet subscriptions
// 4. Track metrics per subnet
// 5. Handle validator duty rotations
```

## Running the Examples

### Prerequisites

1. **Database Setup** (for beacon block example):
```sql
CREATE TABLE beacon_blocks (
    root BYTEA PRIMARY KEY,
    slot BIGINT NOT NULL,
    proposer_index BIGINT NOT NULL,
    parent_root BYTEA NOT NULL,
    state_root BYTEA NOT NULL,
    body_root BYTEA NOT NULL,
    signature BYTEA NOT NULL,
    raw_data BYTEA NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE block_metadata (
    id SERIAL PRIMARY KEY,
    block_root BYTEA NOT NULL REFERENCES beacon_blocks(root),
    received_from TEXT NOT NULL,
    received_at TIMESTAMP NOT NULL,
    attestation_count INTEGER NOT NULL,
    deposit_count INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

2. **Dependencies**:
```bash
go mod tidy
```

### Basic Usage

```go
package main

import (
    "context"
    "database/sql"
    "log"

    "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/examples"
    "github.com/sirupsen/logrus"
    _ "github.com/lib/pq" // PostgreSQL driver
)

func main() {
    // Set up logging
    logger := logrus.StandardLogger()
    logger.SetLevel(logrus.DebugLevel)

    // Set up database (for beacon block example)
    db, err := sql.Open("postgres", "postgres://user:pass@localhost/dbname?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    ctx := context.Background()

    // Run beacon block example
    if err := examples.ExampleBeaconBlockSetup(ctx, db, logger); err != nil {
        log.Fatal(err)
    }

    // Or run attestation example
    // if err := examples.ExampleAttestationSetup(ctx, logger); err != nil {
    //     log.Fatal(err)
    // }
}
```

## Configuration

### Network Configuration

The examples use mainnet Capella fork digest by default:
```go
forkDigest := [4]byte{0x6a, 0x95, 0xa1, 0xa3} // Mainnet Capella
```

For other networks:
```go
// Goerli testnet (example)
forkDigest := [4]byte{0x90, 0xc6, 0x8c, 0x46}

// Custom network - compute from:
// - Genesis validators root
// - Fork version  
// - Domain type
```

### Gossipsub Configuration

Production settings used in examples:
```go
gs, err := v1.New(ctx, host,
    v1.WithLogger(log),
    v1.WithMetrics(metrics),                     // Enable metrics
    v1.WithMaxMessageSize(10*1024*1024),         // 10 MB for blocks
    v1.WithValidationConcurrency(100),           // Concurrent validation
    v1.WithPublishTimeout(10*time.Second),       // Publish timeout
    v1.WithGossipSubParams(pubsub.DefaultGossipSubParams()),
)
```

### Topic Scoring

Examples include proper topic scoring configuration:
```go
scoreParams := &pubsub.TopicScoreParams{
    TopicWeight:                     0.5,  // Topic importance
    TimeInMeshWeight:                1,    // Reward mesh participation  
    FirstMessageDeliveriesWeight:    1,    // Reward first delivery
    MeshMessageDeliveriesWeight:     -1,   // Penalty for missed deliveries
    InvalidMessageDeliveriesWeight:  -1,   // Penalty for invalid messages
    // ... additional parameters
}
```

## Validation Patterns

### Beacon Block Validation

```go
func validateBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) v1.ValidationResult {
    // Add timeout to prevent blocking
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // Basic structure validation
    if block == nil || block.Block == nil {
        return v1.ValidationReject
    }

    // Slot timing validation
    currentSlot := getCurrentSlot()
    if uint64(block.Block.Slot) > currentSlot+10 {
        return v1.ValidationIgnore // Future blocks
    }

    // Signature length validation
    if len(block.Signature) != 96 {
        return v1.ValidationReject
    }

    // Additional validations in production:
    // - Parent block exists
    // - Proposer authorization  
    // - State transition validity
    // - BLS signature verification

    return v1.ValidationAccept
}
```

### Attestation Validation

```go
func validateAttestation(ctx context.Context, att *eth.Attestation, from peer.ID, subnet uint64) v1.ValidationResult {
    // Timeout for validation
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    // Subnet assignment validation
    expectedSubnet := computeSubnetForAttestation(att)
    if expectedSubnet != subnet {
        return v1.ValidationReject
    }

    // Slot timing validation
    currentSlot := getCurrentSlot()
    if uint64(att.Data.Slot) > currentSlot+1 {
        return v1.ValidationIgnore
    }

    // Structure validation
    if att.AggregationBits == nil || len(att.Signature) != 96 {
        return v1.ValidationReject
    }

    return v1.ValidationAccept
}
```

## Processing Patterns

### Database Integration

```go
func processBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) error {
    // Add processing timeout
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    // Use database transactions
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()

    // Save block atomically
    if err := saveBlock(ctx, tx, block); err != nil {
        return fmt.Errorf("failed to save block: %w", err)
    }

    // Commit transaction
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit: %w", err)
    }

    // Trigger async processing
    go updateForkChoice(block)

    return nil
}
```

### Error Handling

```go
func processMessage(ctx context.Context, msg Message, from peer.ID) error {
    // Recover from panics
    defer func() {
        if r := recover(); r != nil {
            log.WithField("panic", r).Error("recovered from panic in message processing")
        }
    }()

    // Handle expected errors gracefully
    if err := validateMessage(msg); err != nil {
        // Log but don't fail processing
        log.WithError(err).Warn("message validation warning")
        return nil
    }

    // Process with retries
    for attempts := 0; attempts < 3; attempts++ {
        if err := saveMessage(ctx, msg); err != nil {
            if attempts == 2 {
                return fmt.Errorf("failed after 3 attempts: %w", err)
            }
            time.Sleep(time.Duration(attempts+1) * time.Second)
            continue
        }
        break
    }

    return nil
}
```

## Monitoring and Observability

### Metrics Collection

```go
// Create metrics instance
metrics := v1.NewMetrics("eth_consensus")

// Register with Prometheus
registry := prometheus.NewRegistry()
metrics.Register(registry)

// Use in gossipsub
gs, err := v1.New(ctx, host,
    v1.WithMetrics(metrics),
)

// Access metrics
fmt.Printf("Messages received: %d\n", metrics.MessagesReceived.Get())
```

### Event Monitoring

```go
events := make(chan v1.Event, 1000)

// Configure handler with events
handler := v1.NewHandlerConfig[T](
    v1.WithValidator(validator),
    v1.WithProcessor(processor),
    v1.WithEvents(events),
)

// Monitor events
go func() {
    for event := range events {
        switch e := event.(type) {
        case *v1.GossipsubEvent:
            log.WithFields(logrus.Fields{
                "type":  e.EventType,
                "topic": e.Topic,
                "peer":  e.PeerID,
            }).Debug("gossipsub event")
        }
    }
}()
```

### Structured Logging

```go
log.WithFields(logrus.Fields{
    "slot":       block.Block.Slot,
    "proposer":   block.Block.ProposerIndex,
    "block_root": fmt.Sprintf("%x", blockRoot),
    "from":       from,
    "size":       len(blockData),
    "duration":   processingTime,
}).Info("processed beacon block")
```

## Performance Considerations

### Memory Management

- Use buffered channels for high-volume events
- Implement proper pool size limits
- Monitor memory usage under load
- Use efficient data structures

### Concurrency

- Configure appropriate validation concurrency
- Use timeouts to prevent blocking
- Implement proper synchronization
- Monitor goroutine counts

### Network Efficiency

- Use SSZ+Snappy encoding
- Implement message deduplication
- Configure appropriate buffer sizes
- Monitor bandwidth usage

## Security Best Practices

1. **Validation**: Always validate message structure before processing
2. **Timeouts**: Use timeouts to prevent DoS attacks
3. **Scoring**: Configure topic scoring to penalize bad behavior
4. **Rate Limiting**: Implement rate limiting for processing
5. **Signature Verification**: Use secure BLS signature verification
6. **Fork Isolation**: Validate fork digests to prevent cross-network attacks

## Testing

Run the examples with race detection:
```bash
go run -race beacon_block_handler.go
go run -race attestation_handler.go
```

Run with verbose logging:
```bash
RUST_LOG=debug go run beacon_block_handler.go
```

## Production Deployment

### Resource Requirements

- **CPU**: Multi-core recommended for validation concurrency
- **Memory**: 4-8 GB RAM for typical loads
- **Network**: Stable internet connection with sufficient bandwidth
- **Storage**: SSD recommended for database operations

### Monitoring

Set up monitoring for:
- Message processing rates
- Validation success/failure rates
- Memory and CPU usage  
- Network connectivity
- Database performance
- Peer connectivity and scoring

### High Availability

- Run multiple instances with load balancing
- Implement proper health checks
- Use database clustering for persistence
- Monitor and alert on critical metrics
- Implement graceful shutdown procedures

## Further Reading

- [Ethereum Consensus Layer Specification](https://github.com/ethereum/consensus-specs)
- [libp2p PubSub Documentation](https://docs.libp2p.io/concepts/publish-subscribe/)
- [SSZ Specification](https://github.com/ethereum/consensus-specs/blob/dev/ssz/simple-serialize.md)
- [Gossipsub Specification](https://github.com/libp2p/specs/blob/master/pubsub/gossipsub/README.md)

## Support

For questions and issues:
- Check the package documentation
- Review the test files for additional examples
- Submit issues on the GitHub repository
- Join the Ethereum R&D Discord for community support