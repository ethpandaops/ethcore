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

// BeaconchainGossipsub provides Ethereum consensus layer gossipsub functionality.
type BeaconchainGossipsub struct {
	*pubsub.Gossipsub
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Log        logrus.FieldLogger
}

// NewBeaconchainGossipsub creates a new Ethereum beacon chain gossipsub wrapper.
func NewBeaconchainGossipsub(
	ps *pubsub.Gossipsub,
	forkDigest [4]byte,
	encoder encoder.SszNetworkEncoder,
	log logrus.FieldLogger,
) *BeaconchainGossipsub {
	return &BeaconchainGossipsub{
		Gossipsub:  ps,
		ForkDigest: forkDigest,
		Encoder:    encoder,
		Log:        log.WithField("component", "beaconchain_gossipsub"),
	}
}

// RegisterBeaconBlock registers a beacon block processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterBeaconBlock(
	Validator func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.SignedBeaconBlock, peer.ID) error,
	ScoreParams *pubsub.TopicScoreParams, // Optional: provide custom scoring parameters
) error {
	processor := &BeaconBlockProcessor{
		ForkDigest:  g.ForkDigest,
		Encoder:     g.Encoder,
		Validator:   Validator,
		Handler:     Handler,
		ScoreParams: ScoreParams,
		Log:         g.Log.WithField("processor", "beacon_block"),
		Gossipsub:   g.Gossipsub,
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterAttestation registers an attestation processor for specific subnets with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterAttestation(
	Subnets []uint64,
	Validator func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.Attestation, uint64, peer.ID) error,
) error {
	if len(Subnets) == 0 {
		return fmt.Errorf("no subnets specified")
	}

	processor := &AttestationProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Subnets:    Subnets,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "attestation"),
		Gossipsub:  g.Gossipsub,
	}

	// Register multi-processor with a unique name based on subnets
	name := fmt.Sprintf("attestation_subnets_%v", Subnets)

	return pubsub.RegisterMultiProcessor(g.Gossipsub, name, processor)
}

// RegisterAggregateAndProof registers an aggregate and proof processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterAggregateAndProof(
	Validator func(context.Context, *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error,
) error {
	processor := &AggregateProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "aggregate"),
		Gossipsub:  g.Gossipsub,
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterSyncCommittee registers a sync committee processor for specific subnets with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterSyncCommittee(
	Subnets []uint64,
	Validator func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error), //nolint:staticcheck // deprecated but still functional
	Handler func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error, //nolint:staticcheck // deprecated but still functional
) error {
	if len(Subnets) == 0 {
		return fmt.Errorf("no subnets specified")
	}

	processor := &SyncCommitteeProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Subnets:    Subnets,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "sync_committee"),
		Gossipsub:  g.Gossipsub,
	}

	// Register multi-processor with a unique name based on subnets
	name := fmt.Sprintf("sync_committee_subnets_%v", Subnets)

	return pubsub.RegisterMultiProcessor(g.Gossipsub, name, processor)
}

// RegisterVoluntaryExit registers a voluntary exit processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterVoluntaryExit(
	Validator func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error,
) error {
	processor := &VoluntaryExitProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "voluntary_exit"),
		Gossipsub:  g.Gossipsub,
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterProposerSlashing registers a proposer slashing processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterProposerSlashing(
	Validator func(context.Context, *pb.ProposerSlashing) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.ProposerSlashing, peer.ID) error,
) error {
	processor := &ProposerSlashingProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "proposer_slashing"),
		Gossipsub:  g.Gossipsub,
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterAttesterSlashing registers an attester slashing processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterAttesterSlashing(
	Validator func(context.Context, *pb.AttesterSlashing) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.AttesterSlashing, peer.ID) error,
) error {
	processor := &AttesterSlashingProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "attester_slashing"),
		Gossipsub:  g.Gossipsub,
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterSyncContributionAndProof registers a sync contribution and proof processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterSyncContributionAndProof(
	Validator func(context.Context, *pb.SignedContributionAndProof) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.SignedContributionAndProof, peer.ID) error,
) error {
	processor := &SyncContributionProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "sync_contribution"),
		Gossipsub:  g.Gossipsub,
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterBlsToExecutionChange registers a BLS to execution change processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterBlsToExecutionChange(
	Validator func(context.Context, *pb.SignedBLSToExecutionChange) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.SignedBLSToExecutionChange, peer.ID) error,
) error {
	processor := &BlsToExecutionProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "bls_to_execution"),
		Gossipsub:  g.Gossipsub,
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// SubscribeBeaconBlock subscribes to beacon block messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeBeaconBlock(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &BeaconBlockProcessor{ForkDigest: g.ForkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no beacon block processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*BeaconBlockProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for beacon block")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeAttestation subscribes to attestation messages for previously registered subnets.
func (g *BeaconchainGossipsub) SubscribeAttestation(ctx context.Context, name string) error {
	processorInterface, exists := g.GetRegisteredMultiProcessor(name)
	if !exists {
		return fmt.Errorf("no attestation processor registered with name: %s", name)
	}

	processor, ok := processorInterface.(*AttestationProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for attestation")
	}

	return pubsub.SubscribeMultiWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeAggregateAndProof subscribes to aggregate and proof messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeAggregateAndProof(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &AggregateProcessor{ForkDigest: g.ForkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no aggregate and proof processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*AggregateProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for aggregate and proof")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeSyncCommittee subscribes to sync committee messages for previously registered subnets.
func (g *BeaconchainGossipsub) SubscribeSyncCommittee(ctx context.Context, name string) error {
	processorInterface, exists := g.GetRegisteredMultiProcessor(name)
	if !exists {
		return fmt.Errorf("no sync committee processor registered with name: %s", name)
	}

	processor, ok := processorInterface.(*SyncCommitteeProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for sync committee")
	}

	return pubsub.SubscribeMultiWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeVoluntaryExit subscribes to voluntary exit messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeVoluntaryExit(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &VoluntaryExitProcessor{ForkDigest: g.ForkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no voluntary exit processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*VoluntaryExitProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for voluntary exit")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeProposerSlashing subscribes to proposer slashing messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeProposerSlashing(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &ProposerSlashingProcessor{ForkDigest: g.ForkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no proposer slashing processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*ProposerSlashingProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for proposer slashing")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeAttesterSlashing subscribes to attester slashing messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeAttesterSlashing(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &AttesterSlashingProcessor{ForkDigest: g.ForkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no attester slashing processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*AttesterSlashingProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for attester slashing")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeSyncContributionAndProof subscribes to sync contribution and proof messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeSyncContributionAndProof(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &SyncContributionProcessor{ForkDigest: g.ForkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no sync contribution and proof processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*SyncContributionProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for sync contribution and proof")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeBlsToExecutionChange subscribes to BLS to execution change messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeBlsToExecutionChange(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &BlsToExecutionProcessor{ForkDigest: g.ForkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no BLS to execution change processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*BlsToExecutionProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for BLS to execution change")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}
