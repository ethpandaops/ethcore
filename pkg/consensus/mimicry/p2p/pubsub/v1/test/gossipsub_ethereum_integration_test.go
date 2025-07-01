package v1_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"

	primitives "github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	eth "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	fastssz "github.com/prysmaticlabs/fastssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Ethereum types for testing

// MockBeaconBlock represents a simplified beacon block
type MockBeaconBlock struct {
	Slot       uint64
	Root       [32]byte
	ParentRoot [32]byte
	StateRoot  [32]byte
	Body       []byte
}

// Implement SSZ marshaling
func (b *MockBeaconBlock) MarshalSSZ() ([]byte, error) {
	buf := make([]byte, 0, 132+len(b.Body))

	// Slot (8 bytes)
	slotBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(slotBytes, b.Slot)
	buf = append(buf, slotBytes...)

	// Roots (32 bytes each)
	buf = append(buf, b.Root[:]...)
	buf = append(buf, b.ParentRoot[:]...)
	buf = append(buf, b.StateRoot[:]...)

	// Body
	buf = append(buf, b.Body...)

	return buf, nil
}

func (b *MockBeaconBlock) UnmarshalSSZ(data []byte) error {
	if len(data) < 104 {
		return fmt.Errorf("insufficient data for beacon block")
	}

	b.Slot = binary.LittleEndian.Uint64(data[0:8])
	copy(b.Root[:], data[8:40])
	copy(b.ParentRoot[:], data[40:72])
	copy(b.StateRoot[:], data[72:104])

	if len(data) > 104 {
		b.Body = data[104:]
	}

	return nil
}

func (b *MockBeaconBlock) MarshalSSZTo(dst []byte) ([]byte, error) {
	marshaled, err := b.MarshalSSZ()
	if err != nil {
		return nil, err
	}
	return append(dst, marshaled...), nil
}

func (b *MockBeaconBlock) SizeSSZ() int {
	return 104 + len(b.Body)
}

// SSZ encoder for Ethereum types
type sszEncoder[T fastssz.Marshaler] struct{}

func (e *sszEncoder[T]) Encode(msg T) ([]byte, error) {
	return msg.MarshalSSZ()
}

func (e *sszEncoder[T]) Decode(data []byte) (T, error) {
	var msg T
	// Create a new instance
	msgPtr := new(T)
	if unmarshaler, ok := any(msgPtr).(fastssz.Unmarshaler); ok {
		if err := unmarshaler.UnmarshalSSZ(data); err != nil {
			return msg, err
		}
		return *msgPtr, nil
	}
	return msg, fmt.Errorf("type does not implement Unmarshaler")
}

// TestEthereum_BeaconBlockPropagation tests beacon block propagation
func TestEthereum_BeaconBlockPropagation(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h,
		v1.WithMaxMessageSize(10<<20), // 10MB for blocks
		v1.WithGossipSubParams(pubsub.GossipSubParams{
			D:                 8, // Higher degree for block topics
			Dlo:               6,
			Dhi:               12,
			HeartbeatInterval: 700 * time.Millisecond,
		}),
	)
	require.NoError(t, err)
	defer gs.Stop()

	// Create beacon block topic with SSZ encoding
	encoder := &sszEncoder[*MockBeaconBlock]{}
	forkDigest := [4]byte{0xab, 0xcd, 0xef, 0x01}

	baseTopic, err := v1.NewTopic[*MockBeaconBlock]("beacon_block", encoder)
	require.NoError(t, err)

	topic := baseTopic.WithForkDigest(forkDigest)

	// Track block propagation
	var (
		blocksValidated atomic.Int32
		blocksProcessed atomic.Int32
		blockErrors     atomic.Int32
		latestSlot      atomic.Uint64
	)

	handler := v1.NewHandlerConfig[*MockBeaconBlock](
		v1.WithSSZDecoding[*MockBeaconBlock](),
		v1.WithValidator(func(ctx context.Context, block *MockBeaconBlock, from peer.ID) v1.ValidationResult {
			blocksValidated.Add(1)

			// Basic validation
			if block.Slot == 0 {
				blockErrors.Add(1)
				return v1.ValidationReject
			}

			// Check if block is from the future
			currentSlot := latestSlot.Load()
			if block.Slot > currentSlot+1 {
				return v1.ValidationIgnore
			}

			// Simulate signature verification delay
			time.Sleep(10 * time.Millisecond)

			return v1.ValidationAccept
		}),
		v1.WithProcessor(func(ctx context.Context, block *MockBeaconBlock, from peer.ID) error {
			blocksProcessed.Add(1)

			// Update latest slot
			for {
				current := latestSlot.Load()
				if current >= block.Slot {
					break
				}
				if latestSlot.CompareAndSwap(current, block.Slot) {
					break
				}
			}

			return nil
		}),
		v1.WithScoreParams[*MockBeaconBlock](&pubsub.TopicScoreParams{
			TopicWeight:                     1.0,
			TimeInMeshWeight:                0.01,
			TimeInMeshQuantum:               time.Second,
			TimeInMeshCap:                   300.0,
			FirstMessageDeliveriesWeight:    1.0,
			FirstMessageDeliveriesDecay:     0.5,
			FirstMessageDeliveriesCap:       10.0,
			MeshMessageDeliveriesWeight:     -1.0,
			MeshMessageDeliveriesDecay:      0.5,
			MeshMessageDeliveriesCap:        10.0,
			MeshMessageDeliveriesThreshold:  5.0,
			MeshMessageDeliveriesWindow:     time.Minute,
			MeshMessageDeliveriesActivation: time.Minute,
			MeshFailurePenaltyWeight:        -1.0,
			MeshFailurePenaltyDecay:         0.5,
			InvalidMessageDeliveriesWeight:  -100.0,
			InvalidMessageDeliveriesDecay:   0.5,
		}),
	)

	err = registerTopic(gs, topic, handler)
	require.NoError(t, err)

	sub, err := subscribeTopic(gs, topic)
	require.NoError(t, err)
	defer sub.Cancel()

	// Simulate block production
	numBlocks := 10
	for i := uint64(1); i <= uint64(numBlocks); i++ {
		block := &MockBeaconBlock{
			Slot:       i,
			Root:       [32]byte{byte(i)},
			ParentRoot: [32]byte{byte(i - 1)},
			StateRoot:  [32]byte{byte(i), byte(i)},
			Body:       []byte(fmt.Sprintf("block body for slot %d", i)),
		}

		err := publishTopic(gs, topic, block)
		require.NoError(t, err)

		// Simulate block production rate
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify results
	assert.Equal(t, int32(numBlocks), blocksValidated.Load(), "All blocks should be validated")
	assert.Equal(t, int32(numBlocks), blocksProcessed.Load(), "All blocks should be processed")
	assert.Equal(t, int32(0), blockErrors.Load(), "No block errors expected")
	assert.Equal(t, uint64(numBlocks), latestSlot.Load(), "Latest slot should be updated")
}

// TestEthereum_AttestationSubnets tests attestation subnet handling
func TestEthereum_AttestationSubnets(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h,
		v1.WithValidationConcurrency(64), // Higher concurrency for attestations
	)
	require.NoError(t, err)
	defer gs.Stop()

	// Create attestation subnet topic
	attestationEncoder := &sszEncoder[*eth.Attestation]{}
	subnetTopic, err := v1.NewSubnetTopic[*eth.Attestation](
		"beacon_attestation_%d",
		64, // 64 attestation subnets
		attestationEncoder,
	)
	require.NoError(t, err)

	forkDigest := [4]byte{0xab, 0xcd, 0xef, 0x01}

	// Track attestations per subnet
	attestationCounts := make(map[uint64]*atomic.Int32)
	var countMu sync.RWMutex

	getCounter := func(subnet uint64) *atomic.Int32 {
		countMu.RLock()
		counter := attestationCounts[subnet]
		countMu.RUnlock()

		if counter != nil {
			return counter
		}

		countMu.Lock()
		defer countMu.Unlock()

		// Double-check after acquiring write lock
		if counter = attestationCounts[subnet]; counter == nil {
			counter = &atomic.Int32{}
			attestationCounts[subnet] = counter
		}
		return counter
	}

	// Register subnet handler
	handler := v1.NewHandlerConfig[*eth.Attestation](
		v1.WithSSZDecoding[*eth.Attestation](),
		v1.WithProcessor(func(ctx context.Context, att *eth.Attestation, from peer.ID) error {
			// In real scenario, we'd extract subnet from the attestation
			// For testing, we'll use committee index % 64
			subnet := uint64(att.Data.CommitteeIndex) % 64
			getCounter(subnet).Add(1)
			return nil
		}),
	)

	err = registerSubnetTopic(gs, subnetTopic, handler)
	require.NoError(t, err)

	// Subscribe to a subset of subnets (simulating validator duties)
	activeSubnets := []uint64{0, 5, 10, 15, 20, 25, 30, 35}
	subscriptions := make(map[uint64]*v1.Subscription)

	for _, subnet := range activeSubnets {
		sub, err := subscribeSubnetTopic(gs, subnetTopic, subnet, forkDigest)
		require.NoError(t, err)
		subscriptions[subnet] = sub
	}

	// Publish attestations to various subnets
	numAttestations := 100
	for i := 0; i < numAttestations; i++ {
		subnet := uint64(i % 64)

		attestation := &eth.Attestation{
			Data: &eth.AttestationData{
				Slot:            primitives.Slot(100 + i/64),
				CommitteeIndex:  primitives.CommitteeIndex(subnet),
				BeaconBlockRoot: []byte{byte(i)},
				Source: &eth.Checkpoint{
					Epoch: primitives.Epoch(1),
					Root:  []byte{1},
				},
				Target: &eth.Checkpoint{
					Epoch: primitives.Epoch(2),
					Root:  []byte{2},
				},
			},
			Signature: make([]byte, 96), // BLS signature
		}

		// Get the topic for this subnet
		topic, err := subnetTopic.TopicForSubnet(subnet, forkDigest)
		require.NoError(t, err)

		// Only publish if we're subscribed to this subnet
		if _, subscribed := subscriptions[subnet]; subscribed {
			err = publishTopic(gs, topic, attestation)
			require.NoError(t, err)
		}
	}

	// Wait for processing
	time.Sleep(1 * time.Second)

	// Verify we only received attestations for subscribed subnets
	countMu.RLock()
	defer countMu.RUnlock()

	for subnet, counter := range attestationCounts {
		count := counter.Load()
		_, subscribed := subscriptions[subnet]

		if subscribed {
			assert.Greater(t, count, int32(0),
				"Should have received attestations for subscribed subnet %d", subnet)
		} else {
			assert.Equal(t, int32(0), count,
				"Should not receive attestations for unsubscribed subnet %d", subnet)
		}
	}

	// Dynamic subnet subscription (simulating changing validator duties)
	t.Run("dynamic subscription", func(t *testing.T) {
		// Unsubscribe from some subnets
		for _, subnet := range activeSubnets[:4] {
			if sub, exists := subscriptions[subnet]; exists {
				sub.Cancel()
				delete(subscriptions, subnet)
			}
		}

		// Subscribe to new subnets
		newSubnets := []uint64{40, 45, 50, 55}
		for _, subnet := range newSubnets {
			sub, err := subscribeSubnetTopic(gs, subnetTopic, subnet, forkDigest)
			require.NoError(t, err)
			subscriptions[subnet] = sub
		}

		// Wait for mesh reconfiguration
		time.Sleep(1 * time.Second)

		// Reset counters
		attestationCounts = make(map[uint64]*atomic.Int32)

		// Publish more attestations
		for i := 0; i < 64; i++ {
			subnet := uint64(i)

			attestation := &eth.Attestation{
				Data: &eth.AttestationData{
					Slot:            primitives.Slot(200),
					CommitteeIndex:  primitives.CommitteeIndex(subnet),
					BeaconBlockRoot: []byte{byte(i)},
					Source: &eth.Checkpoint{
						Epoch: primitives.Epoch(3),
						Root:  []byte{3},
					},
					Target: &eth.Checkpoint{
						Epoch: primitives.Epoch(4),
						Root:  []byte{4},
					},
				},
				Signature: make([]byte, 96),
			}

			topic, err := subnetTopic.TopicForSubnet(subnet, forkDigest)
			require.NoError(t, err)

			if _, subscribed := subscriptions[subnet]; subscribed {
				err = publishTopic(gs, topic, attestation)
				require.NoError(t, err)
			}
		}

		// Wait and verify
		time.Sleep(500 * time.Millisecond)

		// Should receive on new subnets but not old ones
		for _, subnet := range newSubnets {
			counter := getCounter(subnet)
			assert.Greater(t, counter.Load(), int32(0),
				"Should receive attestations on newly subscribed subnet %d", subnet)
		}

		for _, subnet := range activeSubnets[:4] {
			counter := getCounter(subnet)
			assert.Equal(t, int32(0), counter.Load(),
				"Should not receive attestations on unsubscribed subnet %d", subnet)
		}
	})

	// Cleanup
	for _, sub := range subscriptions {
		sub.Cancel()
	}
}

// TestEthereum_SyncCommitteeSubnets tests sync committee subnet handling
func TestEthereum_SyncCommitteeSubnets(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h)
	require.NoError(t, err)
	defer gs.Stop()

	// Mock sync committee message
	type SyncCommitteeMessage struct {
		Slot            uint64
		BeaconBlockRoot [32]byte
		ValidatorIndex  uint64
		Signature       [96]byte
	}

	// Simple encoder for sync committee messages
	syncEncoder := &testEncoder[*SyncCommitteeMessage]{
		EncodeFn: func(msg *SyncCommitteeMessage) ([]byte, error) {
			buf := make([]byte, 0, 144)

			slotBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(slotBytes, msg.Slot)
			buf = append(buf, slotBytes...)

			buf = append(buf, msg.BeaconBlockRoot[:]...)

			indexBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(indexBytes, msg.ValidatorIndex)
			buf = append(buf, indexBytes...)

			buf = append(buf, msg.Signature[:]...)

			return buf, nil
		},
		DecodeFn: func(data []byte) (*SyncCommitteeMessage, error) {
			if len(data) < 144 {
				return nil, fmt.Errorf("insufficient data for sync committee message")
			}

			msg := &SyncCommitteeMessage{}
			msg.Slot = binary.LittleEndian.Uint64(data[0:8])
			copy(msg.BeaconBlockRoot[:], data[8:40])
			msg.ValidatorIndex = binary.LittleEndian.Uint64(data[40:48])
			copy(msg.Signature[:], data[48:144])

			return msg, nil
		},
	}

	// Create sync committee subnet topic (4 subnets as per spec)
	subnetTopic, err := v1.NewSubnetTopic[*SyncCommitteeMessage](
		"sync_committee_%d",
		4,
		syncEncoder,
	)
	require.NoError(t, err)

	forkDigest := [4]byte{0xab, 0xcd, 0xef, 0x01}

	// Track messages per subnet
	var messageCount sync.Map

	// Register handler
	handler := v1.NewHandlerConfig[*SyncCommitteeMessage](
		v1.WithProcessor(func(ctx context.Context, msg *SyncCommitteeMessage, from peer.ID) error {
			// Determine subnet based on validator index
			subnet := msg.ValidatorIndex % 4

			key := fmt.Sprintf("subnet_%d", subnet)
			val, _ := messageCount.LoadOrStore(key, &atomic.Int32{})
			val.(*atomic.Int32).Add(1)

			return nil
		}),
	)

	err = registerSubnetTopic(gs, subnetTopic, handler)
	require.NoError(t, err)

	// Subscribe to all sync committee subnets
	subscriptions := make([]*v1.Subscription, 4)
	for subnet := uint64(0); subnet < 4; subnet++ {
		sub, err := subscribeSubnetTopic(gs, subnetTopic, subnet, forkDigest)
		require.NoError(t, err)
		subscriptions[subnet] = sub
	}

	// Simulate sync committee messages
	numValidators := 100
	currentSlot := uint64(1000)

	for i := 0; i < numValidators; i++ {
		validatorIndex := uint64(i)
		subnet := validatorIndex % 4

		msg := &SyncCommitteeMessage{
			Slot:            currentSlot,
			BeaconBlockRoot: [32]byte{byte(currentSlot)},
			ValidatorIndex:  validatorIndex,
			Signature:       [96]byte{byte(i)},
		}

		topic, err := subnetTopic.TopicForSubnet(subnet, forkDigest)
		require.NoError(t, err)

		err = publishTopic(gs, topic, msg)
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify distribution across subnets
	totalMessages := 0
	for subnet := uint64(0); subnet < 4; subnet++ {
		key := fmt.Sprintf("subnet_%d", subnet)
		val, exists := messageCount.Load(key)
		assert.True(t, exists, "Should have messages for subnet %d", subnet)

		if exists {
			count := val.(*atomic.Int32).Load()
			totalMessages += int(count)

			// Should be roughly evenly distributed
			expectedPerSubnet := numValidators / 4
			assert.InDelta(t, expectedPerSubnet, count, 5,
				"Messages should be evenly distributed across subnets")
		}
	}

	assert.Equal(t, numValidators, totalMessages, "All messages should be received")

	// Cleanup
	for _, sub := range subscriptions {
		sub.Cancel()
	}
}

// testEncoder is a generic encoder implementation for testing
type testEncoder[T any] struct {
	EncodeFn func(T) ([]byte, error)
	DecodeFn func([]byte) (T, error)
}

func (e *testEncoder[T]) Encode(msg T) ([]byte, error) {
	return e.EncodeFn(msg)
}

func (e *testEncoder[T]) Decode(data []byte) (T, error) {
	return e.DecodeFn(data)
}

// ethereumTestEncoder is a generic encoder implementation for Ethereum integration tests
type ethereumTestEncoder[T any] struct {
	EncodeFn func(T) ([]byte, error)
	DecodeFn func([]byte) (T, error)
}

func (e *ethereumTestEncoder[T]) Encode(msg T) ([]byte, error) {
	return e.EncodeFn(msg)
}

func (e *ethereumTestEncoder[T]) Decode(data []byte) (T, error) {
	return e.DecodeFn(data)
}

// TestEthereum_SlasherTopics tests slashing-related topics
func TestEthereum_SlasherTopics(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	gs, err := v1.New(ctx, h)
	require.NoError(t, err)
	defer gs.Stop()

	forkDigest := [4]byte{0xab, 0xcd, 0xef, 0x01}

	t.Run("proposer slashing", func(t *testing.T) {
		// Mock proposer slashing
		type ProposerSlashing struct {
			ProposerIndex uint64
			Header1       []byte
			Header2       []byte
		}

		encoder := &ethereumTestEncoder[*ProposerSlashing]{
			EncodeFn: func(ps *ProposerSlashing) ([]byte, error) {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint64(buf, ps.ProposerIndex)
				buf = append(buf, ps.Header1...)
				buf = append(buf, ps.Header2...)
				return buf, nil
			},
			DecodeFn: func(data []byte) (*ProposerSlashing, error) {
				if len(data) < 8 {
					return nil, fmt.Errorf("insufficient data")
				}
				ps := &ProposerSlashing{
					ProposerIndex: binary.LittleEndian.Uint64(data[0:8]),
				}
				// Simplified decoding
				return ps, nil
			},
		}

		baseTopic, err := v1.NewTopic[*ProposerSlashing]("proposer_slashing", encoder)
		require.NoError(t, err)

		topic := baseTopic.WithForkDigest(forkDigest)

		slashingReceived := false
		handler := v1.NewHandlerConfig[*ProposerSlashing](
			v1.WithValidator(func(ctx context.Context, ps *ProposerSlashing, from peer.ID) v1.ValidationResult {
				// Validate slashing evidence
				if ps.ProposerIndex == 0 {
					return v1.ValidationReject
				}
				return v1.ValidationAccept
			}),
			v1.WithProcessor(func(ctx context.Context, ps *ProposerSlashing, from peer.ID) error {
				slashingReceived = true
				return nil
			}),
		)

		err = registerTopic(gs, topic, handler)
		require.NoError(t, err)

		sub, err := subscribeTopic(gs, topic)
		require.NoError(t, err)
		defer sub.Cancel()

		// Publish slashing
		slashing := &ProposerSlashing{
			ProposerIndex: 123,
			Header1:       []byte("header1"),
			Header2:       []byte("header2"),
		}

		err = publishTopic(gs, topic, slashing)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)
		assert.True(t, slashingReceived, "Proposer slashing should be received")
	})

	t.Run("attester slashing", func(t *testing.T) {
		// Mock attester slashing
		type AttesterSlashing struct {
			Attestation1 []byte
			Attestation2 []byte
		}

		encoder := &ethereumTestEncoder[*AttesterSlashing]{
			EncodeFn: func(as *AttesterSlashing) ([]byte, error) {
				lenBytes := make([]byte, 4)
				binary.LittleEndian.PutUint32(lenBytes, uint32(len(as.Attestation1)))

				buf := append(lenBytes, as.Attestation1...)
				buf = append(buf, as.Attestation2...)
				return buf, nil
			},
			DecodeFn: func(data []byte) (*AttesterSlashing, error) {
				if len(data) < 4 {
					return nil, fmt.Errorf("insufficient data")
				}
				// Simplified decoding
				return &AttesterSlashing{}, nil
			},
		}

		baseTopic, err := v1.NewTopic[*AttesterSlashing]("attester_slashing", encoder)
		require.NoError(t, err)

		topic := baseTopic.WithForkDigest(forkDigest)

		slashingCount := 0
		handler := v1.NewHandlerConfig[*AttesterSlashing](
			v1.WithProcessor(func(ctx context.Context, as *AttesterSlashing, from peer.ID) error {
				slashingCount++
				return nil
			}),
		)

		err = registerTopic(gs, topic, handler)
		require.NoError(t, err)

		sub, err := subscribeTopic(gs, topic)
		require.NoError(t, err)
		defer sub.Cancel()

		// Publish multiple slashings
		for i := 0; i < 3; i++ {
			slashing := &AttesterSlashing{
				Attestation1: []byte(fmt.Sprintf("att1-%d", i)),
				Attestation2: []byte(fmt.Sprintf("att2-%d", i)),
			}

			err = publishTopic(gs, topic, slashing)
			require.NoError(t, err)
		}

		time.Sleep(200 * time.Millisecond)
		assert.Equal(t, 3, slashingCount, "All attester slashings should be received")
	})
}
