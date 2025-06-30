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

// Sync contribution and proof topic constants and functions.
const (
	SyncContributionAndProofTopicName = "sync_committee_contribution_and_proof"
)

// SyncContributionAndProofTopic constructs the sync contribution and proof gossipsub topic name.
func SyncContributionAndProofTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, SyncContributionAndProofTopicName)
}

// syncContributionProcessor handles sync contribution and proof messages.
type SyncContributionProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Handler    func(context.Context, *pb.SignedContributionAndProof, peer.ID) error
	Validator  func(context.Context, *pb.SignedContributionAndProof) (pubsub.ValidationResult, error)
	Gossipsub  *pubsub.Gossipsub
	Log        logrus.FieldLogger
}

func (p *SyncContributionProcessor) Topic() string {
	return SyncContributionAndProofTopic(p.ForkDigest)
}

func (p *SyncContributionProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *SyncContributionProcessor) Subscribe(ctx context.Context) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.Gossipsub.SubscribeToProcessorTopic(ctx, p.Topic())
}

func (p *SyncContributionProcessor) Unsubscribe(ctx context.Context) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.Gossipsub.Unsubscribe(p.Topic())
}

func (p *SyncContributionProcessor) Decode(ctx context.Context, data []byte) (*pb.SignedContributionAndProof, error) {
	contrib := &pb.SignedContributionAndProof{}
	if err := p.Encoder.DecodeGossip(data, contrib); err != nil {
		return nil, fmt.Errorf("failed to decode sync contribution and proof: %w", err)
	}

	return contrib, nil
}

func (p *SyncContributionProcessor) Validate(ctx context.Context, contrib *pb.SignedContributionAndProof, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, contrib)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *SyncContributionProcessor) Process(ctx context.Context, contrib *pb.SignedContributionAndProof, from string) error {
	if p.Handler == nil {
		p.Log.Debug("No handler provided, sync contribution received but not processed")

		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.Handler(ctx, contrib, peerID)
}

func (p *SyncContributionProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that syncContributionProcessor implements pubsub.Processor.
var _ pubsub.Processor[*pb.SignedContributionAndProof] = (*SyncContributionProcessor)(nil)
