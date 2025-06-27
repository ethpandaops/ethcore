# ETHCore

[![Go Reference](https://pkg.go.dev/badge/github.com/ethpandaops/ethcore.svg)](https://pkg.go.dev/github.com/ethpandaops/ethcore)
[![codecov](https://codecov.io/gh/ethpandaops/ethcore/graph/badge.svg?token=N85VGIQM3V)](https://codecov.io/gh/ethpandaops/ethcore)
[![Go Report Card](https://goreportcard.com/badge/github.com/ethpandaops/ethcore)](https://goreportcard.com/report/github.com/ethpandaops/ethcore)
[![License](https://img.shields.io/github/license/ethpandaops/ethcore)](LICENSE)

ETHCore is a comprehensive Go library that provides utilities and implementations for interacting with Ethereum networks. Developed by [ethpandaops](https://github.com/ethpandaops), it offers tools for both consensus and execution layer interactions, node discovery, and network analysis.

## Features

### **Node Discovery**
- **Discovery v5 Protocol**: Full implementation of Ethereum's discovery v5 protocol for finding peers
- **Manual Discovery**: Support for manually specified bootstrap nodes
- **Flexible Interface**: Pluggable discovery mechanisms through common interfaces

### **Consensus Layer (Beacon Chain)**
- **Network Crawler**: Connect to beacon nodes and crawl the network topology
- **Status Monitoring**: Request and track node status updates
- **P2P Communication**: Built on libp2p for robust peer-to-peer messaging
- **Protocol Support**: Implements beacon chain networking protocols

### **Execution Layer**
- **Client Mimicry**: Connect to execution layer nodes as a lightweight client
- **RLPx Protocol**: Full RLPx protocol implementation for execution layer communication
- **Message Handling**: Support for blocks, transactions, receipts, and other protocol messages
- **Network Analysis**: Tools for analyzing execution layer network behavior

### **Ethereum Utilities**
- **Multi-Network Support**: Configurations for various Ethereum networks
- **Fork Management**: Handle Ethereum protocol upgrades and forks
- **Serialization**: Utilities for Ethereum data serialization/deserialization
- **Client Utilities**: Common functionality for building Ethereum clients

## Installation

```bash
go get github.com/ethpandaops/ethcore
```

## Requirements

- Go 1.24.0 or higher
- Dependencies are managed via Go modules

## Usage Examples

### Using the Discovery Service

```go
import (
    "context"
    "github.com/ethpandaops/ethcore/pkg/discovery"
    "github.com/ethpandaops/ethcore/pkg/discovery/disc_v5"
)

// Create a new Discovery v5 service
config := disc_v5.NewConfig()
discovery, err := disc_v5.New(ctx, config)
if err != nil {
    log.Fatal(err)
}

// Start discovery
if err := discovery.Start(ctx); err != nil {
    log.Fatal(err)
}

// Find nodes
nodes := discovery.FindNodes(ctx, 10)
```

### Consensus Layer Crawler

```go
import (
    "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/crawler"
)

// Create and start a beacon chain crawler
crawler := crawler.New(crawlerConfig)
if err := crawler.Start(ctx); err != nil {
    log.Fatal(err)
}
```

### Execution Layer Client

```go
import (
    "github.com/ethpandaops/ethcore/pkg/execution/mimicry"
)

// Connect to an execution layer node
client := mimicry.New(executionConfig)
if err := client.Connect(ctx, nodeAddress); err != nil {
    log.Fatal(err)
}
```

## Project Structure

```
ethcore/
├── pkg/
│   ├── consensus/          # Consensus layer implementations
│   │   └── mimicry/       # Beacon chain client mimicry
│   ├── discovery/          # Node discovery protocols
│   │   ├── disc_v5/       # Discovery v5 implementation
│   │   └── manual/        # Manual peer discovery
│   ├── ethereum/           # Core Ethereum utilities
│   │   ├── beacon/        # Beacon chain utilities
│   │   ├── clients/       # Client implementations
│   │   ├── config/        # Configuration management
│   │   ├── fork/          # Fork handling
│   │   ├── networks/      # Network configurations
│   │   └── serialize/     # Serialization utilities
│   └── execution/          # Execution layer implementations
│       └── mimicry/       # Execution client mimicry
```

## Development

### Running Tests

```bash
# Run all tests
make test
```

### Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```
