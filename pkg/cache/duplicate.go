package cache

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
)

// DuplicateCache is a generic interface for managing duplicate detection with TTL-based expiration.
type DuplicateCache[K comparable, V any] interface {
	// Start initializes the cache and begins background cleanup operations.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the cache and its background operations.
	Stop() error
	// Set adds an item to the cache with metrics tracking.
	Set(key K, value V, ttl ...time.Duration) *ttlcache.Item[K, V]
	// Get retrieves an item from the cache with metrics tracking.
	Get(key K) *ttlcache.Item[K, V]
	// Has checks if a key exists in the cache with metrics tracking.
	Has(key K) bool
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
	// Metrics configuration. If nil, metrics are disabled.
	Metrics *MetricsConfig
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
	cache  *ttlcache.Cache[K, V]
	log    logrus.FieldLogger
	config Config

	// Metrics
	metrics       *Metrics
	metricsLabels []string // Cached label values in correct order

	// Scheduler for metrics updates
	scheduler gocron.Scheduler

	// Atomic counters for metrics that aren't exposed by ttlcache
	insertions atomic.Uint64

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

	d := &duplicateCache[K, V]{
		cache:  cache,
		log:    log.WithField("module", "ethcore/cache"),
		config: config,
	}

	// Set up eviction callback
	if config.OnEviction != nil || config.Metrics != nil {
		cache.OnEviction(func(ctx context.Context, reason ttlcache.EvictionReason, item *ttlcache.Item[K, V]) {
			if config.OnEviction != nil {
				config.OnEviction(item.Key(), item.Value(), reason)
			}

			if d.metrics != nil {
				d.metrics.IncEvictions(d.metricsLabels...)
			}
		})
	}

	// Initialize metrics if configured
	if config.Metrics != nil {
		d.metrics = NewMetrics(*config.Metrics)
		if err := d.metrics.Register(config.Metrics.Registerer); err != nil {
			log.WithError(err).Warn("Failed to register cache metrics")
		}

		// Cache the label values for efficient metric updates
		d.metricsLabels = d.metrics.ExtractLabelValues(config.Metrics.InstanceLabels)
	}

	return d
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
	ctx, d.cancel = context.WithCancel(ctx)

	go func() {
		d.cache.Start()
	}()

	// Start metrics scheduler if metrics are enabled
	if d.metrics != nil {
		if err := d.startMetricsScheduler(ctx); err != nil {
			return err
		}
	}

	d.log.Info("Duplicate cache started")

	return nil
}

// Stop gracefully shuts down the cache and its background operations.
func (d *duplicateCache[K, V]) Stop() error {
	d.log.Info("Stopping duplicate cache")

	if d.cancel != nil {
		d.cancel()
	}

	// Stop metrics scheduler
	if d.scheduler != nil {
		if err := d.scheduler.Shutdown(); err != nil {
			d.log.WithError(err).Warn("Failed to shutdown metrics scheduler")
		}
	}

	d.cache.Stop()

	// Unregister metrics
	if d.metrics != nil && d.config.Metrics != nil {
		d.metrics.Unregister(d.config.Metrics.Registerer)
	}

	d.log.Info("Duplicate cache stopped")

	return nil
}

// Set adds an item to the cache with metrics tracking.
func (d *duplicateCache[K, V]) Set(key K, value V, ttl ...time.Duration) *ttlcache.Item[K, V] {
	var item *ttlcache.Item[K, V]

	if len(ttl) > 0 {
		item = d.cache.Set(key, value, ttl[0])
	} else {
		item = d.cache.Set(key, value, ttlcache.DefaultTTL)
	}

	if d.metrics != nil {
		d.insertions.Add(1)
		d.metrics.IncInsertions(d.metricsLabels...)
	}

	return item
}

// Get retrieves an item from the cache with metrics tracking.
func (d *duplicateCache[K, V]) Get(key K) *ttlcache.Item[K, V] {
	item := d.cache.Get(key)

	if d.metrics != nil {
		if item != nil {
			d.metrics.IncHits(d.metricsLabels...)
		} else {
			d.metrics.IncMisses(d.metricsLabels...)
		}
	}

	return item
}

// Has checks if a key exists in the cache with metrics tracking.
func (d *duplicateCache[K, V]) Has(key K) bool {
	has := d.cache.Has(key)

	if d.metrics != nil {
		if has {
			d.metrics.IncHits(d.metricsLabels...)
		} else {
			d.metrics.IncMisses(d.metricsLabels...)
		}
	}

	return has
}

// GetCache returns the underlying TTL cache.
func (d *duplicateCache[K, V]) GetCache() *ttlcache.Cache[K, V] {
	return d.cache
}

// startMetricsScheduler starts the scheduler for periodic metrics updates.
func (d *duplicateCache[K, V]) startMetricsScheduler(ctx context.Context) error {
	scheduler, err := gocron.NewScheduler(gocron.WithLocation(time.Local))
	if err != nil {
		return err
	}

	d.scheduler = scheduler

	// Determine update interval
	interval := d.config.Metrics.UpdateInterval
	if interval == 0 {
		interval = 5 * time.Second
	}

	// Schedule periodic metrics updates
	if _, err := d.scheduler.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(
			func(ctx context.Context) {
				d.updateMetrics()
			},
			ctx,
		),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	); err != nil {
		return err
	}

	d.scheduler.Start()

	return nil
}

// updateMetrics updates the metrics with current cache statistics.
func (d *duplicateCache[K, V]) updateMetrics() {
	if d.metrics == nil {
		return
	}

	// Update size metric
	d.metrics.SetSize(float64(d.cache.Len()), d.metricsLabels...)

	// Log metrics for debugging
	metrics := d.cache.Metrics()

	d.log.WithFields(logrus.Fields{
		"insertions": d.insertions.Load(),
		"hits":       metrics.Hits,
		"misses":     metrics.Misses,
		"evictions":  metrics.Evictions,
		"size":       d.cache.Len(),
	}).Debug("Cache metrics updated")
}
