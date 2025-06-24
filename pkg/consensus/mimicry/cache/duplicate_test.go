package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDuplicateCache(t *testing.T) {
	tests := []struct {
		name              string
		nodeCacheDuration time.Duration
	}{
		{
			name:              "with 1 minute duration",
			nodeCacheDuration: time.Minute,
		},
		{
			name:              "with 5 minute duration",
			nodeCacheDuration: 5 * time.Minute,
		},
		{
			name:              "with 1 hour duration",
			nodeCacheDuration: time.Hour,
		},
		{
			name:              "with 100ms duration",
			nodeCacheDuration: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewDuplicateCache(tt.nodeCacheDuration)
			require.NotNil(t, cache)
			require.NotNil(t, cache.Nodes)
		})
	}
}

func TestDuplicateCache_StartStop(t *testing.T) {
	cache := NewDuplicateCache(time.Minute)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Stop should not panic
	assert.NotPanics(t, func() {
		cache.Stop()
	})
}

func TestDuplicateCache_WithContext(t *testing.T) {
	cache := NewDuplicateCache(time.Minute)
	require.NotNil(t, cache)

	ctx, cancel := context.WithCancel(context.Background())
	err := cache.Start(ctx)
	assert.NoError(t, err)

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Stop should still work after context cancellation
	assert.NotPanics(t, func() {
		cache.Stop()
	})
}

func TestDuplicateCache_CacheOperations(t *testing.T) {
	ttl := 100 * time.Millisecond
	cache := NewDuplicateCache(ttl)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)
	defer cache.Stop()

	// Give the cache time to start
	time.Sleep(10 * time.Millisecond)

	// Test setting values
	now := time.Now()
	cache.Nodes.Set("node1", now, ttlcache.DefaultTTL)
	cache.Nodes.Set("node2", now.Add(time.Second), ttlcache.DefaultTTL)

	// Test getting values
	item1 := cache.Nodes.Get("node1")
	require.NotNil(t, item1)
	assert.Equal(t, now, item1.Value())

	item2 := cache.Nodes.Get("node2")
	require.NotNil(t, item2)
	assert.Equal(t, now.Add(time.Second), item2.Value())

	// Test TTL expiration
	time.Sleep(ttl + 50*time.Millisecond)

	// Values should be expired
	item1Expired := cache.Nodes.Get("node1")
	assert.Nil(t, item1Expired)

	item2Expired := cache.Nodes.Get("node2")
	assert.Nil(t, item2Expired)
}

func TestDuplicateCache_MultipleStartStop(t *testing.T) {
	cache := NewDuplicateCache(time.Minute)
	require.NotNil(t, cache)

	ctx := context.Background()

	// First start/stop cycle
	err := cache.Start(ctx)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	cache.Stop()

	// Second start/stop cycle should work
	err = cache.Start(ctx)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	cache.Stop()
}

func TestDuplicateCache_ConcurrentAccess(t *testing.T) {
	cache := NewDuplicateCache(time.Minute)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)
	defer cache.Stop()

	// Give the cache time to start
	time.Sleep(10 * time.Millisecond)

	// Test concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			key := fmt.Sprintf("node%d", idx)
			cache.Nodes.Set(key, time.Now(), ttlcache.DefaultTTL)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all items were set
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("node%d", i)
		item := cache.Nodes.Get(key)
		assert.NotNil(t, item, "Expected item for key %s", key)
	}
}

func TestDuplicateCache_DeleteOperations(t *testing.T) {
	cache := NewDuplicateCache(time.Minute)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)
	defer cache.Stop()

	// Give the cache time to start
	time.Sleep(10 * time.Millisecond)

	// Set some values
	now := time.Now()
	cache.Nodes.Set("node1", now, ttlcache.DefaultTTL)
	cache.Nodes.Set("node2", now, ttlcache.DefaultTTL)
	cache.Nodes.Set("node3", now, ttlcache.DefaultTTL)

	// Verify they exist
	assert.NotNil(t, cache.Nodes.Get("node1"))
	assert.NotNil(t, cache.Nodes.Get("node2"))
	assert.NotNil(t, cache.Nodes.Get("node3"))

	// Delete one item
	cache.Nodes.Delete("node2")

	// Verify deletion
	assert.NotNil(t, cache.Nodes.Get("node1"))
	assert.Nil(t, cache.Nodes.Get("node2"))
	assert.NotNil(t, cache.Nodes.Get("node3"))

	// Delete all
	cache.Nodes.DeleteAll()

	// Verify all are deleted
	assert.Nil(t, cache.Nodes.Get("node1"))
	assert.Nil(t, cache.Nodes.Get("node2"))
	assert.Nil(t, cache.Nodes.Get("node3"))
}
