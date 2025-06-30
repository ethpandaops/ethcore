package eth_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncContributionProcessor(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	encoder := encoder.SszNetworkEncoder{}
	logger := logrus.New()

	t.Run("Topic", func(t *testing.T) {
		processor := &eth.SyncContributionProcessor{
			ForkDigest: forkDigest,
		}
		
		expectedTopic := eth.SyncContributionAndProofTopic(forkDigest)
		assert.Equal(t, expectedTopic, processor.Topic())
	})

	t.Run("AllPossibleTopics", func(t *testing.T) {
		processor := &eth.SyncContributionProcessor{
			ForkDigest: forkDigest,
		}
		
		topics := processor.AllPossibleTopics()
		assert.Len(t, topics, 1)
		assert.Equal(t, processor.Topic(), topics[0])
	})

	t.Run("Subscribe", func(t *testing.T) {
		// Test with nil gossipsub
		processor := &eth.SyncContributionProcessor{
			ForkDigest: forkDigest,
		}
		
		err := processor.Subscribe(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gossipsub reference not set")

		// Test with mock gossipsub
		mockGossipsub := &mockGossipsub{
			subscribedTopics: make(map[string]bool),
		}
		
		processor.Gossipsub = mockGossipsub
		err = processor.Subscribe(context.Background())
		assert.NoError(t, err)
		assert.True(t, mockGossipsub.subscribedTopics[processor.Topic()])
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		// Test with nil gossipsub
		processor := &eth.SyncContributionProcessor{
			ForkDigest: forkDigest,
		}
		
		err := processor.Unsubscribe(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gossipsub reference not set")

		// Test with mock gossipsub
		mockGossipsub := &mockGossipsub{
			subscribedTopics: map[string]bool{
				processor.Topic(): true,
			},
		}
		
		processor.Gossipsub = mockGossipsub
		err = processor.Unsubscribe(context.Background())
		assert.NoError(t, err)
		assert.False(t, mockGossipsub.subscribedTopics[processor.Topic()])
	})

	t.Run("Decode", func(t *testing.T) {
		processor := &eth.SyncContributionProcessor{
			ForkDigest: forkDigest,
			Encoder:    encoder,
		}

		// Create test data
		contrib := &pb.SignedContributionAndProof{
			Message: &pb.ContributionAndProof{
				AggregatorIndex: 100,
				Contribution: &pb.SyncCommitteeContribution{
					Slot:            12345,
					BlockRoot:       []byte("test-block-root"),
					SubcommitteeIndex: 3,
				},
			},
			Signature: []byte("test-signature"),
		}

		// Encode
		var buf bytes.Buffer
		_, err = encoder.EncodeGossip(&buf, contrib)
		require.NoError(t, err)
		encoded := buf.Bytes()

		// Decode
		decoded, err := processor.Decode(context.Background(), encoded)
		assert.NoError(t, err)
		assert.Equal(t, contrib.Message.AggregatorIndex, decoded.Message.AggregatorIndex)
		assert.Equal(t, contrib.Message.Contribution.Slot, decoded.Message.Contribution.Slot)
		assert.Equal(t, contrib.Message.Contribution.SubcommitteeIndex, decoded.Message.Contribution.SubcommitteeIndex)

		// Test decode error
		_, err = processor.Decode(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode sync contribution and proof")
	})

	t.Run("Validate", func(t *testing.T) {
		contrib := &pb.SignedContributionAndProof{
			Message: &pb.ContributionAndProof{
				AggregatorIndex: 100,
				Contribution: &pb.SyncCommitteeContribution{
					Slot: 12345,
				},
			},
		}

		// Test without validator
		processor := &eth.SyncContributionProcessor{
			ForkDigest: forkDigest,
			Encoder:    encoder,
		}
		
		result, err := processor.Validate(context.Background(), contrib, "peer123")
		assert.NoError(t, err)
		assert.Equal(t, pubsub.ValidationAccept, result)

		// Test with validator that accepts
		processor.Validator = func(ctx context.Context, c *pb.SignedContributionAndProof) (pubsub.ValidationResult, error) {
			assert.Equal(t, contrib.Message.AggregatorIndex, c.Message.AggregatorIndex)
			return pubsub.ValidationAccept, nil
		}
		
		result, err = processor.Validate(context.Background(), contrib, "peer123")
		assert.NoError(t, err)
		assert.Equal(t, pubsub.ValidationAccept, result)

		// Test with validator that rejects
		processor.Validator = func(ctx context.Context, c *pb.SignedContributionAndProof) (pubsub.ValidationResult, error) {
			return pubsub.ValidationReject, nil
		}
		
		result, err = processor.Validate(context.Background(), contrib, "peer123")
		assert.NoError(t, err)
		assert.Equal(t, pubsub.ValidationReject, result)
	})

	t.Run("Process", func(t *testing.T) {
		contrib := &pb.SignedContributionAndProof{
			Message: &pb.ContributionAndProof{
				AggregatorIndex: 100,
				Contribution: &pb.SyncCommitteeContribution{
					Slot: 12345,
				},
			},
		}

		// Test without handler
		processor := &eth.SyncContributionProcessor{
			ForkDigest: forkDigest,
			Encoder:    encoder,
			Log:        logger,
		}
		
		err := processor.Process(context.Background(), contrib, "12D3KooWCRscMgHgEo3ojm8ovzheydpvTEqsDtq7Vby38cMHrYjt")
		assert.NoError(t, err)

		// Test with handler
		handlerCalled := false
		processor.Handler = func(ctx context.Context, c *pb.SignedContributionAndProof, from peer.ID) error {
			handlerCalled = true
			assert.Equal(t, contrib.Message.AggregatorIndex, c.Message.AggregatorIndex)
			return nil
		}
		
		err = processor.Process(context.Background(), contrib, "12D3KooWCRscMgHgEo3ojm8ovzheydpvTEqsDtq7Vby38cMHrYjt")
		assert.NoError(t, err)
		assert.True(t, handlerCalled)

		// Test with invalid peer ID
		err = processor.Process(context.Background(), contrib, "invalid-peer-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid peer ID")
	})

	t.Run("GetTopicScoreParams", func(t *testing.T) {
		processor := &eth.SyncContributionProcessor{
			ForkDigest: forkDigest,
		}
		
		params := processor.GetTopicScoreParams()
		assert.Nil(t, params)
	})
}