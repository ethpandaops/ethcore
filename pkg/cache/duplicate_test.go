package cache

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDuplicateCacheWithConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "with 1 minute duration",
			config: Config{
				TTL: time.Minute,
			},
		},
		{
			name: "with 5 minute duration",
			config: Config{
				TTL: 5 * time.Minute,
			},
		},
		{
			name: "with 1 hour duration",
			config: Config{
				TTL: time.Hour,
			},
		},
		{
			name: "with 100ms duration",
			config: Config{
				TTL: 100 * time.Millisecond,
			},
		},
		{
			name: "with capacity limit",
			config: Config{
				TTL:      time.Minute,
				Capacity: 100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			cache := NewDuplicateCacheWithConfig[string, time.Time](log, tt.config)
			require.NotNil(t, cache)
			require.NotNil(t, cache.GetCache())
		})
	}
}

func TestNewDuplicateCache(t *testing.T) {
	log := logrus.New()
	ttl := 5 * time.Minute
	cache := NewDuplicateCache[string, time.Time](log, ttl)
	require.NotNil(t, cache)
	require.NotNil(t, cache.GetCache())
}

func TestDuplicateCache_StartStop(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	cache := NewDuplicateCache[string, time.Time](log, time.Minute)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Stop should not panic and should return nil
	err = cache.Stop()
	assert.NoError(t, err)
}

func TestDuplicateCache_WithContext(t *testing.T) {
	log := logrus.New()
	cache := NewDuplicateCache[string, time.Time](log, time.Minute)
	require.NotNil(t, cache)

	ctx, cancel := context.WithCancel(context.Background())
	err := cache.Start(ctx)
	assert.NoError(t, err)

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Stop should still work after context cancellation
	err = cache.Stop()
	assert.NoError(t, err)
}

func TestDuplicateCache_CacheOperations(t *testing.T) {
	log := logrus.New()
	ttl := 100 * time.Millisecond
	cache := NewDuplicateCache[string, time.Time](log, ttl)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := cache.Stop()
		assert.NoError(t, err)
	}()

	// Give the cache time to start
	time.Sleep(10 * time.Millisecond)

	// Test setting values
	now := time.Now()
	nodes := cache.GetCache()
	nodes.Set("node1", now, ttlcache.DefaultTTL)
	nodes.Set("node2", now.Add(time.Second), ttlcache.DefaultTTL)

	// Test getting values
	item1 := nodes.Get("node1")
	require.NotNil(t, item1)
	assert.Equal(t, now, item1.Value())

	item2 := nodes.Get("node2")
	require.NotNil(t, item2)
	assert.Equal(t, now.Add(time.Second), item2.Value())

	// Test TTL expiration
	time.Sleep(ttl + 50*time.Millisecond)

	// Values should be expired
	item1Expired := nodes.Get("node1")
	assert.Nil(t, item1Expired)

	item2Expired := nodes.Get("node2")
	assert.Nil(t, item2Expired)
}

func TestDuplicateCache_IntegerKeys(t *testing.T) {
	log := logrus.New()
	cache := NewDuplicateCache[int, string](log, time.Minute)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := cache.Stop()
		assert.NoError(t, err)
	}()

	// Test with integer keys
	c := cache.GetCache()
	c.Set(1, "value1", ttlcache.DefaultTTL)
	c.Set(2, "value2", ttlcache.DefaultTTL)
	c.Set(3, "value3", ttlcache.DefaultTTL)

	// Verify values
	item1 := c.Get(1)
	require.NotNil(t, item1)
	assert.Equal(t, "value1", item1.Value())

	item2 := c.Get(2)
	require.NotNil(t, item2)
	assert.Equal(t, "value2", item2.Value())
}

func TestDuplicateCache_StructValues(t *testing.T) {
	type TestStruct struct {
		ID   int
		Name string
	}

	log := logrus.New()
	cache := NewDuplicateCache[string, TestStruct](log, time.Minute)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := cache.Stop()
		assert.NoError(t, err)
	}()

	// Test with struct values
	c := cache.GetCache()
	c.Set("user1", TestStruct{ID: 1, Name: "Alice"}, ttlcache.DefaultTTL)
	c.Set("user2", TestStruct{ID: 2, Name: "Bob"}, ttlcache.DefaultTTL)

	// Verify values
	item1 := c.Get("user1")
	require.NotNil(t, item1)
	assert.Equal(t, TestStruct{ID: 1, Name: "Alice"}, item1.Value())
}

func TestDuplicateCache_MultipleStartStop(t *testing.T) {
	log := logrus.New()
	cache := NewDuplicateCache[string, time.Time](log, time.Minute)
	require.NotNil(t, cache)

	ctx := context.Background()

	// First start/stop cycle
	err := cache.Start(ctx)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	err = cache.Stop()
	assert.NoError(t, err)

	// Second start/stop cycle should work
	err = cache.Start(ctx)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	err = cache.Stop()
	assert.NoError(t, err)
}

func TestDuplicateCache_ConcurrentAccess(t *testing.T) {
	log := logrus.New()
	cache := NewDuplicateCache[string, time.Time](log, time.Minute)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := cache.Stop()
		assert.NoError(t, err)
	}()

	// Give the cache time to start
	time.Sleep(10 * time.Millisecond)

	// Test concurrent writes
	done := make(chan bool)
	nodes := cache.GetCache()
	for i := 0; i < 10; i++ {
		go func(idx int) {
			key := fmt.Sprintf("node%d", idx)
			nodes.Set(key, time.Now(), ttlcache.DefaultTTL)
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
		item := nodes.Get(key)
		assert.NotNil(t, item, "Expected item for key %s", key)
	}
}

func TestDuplicateCache_DeleteOperations(t *testing.T) {
	log := logrus.New()
	cache := NewDuplicateCache[string, time.Time](log, time.Minute)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := cache.Stop()
		assert.NoError(t, err)
	}()

	// Give the cache time to start
	time.Sleep(10 * time.Millisecond)

	// Set some values
	now := time.Now()
	nodes := cache.GetCache()
	nodes.Set("node1", now, ttlcache.DefaultTTL)
	nodes.Set("node2", now, ttlcache.DefaultTTL)
	nodes.Set("node3", now, ttlcache.DefaultTTL)

	// Verify they exist
	assert.NotNil(t, nodes.Get("node1"))
	assert.NotNil(t, nodes.Get("node2"))
	assert.NotNil(t, nodes.Get("node3"))

	// Delete one item
	nodes.Delete("node2")

	// Verify deletion
	assert.NotNil(t, nodes.Get("node1"))
	assert.Nil(t, nodes.Get("node2"))
	assert.NotNil(t, nodes.Get("node3"))

	// Delete all
	nodes.DeleteAll()

	// Verify all are deleted
	assert.Nil(t, nodes.Get("node1"))
	assert.Nil(t, nodes.Get("node2"))
	assert.Nil(t, nodes.Get("node3"))
}

func TestDuplicateCache_EvictionCallback(t *testing.T) {
	log := logrus.New()
	var evictionCount int32

	config := Config{
		TTL:      100 * time.Millisecond,
		Capacity: 3,
		OnEviction: func(key any, value any, reason ttlcache.EvictionReason) {
			atomic.AddInt32(&evictionCount, 1)
			t.Logf("Evicted key=%v, reason=%v", key, reason)
		},
	}

	cache := NewDuplicateCacheWithConfig[string, int](log, config)
	require.NotNil(t, cache)

	ctx := context.Background()
	err := cache.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := cache.Stop()
		assert.NoError(t, err)
	}()

	// Give the cache time to start
	time.Sleep(10 * time.Millisecond)

	c := cache.GetCache()

	// Add items up to capacity
	c.Set("item1", 1, ttlcache.DefaultTTL)
	c.Set("item2", 2, ttlcache.DefaultTTL)
	c.Set("item3", 3, ttlcache.DefaultTTL)

	// Adding a 4th item should trigger eviction
	c.Set("item4", 4, ttlcache.DefaultTTL)

	// Wait a bit for eviction callback
	time.Sleep(50 * time.Millisecond)

	// Should have evicted 1 item due to capacity
	count1 := atomic.LoadInt32(&evictionCount)
	assert.GreaterOrEqual(t, count1, int32(1))

	// Wait for TTL expiration
	time.Sleep(100 * time.Millisecond)

	// Force cleanup by accessing items
	c.Get("item1")
	c.Get("item2")
	c.Get("item3")
	c.Get("item4")

	// Should have more evictions due to TTL
	count2 := atomic.LoadInt32(&evictionCount)
	assert.Greater(t, count2, int32(1))
}

func TestDuplicateCache_InterfaceCompliance(t *testing.T) {
	// Verify interface compliance at compile time
	var _ DuplicateCache[string, time.Time] = (*duplicateCache[string, time.Time])(nil)
}
