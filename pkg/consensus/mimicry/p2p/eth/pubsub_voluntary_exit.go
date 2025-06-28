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

// VoluntaryExitTopic returns the topic name for voluntary exit messages.
func VoluntaryExitTopic(forkDigest []byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, "voluntary_exit")
}

// voluntaryExitProcessor handles voluntary exit messages.
type VoluntaryExitProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Handler    func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error
	Validator  func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error)
	Gossipsub  *pubsub.Gossipsub
	Log        logrus.FieldLogger
}

func (p *VoluntaryExitProcessor) Topic() string {
	return VoluntaryExitTopic(p.ForkDigest[:])
}

func (p *VoluntaryExitProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}

func (p *VoluntaryExitProcessor) Subscribe(ctx context.Context) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.Gossipsub.SubscribeToProcessorTopic(ctx, p.Topic())
}

func (p *VoluntaryExitProcessor) Unsubscribe(ctx context.Context) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	return p.Gossipsub.Unsubscribe(p.Topic())
}

func (p *VoluntaryExitProcessor) Decode(ctx context.Context, data []byte) (*pb.SignedVoluntaryExit, error) {
	exit := &pb.SignedVoluntaryExit{}
	if err := p.Encoder.DecodeGossip(data, exit); err != nil {
		return nil, fmt.Errorf("failed to decode voluntary exit: %w", err)
	}

	return exit, nil
}

func (p *VoluntaryExitProcessor) Validate(ctx context.Context, exit *pb.SignedVoluntaryExit, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, exit)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *VoluntaryExitProcessor) Process(ctx context.Context, exit *pb.SignedVoluntaryExit, from string) error {
	if p.Handler == nil {
		p.Log.Debug("No handler provided, voluntary exit received but not processed")

		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.Handler(ctx, exit, peerID)
}

func (p *VoluntaryExitProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that voluntaryExitProcessor implements pubsub.Processor.
var _ pubsub.Processor[*pb.SignedVoluntaryExit] = (*VoluntaryExitProcessor)(nil)
