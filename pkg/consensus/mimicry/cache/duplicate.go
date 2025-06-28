package cache

import (
	"context"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
)

// DuplicateCache is an interface for managing duplicate node detection with TTL-based expiration.
type DuplicateCache interface {
	// Start initializes the cache and begins background cleanup operations.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the cache and its background operations.
	Stop() error
	// GetNodesCache returns the underlying TTL cache for node data.
	GetNodesCache() *ttlcache.Cache[string, time.Time]
}

// duplicateCache implements the DuplicateCache interface.
type duplicateCache struct {
	nodes *ttlcache.Cache[string, time.Time]
	log   logrus.FieldLogger

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewDuplicateCache creates a new DuplicateCache instance with the specified TTL duration for cached entries.
func NewDuplicateCache(log logrus.FieldLogger, nodeCacheDuration time.Duration) DuplicateCache {
	return &duplicateCache{
		nodes: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](nodeCacheDuration),
		),
		log: log.WithField("component", "duplicate_cache"),
	}
}

// Start initializes the cache and begins background cleanup operations.
func (d *duplicateCache) Start(ctx context.Context) error {
	_, d.cancel = context.WithCancel(ctx)

	d.wg.Add(1)

	startFunc := func() {
		defer d.wg.Done()
		d.log.Debug("Starting duplicate cache")
		d.nodes.Start()
		d.log.Debug("Duplicate cache stopped")
	}
	go startFunc()

	d.log.Info("Duplicate cache started")

	return nil
}

// Stop gracefully shuts down the cache and its background operations.
func (d *duplicateCache) Stop() error {
	d.log.Info("Stopping duplicate cache")

	if d.cancel != nil {
		d.cancel()
	}

	d.nodes.Stop()
	d.wg.Wait()

	d.log.Info("Duplicate cache stopped")

	return nil
}

// GetNodesCache returns the underlying TTL cache for node data.
func (d *duplicateCache) GetNodesCache() *ttlcache.Cache[string, time.Time] {
	return d.nodes
}
