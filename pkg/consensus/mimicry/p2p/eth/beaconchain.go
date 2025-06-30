package eth

import (
	"context"

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

// CreateBeaconBlockProcessor creates a beacon block processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) CreateBeaconBlockProcessor(
	Validator func(context.Context, *pb.SignedBeaconBlock) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.SignedBeaconBlock, peer.ID) error,
	ScoreParams *pubsub.TopicScoreParams, // Optional: provide custom scoring parameters
) *DefaultBeaconBlockProcessor {
	return &DefaultBeaconBlockProcessor{
		ForkDigest:  g.ForkDigest,
		Encoder:     g.Encoder,
		Validator:   Validator,
		Handler:     Handler,
		ScoreParams: ScoreParams,
		Log:         g.Log.WithField("processor", "beacon_block"),
	}
}

// CreateAttestationProcessor creates an attestation processor for specific subnets with custom validation and processing functions.
func (g *BeaconchainGossipsub) CreateAttestationProcessor(
	Subnets []uint64,
	Validator func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.Attestation, uint64, peer.ID) error,
) AttestationProcessor {
	return &DefaultAttestationProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Subnets:    Subnets,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "attestation"),
	}
}

// CreateAggregateAndProofProcessor creates an aggregate and proof processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) CreateAggregateAndProofProcessor(
	Validator func(context.Context, *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error,
) *DefaultAggregateProcessor {
	return &DefaultAggregateProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "aggregate"),
	}
}

// CreateSyncCommitteeProcessor creates a sync committee processor for specific subnets with custom validation and processing functions.
func (g *BeaconchainGossipsub) CreateSyncCommitteeProcessor(
	Subnets []uint64,
	Validator func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error), //nolint:staticcheck // deprecated but still functional
	Handler func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error, //nolint:staticcheck // deprecated but still functional
) SyncCommitteeProcessor {
	return &DefaultSyncCommitteeProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Subnets:    Subnets,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "sync_committee"),
	}
}

// CreateVoluntaryExitProcessor creates a voluntary exit processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) CreateVoluntaryExitProcessor(
	Validator func(context.Context, *pb.SignedVoluntaryExit) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error,
) *DefaultVoluntaryExitProcessor {
	return &DefaultVoluntaryExitProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "voluntary_exit"),
	}
}

// CreateProposerSlashingProcessor creates a proposer slashing processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) CreateProposerSlashingProcessor(
	Validator func(context.Context, *pb.ProposerSlashing) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.ProposerSlashing, peer.ID) error,
) *DefaultProposerSlashingProcessor {
	return &DefaultProposerSlashingProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "proposer_slashing"),
	}
}

// CreateAttesterSlashingProcessor creates an attester slashing processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) CreateAttesterSlashingProcessor(
	Validator func(context.Context, *pb.AttesterSlashing) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.AttesterSlashing, peer.ID) error,
) *DefaultAttesterSlashingProcessor {
	return &DefaultAttesterSlashingProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "attester_slashing"),
	}
}

// CreateSyncContributionAndProofProcessor creates a sync contribution and proof processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) CreateSyncContributionAndProofProcessor(
	Validator func(context.Context, *pb.SignedContributionAndProof) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.SignedContributionAndProof, peer.ID) error,
) SyncContributionProcessor {
	return &DefaultSyncContributionProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "sync_contribution"),
	}
}

// CreateBlsToExecutionChangeProcessor creates a BLS to execution change processor with custom validation and processing functions.
func (g *BeaconchainGossipsub) CreateBlsToExecutionChangeProcessor(
	Validator func(context.Context, *pb.SignedBLSToExecutionChange) (pubsub.ValidationResult, error),
	Handler func(context.Context, *pb.SignedBLSToExecutionChange, peer.ID) error,
) *DefaultBlsToExecutionProcessor {
	return &DefaultBlsToExecutionProcessor{
		ForkDigest: g.ForkDigest,
		Encoder:    g.Encoder,
		Validator:  Validator,
		Handler:    Handler,
		Log:        g.Log.WithField("processor", "bls_to_execution"),
	}
}

// SubscribeBeaconBlock subscribes to beacon block messages using the provided processor.
func (g *BeaconchainGossipsub) SubscribeBeaconBlock(ctx context.Context, processor BeaconBlockProcessor) error {
	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeAttestation subscribes to attestation messages using the provided processor.
func (g *BeaconchainGossipsub) SubscribeAttestation(ctx context.Context, processor AttestationProcessor) error {
	return pubsub.SubscribeMultiWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeAggregateAndProof subscribes to aggregate and proof messages using the provided processor.
func (g *BeaconchainGossipsub) SubscribeAggregateAndProof(ctx context.Context, processor AggregateProcessor) error {
	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeSyncCommittee subscribes to sync committee messages using the provided processor.
func (g *BeaconchainGossipsub) SubscribeSyncCommittee(ctx context.Context, processor SyncCommitteeProcessor) error {
	return pubsub.SubscribeMultiWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeVoluntaryExit subscribes to voluntary exit messages using the provided processor.
func (g *BeaconchainGossipsub) SubscribeVoluntaryExit(ctx context.Context, processor VoluntaryExitProcessor) error {
	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeProposerSlashing subscribes to proposer slashing messages using the provided processor.
func (g *BeaconchainGossipsub) SubscribeProposerSlashing(ctx context.Context, processor ProposerSlashingProcessor) error {
	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeAttesterSlashing subscribes to attester slashing messages using the provided processor.
func (g *BeaconchainGossipsub) SubscribeAttesterSlashing(ctx context.Context, processor AttesterSlashingProcessor) error {
	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeSyncContributionAndProof subscribes to sync contribution and proof messages using the provided processor.
func (g *BeaconchainGossipsub) SubscribeSyncContributionAndProof(ctx context.Context, processor SyncContributionProcessor) error {
	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}

// SubscribeBlsToExecutionChange subscribes to BLS to execution change messages using the provided processor.
func (g *BeaconchainGossipsub) SubscribeBlsToExecutionChange(ctx context.Context, processor BlsToExecutionProcessor) error {
	return pubsub.SubscribeWithProcessor(g.Gossipsub, ctx, processor)
}
