// Package v1 provides a type-safe, production-ready implementation of gossipsub topics
// for Ethereum consensus layer p2p communication.
//
// This package introduces generic type parameters for compile-time type safety,
// ensuring that messages published and received on topics are of the correct type.
// It supports both regular topics and subnet-based topics commonly used in Ethereum's
// attestation and sync committee protocols.
//
// Key features:
//   - Type-safe topic definitions with generic type parameters
//   - Support for Ethereum fork digests
//   - Subnet topic management with pattern-based topic names
//   - Encoder abstraction for flexible message serialization
//   - Comprehensive error handling and validation
//   - Optional Prometheus metrics integration for monitoring
package v1
