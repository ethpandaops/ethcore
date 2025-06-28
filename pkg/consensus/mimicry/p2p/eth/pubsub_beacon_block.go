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

// beaconBlockProcessor handles beacon block messages.
type BeaconBlockProcessor struct {
	ForkDigest  [4]byte
	Encoder     encoder.SszNetworkEncoder
	Handler     func(context.Context, *pb.SignedBeaconBlock, peer.ID) error
	Validator   func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error)
	ScoreParams *pubsub.TopicScoreParams
	Gossipsub   *pubsub.Gossipsub // Reference to gossipsub for subscription management
	Log         logrus.FieldLogger
}

func (p *BeaconBlockProcessor) Topic() string {
	return BeaconBlockTopic(p.ForkDigest)
}

func (p *BeaconBlockProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *BeaconBlockProcessor) Subscribe(ctx context.Context) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.Gossipsub.SubscribeToProcessorTopic(ctx, p.Topic())
}

func (p *BeaconBlockProcessor) Unsubscribe(ctx context.Context) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.Gossipsub.Unsubscribe(p.Topic())
}

func (p *BeaconBlockProcessor) Decode(ctx context.Context, data []byte) (*pb.SignedBeaconBlock, error) {
	block := &pb.SignedBeaconBlock{}
	if err := p.Encoder.DecodeGossip(data, block); err != nil {
		return nil, fmt.Errorf("failed to decode beacon block: %w", err)
	}

	return block, nil
}

func (p *BeaconBlockProcessor) Validate(ctx context.Context, block *pb.SignedBeaconBlock, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, block)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *BeaconBlockProcessor) Process(ctx context.Context, block *pb.SignedBeaconBlock, from string) error {
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

func (p *BeaconBlockProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return the scoreParams provided by the user, or nil for default/no scoring
	return p.ScoreParams
}

// Compile-time check that beaconBlockProcessor implements pubsub.Processor.
var _ pubsub.Processor[*pb.SignedBeaconBlock] = (*BeaconBlockProcessor)(nil)
