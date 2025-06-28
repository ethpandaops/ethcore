package eth

import (
	"context"
	"fmt"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// BLS to execution change topic constants and functions.
const (
	BlsToExecutionChangeTopicName = "bls_to_execution_change"
)

// BlsToExecutionChangeTopic constructs the BLS to execution change gossipsub topic name.
func BlsToExecutionChangeTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BlsToExecutionChangeTopicName)
}

// blsToExecutionProcessor handles BLS to execution change messages.
type BlsToExecutionProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Handler    func(context.Context, *pb.SignedBLSToExecutionChange, peer.ID) error
	Validator  func(context.Context, *pb.SignedBLSToExecutionChange) (pubsub.ValidationResult, error)
	Gossipsub  *pubsub.Gossipsub
	Log        logrus.FieldLogger
}

func (p *BlsToExecutionProcessor) Topic() string {
	return BlsToExecutionChangeTopic(p.ForkDigest)
}

func (p *BlsToExecutionProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *BlsToExecutionProcessor) Subscribe(ctx context.Context) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.Gossipsub.SubscribeToProcessorTopic(ctx, p.Topic())
}

func (p *BlsToExecutionProcessor) Unsubscribe(ctx context.Context) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.Gossipsub.Unsubscribe(p.Topic())
}

func (p *BlsToExecutionProcessor) Decode(ctx context.Context, data []byte) (*pb.SignedBLSToExecutionChange, error) {
	change := &pb.SignedBLSToExecutionChange{}
	if err := p.Encoder.DecodeGossip(data, change); err != nil {
		return nil, fmt.Errorf("failed to decode BLS to execution change: %w", err)
	}

	return change, nil
}

func (p *BlsToExecutionProcessor) Validate(ctx context.Context, change *pb.SignedBLSToExecutionChange, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, change)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *BlsToExecutionProcessor) Process(ctx context.Context, change *pb.SignedBLSToExecutionChange, from string) error {
	if p.Handler == nil {
		p.Log.Debug("No handler provided, BLS to execution change received but not processed")

		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.Handler(ctx, change, peerID)
}

func (p *BlsToExecutionProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that blsToExecutionProcessor implements pubsub.Processor.
var _ pubsub.Processor[*pb.SignedBLSToExecutionChange] = (*BlsToExecutionProcessor)(nil)
