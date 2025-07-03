# Cache Package

This package provides a generic duplicate cache with optional Prometheus metrics support.

## Features

- Generic type-safe cache implementation
- TTL-based expiration
- Optional capacity limits
- Prometheus metrics integration (opt-in)
- Scheduled metrics updates
- Thread-safe operations

## Usage

### Basic Usage (without metrics)

```go
import (
    "time"
    "github.com/sirupsen/logrus"
    "github.com/ethpandaops/ethcore/pkg/cache"
)

// Create a simple cache
log := logrus.New()
ttl := 5 * time.Minute
cache := cache.NewDuplicateCache[string, string](log, ttl)

// Start the cache
ctx := context.Background()
cache.Start(ctx)
defer cache.Stop()

// Use the cache
cache.Set("key1", "value1")
if cache.Has("key1") {
    item := cache.Get("key1")
    fmt.Println(item.Value()) // "value1"
}
```

### Advanced Usage (with metrics)

```go
config := cache.Config{
    TTL:      120 * time.Minute,
    Capacity: 10000, // Optional capacity limit
    Metrics: &cache.MetricsConfig{
        Namespace: "myapp",
        Subsystem: "cache",
        InstanceLabels: map[string]string{
            "cache_type": "session",
            "region":     "us-east",
        },
        UpdateInterval: 10 * time.Second,
        Registerer:     prometheus.DefaultRegisterer,
    },
}

cache := cache.NewDuplicateCacheWithConfig[string, time.Time](log, config)
```

## Metrics

When metrics are enabled, the following Prometheus metrics are exposed:

- `{namespace}_{subsystem}_insertions_total` - Total number of cache insertions
- `{namespace}_{subsystem}_hits_total` - Total number of cache hits
- `{namespace}_{subsystem}_misses_total` - Total number of cache misses
- `{namespace}_{subsystem}_evictions_total` - Total number of cache evictions
- `{namespace}_{subsystem}_size` - Current number of items in the cache

All metrics include the labels specified in `InstanceLabels`.
