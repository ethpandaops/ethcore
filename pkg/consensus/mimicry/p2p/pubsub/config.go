package pubsub

import (
	"fmt"
	"time"
)

// Config contains configuration parameters for the gossipsub implementation.
type Config struct {
	// Core gossipsub parameters
	MessageBufferSize    int
	ValidationBufferSize int
	MaxMessageSize       int

	// Gossipsub protocol parameters
	D                 int           // target degree
	Dlo               int           // lower bound for mesh degree
	Dhi               int           // upper bound for mesh degree
	Dlazy             int           // gossip degree
	HeartbeatInterval time.Duration // heartbeat interval
	HistoryLength     int           // message cache history length
	HistoryGossip     int           // message IDs to gossip

	// Performance tuning
	ValidationConcurrency int
	PublishTimeout        time.Duration

	// Feature flags
	EnablePeerScoring     bool
	EnableFloodPublishing bool
}

// DefaultConfig returns a Config with sensible defaults for Ethereum consensus layer gossipsub.
func DefaultConfig() *Config {
	return &Config{
		MessageBufferSize:     1000,
		ValidationBufferSize:  100,
		MaxMessageSize:        1 << 20, // 1MB
		D:                     8,
		Dlo:                   6,
		Dhi:                   12,
		Dlazy:                 6,
		HeartbeatInterval:     time.Second,
		HistoryLength:         5,
		HistoryGossip:         3,
		ValidationConcurrency: 10,
		PublishTimeout:        5 * time.Second,
		EnablePeerScoring:     true,
		EnableFloodPublishing: true,
	}
}

// Validate checks if the configuration is valid and returns an error if not.
func (c *Config) Validate() error {
	if c.MessageBufferSize <= 0 {
		return fmt.Errorf("MessageBufferSize must be positive, got %d", c.MessageBufferSize)
	}
	if c.ValidationBufferSize <= 0 {
		return fmt.Errorf("ValidationBufferSize must be positive, got %d", c.ValidationBufferSize)
	}
	if c.MaxMessageSize <= 0 {
		return fmt.Errorf("MaxMessageSize must be positive, got %d", c.MaxMessageSize)
	}
	if c.D <= 0 {
		return fmt.Errorf("d (target degree) must be positive, got %d", c.D)
	}
	if c.Dlo <= 0 {
		return fmt.Errorf("dlo (lower bound) must be positive, got %d", c.Dlo)
	}
	if c.Dhi <= c.D {
		return fmt.Errorf("dhi (%d) must be greater than d (%d)", c.Dhi, c.D)
	}
	if c.Dlo > c.D {
		return fmt.Errorf("dlo (%d) must be less than or equal to d (%d)", c.Dlo, c.D)
	}
	if c.Dlazy <= 0 {
		return fmt.Errorf("dlazy (gossip degree) must be positive, got %d", c.Dlazy)
	}
	if c.HeartbeatInterval <= 0 {
		return fmt.Errorf("HeartbeatInterval must be positive, got %v", c.HeartbeatInterval)
	}

	if c.HistoryLength <= 0 {
		return fmt.Errorf("HistoryLength must be positive, got %d", c.HistoryLength)
	}
	if c.HistoryGossip <= 0 {
		return fmt.Errorf("HistoryGossip must be positive, got %d", c.HistoryGossip)
	}

	if c.HistoryGossip > c.HistoryLength {
		return fmt.Errorf("HistoryGossip (%d) must be less than or equal to HistoryLength (%d)", c.HistoryGossip, c.HistoryLength)
	}

	if c.ValidationConcurrency <= 0 {
		return fmt.Errorf("ValidationConcurrency must be positive, got %d", c.ValidationConcurrency)
	}
	if c.PublishTimeout <= 0 {
		return fmt.Errorf("PublishTimeout must be positive, got %v", c.PublishTimeout)
	}

	return nil
}
