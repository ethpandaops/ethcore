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

// ProposerSlashingTopic returns the topic name for proposer slashing messages
func ProposerSlashingTopic(forkDigest []byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, "proposer_slashing")
}

// proposerSlashingProcessor handles proposer slashing messages
type proposerSlashingProcessor struct {
	forkDigest [4]byte
	encoder    encoder.SszNetworkEncoder
	handler    func(context.Context, *pb.ProposerSlashing, peer.ID) error
	validator  func(context.Context, *pb.ProposerSlashing) (pubsub.ValidationResult, error)
	gossipsub  *pubsub.Gossipsub
	log        logrus.FieldLogger
}

func (p *proposerSlashingProcessor) Topic() string {
	return ProposerSlashingTopic(p.forkDigest[:])
}

func (p *proposerSlashingProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *proposerSlashingProcessor) Subscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.gossipsub.SubscribeToProcessorTopic(ctx, p.Topic())
}

func (p *proposerSlashingProcessor) Unsubscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.gossipsub.Unsubscribe(p.Topic())
}

func (p *proposerSlashingProcessor) Decode(ctx context.Context, data []byte) (*pb.ProposerSlashing, error) {
	slashing := &pb.ProposerSlashing{}
	if err := p.encoder.DecodeGossip(data, slashing); err != nil {
		return nil, fmt.Errorf("failed to decode proposer slashing: %w", err)
	}

	return slashing, nil
}

func (p *proposerSlashingProcessor) Validate(ctx context.Context, slashing *pb.ProposerSlashing, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external validator function
	if p.validator != nil {
		return p.validator(ctx, slashing)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *proposerSlashingProcessor) Process(ctx context.Context, slashing *pb.ProposerSlashing, from string) error {
	if p.handler == nil {
		p.log.Debug("No handler provided, proposer slashing received but not processed")
		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.handler(ctx, slashing, peerID)
}

func (p *proposerSlashingProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that proposerSlashingProcessor implements pubsub.Processor
var _ pubsub.Processor[*pb.ProposerSlashing] = (*proposerSlashingProcessor)(nil)
