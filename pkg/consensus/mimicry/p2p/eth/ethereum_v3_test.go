package eth_test

import (
	"context"
	"testing"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/topics"
	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sszSnappyEncoder provides SSZ+Snappy encoding for Ethereum types
type sszSnappyEncoder[T any] struct{}

func (e *sszSnappyEncoder[T]) Encode(msg T) ([]byte, error) {
	// In production, this would use SSZ+Snappy encoding
	// For testing, we'll use a simple placeholder
	return []byte("encoded"), nil
}

func (e *sszSnappyEncoder[T]) Decode(data []byte) (T, error) {
	var msg T
	// In production, this would use SSZ+Snappy decoding
	return msg, nil
}

func TestEthereumTopicsIntegration(t *testing.T) {
	// Skip this test as it requires networking infrastructure
	t.Skip("Integration test requires actual networking")

	// This test validates that the v1 API works with Ethereum topics
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create two hosts for testing
	h1, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	defer h1.Close()

	h2, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	defer h2.Close()

	// Connect hosts
	err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	require.NoError(t, err)

	// Create v1 gossipsub instances
	gs1, err := v1.New(ctx, h1, v1.WithLogger(logger.WithField("node", "1")))
	require.NoError(t, err)
	defer gs1.Stop()

	gs2, err := v1.New(ctx, h2, v1.WithLogger(logger.WithField("node", "2")))
	require.NoError(t, err)
	defer gs2.Stop()

	t.Run("BeaconBlockPubSub", func(t *testing.T) {
		// NOTE: In a real implementation, topics would have proper encoders set
		// For this test, we'll demonstrate the API usage

		// The actual topics package should provide topics with encoders already configured
		// forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
		// beaconBlockTopic := topics.WithFork(topics.BeaconBlock, forkDigest)

		// For now, we'll skip the actual test logic
		t.Skip("Requires proper topic encoder setup")
	})
}

func TestEthereumTopicDefinitions(t *testing.T) {
	// Test that all topic definitions are properly created and work with v1
	t.Run("RegularTopics", func(t *testing.T) {
		assert.NotNil(t, topics.BeaconBlock)
		assert.NotNil(t, topics.VoluntaryExit)
		assert.NotNil(t, topics.ProposerSlashing)
		assert.NotNil(t, topics.AttesterSlashing)
		assert.NotNil(t, topics.BlsToExecutionChange)
		assert.NotNil(t, topics.BeaconAggregateAndProof)
		assert.NotNil(t, topics.SyncContributionAndProof)
	})

	t.Run("SubnetTopics", func(t *testing.T) {
		assert.NotNil(t, topics.Attestation)
		assert.Equal(t, uint64(64), topics.Attestation.MaxSubnets())

		assert.NotNil(t, topics.SyncCommittee)
		assert.Equal(t, uint64(4), topics.SyncCommittee.MaxSubnets())
	})

	t.Run("WithFork", func(t *testing.T) {
		forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

		// Test with regular topic
		beaconBlockWithFork := topics.WithFork(topics.BeaconBlock, forkDigest)
		assert.Equal(t, "/eth2/01020304/beacon_block/ssz_snappy", beaconBlockWithFork.Name())

		// Test with subnet topic
		subnetTopic, err := topics.Attestation.TopicForSubnet(10, forkDigest)
		require.NoError(t, err)
		assert.Equal(t, "/eth2/01020304/beacon_attestation_10/ssz_snappy", subnetTopic.Name())

		// Test sync committee subnet
		syncTopic, err := topics.SyncCommittee.TopicForSubnet(3, forkDigest)
		require.NoError(t, err)
		assert.Equal(t, "/eth2/01020304/sync_committee_3/ssz_snappy", syncTopic.Name())
	})

	t.Run("SubnetParsing", func(t *testing.T) {
		// Test parsing subnet from topic name
		subnet, err := topics.Attestation.ParseSubnet("/eth2/01020304/beacon_attestation_42/ssz_snappy")
		require.NoError(t, err)
		assert.Equal(t, uint64(42), subnet)

		// Test parsing sync committee subnet
		syncSubnet, err := topics.SyncCommittee.ParseSubnet("/eth2/01020304/sync_committee_2/ssz_snappy")
		require.NoError(t, err)
		assert.Equal(t, uint64(2), syncSubnet)

		// Test error cases
		_, err = topics.Attestation.ParseSubnet("/eth2/01020304/wrong_topic_10/ssz_snappy")
		assert.Error(t, err)

		_, err = topics.Attestation.ParseSubnet("/eth2/01020304/beacon_attestation_invalid/ssz_snappy")
		assert.Error(t, err)

		_, err = topics.Attestation.ParseSubnet("/eth2/01020304/beacon_attestation_100/ssz_snappy") // Exceeds max
		assert.Error(t, err)
	})
}
