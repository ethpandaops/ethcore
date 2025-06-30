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

// Attester slashing topic constants and functions.
const (
	AttesterSlashingTopicName = "attester_slashing"
)

// AttesterSlashingTopic constructs the attester slashing gossipsub topic name.
func AttesterSlashingTopic(forkDigest [4]byte) string {
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, AttesterSlashingTopicName)
}

// AttesterSlashingProcessor defines the interface for processing attester slashing messages.
type AttesterSlashingProcessor interface {
	pubsub.Processor[*pb.AttesterSlashing]
}

// DefaultAttesterSlashingProcessor handles attester slashing messages.
type DefaultAttesterSlashingProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Handler    func(context.Context, *pb.AttesterSlashing, peer.ID) error
	Validator  func(context.Context, *pb.AttesterSlashing) (pubsub.ValidationResult, error)
	Log        logrus.FieldLogger
}

func (p *DefaultAttesterSlashingProcessor) Topic() string {
	return AttesterSlashingTopic(p.ForkDigest)
}

func (p *DefaultAttesterSlashingProcessor) AllPossibleTopics() []string {
	return []string{p.Topic()}
}


func (p *DefaultAttesterSlashingProcessor) Decode(ctx context.Context, data []byte) (*pb.AttesterSlashing, error) {
	slashing := &pb.AttesterSlashing{}
	if err := p.Encoder.DecodeGossip(data, slashing); err != nil {
		return nil, fmt.Errorf("failed to decode attester slashing: %w", err)
	}

	return slashing, nil
}

func (p *DefaultAttesterSlashingProcessor) Validate(ctx context.Context, slashing *pb.AttesterSlashing, from string) (pubsub.ValidationResult, error) {
	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, slashing)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *DefaultAttesterSlashingProcessor) Process(ctx context.Context, slashing *pb.AttesterSlashing, from string) error {
	if p.Handler == nil {
		p.Log.Debug("No handler provided, attester slashing received but not processed")

		return nil
	}

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.Handler(ctx, slashing, peerID)
}

func (p *DefaultAttesterSlashingProcessor) GetTopicScoreParams() *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that DefaultAttesterSlashingProcessor implements AttesterSlashingProcessor.
var _ AttesterSlashingProcessor = (*DefaultAttesterSlashingProcessor)(nil)
