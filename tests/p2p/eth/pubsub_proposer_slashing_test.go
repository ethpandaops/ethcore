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

func TestProposerSlashingProcessorTopic(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &proposerSlashingProcessor{
		forkDigest: forkDigest,
	}

	expected := ProposerSlashingTopic(forkDigest[:])
	assert.Equal(t, expected, processor.Topic())
}

func TestProposerSlashingProcessorAllPossibleTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &proposerSlashingProcessor{
		forkDigest: forkDigest,
	}

	topics := processor.AllPossibleTopics()
	assert.Len(t, topics, 1)
	assert.Equal(t, ProposerSlashingTopic(forkDigest[:]), topics[0])
}

func TestProposerSlashingProcessorGetTopicScoreParams(t *testing.T) {
	processor := &proposerSlashingProcessor{}
	// proposerSlashingProcessor always returns nil for score params
	assert.Nil(t, processor.GetTopicScoreParams())
}

func TestProposerSlashingProcessorDecode(t *testing.T) {
	processor := &proposerSlashingProcessor{
		encoder: encoder.SszNetworkEncoder{},
		log:     logrus.New(),
	}

	// Create a test proposer slashing
	proposerSlashing := &pb.ProposerSlashing{
		Header_1: &pb.SignedBeaconBlockHeader{
			Header: &pb.BeaconBlockHeader{
				Slot:          primitives.Slot(100),
				ProposerIndex: primitives.ValidatorIndex(10),
				ParentRoot:    make([]byte, 32),
				StateRoot:     make([]byte, 32),
				BodyRoot:      make([]byte, 32),
			},
			Signature: make([]byte, 96),
		},
		Header_2: &pb.SignedBeaconBlockHeader{
			Header: &pb.BeaconBlockHeader{
				Slot:          primitives.Slot(100),
				ProposerIndex: primitives.ValidatorIndex(10),
				ParentRoot:    make([]byte, 32),
				StateRoot:     make([]byte, 32),
				BodyRoot:      make([]byte, 32),
			},
			Signature: make([]byte, 96),
		},
	}

	// Encode the proposer slashing
	var buf bytes.Buffer
	_, err := processor.encoder.EncodeGossip(&buf, proposerSlashing)
	require.NoError(t, err)
	encoded := buf.Bytes()

	// Test successful decode
	decoded, err := processor.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded)
	assert.Equal(t, primitives.Slot(100), decoded.Header_1.Header.Slot)
	assert.Equal(t, primitives.ValidatorIndex(10), decoded.Header_1.Header.ProposerIndex)

	// Test decode error with invalid data
	_, err = processor.Decode(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestProposerSlashingProcessorValidate(t *testing.T) {
	tests := []struct {
		name           string
		setupValidator func() func(context.Context, *pb.ProposerSlashing) (pubsub.ValidationResult, error)
		expectedResult pubsub.ValidationResult
		expectError    bool
	}{
		{
			name: "successful validation",
			setupValidator: func() func(context.Context, *pb.ProposerSlashing) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, slashing *pb.ProposerSlashing) (pubsub.ValidationResult, error) {
					return pubsub.ValidationAccept, nil
				}
			},
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
		{
			name: "validation reject",
			setupValidator: func() func(context.Context, *pb.ProposerSlashing) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, slashing *pb.ProposerSlashing) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, nil
				}
			},
			expectedResult: pubsub.ValidationReject,
			expectError:    false,
		},
		{
			name: "validation error",
			setupValidator: func() func(context.Context, *pb.ProposerSlashing) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, slashing *pb.ProposerSlashing) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, errors.New("validation failed")
				}
			},
			expectedResult: pubsub.ValidationReject,
			expectError:    true,
		},
		{
			name:           "no validator",
			setupValidator: func() func(context.Context, *pb.ProposerSlashing) (pubsub.ValidationResult, error) { return nil },
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &proposerSlashingProcessor{
				validator: tt.setupValidator(),
				log:       logrus.New(),
			}

			proposerSlashing := &pb.ProposerSlashing{
				Header_1: &pb.SignedBeaconBlockHeader{
					Header: &pb.BeaconBlockHeader{
						Slot:          primitives.Slot(100),
						ProposerIndex: primitives.ValidatorIndex(10),
						ParentRoot:    make([]byte, 32),
						StateRoot:     make([]byte, 32),
						BodyRoot:      make([]byte, 32),
					},
					Signature: make([]byte, 96),
				},
				Header_2: &pb.SignedBeaconBlockHeader{
					Header: &pb.BeaconBlockHeader{
						Slot:          primitives.Slot(100),
						ProposerIndex: primitives.ValidatorIndex(10),
						ParentRoot:    make([]byte, 32),
						StateRoot:     make([]byte, 32),
						BodyRoot:      make([]byte, 32),
					},
					Signature: make([]byte, 96),
				},
			}

			result, err := processor.Validate(context.Background(), proposerSlashing, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestProposerSlashingProcessorProcess(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func() func(context.Context, *pb.ProposerSlashing, peer.ID) error
		expectError  bool
	}{
		{
			name: "successful processing",
			setupHandler: func() func(context.Context, *pb.ProposerSlashing, peer.ID) error {
				return func(ctx context.Context, slashing *pb.ProposerSlashing, from peer.ID) error {
					return nil
				}
			},
			expectError: false,
		},
		{
			name: "processing error",
			setupHandler: func() func(context.Context, *pb.ProposerSlashing, peer.ID) error {
				return func(ctx context.Context, slashing *pb.ProposerSlashing, from peer.ID) error {
					return errors.New("processing failed")
				}
			},
			expectError: true,
		},
		{
			name:         "no handler",
			setupHandler: func() func(context.Context, *pb.ProposerSlashing, peer.ID) error { return nil },
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &proposerSlashingProcessor{
				handler: tt.setupHandler(),
				log:     logrus.New(),
			}

			proposerSlashing := &pb.ProposerSlashing{
				Header_1: &pb.SignedBeaconBlockHeader{
					Header: &pb.BeaconBlockHeader{
						Slot:          primitives.Slot(100),
						ProposerIndex: primitives.ValidatorIndex(10),
						ParentRoot:    make([]byte, 32),
						StateRoot:     make([]byte, 32),
						BodyRoot:      make([]byte, 32),
					},
					Signature: make([]byte, 96),
				},
				Header_2: &pb.SignedBeaconBlockHeader{
					Header: &pb.BeaconBlockHeader{
						Slot:          primitives.Slot(100),
						ProposerIndex: primitives.ValidatorIndex(10),
						ParentRoot:    make([]byte, 32),
						StateRoot:     make([]byte, 32),
						BodyRoot:      make([]byte, 32),
					},
					Signature: make([]byte, 96),
				},
			}

			err := processor.Process(context.Background(), proposerSlashing, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProposerSlashingProcessorSubscribeUnsubscribe(t *testing.T) {
	mockGS := &pubsub.Gossipsub{}
	processor := &proposerSlashingProcessor{
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

func TestProposerSlashingProcessorWithNilFields(t *testing.T) {
	// Test that the processor handles nil fields gracefully
	processor := &proposerSlashingProcessor{
		encoder: encoder.SszNetworkEncoder{},
		log:     logrus.New(),
		handler: func(ctx context.Context, slashing *pb.ProposerSlashing, from peer.ID) error {
			// Verify slashing is not nil
			assert.NotNil(t, slashing)
			return nil
		},
	}

	proposerSlashing := &pb.ProposerSlashing{
		Header_1: &pb.SignedBeaconBlockHeader{
			Header: &pb.BeaconBlockHeader{
				Slot:          primitives.Slot(100),
				ProposerIndex: primitives.ValidatorIndex(10),
				ParentRoot:    make([]byte, 32),
				StateRoot:     make([]byte, 32),
				BodyRoot:      make([]byte, 32),
			},
			Signature: make([]byte, 96),
		},
		Header_2: &pb.SignedBeaconBlockHeader{
			Header: &pb.BeaconBlockHeader{
				Slot:          primitives.Slot(100),
				ProposerIndex: primitives.ValidatorIndex(10),
				ParentRoot:    make([]byte, 32),
				StateRoot:     make([]byte, 32),
				BodyRoot:      make([]byte, 32),
			},
			Signature: make([]byte, 96),
		},
	}

	err := processor.Process(context.Background(), proposerSlashing, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	assert.NoError(t, err)
}

func TestProposerSlashingTopicFormat(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	topic := ProposerSlashingTopic(forkDigest[:])
	assert.Equal(t, "/eth2/01020304/proposer_slashing/ssz_snappy", topic)
}
