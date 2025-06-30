package eth_test

import (
	"testing"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eth "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
)

// TestAttestationProcessorDynamicTopics demonstrates dynamic topic subscription changes for attestations.
func TestAttestationProcessorDynamicTopics(t *testing.T) {
	// Create a real DefaultAttestationProcessor
	processor := &eth.DefaultAttestationProcessor{
		ForkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
		Encoder:    encoder.SszNetworkEncoder{},
		Subnets:    []uint64{0}, // start with subnet 0
	}

	// Test initial active topics
	active := processor.ActiveTopics()
	assert.Len(t, active, 1)
	assert.Contains(t, active[0], "beacon_attestation_0")

	// Test switching from subnet 0 to subnet 45
	forkDigest := processor.ForkDigest
	topic45 := eth.AttestationSubnetTopic(forkDigest, 45)
	newTopics := []string{topic45}

	toSubscribe, toUnsubscribe, err := processor.UpdateActiveTopics(newTopics)
	require.NoError(t, err)

	// Should unsubscribe from subnet 0 and subscribe to subnet 45
	assert.Len(t, toSubscribe, 1)
	assert.Contains(t, toSubscribe[0], "beacon_attestation_45")
	assert.Len(t, toUnsubscribe, 1)
	assert.Contains(t, toUnsubscribe[0], "beacon_attestation_0")

	// Verify the processor's state was updated
	active = processor.ActiveTopics()
	assert.Len(t, active, 1)
	assert.Contains(t, active[0], "beacon_attestation_45")
	assert.Equal(t, []uint64{45}, processor.Subnets)

	// Test switching to multiple subnets
	topic1 := eth.AttestationSubnetTopic(forkDigest, 1)
	topic2 := eth.AttestationSubnetTopic(forkDigest, 2)
	newTopics = []string{topic45, topic1, topic2}

	toSubscribe, toUnsubscribe, err = processor.UpdateActiveTopics(newTopics)
	require.NoError(t, err)

	// Should subscribe to subnets 1 and 2, no unsubscriptions (subnet 45 stays)
	assert.Len(t, toSubscribe, 2)
	assert.Empty(t, toUnsubscribe)

	// Verify final state
	active = processor.ActiveTopics()
	assert.Len(t, active, 3)
	assert.ElementsMatch(t, []uint64{45, 1, 2}, processor.Subnets)
}

// TestAttestationProcessorErrorHandling tests error conditions for dynamic topic updates.
func TestAttestationProcessorErrorHandling(t *testing.T) {
	processor := &eth.DefaultAttestationProcessor{
		ForkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
		Encoder:    encoder.SszNetworkEncoder{},
		Subnets:    []uint64{0},
	}

	// Test invalid topic format
	_, _, err := processor.UpdateActiveTopics([]string{"invalid_topic_format"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid attestation topic format")

	// Test subnet exceeding maximum
	forkDigest := processor.ForkDigest
	topic64 := eth.AttestationSubnetTopic(forkDigest, 64) // subnet 64 is invalid (max is 63)
	_, _, err = processor.UpdateActiveTopics([]string{topic64})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum subnet count")

	// Verify original state unchanged after errors
	assert.Equal(t, []uint64{0}, processor.Subnets)
}
