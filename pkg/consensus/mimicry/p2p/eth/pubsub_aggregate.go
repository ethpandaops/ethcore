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

// Aggregate and proof topic constants and functions
const (
	BeaconAggregateAndProofTopicName = "beacon_aggregate_and_proof"
)

// BeaconAggregateAndProofTopic constructs the aggregate and proof gossipsub topic name
func BeaconAggregateAndProofTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BeaconAggregateAndProofTopicName)
}

// aggregateProcessor handles aggregate and proof messages
type aggregateProcessor struct {
	forkDigest [4]byte
	encoder    encoder.SszNetworkEncoder
	handler    func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error
	validator  func(context.Context, *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error)
	gossipsub  *pubsub.Gossipsub
	log        logrus.FieldLogger
}

func (p *aggregateProcessor) Topic() string {
	return BeaconAggregateAndProofTopic(p.forkDigest)
}

func (p *aggregateProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *aggregateProcessor) Subscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.gossipsub.SubscribeToProcessorTopic(ctx, p.Topic())
}

func (p *aggregateProcessor) Unsubscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.gossipsub.Unsubscribe(p.Topic())
}

func (p *aggregateProcessor) Decode(ctx context.Context, data []byte) (*pb.AggregateAttestationAndProof, error) {
	aggProof := &pb.AggregateAttestationAndProof{}
	if err := p.encoder.DecodeGossip(data, aggProof); err != nil {
		return nil, fmt.Errorf("failed to decode aggregate and proof: %w", err)
	}

	return aggProof, nil
}

func (p *aggregateProcessor) Validate(ctx context.Context, aggProof *pb.AggregateAttestationAndProof, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external validator function
	if p.validator != nil {
		return p.validator(ctx, aggProof)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *aggregateProcessor) Process(ctx context.Context, aggProof *pb.AggregateAttestationAndProof, from string) error {
	if p.handler == nil {
		p.log.Debug("No handler provided, aggregate proof received but not processed")

		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.handler(ctx, aggProof, peerID)
}

func (p *aggregateProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that aggregateProcessor implements pubsub.Processor
var _ pubsub.Processor[*pb.AggregateAttestationAndProof] = (*aggregateProcessor)(nil)
