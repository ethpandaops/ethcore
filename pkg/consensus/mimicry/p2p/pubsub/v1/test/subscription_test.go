package v1_test

import (
	"context"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscription(t *testing.T) {
	t.Run("Topic", func(t *testing.T) {
		_, cancel := context.WithCancel(context.Background())
		defer cancel()

		sub := createMockSubscription("test-topic", cancel)

		assert.Equal(t, "test-topic", sub.Topic())
	})

	t.Run("Cancel", func(t *testing.T) {
		cancelled := false
		sub := createMockSubscription("test-topic", func() {
			cancelled = true
		})

		// First cancel should execute the cancel function
		sub.Cancel()
		assert.True(t, cancelled)
		assert.True(t, sub.IsCancelled())

		// Second cancel should be safe
		sub.Cancel()
		assert.True(t, sub.IsCancelled())
	})

	t.Run("Nilv1.Subscription", func(t *testing.T) {
		var sub *v1.Subscription
		assert.Empty(t, sub.Topic())
		assert.True(t, sub.IsCancelled())
		sub.Cancel() // Should not panic
	})
}

func TestSubnetSubscription(t *testing.T) {
	// Create a test encoder
	encoder := &subscriptionTestEncoder[string]{}

	// Create a subnet topic
	subnetTopic, err := v1.NewSubnetTopic[string]("test_subnet_%d", 64, encoder)
	require.NoError(t, err)

	t.Run("v1.NewSubnetSubscription", func(t *testing.T) {
		ss, err := v1.NewSubnetSubscription(subnetTopic)
		require.NoError(t, err)
		assert.NotNil(t, ss)
		assert.Empty(t, ss.Active())

		// Test with nil subnet topic
		_, err = v1.NewSubnetSubscription[string](nil)
		assert.Error(t, err)
	})

	t.Run("Add", func(t *testing.T) {
		ss, err := v1.NewSubnetSubscription(subnetTopic)
		require.NoError(t, err)

		sub1 := createMockSubscription("test", func() {})
		err = ss.Add(0, sub1)
		require.NoError(t, err)

		active := ss.Active()
		assert.Len(t, active, 1)
		assert.Contains(t, active, uint64(0))

		// Test adding nil subscription
		err = ss.Add(1, nil)
		assert.Error(t, err)

		// Test adding subnet beyond max
		err = ss.Add(64, sub1)
		assert.Error(t, err)

		// Test replacing existing subscription
		sub2 := createMockSubscription("test", func() {})
		err = ss.Add(0, sub2)
		require.NoError(t, err)
		assert.Equal(t, sub2, ss.Get(0))
	})

	t.Run("Remove", func(t *testing.T) {
		ss, err := v1.NewSubnetSubscription(subnetTopic)
		require.NoError(t, err)

		sub := createMockSubscription("test", func() {})
		err = ss.Add(5, sub)
		require.NoError(t, err)

		// Remove existing subscription
		removed := ss.Remove(5)
		assert.True(t, removed)
		assert.Empty(t, ss.Active())

		// Remove non-existing subscription
		removed = ss.Remove(5)
		assert.False(t, removed)
	})

	t.Run("Set", func(t *testing.T) {
		ss, err := v1.NewSubnetSubscription(subnetTopic)
		require.NoError(t, err)

		// Add initial subscriptions
		sub1 := createMockSubscription("test", func() {})
		sub2 := createMockSubscription("test", func() {})
		err = ss.Add(1, sub1)
		require.NoError(t, err)
		err = ss.Add(2, sub2)
		require.NoError(t, err)

		// Set new subscriptions
		sub3 := createMockSubscription("test", func() {})
		sub4 := createMockSubscription("test", func() {})
		newSubs := map[uint64]*v1.Subscription{
			2: sub2, // Keep subnet 2
			3: sub3, // Add subnet 3
			4: sub4, // Add subnet 4
		}

		err = ss.Set(newSubs)
		require.NoError(t, err)

		active := ss.Active()
		assert.Len(t, active, 3)
		assert.ElementsMatch(t, []uint64{2, 3, 4}, active)

		// Test with invalid subnet
		invalidSubs := map[uint64]*v1.Subscription{
			65: sub3,
		}
		err = ss.Set(invalidSubs)
		assert.Error(t, err)
	})

	t.Run("Get", func(t *testing.T) {
		ss, err := v1.NewSubnetSubscription(subnetTopic)
		require.NoError(t, err)

		sub := createMockSubscription("test", func() {})
		err = ss.Add(10, sub)
		require.NoError(t, err)

		// Get existing subscription
		retrieved := ss.Get(10)
		assert.Equal(t, sub, retrieved)

		// Get non-existing subscription
		retrieved = ss.Get(20)
		assert.Nil(t, retrieved)
	})

	t.Run("Clear", func(t *testing.T) {
		ss, err := v1.NewSubnetSubscription(subnetTopic)
		require.NoError(t, err)

		// Add multiple subscriptions
		for i := uint64(0); i < 5; i++ {
			sub := createMockSubscription("test", func() {})
			err = ss.Add(i, sub)
			require.NoError(t, err)
		}

		assert.Equal(t, 5, ss.Count())

		// Clear all subscriptions
		ss.Clear()
		assert.Empty(t, ss.Active())
		assert.Equal(t, 0, ss.Count())
	})

	t.Run("Count", func(t *testing.T) {
		ss, err := v1.NewSubnetSubscription(subnetTopic)
		require.NoError(t, err)

		// Test with no subscriptions
		assert.Equal(t, 0, ss.Count())

		// Add subscriptions
		sub1 := createMockSubscription("test", func() {})
		sub2 := createMockSubscription("test", func() {})
		err = ss.Add(1, sub1)
		require.NoError(t, err)
		err = ss.Add(2, sub2)
		require.NoError(t, err)

		assert.Equal(t, 2, ss.Count())

		// Cancel one subscription
		sub1.Cancel()
		assert.Equal(t, 1, ss.Count())
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		ss, err := v1.NewSubnetSubscription(subnetTopic)
		require.NoError(t, err)

		// Run concurrent operations
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; i < 100; i++ {
				sub := createMockSubscription("test", func() {})
				_ = ss.Add(uint64(i%10), sub)
			}
		}()

		go func() {
			for i := 0; i < 100; i++ {
				_ = ss.Remove(uint64(i % 10))
			}
		}()

		go func() {
			for i := 0; i < 100; i++ {
				_ = ss.Active()
			}
		}()

		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent test timed out")
		}
	})
}

// subscriptionTestEncoder is a simple encoder for testing subscription functionality
type subscriptionTestEncoder[T any] struct{}

func (e *subscriptionTestEncoder[T]) Encode(msg T) ([]byte, error) {
	return []byte("encoded"), nil
}

func (e *subscriptionTestEncoder[T]) Decode(data []byte) (T, error) {
	var zero T
	return zero, nil
}
