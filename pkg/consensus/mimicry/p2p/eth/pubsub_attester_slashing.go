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

// Attester slashing topic constants and functions
const (
	AttesterSlashingTopicName = "attester_slashing"
)

// AttesterSlashingTopic constructs the attester slashing gossipsub topic name
func AttesterSlashingTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, AttesterSlashingTopicName)
}

// attesterSlashingProcessor handles attester slashing messages
type attesterSlashingProcessor struct {
	forkDigest [4]byte
	encoder    encoder.SszNetworkEncoder
	handler    func(context.Context, *pb.AttesterSlashing, peer.ID) error
	validator  func(context.Context, *pb.AttesterSlashing) (pubsub.ValidationResult, error)
	gossipsub  *pubsub.Gossipsub
	log        logrus.FieldLogger
}

func (p *attesterSlashingProcessor) Topic() string {
	return AttesterSlashingTopic(p.forkDigest)
}

func (p *attesterSlashingProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *attesterSlashingProcessor) Subscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.gossipsub.SubscribeToProcessorTopic(ctx, p.Topic())
}

func (p *attesterSlashingProcessor) Unsubscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.gossipsub.Unsubscribe(p.Topic())
}

func (p *attesterSlashingProcessor) Decode(ctx context.Context, data []byte) (*pb.AttesterSlashing, error) {
	slashing := &pb.AttesterSlashing{}
	if err := p.encoder.DecodeGossip(data, slashing); err != nil {
		return nil, fmt.Errorf("failed to decode attester slashing: %w", err)
	}

	return slashing, nil
}

func (p *attesterSlashingProcessor) Validate(ctx context.Context, slashing *pb.AttesterSlashing, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external validator function
	if p.validator != nil {
		return p.validator(ctx, slashing)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *attesterSlashingProcessor) Process(ctx context.Context, slashing *pb.AttesterSlashing, from string) error {
	if p.handler == nil {
		p.log.Debug("No handler provided, attester slashing received but not processed")
		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.handler(ctx, slashing, peerID)
}

func (p *attesterSlashingProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that attesterSlashingProcessor implements pubsub.Processor
var _ pubsub.Processor[*pb.AttesterSlashing] = (*attesterSlashingProcessor)(nil)
