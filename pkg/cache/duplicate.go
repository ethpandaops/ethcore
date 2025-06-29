package cache

import (
	"context"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
)

// DuplicateCache is a generic interface for managing duplicate detection with TTL-based expiration.
type DuplicateCache[K comparable, V any] interface {
	// Start initializes the cache and begins background cleanup operations.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the cache and its background operations.
	Stop() error
	// GetCache returns the underlying TTL cache.
	GetCache() *ttlcache.Cache[K, V]
}

// Config holds configuration options for the DuplicateCache.
type Config struct {
	// TTL is the time-to-live for cached entries.
	TTL time.Duration
	// Capacity sets the maximum number of items the cache can hold.
	// If 0, the cache has no size limit.
	Capacity uint64
	// OnEviction is called when an item is evicted from the cache.
	OnEviction func(key any, value any, reason ttlcache.EvictionReason)
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		TTL:      5 * time.Minute,
		Capacity: 0, // No limit
	}
}

// duplicateCache implements the DuplicateCache interface.
type duplicateCache[K comparable, V any] struct {
	cache *ttlcache.Cache[K, V]
	log   logrus.FieldLogger

	cancel context.CancelFunc
}

// NewDuplicateCacheWithConfig creates a new generic DuplicateCache instance with a full configuration.
func NewDuplicateCacheWithConfig[K comparable, V any](log logrus.FieldLogger, config Config) DuplicateCache[K, V] {
	opts := []ttlcache.Option[K, V]{
		ttlcache.WithTTL[K, V](config.TTL),
	}

	if config.Capacity > 0 {
		opts = append(opts, ttlcache.WithCapacity[K, V](config.Capacity))
	}

	cache := ttlcache.New(opts...)

	if config.OnEviction != nil {
		cache.OnEviction(func(ctx context.Context, reason ttlcache.EvictionReason, item *ttlcache.Item[K, V]) {
			config.OnEviction(item.Key(), item.Value(), reason)
		})
	}

	return &duplicateCache[K, V]{
		cache: cache,
		log:   log.WithField("component", "cache"),
	}
}

// NewDuplicateCache creates a new DuplicateCache with just a TTL duration.
// This is a convenience function for simple use cases.
func NewDuplicateCache[K comparable, V any](log logrus.FieldLogger, ttl time.Duration) DuplicateCache[K, V] {
	config := DefaultConfig()
	config.TTL = ttl

	return NewDuplicateCacheWithConfig[K, V](log, config)
}

// Start initializes the cache and begins background cleanup operations.
func (d *duplicateCache[K, V]) Start(ctx context.Context) error {
	_, d.cancel = context.WithCancel(ctx)

	go func() {
		d.log.Debug("Starting cache")
		d.cache.Start()
		d.log.Debug("Cache stopped")
	}()

	d.log.Info("Cache started")

	return nil
}

// Stop gracefully shuts down the cache and its background operations.
func (d *duplicateCache[K, V]) Stop() error {
	d.log.Info("Stopping cache")

	if d.cancel != nil {
		d.cancel()
	}

	d.cache.Stop()

	d.log.Info("Cache stopped")

	return nil
}

// GetCache returns the underlying TTL cache.
func (d *duplicateCache[K, V]) GetCache() *ttlcache.Cache[K, V] {
	return d.cache
}
