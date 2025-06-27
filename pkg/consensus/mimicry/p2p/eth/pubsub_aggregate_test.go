package eth

import (
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

func TestAggregateProcessorTopic(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &aggregateProcessor{
		forkDigest: forkDigest,
	}

	expected := BeaconAggregateAndProofTopic(forkDigest)
	assert.Equal(t, expected, processor.Topic())
}

func TestAggregateProcessorAllPossibleTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &aggregateProcessor{
		forkDigest: forkDigest,
	}

	topics := processor.AllPossibleTopics()
	assert.Len(t, topics, 1)
	assert.Equal(t, BeaconAggregateAndProofTopic(forkDigest), topics[0])
}

func TestAggregateProcessorGetTopicScoreParams(t *testing.T) {
	processor := &aggregateProcessor{}
	// aggregateProcessor always returns nil for score params
	assert.Nil(t, processor.GetTopicScoreParams())
}

func TestAggregateProcessorDecode(t *testing.T) {
	processor := &aggregateProcessor{
		encoder: encoder.SszNetworkEncoder{},
		log:     logrus.New(),
	}

	// Create a test aggregate and proof
	aggregateAndProof := &pb.AggregateAttestationAndProof{
		AggregatorIndex: primitives.ValidatorIndex(100),
		Aggregate: &pb.Attestation{
			AggregationBits: []byte{0xff, 0xff},
			Data: &pb.AttestationData{
				Slot:            primitives.Slot(150),
				CommitteeIndex:  primitives.CommitteeIndex(3),
				BeaconBlockRoot: make([]byte, 32),
				Source: &pb.Checkpoint{
					Epoch: primitives.Epoch(20),
					Root:  make([]byte, 32),
				},
				Target: &pb.Checkpoint{
					Epoch: primitives.Epoch(21),
					Root:  make([]byte, 32),
				},
			},
			Signature: make([]byte, 96),
		},
		SelectionProof: make([]byte, 96),
	}

	// Encode the aggregate and proof
	var buf bytes.Buffer
	_, err := processor.encoder.EncodeGossip(&buf, aggregateAndProof)
	require.NoError(t, err)
	encoded := buf.Bytes()

	// Test successful decode
	decoded, err := processor.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded)
	assert.Equal(t, primitives.ValidatorIndex(100), decoded.AggregatorIndex)
	assert.Equal(t, primitives.Slot(150), decoded.Aggregate.Data.Slot)

	// Test decode error with invalid data
	_, err = processor.Decode(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestAggregateProcessorValidate(t *testing.T) {
	tests := []struct {
		name           string
		setupValidator func() func(context.Context, *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error)
		expectedResult pubsub.ValidationResult
		expectError    bool
	}{
		{
			name: "successful validation",
			setupValidator: func() func(context.Context, *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, agg *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error) {
					return pubsub.ValidationAccept, nil
				}
			},
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
		{
			name: "validation reject",
			setupValidator: func() func(context.Context, *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, agg *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, nil
				}
			},
			expectedResult: pubsub.ValidationReject,
			expectError:    false,
		},
		{
			name: "validation error",
			setupValidator: func() func(context.Context, *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, agg *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, errors.New("validation failed")
				}
			},
			expectedResult: pubsub.ValidationReject,
			expectError:    true,
		},
		{
			name:           "no validator",
			setupValidator: func() func(context.Context, *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error) { return nil },
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &aggregateProcessor{
				validator: tt.setupValidator(),
				log:       logrus.New(),
			}

			aggregateAndProof := &pb.AggregateAttestationAndProof{
				AggregatorIndex: primitives.ValidatorIndex(100),
				Aggregate: &pb.Attestation{
					AggregationBits: []byte{0xff, 0xff},
					Data: &pb.AttestationData{
						Slot:            primitives.Slot(150),
						CommitteeIndex:  primitives.CommitteeIndex(3),
						BeaconBlockRoot: make([]byte, 32),
						Source: &pb.Checkpoint{
							Epoch: primitives.Epoch(20),
							Root:  make([]byte, 32),
						},
						Target: &pb.Checkpoint{
							Epoch: primitives.Epoch(21),
							Root:  make([]byte, 32),
						},
					},
					Signature: make([]byte, 96),
				},
				SelectionProof: make([]byte, 96),
			}

			result, err := processor.Validate(context.Background(), aggregateAndProof, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestAggregateProcessorProcess(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func() func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error
		expectError  bool
	}{
		{
			name: "successful processing",
			setupHandler: func() func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error {
				return func(ctx context.Context, agg *pb.AggregateAttestationAndProof, from peer.ID) error {
					return nil
				}
			},
			expectError: false,
		},
		{
			name: "processing error",
			setupHandler: func() func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error {
				return func(ctx context.Context, agg *pb.AggregateAttestationAndProof, from peer.ID) error {
					return errors.New("processing failed")
				}
			},
			expectError: true,
		},
		{
			name:         "no handler",
			setupHandler: func() func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error { return nil },
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &aggregateProcessor{
				handler: tt.setupHandler(),
				log:     logrus.New(),
			}

			aggregateAndProof := &pb.AggregateAttestationAndProof{
				AggregatorIndex: primitives.ValidatorIndex(100),
				Aggregate: &pb.Attestation{
					AggregationBits: []byte{0xff, 0xff},
					Data: &pb.AttestationData{
						Slot:            primitives.Slot(150),
						CommitteeIndex:  primitives.CommitteeIndex(3),
						BeaconBlockRoot: make([]byte, 32),
						Source: &pb.Checkpoint{
							Epoch: primitives.Epoch(20),
							Root:  make([]byte, 32),
						},
						Target: &pb.Checkpoint{
							Epoch: primitives.Epoch(21),
							Root:  make([]byte, 32),
						},
					},
					Signature: make([]byte, 96),
				},
				SelectionProof: make([]byte, 96),
			}

			err := processor.Process(context.Background(), aggregateAndProof, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAggregateProcessorSubscribeUnsubscribe(t *testing.T) {
	mockGS := &pubsub.Gossipsub{}
	processor := &aggregateProcessor{
		gossipsub:  mockGS,
		forkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
		log:        logrus.New(),
	}

	// Subscribe should return error due to gossipsub.SubscribeToProcessorTopic not implemented
	err := processor.Subscribe(context.Background())
	assert.Error(t, err)

	// Unsubscribe should also return error
	err = processor.Unsubscribe(context.Background())
	assert.Error(t, err)

	// Test with nil gossipsub
	processor.gossipsub = nil
	err = processor.Subscribe(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gossipsub reference not set")

	err = processor.Unsubscribe(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gossipsub reference not set")
}

func TestAggregateProcessorWithNilFields(t *testing.T) {
	// Test that the processor handles nil fields gracefully
	processor := &aggregateProcessor{
		encoder: encoder.SszNetworkEncoder{},
		log:     logrus.New(),
		handler: func(ctx context.Context, agg *pb.AggregateAttestationAndProof, from peer.ID) error {
			// Verify aggregate is not nil
			assert.NotNil(t, agg)
			return nil
		},
	}

	aggregateAndProof := &pb.AggregateAttestationAndProof{
		AggregatorIndex: primitives.ValidatorIndex(100),
		Aggregate: &pb.Attestation{
			AggregationBits: []byte{0xff, 0xff},
			Data: &pb.AttestationData{
				Slot:            primitives.Slot(150),
				CommitteeIndex:  primitives.CommitteeIndex(3),
				BeaconBlockRoot: make([]byte, 32),
				Source: &pb.Checkpoint{
					Epoch: primitives.Epoch(20),
					Root:  make([]byte, 32),
				},
				Target: &pb.Checkpoint{
					Epoch: primitives.Epoch(21),
					Root:  make([]byte, 32),
				},
			},
			Signature: make([]byte, 96),
		},
		SelectionProof: make([]byte, 96),
	}

	err := processor.Process(context.Background(), aggregateAndProof, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	assert.NoError(t, err)
}

func TestBeaconAggregateAndProofTopicName(t *testing.T) {
	assert.Equal(t, "beacon_aggregate_and_proof", BeaconAggregateAndProofTopicName)
}