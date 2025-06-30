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

// ProposerSlashingTopic returns the topic name for proposer slashing messages.
func ProposerSlashingTopic(forkDigest []byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, "proposer_slashing")
}

// ProposerSlashingProcessor defines the interface for processing proposer slashing messages.
type ProposerSlashingProcessor interface {
	pubsub.Processor[*pb.ProposerSlashing]
}

// DefaultProposerSlashingProcessor handles proposer slashing messages.
type DefaultProposerSlashingProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Handler    func(context.Context, *pb.ProposerSlashing, peer.ID) error
	Validator  func(context.Context, *pb.ProposerSlashing) (pubsub.ValidationResult, error)
	Log        logrus.FieldLogger
}

func (p *DefaultProposerSlashingProcessor) Topic() string {
	return ProposerSlashingTopic(p.ForkDigest[:])
}

func (p *DefaultProposerSlashingProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}


func (p *DefaultProposerSlashingProcessor) Decode(ctx context.Context, data []byte) (*pb.ProposerSlashing, error) {
	slashing := &pb.ProposerSlashing{}
	if err := p.Encoder.DecodeGossip(data, slashing); err != nil {
		return nil, fmt.Errorf("failed to decode proposer slashing: %w", err)
	}

	return slashing, nil
}

func (p *DefaultProposerSlashingProcessor) Validate(ctx context.Context, slashing *pb.ProposerSlashing, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, slashing)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *DefaultProposerSlashingProcessor) Process(ctx context.Context, slashing *pb.ProposerSlashing, from string) error {
	if p.Handler == nil {
		p.Log.Debug("No handler provided, proposer slashing received but not processed")

		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.Handler(ctx, slashing, peerID)
}

func (p *DefaultProposerSlashingProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that DefaultProposerSlashingProcessor implements ProposerSlashingProcessor.
var _ ProposerSlashingProcessor = (*DefaultProposerSlashingProcessor)(nil)
