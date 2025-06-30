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

// Beacon block topic constants and functions.
const (
	BeaconBlockTopicName = "beacon_block"
)

// BeaconBlockTopic constructs the beacon block gossipsub topic name.
func BeaconBlockTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BeaconBlockTopicName)
}

// BeaconBlockProcessor defines the interface for processing beacon block messages.
type BeaconBlockProcessor interface {
	pubsub.Processor[*pb.SignedBeaconBlock]
}

// DefaultBeaconBlockProcessor handles beacon block messages.
type DefaultBeaconBlockProcessor struct {
	ForkDigest  [4]byte
	Encoder     encoder.SszNetworkEncoder
	Handler     func(context.Context, *pb.SignedBeaconBlock, peer.ID) error
	Validator   func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error)
	ScoreParams *pubsub.TopicScoreParams
	Log         logrus.FieldLogger
}

func (p *DefaultBeaconBlockProcessor) Topic() string {
	return BeaconBlockTopic(p.ForkDigest)
}

func (p *DefaultBeaconBlockProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *DefaultBeaconBlockProcessor) Decode(ctx context.Context, data []byte) (*pb.SignedBeaconBlock, error) {
	block := &pb.SignedBeaconBlock{}
	if err := p.Encoder.DecodeGossip(data, block); err != nil {
		return nil, fmt.Errorf("failed to decode beacon block: %w", err)
	}

	return block, nil
}

func (p *DefaultBeaconBlockProcessor) Validate(ctx context.Context, block *pb.SignedBeaconBlock, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, block)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *DefaultBeaconBlockProcessor) Process(ctx context.Context, block *pb.SignedBeaconBlock, from string) error {
	if p.Handler == nil {
		p.Log.Debug("No handler provided, message received but not processed")

		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.Handler(ctx, block, peerID)
}

func (p *DefaultBeaconBlockProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return the scoreParams provided by the user, or nil for default/no scoring
	return p.ScoreParams
}

// Compile-time check that DefaultBeaconBlockProcessor implements BeaconBlockProcessor.
var _ BeaconBlockProcessor = (*DefaultBeaconBlockProcessor)(nil)
