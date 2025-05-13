package cache

import (
	"context"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

type DuplicateCache struct {
	Nodes *ttlcache.Cache[string, time.Time]
}

func NewDuplicateCache(nodeCacheDuration time.Duration) *DuplicateCache {
	return &DuplicateCache{
		Nodes: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](nodeCacheDuration),
		),
	}
}

func (d *DuplicateCache) Start(ctx context.Context) error {
	go d.Nodes.Start()

	return nil
}

func (d *DuplicateCache) Stop() {
	d.Nodes.Stop()
}
