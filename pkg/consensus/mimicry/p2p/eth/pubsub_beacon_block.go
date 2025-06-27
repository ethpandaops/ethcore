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

// Beacon block topic constants and functions
const (
	BeaconBlockTopicName = "beacon_block"
)

// BeaconBlockTopic constructs the beacon block gossipsub topic name
func BeaconBlockTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BeaconBlockTopicName)
}

// beaconBlockProcessor handles beacon block messages
type beaconBlockProcessor struct {
	forkDigest  [4]byte
	encoder     encoder.SszNetworkEncoder
	handler     func(context.Context, *pb.SignedBeaconBlock, peer.ID) error
	validator   func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error)
	scoreParams *pubsub.TopicScoreParams
	gossipsub   *pubsub.Gossipsub // Reference to gossipsub for subscription management
	log         logrus.FieldLogger
}

func (p *beaconBlockProcessor) Topic() string {
	return BeaconBlockTopic(p.forkDigest)
}

func (p *beaconBlockProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *beaconBlockProcessor) Subscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.gossipsub.SubscribeToProcessorTopic(ctx, p.Topic())
}

func (p *beaconBlockProcessor) Unsubscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.gossipsub.Unsubscribe(p.Topic())
}

func (p *beaconBlockProcessor) Decode(ctx context.Context, data []byte) (*pb.SignedBeaconBlock, error) {
	block := &pb.SignedBeaconBlock{}
	if err := p.encoder.DecodeGossip(data, block); err != nil {
		return nil, fmt.Errorf("failed to decode beacon block: %w", err)
	}

	return block, nil
}

func (p *beaconBlockProcessor) Validate(ctx context.Context, block *pb.SignedBeaconBlock, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external validator function
	if p.validator != nil {
		return p.validator(ctx, block)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *beaconBlockProcessor) Process(ctx context.Context, block *pb.SignedBeaconBlock, from string) error {
	if p.handler == nil {
		p.log.Debug("No handler provided, message received but not processed")
		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.handler(ctx, block, peerID)
}

func (p *beaconBlockProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return the scoreParams provided by the user, or nil for default/no scoring
	return p.scoreParams
}

// Compile-time check that beaconBlockProcessor implements pubsub.Processor
var _ pubsub.Processor[*pb.SignedBeaconBlock] = (*beaconBlockProcessor)(nil)
