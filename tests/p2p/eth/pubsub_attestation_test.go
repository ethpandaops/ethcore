package eth_test

import (
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttestationProcessorTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &eth.AttestationProcessor{
		ForkDigest: forkDigest,
		Subnets: []uint64{10, 20, 30},
	}

	topics := processor.Topics()
	assert.Len(t, topics, 3)
	assert.Equal(t, eth.AttestationSubnetTopic(forkDigest, 10), topics[0])
	assert.Equal(t, eth.AttestationSubnetTopic(forkDigest, 20), topics[1])
	assert.Equal(t, eth.AttestationSubnetTopic(forkDigest, 30), topics[2])
}

func TestAttestationProcessorAllPossibleTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &eth.AttestationProcessor{
		ForkDigest: forkDigest,
	}

	topics := processor.AllPossibleTopics()
	assert.Len(t, topics, eth.AttestationSubnetCount)

	// Verify all topics are unique and properly formatted
	seen := make(map[string]bool)
	for i, topic := range topics {
		assert.Equal(t, eth.AttestationSubnetTopic(forkDigest, uint64(i)), topic)
		assert.False(t, seen[topic])
		seen[topic] = true
	}
}

func TestAttestationProcessorGetTopicScoreParams(t *testing.T) {
	processor := &eth.AttestationProcessor{}
	// eth.AttestationProcessor always returns nil for score params
	params := processor.GetTopicScoreParams("any_topic")
	assert.Nil(t, params)
}

func TestAttestationProcessorDecode(t *testing.T) {
	processor := &eth.AttestationProcessor{
		Encoder: encoder.SszNetworkEncoder{},
		Log:     logrus.New(),
	}

	// Create a test attestation
	attestation := &pb.Attestation{
		AggregationBits: []byte{0xff},
		Data: &pb.AttestationData{
			Slot:            primitives.Slot(100),
			CommitteeIndex:  primitives.CommitteeIndex(2),
			BeaconBlockRoot: make([]byte, 32),
			Source: &pb.Checkpoint{
				Epoch: primitives.Epoch(10),
				Root:  make([]byte, 32),
			},
			Target: &pb.Checkpoint{
				Epoch: primitives.Epoch(11),
				Root:  make([]byte, 32),
			},
		},
		Signature: make([]byte, 96),
	}

	// Encode the attestation
	var buf bytes.Buffer
	_, err := processor.Encoder.EncodeGossip(&buf, attestation)
	require.NoError(t, err)
	encoded := buf.Bytes()

	// Test successful decode
	decoded, err := processor.Decode(context.Background(), "beacon_attestation_10", encoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded)
	assert.Equal(t, primitives.Slot(100), decoded.Data.Slot)

	// Test decode error with invalid data
	_, err = processor.Decode(context.Background(), "beacon_attestation_10", []byte("invalid"))
	assert.Error(t, err)
}

func TestAttestationProcessorValidate(t *testing.T) {
	tests := []struct {
		name           string
		setupValidator func() func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error)
		subnets        []uint64
		topic          string
		expectedResult pubsub.ValidationResult
		expectError    bool
	}{
		{
			name: "successful validation",
			setupValidator: func() func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, att *pb.Attestation, subnet uint64) (pubsub.ValidationResult, error) {
					assert.Equal(t, uint64(10), subnet)
					return pubsub.ValidationAccept, nil
				}
			},
			subnets: []uint64{10, 20},
			topic:          "/eth2/01020304/beacon_attestation_10/ssz_snappy",
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
		{
			name: "validation reject",
			setupValidator: func() func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, att *pb.Attestation, subnet uint64) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, nil
				}
			},
			subnets: []uint64{10},
			topic:          "/eth2/01020304/beacon_attestation_10/ssz_snappy",
			expectedResult: pubsub.ValidationReject,
			expectError:    false,
		},
		{
			name: "validation error",
			setupValidator: func() func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, att *pb.Attestation, subnet uint64) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, errors.New("validation failed")
				}
			},
			subnets: []uint64{10},
			topic:          "/eth2/01020304/beacon_attestation_10/ssz_snappy",
			expectedResult: pubsub.ValidationReject,
			expectError:    true,
		},
		{
			name:           "no validator",
			setupValidator: func() func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error) { return nil },
			subnets: []uint64{10},
			topic:          "/eth2/01020304/beacon_attestation_10/ssz_snappy",
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
		{
			name: "invalid topic",
			setupValidator: func() func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, att *pb.Attestation, subnet uint64) (pubsub.ValidationResult, error) {
					return pubsub.ValidationAccept, nil
				}
			},
			subnets: []uint64{10},
			topic:          "/eth2/01020304/beacon_attestation_999/ssz_snappy",
			expectedResult: pubsub.ValidationReject,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &eth.AttestationProcessor{
				Validator:  tt.setupValidator(),
				Log:        logrus.New(),
				Subnets:    tt.subnets,
				ForkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
			}

			attestation := &pb.Attestation{
				AggregationBits: []byte{0xff},
				Data: &pb.AttestationData{
					Slot:            primitives.Slot(100),
					CommitteeIndex:  primitives.CommitteeIndex(2),
					BeaconBlockRoot: make([]byte, 32),
					Source: &pb.Checkpoint{
						Epoch: primitives.Epoch(10),
						Root:  make([]byte, 32),
					},
					Target: &pb.Checkpoint{
						Epoch: primitives.Epoch(11),
						Root:  make([]byte, 32),
					},
				},
				Signature: make([]byte, 96),
			}

			result, err := processor.Validate(context.Background(), tt.topic, attestation, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestAttestationProcessorProcess(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func() func(context.Context, *pb.Attestation, uint64, peer.ID) error
		subnets      []uint64
		topic        string
		expectError  bool
	}{
		{
			name: "successful processing",
			setupHandler: func() func(context.Context, *pb.Attestation, uint64, peer.ID) error {
				return func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error {
					assert.Equal(t, uint64(20), subnet)
					return nil
				}
			},
			subnets: []uint64{10, 20, 30},
			topic:       "/eth2/01020304/beacon_attestation_20/ssz_snappy",
			expectError: false,
		},
		{
			name: "processing error",
			setupHandler: func() func(context.Context, *pb.Attestation, uint64, peer.ID) error {
				return func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error {
					return errors.New("processing failed")
				}
			},
			subnets: []uint64{10},
			topic:       "/eth2/01020304/beacon_attestation_10/ssz_snappy",
			expectError: true,
		},
		{
			name:         "no handler",
			setupHandler: func() func(context.Context, *pb.Attestation, uint64, peer.ID) error { return nil },
			subnets: []uint64{10},
			topic:        "/eth2/01020304/beacon_attestation_10/ssz_snappy",
			expectError:  false,
		},
		{
			name: "invalid topic",
			setupHandler: func() func(context.Context, *pb.Attestation, uint64, peer.ID) error {
				return func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error {
					return nil
				}
			},
			subnets: []uint64{10},
			topic:       "/eth2/01020304/beacon_attestation_999/ssz_snappy",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &eth.AttestationProcessor{
				Handler:    tt.setupHandler(),
				Log:        logrus.New(),
				Subnets:    tt.subnets,
				ForkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
			}

			attestation := &pb.Attestation{
				AggregationBits: []byte{0xff},
				Data: &pb.AttestationData{
					Slot:            primitives.Slot(100),
					CommitteeIndex:  primitives.CommitteeIndex(2),
					BeaconBlockRoot: make([]byte, 32),
					Source: &pb.Checkpoint{
						Epoch: primitives.Epoch(10),
						Root:  make([]byte, 32),
					},
					Target: &pb.Checkpoint{
						Epoch: primitives.Epoch(11),
						Root:  make([]byte, 32),
					},
				},
				Signature: make([]byte, 96),
			}

			err := processor.Process(context.Background(), tt.topic, attestation, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAttestationProcessorSubscribeUnsubscribe(t *testing.T) {
	mockGS := &pubsub.Gossipsub{}
	processor := &eth.AttestationProcessor{
		Gossipsub:  mockGS,
		ForkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
		Log:        logrus.New(),
		Subnets: []uint64{},
	}

	// Test subscribe with subnets
	err := processor.Subscribe(context.Background(), []uint64{1, 2, 3})
	assert.Error(t, err) // Will error because gossipsub.SubscribeToMultiProcessorTopics is not implemented

	// Test with no gossipsub
	processor.Gossipsub = nil
	err = processor.Subscribe(context.Background(), []uint64{1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gossipsub reference not set")

	// Restore gossipsub for unsubscribe test
	processor.Gossipsub = mockGS
	processor.Subnets = []uint64{1, 2, 3}

	// Test unsubscribe
	err = processor.Unsubscribe(context.Background(), []uint64{1, 2})
	assert.NoError(t, err) // Unsubscribe returns nil even if gossipsub.Unsubscribe fails (it just logs errors)

	// Test unsubscribe with no gossipsub
	processor.Gossipsub = nil
	err = processor.Unsubscribe(context.Background(), []uint64{1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gossipsub reference not set")
}

func TestAttestationProcessorTopicIndex(t *testing.T) {
	processor := &eth.AttestationProcessor{
		ForkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
		Subnets: []uint64{10, 20, 30},
		Log:        logrus.New(),
	}

	tests := []struct {
		name        string
		topic       string
		expectedIdx int
		expectError bool
	}{
		{
			name:        "valid subnet 10",
			topic:       "/eth2/01020304/beacon_attestation_10/ssz_snappy",
			expectedIdx: 0,
			expectError: false,
		},
		{
			name:        "valid subnet 20",
			topic:       "/eth2/01020304/beacon_attestation_20/ssz_snappy",
			expectedIdx: 1,
			expectError: false,
		},
		{
			name:        "valid subnet 30",
			topic:       "/eth2/01020304/beacon_attestation_30/ssz_snappy",
			expectedIdx: 2,
			expectError: false,
		},
		{
			name:        "subnet not in list",
			topic:       "/eth2/01020304/beacon_attestation_40/ssz_snappy",
			expectedIdx: -1,
			expectError: true,
		},
		{
			name:        "invalid topic format",
			topic:       "/eth2/01020304/invalid_topic/ssz_snappy",
			expectedIdx: -1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := processor.TopicIndex(tt.topic)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedIdx, idx)
		})
	}
}

func TestAttestationProcessorGetActiveSubnets(t *testing.T) {
	processor := &eth.AttestationProcessor{
		Subnets: []uint64{10, 20, 30},
	}

	active := processor.GetActiveSubnets()
	assert.Equal(t, []uint64{10, 20, 30}, active)

	// Verify it's a copy
	active[0] = 999
	assert.Equal(t, uint64(10), processor.Subnets[0])
}

func TestAttestationSubnetConstants(t *testing.T) {
	assert.Equal(t, 64, eth.AttestationSubnetCount)
	assert.Equal(t, "beacon_attestation_%d", eth.AttestationSubnetTopicTemplate)
}
