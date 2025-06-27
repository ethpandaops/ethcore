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

// BLS to execution change topic constants and functions
const (
	BlsToExecutionChangeTopicName = "bls_to_execution_change"
)

// BlsToExecutionChangeTopic constructs the BLS to execution change gossipsub topic name
func BlsToExecutionChangeTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BlsToExecutionChangeTopicName)
}

// blsToExecutionProcessor handles BLS to execution change messages
type blsToExecutionProcessor struct {
	forkDigest [4]byte
	encoder    encoder.SszNetworkEncoder
	handler    func(context.Context, *pb.SignedBLSToExecutionChange, peer.ID) error
	validator  func(context.Context, *pb.SignedBLSToExecutionChange) (pubsub.ValidationResult, error)
	gossipsub  *pubsub.Gossipsub
	log        logrus.FieldLogger
}

func (p *blsToExecutionProcessor) Topic() string {
	return BlsToExecutionChangeTopic(p.forkDigest)
}

func (p *blsToExecutionProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *blsToExecutionProcessor) Subscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}
	return p.gossipsub.SubscribeToProcessorTopic(ctx, p.Topic())
}

func (p *blsToExecutionProcessor) Unsubscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}
	return p.gossipsub.Unsubscribe(p.Topic())
}

func (p *blsToExecutionProcessor) Decode(ctx context.Context, data []byte) (*pb.SignedBLSToExecutionChange, error) {
	change := &pb.SignedBLSToExecutionChange{}
	if err := p.encoder.DecodeGossip(data, change); err != nil {
		return nil, fmt.Errorf("failed to decode BLS to execution change: %w", err)
	}
	return change, nil
}

func (p *blsToExecutionProcessor) Validate(ctx context.Context, change *pb.SignedBLSToExecutionChange, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external validator function
	if p.validator != nil {
		return p.validator(ctx, change)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *blsToExecutionProcessor) Process(ctx context.Context, change *pb.SignedBLSToExecutionChange, from string) error {
	if p.handler == nil {
		p.log.Debug("No handler provided, BLS to execution change received but not processed")
		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.handler(ctx, change, peerID)
}

func (p *blsToExecutionProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that blsToExecutionProcessor implements pubsub.Processor
var _ pubsub.Processor[*pb.SignedBLSToExecutionChange] = (*blsToExecutionProcessor)(nil)
