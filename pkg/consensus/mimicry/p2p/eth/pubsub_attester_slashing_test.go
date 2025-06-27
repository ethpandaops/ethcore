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

func TestAttesterSlashingProcessorTopic(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &attesterSlashingProcessor{
		forkDigest: forkDigest,
	}

	expected := AttesterSlashingTopic(forkDigest)
	assert.Equal(t, expected, processor.Topic())
}

func TestAttesterSlashingProcessorAllPossibleTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &attesterSlashingProcessor{
		forkDigest: forkDigest,
	}

	topics := processor.AllPossibleTopics()
	assert.Len(t, topics, 1)
	assert.Equal(t, AttesterSlashingTopic(forkDigest), topics[0])
}

func TestAttesterSlashingProcessorGetTopicScoreParams(t *testing.T) {
	processor := &attesterSlashingProcessor{}
	// attesterSlashingProcessor always returns nil for score params
	assert.Nil(t, processor.GetTopicScoreParams())
}

func TestAttesterSlashingProcessorDecode(t *testing.T) {
	processor := &attesterSlashingProcessor{
		encoder: encoder.SszNetworkEncoder{},
		log:     logrus.New(),
	}

	// Create a test attester slashing
	attesterSlashing := &pb.AttesterSlashing{
		Attestation_1: &pb.IndexedAttestation{
			AttestingIndices: []uint64{1, 2, 3},
			Data: &pb.AttestationData{
				Slot:            primitives.Slot(100),
				CommitteeIndex:  primitives.CommitteeIndex(0),
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
		},
		Attestation_2: &pb.IndexedAttestation{
			AttestingIndices: []uint64{1, 2, 3},
			Data: &pb.AttestationData{
				Slot:            primitives.Slot(100),
				CommitteeIndex:  primitives.CommitteeIndex(0),
				BeaconBlockRoot: make([]byte, 32),
				Source: &pb.Checkpoint{
					Epoch: primitives.Epoch(10),
					Root:  make([]byte, 32),
				},
				Target: &pb.Checkpoint{
					Epoch: primitives.Epoch(12), // Different target for slashing
					Root:  make([]byte, 32),
				},
			},
			Signature: make([]byte, 96),
		},
	}

	// Encode the attester slashing
	var buf bytes.Buffer
	_, err := processor.encoder.EncodeGossip(&buf, attesterSlashing)
	require.NoError(t, err)
	encoded := buf.Bytes()

	// Test successful decode
	decoded, err := processor.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded)
	assert.Equal(t, primitives.Slot(100), decoded.Attestation_1.Data.Slot)
	assert.Equal(t, []uint64{1, 2, 3}, decoded.Attestation_1.AttestingIndices)

	// Test decode error with invalid data
	_, err = processor.Decode(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestAttesterSlashingProcessorValidate(t *testing.T) {
	tests := []struct {
		name           string
		setupValidator func() func(context.Context, *pb.AttesterSlashing) (pubsub.ValidationResult, error)
		expectedResult pubsub.ValidationResult
		expectError    bool
	}{
		{
			name: "successful validation",
			setupValidator: func() func(context.Context, *pb.AttesterSlashing) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, slashing *pb.AttesterSlashing) (pubsub.ValidationResult, error) {
					return pubsub.ValidationAccept, nil
				}
			},
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
		{
			name: "validation reject",
			setupValidator: func() func(context.Context, *pb.AttesterSlashing) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, slashing *pb.AttesterSlashing) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, nil
				}
			},
			expectedResult: pubsub.ValidationReject,
			expectError:    false,
		},
		{
			name: "validation error",
			setupValidator: func() func(context.Context, *pb.AttesterSlashing) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, slashing *pb.AttesterSlashing) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, errors.New("validation failed")
				}
			},
			expectedResult: pubsub.ValidationReject,
			expectError:    true,
		},
		{
			name:           "no validator",
			setupValidator: func() func(context.Context, *pb.AttesterSlashing) (pubsub.ValidationResult, error) { return nil },
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &attesterSlashingProcessor{
				validator: tt.setupValidator(),
				log:       logrus.New(),
			}

			attesterSlashing := &pb.AttesterSlashing{
				Attestation_1: &pb.IndexedAttestation{
					AttestingIndices: []uint64{1, 2, 3},
					Data: &pb.AttestationData{
						Slot:            primitives.Slot(100),
						CommitteeIndex:  primitives.CommitteeIndex(0),
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
				},
				Attestation_2: &pb.IndexedAttestation{
					AttestingIndices: []uint64{1, 2, 3},
					Data: &pb.AttestationData{
						Slot:            primitives.Slot(100),
						CommitteeIndex:  primitives.CommitteeIndex(0),
						BeaconBlockRoot: make([]byte, 32),
						Source: &pb.Checkpoint{
							Epoch: primitives.Epoch(10),
							Root:  make([]byte, 32),
						},
						Target: &pb.Checkpoint{
							Epoch: primitives.Epoch(12),
							Root:  make([]byte, 32),
						},
					},
					Signature: make([]byte, 96),
				},
			}

			result, err := processor.Validate(context.Background(), attesterSlashing, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestAttesterSlashingProcessorProcess(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func() func(context.Context, *pb.AttesterSlashing, peer.ID) error
		expectError  bool
	}{
		{
			name: "successful processing",
			setupHandler: func() func(context.Context, *pb.AttesterSlashing, peer.ID) error {
				return func(ctx context.Context, slashing *pb.AttesterSlashing, from peer.ID) error {
					return nil
				}
			},
			expectError: false,
		},
		{
			name: "processing error",
			setupHandler: func() func(context.Context, *pb.AttesterSlashing, peer.ID) error {
				return func(ctx context.Context, slashing *pb.AttesterSlashing, from peer.ID) error {
					return errors.New("processing failed")
				}
			},
			expectError: true,
		},
		{
			name:         "no handler",
			setupHandler: func() func(context.Context, *pb.AttesterSlashing, peer.ID) error { return nil },
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &attesterSlashingProcessor{
				handler: tt.setupHandler(),
				log:     logrus.New(),
			}

			attesterSlashing := &pb.AttesterSlashing{
				Attestation_1: &pb.IndexedAttestation{
					AttestingIndices: []uint64{1, 2, 3},
					Data: &pb.AttestationData{
						Slot:            primitives.Slot(100),
						CommitteeIndex:  primitives.CommitteeIndex(0),
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
				},
				Attestation_2: &pb.IndexedAttestation{
					AttestingIndices: []uint64{1, 2, 3},
					Data: &pb.AttestationData{
						Slot:            primitives.Slot(100),
						CommitteeIndex:  primitives.CommitteeIndex(0),
						BeaconBlockRoot: make([]byte, 32),
						Source: &pb.Checkpoint{
							Epoch: primitives.Epoch(10),
							Root:  make([]byte, 32),
						},
						Target: &pb.Checkpoint{
							Epoch: primitives.Epoch(12),
							Root:  make([]byte, 32),
						},
					},
					Signature: make([]byte, 96),
				},
			}

			err := processor.Process(context.Background(), attesterSlashing, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAttesterSlashingProcessorSubscribeUnsubscribe(t *testing.T) {
	mockGS := &pubsub.Gossipsub{}
	processor := &attesterSlashingProcessor{
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

func TestAttesterSlashingProcessorWithNilFields(t *testing.T) {
	// Test that the processor handles nil fields gracefully
	processor := &attesterSlashingProcessor{
		encoder: encoder.SszNetworkEncoder{},
		log:     logrus.New(),
		handler: func(ctx context.Context, slashing *pb.AttesterSlashing, from peer.ID) error {
			// Verify slashing is not nil
			assert.NotNil(t, slashing)
			return nil
		},
	}

	attesterSlashing := &pb.AttesterSlashing{
		Attestation_1: &pb.IndexedAttestation{
			AttestingIndices: []uint64{1, 2, 3},
			Data: &pb.AttestationData{
				Slot:            primitives.Slot(100),
				CommitteeIndex:  primitives.CommitteeIndex(0),
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
		},
		Attestation_2: &pb.IndexedAttestation{
			AttestingIndices: []uint64{1, 2, 3},
			Data: &pb.AttestationData{
				Slot:            primitives.Slot(100),
				CommitteeIndex:  primitives.CommitteeIndex(0),
				BeaconBlockRoot: make([]byte, 32),
				Source: &pb.Checkpoint{
					Epoch: primitives.Epoch(10),
					Root:  make([]byte, 32),
				},
				Target: &pb.Checkpoint{
					Epoch: primitives.Epoch(12),
					Root:  make([]byte, 32),
				},
			},
			Signature: make([]byte, 96),
		},
	}

	err := processor.Process(context.Background(), attesterSlashing, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	assert.NoError(t, err)
}

func TestAttesterSlashingTopicFormat(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	topic := AttesterSlashingTopic(forkDigest)
	assert.Equal(t, "/eth2/01020304/attester_slashing/ssz_snappy", topic)

	// Test the constant
	assert.Equal(t, "attester_slashing", AttesterSlashingTopicName)
}