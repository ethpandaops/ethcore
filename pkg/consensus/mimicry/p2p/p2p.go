package p2p

import (
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/sirupsen/logrus"
)

// NewGossipsub creates a new Gossipsub instance with the given configuration
func NewGossipsub(log logrus.FieldLogger, host host.Host, config *pubsub.Config) (*pubsub.Gossipsub, error) {
	return pubsub.NewGossipsub(log, host, config)
}

// DefaultGossipsubConfig returns a default gossipsub configuration
func DefaultGossipsubConfig() *pubsub.Config {
	return pubsub.DefaultConfig()
}

// NewGossipsubMetrics creates new gossipsub metrics with the given namespace
func NewGossipsubMetrics(namespace string) *pubsub.Metrics {
	return pubsub.NewMetrics(namespace)
}