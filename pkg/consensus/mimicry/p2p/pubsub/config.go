package pubsub

import (
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// Config contains configuration parameters for the gossipsub implementation.
type Config struct {
	// Essential parameters
	MaxMessageSize        int           // Maximum message size (needed for Ethereum consensus)
	ValidationBufferSize  int           // Size of validation queue
	ValidationConcurrency int           // Number of validation workers
	PublishTimeout        time.Duration // Timeout for publishing messages

	// Optional gossipsub protocol parameters (use libp2p defaults if nil)
	GossipSubParams *pubsub.GossipSubParams // Optional: custom gossipsub protocol parameters
}

// DefaultConfig returns a Config with sensible defaults for Ethereum consensus layer gossipsub.
// GossipSubParams is left nil to use libp2p defaults.
func DefaultConfig() *Config {
	return &Config{
		MaxMessageSize:        10 << 20, // 10MB - needed for large Ethereum messages
		ValidationBufferSize:  100,      // Reasonable validation queue size
		ValidationConcurrency: 10,       // Moderate validation concurrency
		PublishTimeout:        5 * time.Second,
		// Peer scoring is now handled by individual processors

		// GossipSubParams left nil will use libp2p defaults
		// Users can set custom values like:
		// GossipSubParams: &pubsub.GossipSubParams{D: 8, Dlo: 6, Dhi: 12, ...}
		// Or start with defaults: GossipSubParams: func() *pubsub.GossipSubParams { p := pubsub.DefaultGossipSubParams(); p.D = 8; return &p }()
	}
}

// NewConfigWithCustomGossipSub creates a config with custom gossipsub parameters starting from defaults.
func NewConfigWithCustomGossipSub() *Config {
	config := DefaultConfig()
	params := pubsub.DefaultGossipSubParams()
	config.GossipSubParams = &params

	return config
}

// Validate checks if the configuration is valid and returns an error if not.
func (c *Config) Validate() error {
	// Validate required parameters
	if c.MaxMessageSize <= 0 {
		return fmt.Errorf("MaxMessageSize must be positive, got %d", c.MaxMessageSize)
	}

	if c.ValidationBufferSize <= 0 {
		return fmt.Errorf("ValidationBufferSize must be positive, got %d", c.ValidationBufferSize)
	}

	if c.ValidationConcurrency <= 0 {
		return fmt.Errorf("ValidationConcurrency must be positive, got %d", c.ValidationConcurrency)
	}

	if c.PublishTimeout <= 0 {
		return fmt.Errorf("PublishTimeout must be positive, got %v", c.PublishTimeout)
	}

	// GossipSubParams validation is handled by libp2p when the params are used
	// We don't need to duplicate all their validation logic here

	return nil
}
