package pubsub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test default values
	assert.Equal(t, 10<<20, cfg.MaxMessageSize) // 10MB
	assert.Equal(t, 100, cfg.ValidationBufferSize)
	assert.Equal(t, 10, cfg.ValidationConcurrency)
	assert.Equal(t, 5*time.Second, cfg.PublishTimeout)
	assert.Nil(t, cfg.GossipSubParams)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  *DefaultConfig(),
			wantErr: false,
		},
		{
			name: "valid custom config",
			config: Config{
				MaxMessageSize:        2 * 1024 * 1024,
				ValidationBufferSize:  200,
				ValidationConcurrency: 20,
				PublishTimeout:        10 * time.Second,
				GossipSubParams:       nil,
			},
			wantErr: false,
		},
		{
			name: "zero max message size",
			config: Config{
				MaxMessageSize:        0,
				ValidationBufferSize:  100,
				ValidationConcurrency: 10,
				PublishTimeout:        time.Second,
			},
			wantErr: true, // Zero is NOT allowed
		},
		{
			name: "zero validation buffer size",
			config: Config{
				MaxMessageSize:        1024 * 1024,
				ValidationBufferSize:  0,
				ValidationConcurrency: 10,
				PublishTimeout:        time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero validation concurrency",
			config: Config{
				MaxMessageSize:        1024 * 1024,
				ValidationBufferSize:  100,
				ValidationConcurrency: 0,
				PublishTimeout:        time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero publish timeout",
			config: Config{
				MaxMessageSize:        1024 * 1024,
				ValidationBufferSize:  100,
				ValidationConcurrency: 10,
				PublishTimeout:        0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigCopy(t *testing.T) {
	// Test that modifying a config doesn't affect the default
	cfg1 := DefaultConfig()
	cfg2 := *cfg1

	cfg2.MaxMessageSize = 999
	cfg2.ValidationBufferSize = 50

	// Since we properly copied the config, cfg1 should be unchanged
	assert.Equal(t, 10<<20, cfg1.MaxMessageSize)
	assert.Equal(t, 100, cfg1.ValidationBufferSize)
	assert.Equal(t, 999, cfg2.MaxMessageSize)
	assert.Equal(t, 50, cfg2.ValidationBufferSize)
}

func TestNewConfigWithCustomGossipSub(t *testing.T) {
	cfg := NewConfigWithCustomGossipSub()

	// Should have default values
	assert.Equal(t, 10<<20, cfg.MaxMessageSize)
	assert.Equal(t, 100, cfg.ValidationBufferSize)
	assert.Equal(t, 10, cfg.ValidationConcurrency)
	assert.Equal(t, 5*time.Second, cfg.PublishTimeout)

	// Should have GossipSubParams set
	assert.NotNil(t, cfg.GossipSubParams)
}
