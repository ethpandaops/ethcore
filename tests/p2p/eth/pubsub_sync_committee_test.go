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

func TestSyncCommitteeProcessorTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &eth.SyncCommitteeProcessor{
		ForkDigest: forkDigest,
		Subnets: []uint64{0, 1, 2},
	}

	topics := processor.Topics()
	assert.Len(t, topics, 3)
	assert.Equal(t, eth.SyncCommitteeSubnetTopic(forkDigest, 0), topics[0])
	assert.Equal(t, eth.SyncCommitteeSubnetTopic(forkDigest, 1), topics[1])
	assert.Equal(t, eth.SyncCommitteeSubnetTopic(forkDigest, 2), topics[2])
}

func TestSyncCommitteeProcessorAllPossibleTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &eth.SyncCommitteeProcessor{
		ForkDigest: forkDigest,
	}

	topics := processor.AllPossibleTopics()
	assert.Len(t, topics, eth.SyncCommitteeSubnetCount)

	// Verify all topics are unique and properly formatted
	seen := make(map[string]bool)
	for i, topic := range topics {
		assert.Equal(t, eth.SyncCommitteeSubnetTopic(forkDigest, uint64(i)), topic)
		assert.False(t, seen[topic])
		seen[topic] = true
	}
}

func TestSyncCommitteeProcessorGetTopicScoreParams(t *testing.T) {
	processor := &eth.SyncCommitteeProcessor{}
	// eth.SyncCommitteeProcessor always returns nil for score params
	params := processor.GetTopicScoreParams("any_topic")
	assert.Nil(t, params)
}

func TestSyncCommitteeProcessorDecode(t *testing.T) {
	processor := &eth.SyncCommitteeProcessor{
		Encoder: encoder.SszNetworkEncoder{},
		Log:     logrus.New(),
	}

	// Create a test sync committee message
	message := &pb.SyncCommitteeMessage{
		Slot:           primitives.Slot(200),
		BlockRoot:      make([]byte, 32),
		ValidatorIndex: primitives.ValidatorIndex(1000),
		Signature:      make([]byte, 96),
	}

	// Encode the message
	var buf bytes.Buffer
	_, err := processor.Encoder.EncodeGossip(&buf, message)
	require.NoError(t, err)
	encoded := buf.Bytes()

	// Test successful decode
	decoded, err := processor.Decode(context.Background(), "sync_committee_1", encoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded)
	assert.Equal(t, primitives.Slot(200), decoded.Slot)
	assert.Equal(t, primitives.ValidatorIndex(1000), decoded.ValidatorIndex)

	// Test decode error with invalid data
	_, err = processor.Decode(context.Background(), "sync_committee_1", []byte("invalid"))
	assert.Error(t, err)
}

func TestSyncCommitteeProcessorValidate(t *testing.T) {
	tests := []struct {
		name           string
		setupValidator func() func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error)
		subnets        []uint64
		topic          string
		expectedResult pubsub.ValidationResult
		expectError    bool
	}{
		{
			name: "successful validation",
			setupValidator: func() func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64) (pubsub.ValidationResult, error) {
					assert.Equal(t, uint64(1), subnet)
					return pubsub.ValidationAccept, nil
				}
			},
			subnets: []uint64{0, 1, 2},
			topic:          "/eth2/01020304/sync_committee_1/ssz_snappy",
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
		{
			name: "validation reject",
			setupValidator: func() func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, nil
				}
			},
			subnets: []uint64{0},
			topic:          "/eth2/01020304/sync_committee_0/ssz_snappy",
			expectedResult: pubsub.ValidationReject,
			expectError:    false,
		},
		{
			name: "validation error",
			setupValidator: func() func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, errors.New("validation failed")
				}
			},
			subnets: []uint64{2},
			topic:          "/eth2/01020304/sync_committee_2/ssz_snappy",
			expectedResult: pubsub.ValidationReject,
			expectError:    true,
		},
		{
			name: "no validator",
			setupValidator: func() func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error) {
				return nil
			},
			subnets: []uint64{3},
			topic:          "/eth2/01020304/sync_committee_3/ssz_snappy",
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
		{
			name: "invalid topic",
			setupValidator: func() func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64) (pubsub.ValidationResult, error) {
					return pubsub.ValidationAccept, nil
				}
			},
			subnets: []uint64{0},
			topic:          "/eth2/01020304/sync_committee_999/ssz_snappy",
			expectedResult: pubsub.ValidationReject,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &eth.SyncCommitteeProcessor{
				Validator:  tt.setupValidator(),
				Log:        logrus.New(),
				Subnets:    tt.subnets,
				ForkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
			}

			message := &pb.SyncCommitteeMessage{
				Slot:           primitives.Slot(200),
				BlockRoot:      make([]byte, 32),
				ValidatorIndex: primitives.ValidatorIndex(1000),
				Signature:      make([]byte, 96),
			}

			result, err := processor.Validate(context.Background(), tt.topic, message, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestSyncCommitteeProcessorProcess(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func() func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error
		subnets      []uint64
		topic        string
		expectError  bool
	}{
		{
			name: "successful processing",
			setupHandler: func() func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error {
				return func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64, from peer.ID) error {
					assert.Equal(t, uint64(1), subnet)
					return nil
				}
			},
			subnets: []uint64{0, 1, 2},
			topic:       "/eth2/01020304/sync_committee_1/ssz_snappy",
			expectError: false,
		},
		{
			name: "processing error",
			setupHandler: func() func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error {
				return func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64, from peer.ID) error {
					return errors.New("processing failed")
				}
			},
			subnets: []uint64{0},
			topic:       "/eth2/01020304/sync_committee_0/ssz_snappy",
			expectError: true,
		},
		{
			name:         "no handler",
			setupHandler: func() func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error { return nil },
			subnets: []uint64{0},
			topic:        "/eth2/01020304/sync_committee_0/ssz_snappy",
			expectError:  false,
		},
		{
			name: "invalid topic",
			setupHandler: func() func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error {
				return func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64, from peer.ID) error {
					return nil
				}
			},
			subnets: []uint64{0},
			topic:       "/eth2/01020304/sync_committee_999/ssz_snappy",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &eth.SyncCommitteeProcessor{
				Handler:    tt.setupHandler(),
				Log:        logrus.New(),
				Subnets:    tt.subnets,
				ForkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
			}

			message := &pb.SyncCommitteeMessage{
				Slot:           primitives.Slot(200),
				BlockRoot:      make([]byte, 32),
				ValidatorIndex: primitives.ValidatorIndex(1000),
				Signature:      make([]byte, 96),
			}

			err := processor.Process(context.Background(), tt.topic, message, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSyncCommitteeProcessorSubscribeUnsubscribe(t *testing.T) {
	mockGS := &pubsub.Gossipsub{}
	processor := &eth.SyncCommitteeProcessor{
		Gossipsub:  mockGS,
		ForkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
		Log:        logrus.New(),
		Subnets: []uint64{},
	}

	// Test subscribe with subnets
	err := processor.Subscribe(context.Background(), []uint64{0, 1})
	assert.Error(t, err) // Will error because gossipsub.SubscribeToMultiProcessorTopics is not implemented

	// Test with no gossipsub
	processor.Gossipsub = nil
	err = processor.Subscribe(context.Background(), []uint64{0})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gossipsub reference not set")

	// Restore gossipsub for unsubscribe test
	processor.Gossipsub = mockGS
	processor.Subnets = []uint64{0, 1, 2}

	// Test unsubscribe
	err = processor.Unsubscribe(context.Background(), []uint64{0, 1})
	assert.NoError(t, err) // Unsubscribe returns nil even if gossipsub.Unsubscribe fails (it just logs errors)

	// Test unsubscribe with no gossipsub
	processor.Gossipsub = nil
	err = processor.Unsubscribe(context.Background(), []uint64{0})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gossipsub reference not set")
}

func TestSyncCommitteeProcessorTopicIndex(t *testing.T) {
	processor := &eth.SyncCommitteeProcessor{
		ForkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
		Subnets: []uint64{0, 1, 3},
		Log:        logrus.New(),
	}

	tests := []struct {
		name        string
		topic       string
		expectedIdx int
		expectError bool
	}{
		{
			name:        "valid subnet 0",
			topic:       "/eth2/01020304/sync_committee_0/ssz_snappy",
			expectedIdx: 0,
			expectError: false,
		},
		{
			name:        "valid subnet 1",
			topic:       "/eth2/01020304/sync_committee_1/ssz_snappy",
			expectedIdx: 1,
			expectError: false,
		},
		{
			name:        "valid subnet 3",
			topic:       "/eth2/01020304/sync_committee_3/ssz_snappy",
			expectedIdx: 2,
			expectError: false,
		},
		{
			name:        "subnet not in list",
			topic:       "/eth2/01020304/sync_committee_2/ssz_snappy",
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

func TestSyncCommitteeProcessorGetActiveSubnets(t *testing.T) {
	processor := &eth.SyncCommitteeProcessor{
		Subnets: []uint64{0, 1, 3},
	}

	active := processor.GetActiveSubnets()
	assert.Equal(t, []uint64{0, 1, 3}, active)

	// Verify it's a copy
	active[0] = 999
	assert.Equal(t, uint64(0), processor.Subnets[0])
}

func TestSyncCommitteeSubnetConstants(t *testing.T) {
	assert.Equal(t, 4, eth.SyncCommitteeSubnetCount)
	assert.Equal(t, "sync_committee_%d", eth.SyncCommitteeSubnetTopicTemplate)
}
