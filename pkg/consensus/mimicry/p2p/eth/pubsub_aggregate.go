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

// Aggregate and proof topic constants and functions.
const (
	BeaconAggregateAndProofTopicName = "beacon_aggregate_and_proof"
)

// BeaconAggregateAndProofTopic constructs the aggregate and proof gossipsub topic name.
func BeaconAggregateAndProofTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BeaconAggregateAndProofTopicName)
}

// AggregateProcessor defines the interface for processing aggregate and proof messages.
type AggregateProcessor interface {
	pubsub.Processor[*pb.AggregateAttestationAndProof]
}

// DefaultAggregateProcessor handles aggregate and proof messages.
type DefaultAggregateProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Handler    func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error
	Validator  func(context.Context, *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error)
	Log        logrus.FieldLogger
}

func (p *DefaultAggregateProcessor) Topic() string {
	return BeaconAggregateAndProofTopic(p.ForkDigest)
}

func (p *DefaultAggregateProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}


func (p *DefaultAggregateProcessor) Decode(ctx context.Context, data []byte) (*pb.AggregateAttestationAndProof, error) {
	aggProof := &pb.AggregateAttestationAndProof{}
	if err := p.Encoder.DecodeGossip(data, aggProof); err != nil {
		return nil, fmt.Errorf("failed to decode aggregate and proof: %w", err)
	}

	return aggProof, nil
}

func (p *DefaultAggregateProcessor) Validate(ctx context.Context, aggProof *pb.AggregateAttestationAndProof, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, aggProof)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *DefaultAggregateProcessor) Process(ctx context.Context, aggProof *pb.AggregateAttestationAndProof, from string) error {
	if p.Handler == nil {
		p.Log.Debug("No handler provided, aggregate proof received but not processed")

		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.Handler(ctx, aggProof, peerID)
}

func (p *DefaultAggregateProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that DefaultAggregateProcessor implements AggregateProcessor.
var _ AggregateProcessor = (*DefaultAggregateProcessor)(nil)
