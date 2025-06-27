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

// VoluntaryExitTopic returns the topic name for voluntary exit messages
func VoluntaryExitTopic(forkDigest []byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, "voluntary_exit")
}

// voluntaryExitProcessor handles voluntary exit messages
type voluntaryExitProcessor struct {
	forkDigest [4]byte
	encoder    encoder.SszNetworkEncoder
	handler    func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error
	validator  func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error)
	gossipsub  *pubsub.Gossipsub
	log        logrus.FieldLogger
}

func (p *voluntaryExitProcessor) Topic() string {
	return VoluntaryExitTopic(p.forkDigest[:])
}

func (p *voluntaryExitProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *voluntaryExitProcessor) Subscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.gossipsub.SubscribeToProcessorTopic(ctx, p.Topic())
}

func (p *voluntaryExitProcessor) Unsubscribe(ctx context.Context) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.gossipsub.Unsubscribe(p.Topic())
}

func (p *voluntaryExitProcessor) Decode(ctx context.Context, data []byte) (*pb.SignedVoluntaryExit, error) {
	exit := &pb.SignedVoluntaryExit{}
	if err := p.encoder.DecodeGossip(data, exit); err != nil {
		return nil, fmt.Errorf("failed to decode voluntary exit: %w", err)
	}

	return exit, nil
}

func (p *voluntaryExitProcessor) Validate(ctx context.Context, exit *pb.SignedVoluntaryExit, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external validator function
	if p.validator != nil {
		return p.validator(ctx, exit)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *voluntaryExitProcessor) Process(ctx context.Context, exit *pb.SignedVoluntaryExit, from string) error {
	if p.handler == nil {
		p.log.Debug("No handler provided, voluntary exit received but not processed")
		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.handler(ctx, exit, peerID)
}

func (p *voluntaryExitProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that voluntaryExitProcessor implements pubsub.Processor
var _ pubsub.Processor[*pb.SignedVoluntaryExit] = (*voluntaryExitProcessor)(nil)
