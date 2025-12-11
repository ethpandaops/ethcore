package mimicry

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a minimal client for testing pub/sub without network.
func newTestClient(t *testing.T) *Client {
	t.Helper()

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	ctx := context.Background()
	client, err := New(
		ctx,
		log,
		"enode://ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd31387574077f301b421bc84df7266c44e9e6d569fc56be00812904767bf5ccd1fc7f@127.0.0.1:30303",
		"test-client",
	)
	require.NoError(t, err)

	return client
}

func TestOnDisconnect(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	var received *Disconnect

	var wg sync.WaitGroup

	wg.Add(1)

	client.OnDisconnect(ctx, func(ctx context.Context, reason *Disconnect) error {
		received = reason
		wg.Done()

		return nil
	})

	// Publish a disconnect event
	expectedReason := &Disconnect{Reason: p2p.DiscQuitting}
	client.publishDisconnect(ctx, expectedReason)

	// Wait for handler with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		require.NotNil(t, received)
		assert.Equal(t, p2p.DiscQuitting, received.Reason)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for disconnect event")
	}
}

func TestOnHello(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	var received *Hello

	var wg sync.WaitGroup

	wg.Add(1)

	client.OnHello(ctx, func(ctx context.Context, hello *Hello) error {
		received = hello
		wg.Done()

		return nil
	})

	// Publish a hello event
	expectedHello := &Hello{
		Version: P2PProtocolVersion,
		Name:    "peer-client",
		Caps: []p2p.Cap{
			{Name: ETHCapName, Version: 68},
		},
		ListenPort: 30303,
		ID:         make([]byte, 64),
	}
	client.publishHello(ctx, expectedHello)

	// Wait for handler
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		require.NotNil(t, received)
		assert.Equal(t, expectedHello.Version, received.Version)
		assert.Equal(t, expectedHello.Name, received.Name)
		assert.Equal(t, len(expectedHello.Caps), len(received.Caps))
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for hello event")
	}
}

func TestOnStatus(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	var received Status

	var wg sync.WaitGroup

	wg.Add(1)

	client.OnStatus(ctx, func(ctx context.Context, status Status) error {
		received = status
		wg.Done()

		return nil
	})

	// Publish a status event
	expectedStatus := &Status68{
		StatusPacket68: eth.StatusPacket68{
			ProtocolVersion: 68,
			NetworkID:       1,
		},
	}
	client.publishStatus(ctx, expectedStatus)

	// Wait for handler
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		require.NotNil(t, received)
		assert.Equal(t, uint64(1), received.GetNetworkID())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for status event")
	}
}

func TestOnTransactions(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	var received *Transactions

	var wg sync.WaitGroup

	wg.Add(1)

	client.OnTransactions(ctx, func(ctx context.Context, txs *Transactions) error {
		received = txs
		wg.Done()

		return nil
	})

	// Publish a transactions event
	expectedTxs := &Transactions{}
	client.publishTransactions(ctx, expectedTxs)

	// Wait for handler
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		require.NotNil(t, received)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for transactions event")
	}
}

func TestOnNewPooledTransactionHashes(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	var received *NewPooledTransactionHashes

	var wg sync.WaitGroup

	wg.Add(1)

	client.OnNewPooledTransactionHashes(ctx, func(ctx context.Context, hashes *NewPooledTransactionHashes) error {
		received = hashes
		wg.Done()

		return nil
	})

	// Publish a new pooled transaction hashes event
	hashes := NewPooledTransactionHashes{
		Hashes: []common.Hash{
			common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
	}
	client.publishNewPooledTransactionHashes(ctx, &hashes)

	// Wait for handler
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		require.NotNil(t, received)
		require.Len(t, received.Hashes, 1)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for new pooled transaction hashes event")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	var count int

	var mu sync.Mutex

	var wg sync.WaitGroup

	wg.Add(3)

	// Register multiple handlers for the same event
	for i := 0; i < 3; i++ {
		client.OnDisconnect(ctx, func(ctx context.Context, reason *Disconnect) error {
			mu.Lock()
			count++
			mu.Unlock()
			wg.Done()

			return nil
		})
	}

	// Publish a disconnect event
	client.publishDisconnect(ctx, &Disconnect{Reason: p2p.DiscQuitting})

	// Wait for all handlers
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		mu.Lock()
		assert.Equal(t, 3, count, "all three handlers should have been called")
		mu.Unlock()
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for all handlers")
	}
}

func TestHandlerError(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	var wg sync.WaitGroup

	wg.Add(1)

	// Register a handler that returns an error
	client.OnDisconnect(ctx, func(ctx context.Context, reason *Disconnect) error {
		wg.Done()

		return errors.New("handler error")
	})

	// Publish a disconnect event - should not panic even with error
	client.publishDisconnect(ctx, &Disconnect{Reason: p2p.DiscQuitting})

	// Wait for handler
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Handler completed despite returning an error
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handler")
	}
}

func TestTopicConstants(t *testing.T) {
	// Verify topic constants are defined correctly
	assert.Equal(t, "disconnect", topicDisconnect)
	assert.Equal(t, "hello", topicHello)
	assert.Equal(t, "status", topicStatus)
	assert.Equal(t, "transactions", topicTransactions)
	assert.Equal(t, "new_pooled_transaction_hashes", topicNewPooledTransactionHashes)
}
