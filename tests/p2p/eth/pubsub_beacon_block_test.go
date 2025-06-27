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

func TestBeaconBlockProcessorTopic(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &beaconBlockProcessor{
		forkDigest: forkDigest,
	}

	expected := BeaconBlockTopic(forkDigest)
	assert.Equal(t, expected, processor.Topic())
}

func TestBeaconBlockProcessorAllPossibleTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &beaconBlockProcessor{
		forkDigest: forkDigest,
	}

	topics := processor.AllPossibleTopics()
	assert.Len(t, topics, 1)
	assert.Equal(t, BeaconBlockTopic(forkDigest), topics[0])
}

func TestBeaconBlockProcessorGetTopicScoreParams(t *testing.T) {
	tests := []struct {
		name        string
		scoreParams *pubsub.TopicScoreParams
		expected    *pubsub.TopicScoreParams
	}{
		{
			name:        "nil score params",
			scoreParams: nil,
			expected:    nil,
		},
		{
			name: "custom score params",
			scoreParams: &pubsub.TopicScoreParams{
				TopicWeight: 0.5,
			},
			expected: &pubsub.TopicScoreParams{
				TopicWeight: 0.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &beaconBlockProcessor{
				scoreParams: tt.scoreParams,
			}
			assert.Equal(t, tt.expected, processor.GetTopicScoreParams())
		})
	}
}

func TestBeaconBlockProcessorDecode(t *testing.T) {
	processor := &beaconBlockProcessor{
		encoder: encoder.SszNetworkEncoder{},
		log:     logrus.New(),
	}

	// Create a test block
	block := &pb.SignedBeaconBlock{
		Block: &pb.BeaconBlock{
			Slot:          123,
			ProposerIndex: 456,
			ParentRoot:    make([]byte, 32),
			StateRoot:     make([]byte, 32),
			Body: &pb.BeaconBlockBody{
				RandaoReveal: make([]byte, 96),
				Eth1Data: &pb.Eth1Data{
					DepositRoot:  make([]byte, 32),
					DepositCount: 100,
					BlockHash:    make([]byte, 32),
				},
				Graffiti: make([]byte, 32),
			},
		},
		Signature: make([]byte, 96),
	}

	// Encode the block
	var buf bytes.Buffer
	_, err := processor.encoder.EncodeGossip(&buf, block)
	require.NoError(t, err)
	encoded := buf.Bytes()

	// Test successful decode
	decoded, err := processor.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded)
	assert.Equal(t, primitives.Slot(123), decoded.Block.Slot)

	// Test decode error with invalid data
	_, err = processor.Decode(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestBeaconBlockProcessorValidate(t *testing.T) {
	tests := []struct {
		name           string
		setupValidator func() func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error)
		expectedResult pubsub.ValidationResult
		expectError    bool
	}{
		{
			name: "successful validation",
			setupValidator: func() func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, block *pb.SignedBeaconBlock) (pubsub.ValidationResult, error) {
					return pubsub.ValidationAccept, nil
				}
			},
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
		{
			name: "validation reject",
			setupValidator: func() func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, block *pb.SignedBeaconBlock) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, nil
				}
			},
			expectedResult: pubsub.ValidationReject,
			expectError:    false,
		},
		{
			name: "validation error",
			setupValidator: func() func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, block *pb.SignedBeaconBlock) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, errors.New("validation failed")
				}
			},
			expectedResult: pubsub.ValidationReject,
			expectError:    true,
		},
		{
			name:           "no validator",
			setupValidator: func() func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error) { return nil },
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &beaconBlockProcessor{
				validator: tt.setupValidator(),
				log:       logrus.New(),
			}

			block := &pb.SignedBeaconBlock{
				Block: &pb.BeaconBlock{
					Slot:          123,
					ProposerIndex: 456,
					ParentRoot:    make([]byte, 32),
					StateRoot:     make([]byte, 32),
					Body: &pb.BeaconBlockBody{
						RandaoReveal: make([]byte, 96),
						Eth1Data: &pb.Eth1Data{
							DepositRoot:  make([]byte, 32),
							DepositCount: 100,
							BlockHash:    make([]byte, 32),
						},
						Graffiti: make([]byte, 32),
					},
				},
				Signature: make([]byte, 96),
			}

			result, err := processor.Validate(context.Background(), block, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestBeaconBlockProcessorProcess(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func() func(context.Context, *pb.SignedBeaconBlock, peer.ID) error
		expectError  bool
	}{
		{
			name: "successful processing",
			setupHandler: func() func(context.Context, *pb.SignedBeaconBlock, peer.ID) error {
				return func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
					return nil
				}
			},
			expectError: false,
		},
		{
			name: "processing error",
			setupHandler: func() func(context.Context, *pb.SignedBeaconBlock, peer.ID) error {
				return func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
					return errors.New("processing failed")
				}
			},
			expectError: true,
		},
		{
			name:         "no handler",
			setupHandler: func() func(context.Context, *pb.SignedBeaconBlock, peer.ID) error { return nil },
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &beaconBlockProcessor{
				handler: tt.setupHandler(),
				log:     logrus.New(),
			}

			block := &pb.SignedBeaconBlock{
				Block: &pb.BeaconBlock{
					Slot:          123,
					ProposerIndex: 456,
					ParentRoot:    make([]byte, 32),
					StateRoot:     make([]byte, 32),
					Body: &pb.BeaconBlockBody{
						RandaoReveal: make([]byte, 96),
						Eth1Data: &pb.Eth1Data{
							DepositRoot:  make([]byte, 32),
							DepositCount: 100,
							BlockHash:    make([]byte, 32),
						},
						Graffiti: make([]byte, 32),
					},
				},
				Signature: make([]byte, 96),
			}

			err := processor.Process(context.Background(), block, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBeaconBlockProcessorSubscribeUnsubscribe(t *testing.T) {
	mockGS := &pubsub.Gossipsub{}
	processor := &beaconBlockProcessor{
		gossipsub:  mockGS,
		forkDigest: [4]byte{0x01, 0x02, 0x03, 0x04},
		log:        logrus.New(),
	}

	// Subscribe should return error due to circular dependency
	err := processor.Subscribe(context.Background())
	assert.Error(t, err)

	// Unsubscribe should also return error
	err = processor.Unsubscribe(context.Background())
	assert.Error(t, err)
}

func TestBeaconBlockProcessorWithNilFields(t *testing.T) {
	// Test that the processor handles nil fields gracefully
	processor := &beaconBlockProcessor{
		encoder: encoder.SszNetworkEncoder{},
		log:     logrus.New(),
		handler: func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
			// Verify block is not nil
			assert.NotNil(t, block)
			return nil
		},
	}

	block := &pb.SignedBeaconBlock{
		Block: &pb.BeaconBlock{
			Slot:          123,
			ProposerIndex: 456,
			ParentRoot:    make([]byte, 32),
			StateRoot:     make([]byte, 32),
			Body: &pb.BeaconBlockBody{
				RandaoReveal: make([]byte, 96),
				Eth1Data: &pb.Eth1Data{
					DepositRoot:  make([]byte, 32),
					DepositCount: 100,
					BlockHash:    make([]byte, 32),
				},
				Graffiti: make([]byte, 32),
			},
		},
		Signature: make([]byte, 96),
	}

	err := processor.Process(context.Background(), block, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	assert.NoError(t, err)
}

func TestBeaconBlockProcessorNilGossipsub(t *testing.T) {
	processor := &beaconBlockProcessor{
		gossipsub: nil,
		log:       logrus.New(),
	}

	// Should handle nil gossipsub gracefully
	err := processor.Subscribe(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gossipsub reference not set")

	err = processor.Unsubscribe(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gossipsub reference not set")
}
