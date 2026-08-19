package v1_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func racePoCDecoder(data []byte) (any, error) { return data, nil }

func racePoCHandler() *v1.HandlerConfig[any] {
	return v1.NewHandlerConfig[any](
		v1.WithDecoder[any](racePoCDecoder),
		v1.WithProcessor[any](func(ctx context.Context, msg any, from peer.ID) error {
			return nil
		}),
	)
}

// TestSubscribe_ConcurrentCancelDoesNotRaceProcessorsMap guards against
// PUBSUB-01: Subscribe used to write g.processors while holding only
// subMutex, while removeProcessor (triggered by cancelling another
// subscription) deletes from the same map under procMutex. Two distinct
// locks guarding the same map meant a concurrent Subscribe plus
// subscription cancellation could hit a fatal concurrent map write. Run
// under -race.
func TestSubscribe_ConcurrentCancelDoesNotRaceProcessorsMap(t *testing.T) {
	const attempts = 50

	for i := 0; i < attempts; i++ {
		t.Run(fmt.Sprintf("attempt-%d", i), func(t *testing.T) {
			ctx := context.Background()
			infra := NewTestInfrastructure(t)
			defer infra.Cleanup()

			node, err := infra.CreateNode(ctx)
			require.NoError(t, err)

			g := node.Gossipsub

			topic0, err := v1.NewTopic[any](fmt.Sprintf("poc-topic-0-%d", i))
			require.NoError(t, err)
			topic1, err := v1.NewTopic[any](fmt.Sprintf("poc-topic-1-%d", i))
			require.NoError(t, err)

			require.NoError(t, g.Register(topic0, racePoCHandler()))
			require.NoError(t, g.Register(topic1, racePoCHandler()))

			sub0, err := v1.Subscribe[any](ctx, g, topic0)
			require.NoError(t, err)

			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); sub0.Cancel() }() // triggers removeProcessor(topic0) under procMutex
			go func() {
				defer wg.Done()
				_, _ = v1.Subscribe[any](ctx, g, topic1) // writes g.processors[topic1] -- must go through procMutex
			}()
			wg.Wait()
		})
	}
}
