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
	forkDigest [4]byte
	encoder    encoder.SszNetworkEncoder
	log        logrus.FieldLogger
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
		forkDigest: forkDigest,
		encoder:    encoder,
		log:        log.WithField("component", "beaconchain_gossipsub"),
	}
}

// RegisterBeaconBlock registers a beacon block processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterBeaconBlock(
	validator func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error),
	handler func(context.Context, *pb.SignedBeaconBlock, peer.ID) error,
	scoreParams *pubsub.TopicScoreParams, // Optional: provide custom scoring parameters
) error {
	processor := &beaconBlockProcessor{
		forkDigest:  g.forkDigest,
		encoder:     g.encoder,
		validator:   validator,
		handler:     handler,
		scoreParams: scoreParams,
		log:         g.log.WithField("processor", "beacon_block"),
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterAttestation registers an attestation processor for specific subnets with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterAttestation(
	subnets []uint64,
	validator func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error),
	handler func(context.Context, *pb.Attestation, uint64, peer.ID) error,
) error {
	if len(subnets) == 0 {
		return fmt.Errorf("no subnets specified")
	}

	processor := &attestationProcessor{
		forkDigest: g.forkDigest,
		encoder:    g.encoder,
		subnets:    subnets,
		validator:  validator,
		handler:    handler,
		log:        g.log.WithField("processor", "attestation"),
	}

	// Register multi-processor with a unique name based on subnets
	name := fmt.Sprintf("attestation_subnets_%v", subnets)

	return pubsub.RegisterMultiProcessor(g.Gossipsub, name, processor)
}

// RegisterAggregateAndProof registers an aggregate and proof processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterAggregateAndProof(
	validator func(context.Context, *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error),
	handler func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error,
) error {
	processor := &aggregateProcessor{
		forkDigest: g.forkDigest,
		encoder:    g.encoder,
		validator:  validator,
		handler:    handler,
		log:        g.log.WithField("processor", "aggregate"),
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterSyncCommittee registers a sync committee processor for specific subnets with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterSyncCommittee(
	subnets []uint64,
	validator func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error), //nolint:staticcheck // deprecated but still functional
	handler func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error, //nolint:staticcheck // deprecated but still functional
) error {
	if len(subnets) == 0 {
		return fmt.Errorf("no subnets specified")
	}

	processor := &syncCommitteeProcessor{
		forkDigest: g.forkDigest,
		encoder:    g.encoder,
		subnets:    subnets,
		validator:  validator,
		handler:    handler,
		log:        g.log.WithField("processor", "sync_committee"),
	}

	// Register multi-processor with a unique name based on subnets
	name := fmt.Sprintf("sync_committee_subnets_%v", subnets)

	return pubsub.RegisterMultiProcessor(g.Gossipsub, name, processor)
}

// RegisterVoluntaryExit registers a voluntary exit processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterVoluntaryExit(
	validator func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error),
	handler func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error,
) error {
	processor := &voluntaryExitProcessor{
		forkDigest: g.forkDigest,
		encoder:    g.encoder,
		validator:  validator,
		handler:    handler,
		log:        g.log.WithField("processor", "voluntary_exit"),
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterProposerSlashing registers a proposer slashing processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterProposerSlashing(
	validator func(context.Context, *pb.ProposerSlashing) (pubsub.ValidationResult, error),
	handler func(context.Context, *pb.ProposerSlashing, peer.ID) error,
) error {
	processor := &proposerSlashingProcessor{
		forkDigest: g.forkDigest,
		encoder:    g.encoder,
		validator:  validator,
		handler:    handler,
		log:        g.log.WithField("processor", "proposer_slashing"),
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterAttesterSlashing registers an attester slashing processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterAttesterSlashing(
	validator func(context.Context, *pb.AttesterSlashing) (pubsub.ValidationResult, error),
	handler func(context.Context, *pb.AttesterSlashing, peer.ID) error,
) error {
	processor := &attesterSlashingProcessor{
		forkDigest: g.forkDigest,
		encoder:    g.encoder,
		validator:  validator,
		handler:    handler,
		log:        g.log.WithField("processor", "attester_slashing"),
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterSyncContributionAndProof registers a sync contribution and proof processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterSyncContributionAndProof(
	validator func(context.Context, *pb.SignedContributionAndProof) (pubsub.ValidationResult, error),
	handler func(context.Context, *pb.SignedContributionAndProof, peer.ID) error,
) error {
	processor := &syncContributionProcessor{
		forkDigest: g.forkDigest,
		encoder:    g.encoder,
		validator:  validator,
		handler:    handler,
		log:        g.log.WithField("processor", "sync_contribution"),
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// RegisterBlsToExecutionChange registers a BLS to execution change processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) RegisterBlsToExecutionChange(
	validator func(context.Context, *pb.SignedBLSToExecutionChange) (pubsub.ValidationResult, error),
	handler func(context.Context, *pb.SignedBLSToExecutionChange, peer.ID) error,
) error {
	processor := &blsToExecutionProcessor{
		forkDigest: g.forkDigest,
		encoder:    g.encoder,
		validator:  validator,
		handler:    handler,
		log:        g.log.WithField("processor", "bls_to_execution"),
	}

	return pubsub.RegisterProcessor(g.Gossipsub, processor)
}

// SubscribeBeaconBlock subscribes to beacon block messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeBeaconBlock(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &beaconBlockProcessor{forkDigest: g.forkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no beacon block processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*beaconBlockProcessor)
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

	processor, ok := processorInterface.(*attestationProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for attestation")
	}

	return pubsub.SubscribeMultiWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeAggregateAndProof subscribes to aggregate and proof messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeAggregateAndProof(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &aggregateProcessor{forkDigest: g.forkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no aggregate and proof processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*aggregateProcessor)
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

	processor, ok := processorInterface.(*syncCommitteeProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for sync committee")
	}

	return pubsub.SubscribeMultiWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeVoluntaryExit subscribes to voluntary exit messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeVoluntaryExit(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &voluntaryExitProcessor{forkDigest: g.forkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no voluntary exit processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*voluntaryExitProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for voluntary exit")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeProposerSlashing subscribes to proposer slashing messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeProposerSlashing(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &proposerSlashingProcessor{forkDigest: g.forkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no proposer slashing processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*proposerSlashingProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for proposer slashing")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeAttesterSlashing subscribes to attester slashing messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeAttesterSlashing(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &attesterSlashingProcessor{forkDigest: g.forkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no attester slashing processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*attesterSlashingProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for attester slashing")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeSyncContributionAndProof subscribes to sync contribution and proof messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeSyncContributionAndProof(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &syncContributionProcessor{forkDigest: g.forkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no sync contribution and proof processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*syncContributionProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for sync contribution and proof")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeBlsToExecutionChange subscribes to BLS to execution change messages using a previously registered processor.
func (g *BeaconchainGossipsub) SubscribeBlsToExecutionChange(ctx context.Context) error {
	// Create a temporary processor to get the correct topic
	tempProcessor := &blsToExecutionProcessor{forkDigest: g.forkDigest}
	topic := tempProcessor.Topic()

	processorInterface, exists := g.GetRegisteredProcessor(topic)
	if !exists {
		return fmt.Errorf("no BLS to execution change processor registered for topic: %s", topic)
	}

	processor, ok := processorInterface.(*blsToExecutionProcessor)
	if !ok {
		return fmt.Errorf("invalid processor type for BLS to execution change")
	}

	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}
