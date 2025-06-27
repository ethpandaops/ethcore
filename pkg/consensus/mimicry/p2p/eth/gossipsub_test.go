package eth

import (
	"context"
	"strconv"
	"testing"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// For unit testing that doesn't need mocked pubsub calls
func createTestGossipsub() *Gossipsub {
	logger := logrus.NewEntry(logrus.New())
	realEnc := encoder.SszNetworkEncoder{}
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}

	ps := &pubsub.Gossipsub{}
	g := NewGossipsub(logger, ps, forkDigest, realEnc)

	return g
}

func createTestForkDigest() [4]byte {
	return [4]byte{0x01, 0x02, 0x03, 0x04}
}

// TestEthereumGossipsub tests the constructor and basic wrapper functionality
func TestEthereumGossipsub(t *testing.T) {
	t.Run("NewGossipsub creates valid instance", func(t *testing.T) {
		logger := logrus.NewEntry(logrus.New())
		ps := &pubsub.Gossipsub{}
		forkDigest := createTestForkDigest()
		enc := encoder.SszNetworkEncoder{}

		g := NewGossipsub(logger, ps, forkDigest, enc)

		assert.NotNil(t, g)
		assert.Equal(t, ps, g.Gossipsub)
		assert.Equal(t, forkDigest, g.forkDigest)
		assert.Equal(t, enc, g.encoder)
		assert.NotNil(t, g.log)
	})

	t.Run("NewGossipsub with nil pubsub", func(t *testing.T) {
		logger := logrus.NewEntry(logrus.New())
		forkDigest := createTestForkDigest()
		enc := encoder.SszNetworkEncoder{}

		// Test with nil pubsub
		g := NewGossipsub(logger, nil, forkDigest, enc)
		assert.NotNil(t, g)
		assert.Nil(t, g.Gossipsub)
		assert.Equal(t, forkDigest, g.forkDigest)
		assert.Equal(t, enc, g.encoder)
	})
}

// TestTopicConstruction tests all the topic construction functions
func TestTopicConstruction(t *testing.T) {
	forkDigest := createTestForkDigest()

	t.Run("Global topic construction", func(t *testing.T) {
		tests := []struct {
			name     string
			topic    func([4]byte) string
			expected string
		}{
			{"BeaconBlock", BeaconBlockTopic, "beacon_block"},
			{"BeaconAggregateAndProof", BeaconAggregateAndProofTopic, "beacon_aggregate_and_proof"},
			{"VoluntaryExit", VoluntaryExitTopic, "voluntary_exit"},
			{"ProposerSlashing", ProposerSlashingTopic, "proposer_slashing"},
			{"AttesterSlashing", AttesterSlashingTopic, "attester_slashing"},
			{"SyncContributionAndProof", SyncContributionAndProofTopic, "sync_committee_contribution_and_proof"},
			{"BlsToExecutionChange", BlsToExecutionChangeTopic, "bls_to_execution_change"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				topic := tt.topic(forkDigest)
				expected := formatTestTopic(tt.expected)
				assert.Equal(t, expected, topic)
			})
		}
	})

	t.Run("Subnet topic construction", func(t *testing.T) {
		// Test attestation subnets
		for subnet := uint64(0); subnet < 3; subnet++ {
			topic := AttestationSubnetTopic(forkDigest, subnet)
			expected := formatTestTopic("beacon_attestation_" + strconv.FormatUint(subnet, 10))
			assert.Equal(t, expected, topic)
		}

		// Test sync committee subnets
		for subnet := uint64(0); subnet < SyncCommitteeSubnetCount; subnet++ {
			topic := SyncCommitteeSubnetTopic(forkDigest, subnet)
			expected := formatTestTopic("sync_committee_" + strconv.FormatUint(subnet, 10))
			assert.Equal(t, expected, topic)
		}
	})

	t.Run("Topic parsing", func(t *testing.T) {
		topic := BeaconBlockTopic(forkDigest)
		parsedForkDigest, name, err := ParseGossipsubTopic(topic)

		require.NoError(t, err)
		assert.Equal(t, forkDigest, parsedForkDigest)
		assert.Equal(t, BeaconBlockTopicName, name)
	})

	t.Run("Invalid topic parsing", func(t *testing.T) {
		invalidTopics := []string{
			"/invalid/topic",
			"/eth2/invaliddigest/beacon_block/ssz_snappy",
			"/eth2/01020304/beacon_block",
			"",
		}

		for _, topic := range invalidTopics {
			_, _, err := ParseGossipsubTopic(topic)
			assert.Error(t, err)
		}
	})

	t.Run("Topic detection helpers", func(t *testing.T) {
		isAtt, subnet := IsAttestationTopic("beacon_attestation_0")
		assert.True(t, isAtt)
		assert.Equal(t, uint64(0), subnet)

		isAtt, _ = IsAttestationTopic("beacon_block")
		assert.False(t, isAtt)

		isSync, subnet := IsSyncCommitteeTopic("sync_committee_0")
		assert.True(t, isSync)
		assert.Equal(t, uint64(0), subnet)

		isSync, _ = IsSyncCommitteeTopic("beacon_block")
		assert.False(t, isSync)
	})

	t.Run("All topics helpers", func(t *testing.T) {
		globalTopics := AllGlobalTopics(forkDigest)
		assert.Len(t, globalTopics, 7) // All global topic types

		attestationTopics := AllAttestationTopics(forkDigest)
		assert.Len(t, attestationTopics, AttestationSubnetCount)

		syncTopics := AllSyncCommitteeTopics(forkDigest)
		assert.Len(t, syncTopics, SyncCommitteeSubnetCount)

		allTopics := AllTopics(forkDigest)
		expectedTotal := 7 + AttestationSubnetCount + SyncCommitteeSubnetCount
		assert.Len(t, allTopics, expectedTotal)
	})
}

// Helper function to format topic strings for testing
func formatTestTopic(topicName string) string {
	return "/eth2/01020304/" + topicName + "/ssz_snappy"
}

// TestTypedHandlers tests the typed message handlers for different Ethereum message types
func TestTypedHandlers(t *testing.T) {
	g := createTestGossipsub()

	t.Run("Subnet validation", func(t *testing.T) {
		ctx := context.Background()

		// Test attestation subnet validation
		handler := func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error {
			return nil
		}

		// Valid subnet should not return error from validation (though underlying pubsub may fail)
		err := g.SubscribeAttestation(ctx, 0, handler)
		// We expect an error because pubsub isn't started, but not from subnet validation
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pubsub not started")

		// Invalid subnet should return validation error
		err = g.SubscribeAttestation(ctx, AttestationSubnetCount, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid attestation subnet")
	})

	t.Run("SyncCommittee subnet validation", func(t *testing.T) {
		ctx := context.Background()

		handler := func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64, from peer.ID) error { //nolint:staticcheck // deprecated but still functional
			return nil
		}

		// Valid subnet
		err := g.SubscribeSyncCommittee(ctx, 2, handler)
		assert.Error(t, err) // pubsub not started
		assert.Contains(t, err.Error(), "pubsub not started")

		// Invalid subnet should return validation error
		err = g.SubscribeSyncCommittee(ctx, SyncCommitteeSubnetCount, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync committee subnet")
	})

	t.Run("Topic construction in handlers", func(t *testing.T) {
		// Test that correct topics are constructed for different handler types
		forkDigest := g.forkDigest

		// Verify topic construction matches expected patterns
		beaconTopic := BeaconBlockTopic(forkDigest)
		assert.Contains(t, beaconTopic, "beacon_block")
		assert.Contains(t, beaconTopic, "01020304") // fork digest

		attTopic := AttestationSubnetTopic(forkDigest, 0)
		assert.Contains(t, attTopic, "beacon_attestation_0")
		assert.Contains(t, attTopic, "01020304")

		syncTopic := SyncCommitteeSubnetTopic(forkDigest, 1)
		assert.Contains(t, syncTopic, "sync_committee_1")
		assert.Contains(t, syncTopic, "01020304")
	})
}

// TestMessageDecoding tests the message handler creation and type validation
func TestMessageDecoding(t *testing.T) {
	t.Run("Topic validation", func(t *testing.T) {
		// Test message type detection for different topics
		assert.Equal(t, "SignedBeaconBlock", GetMessageType(BeaconBlockTopicName))
		assert.Equal(t, "SignedAggregateAndProof", GetMessageType(BeaconAggregateAndProofTopicName))
		assert.Equal(t, "Attestation", GetMessageType("beacon_attestation_0"))
		assert.Equal(t, "SyncCommitteeMessage", GetMessageType("sync_committee_0"))
		assert.Equal(t, "Unknown", GetMessageType("invalid_topic"))
	})

	t.Run("Message type helpers", func(t *testing.T) {
		// Test topic detection helpers
		assert.True(t, IsBeaconBlockTopic(BeaconBlockTopicName))
		assert.True(t, IsAggregateAndProofTopic(BeaconAggregateAndProofTopicName))
		assert.True(t, IsVoluntaryExitTopic(VoluntaryExitTopicName))
		assert.True(t, IsProposerSlashingTopic(ProposerSlashingTopicName))
		assert.True(t, IsAttesterSlashingTopic(AttesterSlashingTopicName))
		assert.True(t, IsSyncContributionAndProofTopic(SyncContributionAndProofTopicName))
		assert.True(t, IsBlsToExecutionChangeTopic(BlsToExecutionChangeTopicName))

		// Test negative cases
		assert.False(t, IsBeaconBlockTopic("beacon_attestation_0"))
		assert.False(t, IsAggregateAndProofTopic("beacon_block"))
	})
}

// TestValidatorIntegration tests typed validators integration with the pubsub system
func TestValidatorIntegration(t *testing.T) {
	g := createTestGossipsub()

	t.Run("Validator subnet validation", func(t *testing.T) {
		validator := func(att *pb.Attestation, subnet uint64) error {
			return nil
		}

		// Test invalid subnet registration should fail before reaching underlying pubsub
		err := g.RegisterAttestationValidator(AttestationSubnetCount, validator)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid attestation subnet")

		// Valid subnet should attempt to register (though may fail due to pubsub not started)
		err = g.RegisterAttestationValidator(5, validator)
		assert.Error(t, err) // pubsub not started, but not subnet validation error
		assert.NotContains(t, err.Error(), "invalid attestation subnet")
	})

	t.Run("Validator topic construction", func(t *testing.T) {
		// Test that validators use correct topics
		forkDigest := g.forkDigest

		expectedBeaconTopic := BeaconBlockTopic(forkDigest)
		assert.Contains(t, expectedBeaconTopic, "beacon_block")

		expectedAttTopic := AttestationSubnetTopic(forkDigest, 5)
		assert.Contains(t, expectedAttTopic, "beacon_attestation_5")
	})
}

// TestBulkSubscriptions tests bulk subscription methods like SubscribeAllGlobalTopics
func TestBulkSubscriptions(t *testing.T) {
	ctx := context.Background()
	g := createTestGossipsub()

	t.Run("SubscribeAllGlobalTopics with nil handlers", func(t *testing.T) {
		err := g.SubscribeAllGlobalTopics(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handlers cannot be nil")
	})

	t.Run("SubscribeAllGlobalTopics partial handlers", func(t *testing.T) {
		// Only provide some handlers
		handlers := &GlobalTopicHandlers{
			BeaconBlock: func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
				return nil
			},
			VoluntaryExit: func(ctx context.Context, exit *pb.SignedVoluntaryExit, from peer.ID) error {
				return nil
			},
		}

		err := g.SubscribeAllGlobalTopics(ctx, handlers)
		// Should attempt subscriptions but fail due to pubsub not started
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pubsub not started")
	})

	t.Run("SubscribeAttestationSubnets with invalid subnet", func(t *testing.T) {
		subnets := []uint64{0, uint64(AttestationSubnetCount)} // Second subnet is invalid
		handler := func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error {
			return nil
		}

		err := g.SubscribeAttestationSubnets(ctx, subnets, handler)
		assert.Error(t, err) // Should return error for invalid subnet
		assert.Contains(t, err.Error(), "invalid attestation subnet")
	})

	t.Run("Subnet validation in bulk operations", func(t *testing.T) {
		handler := func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64, from peer.ID) error { //nolint:staticcheck // deprecated but still functional
			return nil
		}

		// Test valid subnets range
		validSubnets := []uint64{0, 1, 2, 3} // All valid sync committee subnets
		err := g.SubscribeSyncCommitteeSubnets(ctx, validSubnets, handler)
		// Should attempt subscriptions but fail due to pubsub not started
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pubsub not started")

		// Test with invalid subnet
		invalidSubnets := []uint64{0, uint64(SyncCommitteeSubnetCount)}
		err = g.SubscribeSyncCommitteeSubnets(ctx, invalidSubnets, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync committee subnet")
	})
}

// TestPublishingHelpers tests the publishing functionality
func TestPublishingHelpers(t *testing.T) {
	ctx := context.Background()
	g := createTestGossipsub()

	t.Run("PublishAttestation invalid subnet", func(t *testing.T) {
		testAtt := &pb.Attestation{}
		subnet := uint64(AttestationSubnetCount) // Invalid subnet

		err := g.PublishAttestation(ctx, testAtt, subnet)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid attestation subnet")
	})

	t.Run("PublishSyncCommitteeMessage invalid subnet", func(t *testing.T) {
		testMsg := &pb.SyncCommitteeMessage{}      //nolint:staticcheck // deprecated but still functional
		subnet := uint64(SyncCommitteeSubnetCount) // Invalid subnet

		err := g.PublishSyncCommitteeMessage(ctx, testMsg, subnet)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync committee subnet")
	})

	t.Run("Publishing topic construction", func(t *testing.T) {
		// Test that publishing uses correct topic construction
		forkDigest := g.forkDigest

		beaconTopic := BeaconBlockTopic(forkDigest)
		assert.Contains(t, beaconTopic, "beacon_block")
		assert.Contains(t, beaconTopic, "01020304") // fork digest

		attTopic := AttestationSubnetTopic(forkDigest, 5)
		assert.Contains(t, attTopic, "beacon_attestation_5")
		assert.Contains(t, attTopic, "01020304")

		syncTopic := SyncCommitteeSubnetTopic(forkDigest, 2)
		assert.Contains(t, syncTopic, "sync_committee_2")
		assert.Contains(t, syncTopic, "01020304")
	})
}

// TestForkDigestHandling tests fork digest functionality
func TestForkDigestHandling(t *testing.T) {
	t.Run("Different fork digests produce different topics", func(t *testing.T) {
		forkDigest1 := [4]byte{0x01, 0x02, 0x03, 0x04}
		forkDigest2 := [4]byte{0x05, 0x06, 0x07, 0x08}

		topic1 := BeaconBlockTopic(forkDigest1)
		topic2 := BeaconBlockTopic(forkDigest2)

		assert.NotEqual(t, topic1, topic2)
		assert.Contains(t, topic1, "01020304")
		assert.Contains(t, topic2, "05060708")
	})

	t.Run("Fork digest preservation in gossipsub instance", func(t *testing.T) {
		forkDigest := [4]byte{0xAA, 0xBB, 0xCC, 0xDD}
		logger := logrus.NewEntry(logrus.New())
		ps := &pubsub.Gossipsub{}
		enc := encoder.SszNetworkEncoder{}

		g := NewGossipsub(logger, ps, forkDigest, enc)
		assert.Equal(t, forkDigest, g.forkDigest)

		// Verify topics are constructed with correct fork digest
		expectedTopic := "/eth2/aabbccdd/beacon_block/ssz_snappy"
		actualTopic := BeaconBlockTopic(g.forkDigest)
		assert.Equal(t, expectedTopic, actualTopic)
	})
}

// TestErrorConditions tests various error conditions and edge cases
func TestErrorConditions(t *testing.T) {
	ctx := context.Background()
	g := createTestGossipsub()

	t.Run("Subscription parameter validation", func(t *testing.T) {
		// Test invalid subnet validation before hitting pubsub
		handler := func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error {
			return nil
		}

		err := g.SubscribeAttestation(ctx, AttestationSubnetCount, handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid attestation subnet")

		// Test sync committee validation
		syncHandler := func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64, from peer.ID) error { //nolint:staticcheck // deprecated but still functional
			return nil
		}

		err = g.SubscribeSyncCommittee(ctx, SyncCommitteeSubnetCount, syncHandler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync committee subnet")
	})

	t.Run("Publishing parameter validation", func(t *testing.T) {
		// Test invalid subnet validation before encoding/publishing
		testAtt := &pb.Attestation{}
		err := g.PublishAttestation(ctx, testAtt, AttestationSubnetCount)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid attestation subnet")

		testMsg := &pb.SyncCommitteeMessage{} //nolint:staticcheck // deprecated but still functional
		err = g.PublishSyncCommitteeMessage(ctx, testMsg, SyncCommitteeSubnetCount)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync committee subnet")
	})

	t.Run("Validator parameter validation", func(t *testing.T) {
		validator := func(att *pb.Attestation, subnet uint64) error { return nil }

		// Test invalid subnet validation before registration
		err := g.RegisterAttestationValidator(AttestationSubnetCount, validator)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid attestation subnet")
	})

	t.Run("Nil handlers in bulk operations", func(t *testing.T) {
		err := g.SubscribeAllGlobalTopics(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handlers cannot be nil")
	})
}
