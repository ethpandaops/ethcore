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

func TestVoluntaryExitProcessorTopic(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &voluntaryExitProcessor{
		forkDigest: forkDigest,
	}

	expected := VoluntaryExitTopic(forkDigest[:])
	assert.Equal(t, expected, processor.Topic())
}

func TestVoluntaryExitProcessorAllPossibleTopics(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	processor := &voluntaryExitProcessor{
		forkDigest: forkDigest,
	}

	topics := processor.AllPossibleTopics()
	assert.Len(t, topics, 1)
	assert.Equal(t, VoluntaryExitTopic(forkDigest[:]), topics[0])
}

func TestVoluntaryExitProcessorGetTopicScoreParams(t *testing.T) {
	processor := &voluntaryExitProcessor{}
	// voluntaryExitProcessor always returns nil for score params
	assert.Nil(t, processor.GetTopicScoreParams())
}

func TestVoluntaryExitProcessorDecode(t *testing.T) {
	processor := &voluntaryExitProcessor{
		encoder: encoder.SszNetworkEncoder{},
		log:     logrus.New(),
	}

	// Create a test voluntary exit
	voluntaryExit := &pb.SignedVoluntaryExit{
		Exit: &pb.VoluntaryExit{
			Epoch:          primitives.Epoch(100),
			ValidatorIndex: primitives.ValidatorIndex(1000),
		},
		Signature: make([]byte, 96),
	}

	// Encode the voluntary exit
	var buf bytes.Buffer
	_, err := processor.encoder.EncodeGossip(&buf, voluntaryExit)
	require.NoError(t, err)
	encoded := buf.Bytes()

	// Test successful decode
	decoded, err := processor.Decode(context.Background(), encoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded)
	assert.Equal(t, primitives.Epoch(100), decoded.Exit.Epoch)
	assert.Equal(t, primitives.ValidatorIndex(1000), decoded.Exit.ValidatorIndex)

	// Test decode error with invalid data
	_, err = processor.Decode(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestVoluntaryExitProcessorValidate(t *testing.T) {
	tests := []struct {
		name           string
		setupValidator func() func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error)
		expectedResult pubsub.ValidationResult
		expectError    bool
	}{
		{
			name: "successful validation",
			setupValidator: func() func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, exit *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error) {
					return pubsub.ValidationAccept, nil
				}
			},
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
		{
			name: "validation reject",
			setupValidator: func() func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, exit *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, nil
				}
			},
			expectedResult: pubsub.ValidationReject,
			expectError:    false,
		},
		{
			name: "validation error",
			setupValidator: func() func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error) {
				return func(ctx context.Context, exit *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error) {
					return pubsub.ValidationReject, errors.New("validation failed")
				}
			},
			expectedResult: pubsub.ValidationReject,
			expectError:    true,
		},
		{
			name:           "no validator",
			setupValidator: func() func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error) { return nil },
			expectedResult: pubsub.ValidationAccept,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &voluntaryExitProcessor{
				validator: tt.setupValidator(),
				log:       logrus.New(),
			}

			voluntaryExit := &pb.SignedVoluntaryExit{
				Exit: &pb.VoluntaryExit{
					Epoch:          primitives.Epoch(100),
					ValidatorIndex: primitives.ValidatorIndex(1000),
				},
				Signature: make([]byte, 96),
			}

			result, err := processor.Validate(context.Background(), voluntaryExit, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestVoluntaryExitProcessorProcess(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func() func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error
		expectError  bool
	}{
		{
			name: "successful processing",
			setupHandler: func() func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error {
				return func(ctx context.Context, exit *pb.SignedVoluntaryExit, from peer.ID) error {
					return nil
				}
			},
			expectError: false,
		},
		{
			name: "processing error",
			setupHandler: func() func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error {
				return func(ctx context.Context, exit *pb.SignedVoluntaryExit, from peer.ID) error {
					return errors.New("processing failed")
				}
			},
			expectError: true,
		},
		{
			name:         "no handler",
			setupHandler: func() func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error { return nil },
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &voluntaryExitProcessor{
				handler: tt.setupHandler(),
				log:     logrus.New(),
			}

			voluntaryExit := &pb.SignedVoluntaryExit{
				Exit: &pb.VoluntaryExit{
					Epoch:          primitives.Epoch(100),
					ValidatorIndex: primitives.ValidatorIndex(1000),
				},
				Signature: make([]byte, 96),
			}

			err := processor.Process(context.Background(), voluntaryExit, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVoluntaryExitProcessorSubscribeUnsubscribe(t *testing.T) {
	mockGS := &pubsub.Gossipsub{}
	processor := &voluntaryExitProcessor{
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

func TestVoluntaryExitProcessorWithNilFields(t *testing.T) {
	// Test that the processor handles nil fields gracefully
	processor := &voluntaryExitProcessor{
		encoder: encoder.SszNetworkEncoder{},
		log:     logrus.New(),
		handler: func(ctx context.Context, exit *pb.SignedVoluntaryExit, from peer.ID) error {
			// Verify exit is not nil
			assert.NotNil(t, exit)
			return nil
		},
	}

	voluntaryExit := &pb.SignedVoluntaryExit{
		Exit: &pb.VoluntaryExit{
			Epoch:          primitives.Epoch(100),
			ValidatorIndex: primitives.ValidatorIndex(1000),
		},
		Signature: make([]byte, 96),
	}

	err := processor.Process(context.Background(), voluntaryExit, "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf")
	assert.NoError(t, err)
}

func TestVoluntaryExitTopicFormat(t *testing.T) {
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	topic := VoluntaryExitTopic(forkDigest[:])
	assert.Equal(t, "/eth2/01020304/voluntary_exit/ssz_snappy", topic)
}
