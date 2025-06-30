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

// VoluntaryExitProcessor defines the interface for processing voluntary exit messages.
type VoluntaryExitProcessor interface {
	pubsub.Processor[*pb.SignedVoluntaryExit]
}

// DefaultVoluntaryExitProcessor handles voluntary exit messages.
type DefaultVoluntaryExitProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Handler    func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error
	Validator  func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error)
	Log        logrus.FieldLogger
}

func (p *DefaultVoluntaryExitProcessor) Topic() string {
	return VoluntaryExitTopic(p.ForkDigest[:])
}

func (p *DefaultVoluntaryExitProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}


func (p *DefaultVoluntaryExitProcessor) Decode(ctx context.Context, data []byte) (*pb.SignedVoluntaryExit, error) {
	exit := &pb.SignedVoluntaryExit{}
	if err := p.Encoder.DecodeGossip(data, exit); err != nil {
		return nil, fmt.Errorf("failed to decode voluntary exit: %w", err)
	}

	return exit, nil
}

func (p *DefaultVoluntaryExitProcessor) Validate(ctx context.Context, exit *pb.SignedVoluntaryExit, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, exit)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *DefaultVoluntaryExitProcessor) Process(ctx context.Context, exit *pb.SignedVoluntaryExit, from string) error {
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

func (p *DefaultVoluntaryExitProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that DefaultVoluntaryExitProcessor implements VoluntaryExitProcessor.
var _ VoluntaryExitProcessor = (*DefaultVoluntaryExitProcessor)(nil)
