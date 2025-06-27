package eth

import (
	"bytes"
	"context"
	"fmt"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/sirupsen/logrus"
)

// Gossipsub wraps the generic pubsub implementation with Ethereum-specific functionality.
type Gossipsub struct {
	*pubsub.Gossipsub
	forkDigest [4]byte
	encoder    encoder.SszNetworkEncoder
	log        logrus.FieldLogger
}

// NewGossipsub creates an Ethereum-aware gossipsub instance.
func NewGossipsub(log logrus.FieldLogger, ps *pubsub.Gossipsub, forkDigest [4]byte, enc encoder.SszNetworkEncoder) *Gossipsub {
	return &Gossipsub{
		Gossipsub:  ps,
		forkDigest: forkDigest,
		encoder:    enc,
		log:        log.WithField("component", "eth_gossipsub"),
	}
}

// Global topic subscriptions

// SubscribeBeaconBlock subscribes to beacon block messages.
func (g *Gossipsub) SubscribeBeaconBlock(ctx context.Context, handler BeaconBlockHandler) error {
	topic := BeaconBlockTopic(g.forkDigest)

	msgHandler := g.createBeaconBlockHandler(handler)
	return g.Subscribe(ctx, topic, msgHandler)
}

// SubscribeBeaconAggregateAndProof subscribes to aggregate and proof messages.
func (g *Gossipsub) SubscribeBeaconAggregateAndProof(ctx context.Context, handler BeaconAggregateAndProofHandler) error {
	topic := BeaconAggregateAndProofTopic(g.forkDigest)

	msgHandler := g.createAggregateAndProofHandler(handler)
	return g.Subscribe(ctx, topic, msgHandler)
}

// SubscribeVoluntaryExit subscribes to voluntary exit messages.
func (g *Gossipsub) SubscribeVoluntaryExit(ctx context.Context, handler VoluntaryExitHandler) error {
	topic := VoluntaryExitTopic(g.forkDigest)

	msgHandler := g.createVoluntaryExitHandler(handler)
	return g.Subscribe(ctx, topic, msgHandler)
}

// SubscribeProposerSlashing subscribes to proposer slashing messages.
func (g *Gossipsub) SubscribeProposerSlashing(ctx context.Context, handler ProposerSlashingHandler) error {
	topic := ProposerSlashingTopic(g.forkDigest)

	msgHandler := g.createProposerSlashingHandler(handler)
	return g.Subscribe(ctx, topic, msgHandler)
}

// SubscribeAttesterSlashing subscribes to attester slashing messages
func (g *Gossipsub) SubscribeAttesterSlashing(ctx context.Context, handler AttesterSlashingHandler) error {
	topic := AttesterSlashingTopic(g.forkDigest)

	msgHandler := g.createAttesterSlashingHandler(handler)
	return g.Subscribe(ctx, topic, msgHandler)
}

// SubscribeSyncContributionAndProof subscribes to sync contribution and proof messages
func (g *Gossipsub) SubscribeSyncContributionAndProof(ctx context.Context, handler SyncContributionAndProofHandler) error {
	topic := SyncContributionAndProofTopic(g.forkDigest)

	msgHandler := g.createSyncContributionAndProofHandler(handler)
	return g.Subscribe(ctx, topic, msgHandler)
}

// SubscribeBlsToExecutionChange subscribes to BLS to execution change messages
func (g *Gossipsub) SubscribeBlsToExecutionChange(ctx context.Context, handler BlsToExecutionChangeHandler) error {
	topic := BlsToExecutionChangeTopic(g.forkDigest)

	msgHandler := g.createBlsToExecutionChangeHandler(handler)
	return g.Subscribe(ctx, topic, msgHandler)
}

// Subnet subscriptions

// SubscribeAttestation subscribes to attestation messages for a specific subnet
func (g *Gossipsub) SubscribeAttestation(ctx context.Context, subnet uint64, handler AttestationHandler) error {
	if subnet >= AttestationSubnetCount {
		return fmt.Errorf("invalid attestation subnet: %d (max: %d)", subnet, AttestationSubnetCount-1)
	}

	topic := AttestationSubnetTopic(g.forkDigest, subnet)

	msgHandler := g.createAttestationHandler(handler, subnet)
	return g.Subscribe(ctx, topic, msgHandler)
}

// SubscribeSyncCommittee subscribes to sync committee messages for a specific subnet
func (g *Gossipsub) SubscribeSyncCommittee(ctx context.Context, subnet uint64, handler SyncCommitteeHandler) error {
	if subnet >= SyncCommitteeSubnetCount {
		return fmt.Errorf("invalid sync committee subnet: %d (max: %d)", subnet, SyncCommitteeSubnetCount-1)
	}

	topic := SyncCommitteeSubnetTopic(g.forkDigest, subnet)

	msgHandler := g.createSyncCommitteeHandler(handler, subnet)
	return g.Subscribe(ctx, topic, msgHandler)
}

// Bulk subscriptions

// SubscribeAllGlobalTopics subscribes to all global topics using the provided handlers
func (g *Gossipsub) SubscribeAllGlobalTopics(ctx context.Context, handlers *GlobalTopicHandlers) error {
	if handlers == nil {
		return fmt.Errorf("handlers cannot be nil")
	}

	var lastErr error

	if handlers.BeaconBlock != nil {
		if err := g.SubscribeBeaconBlock(ctx, handlers.BeaconBlock); err != nil {
			g.log.WithError(err).Error("Failed to subscribe to beacon block topic")
			lastErr = err
		}
	}

	if handlers.BeaconAggregateAndProof != nil {
		if err := g.SubscribeBeaconAggregateAndProof(ctx, handlers.BeaconAggregateAndProof); err != nil {
			g.log.WithError(err).Error("Failed to subscribe to aggregate and proof topic")
			lastErr = err
		}
	}

	if handlers.VoluntaryExit != nil {
		if err := g.SubscribeVoluntaryExit(ctx, handlers.VoluntaryExit); err != nil {
			g.log.WithError(err).Error("Failed to subscribe to voluntary exit topic")
			lastErr = err
		}
	}

	if handlers.ProposerSlashing != nil {
		if err := g.SubscribeProposerSlashing(ctx, handlers.ProposerSlashing); err != nil {
			g.log.WithError(err).Error("Failed to subscribe to proposer slashing topic")
			lastErr = err
		}
	}

	if handlers.AttesterSlashing != nil {
		if err := g.SubscribeAttesterSlashing(ctx, handlers.AttesterSlashing); err != nil {
			g.log.WithError(err).Error("Failed to subscribe to attester slashing topic")
			lastErr = err
		}
	}

	if handlers.SyncContributionAndProof != nil {
		if err := g.SubscribeSyncContributionAndProof(ctx, handlers.SyncContributionAndProof); err != nil {
			g.log.WithError(err).Error("Failed to subscribe to sync contribution and proof topic")
			lastErr = err
		}
	}

	if handlers.BlsToExecutionChange != nil {
		if err := g.SubscribeBlsToExecutionChange(ctx, handlers.BlsToExecutionChange); err != nil {
			g.log.WithError(err).Error("Failed to subscribe to BLS to execution change topic")
			lastErr = err
		}
	}

	return lastErr
}

// SubscribeAttestationSubnets subscribes to multiple attestation subnets
func (g *Gossipsub) SubscribeAttestationSubnets(ctx context.Context, subnets []uint64, handler AttestationHandler) error {
	var lastErr error

	for _, subnet := range subnets {
		if err := g.SubscribeAttestation(ctx, subnet, handler); err != nil {
			g.log.WithError(err).WithField("subnet", subnet).Error("Failed to subscribe to attestation subnet")
			lastErr = err
		}
	}

	return lastErr
}

// SubscribeSyncCommitteeSubnets subscribes to multiple sync committee subnets
func (g *Gossipsub) SubscribeSyncCommitteeSubnets(ctx context.Context, subnets []uint64, handler SyncCommitteeHandler) error {
	var lastErr error

	for _, subnet := range subnets {
		if err := g.SubscribeSyncCommittee(ctx, subnet, handler); err != nil {
			g.log.WithError(err).WithField("subnet", subnet).Error("Failed to subscribe to sync committee subnet")
			lastErr = err
		}
	}

	return lastErr
}

// Publishing helpers

// PublishBeaconBlock publishes a beacon block
func (g *Gossipsub) PublishBeaconBlock(ctx context.Context, block *pb.SignedBeaconBlock) error {
	topic := BeaconBlockTopic(g.forkDigest)

	var buf bytes.Buffer
	if _, err := g.encoder.EncodeGossip(&buf, block); err != nil {
		return fmt.Errorf("failed to encode beacon block: %w", err)
	}

	return g.Publish(ctx, topic, buf.Bytes())
}

// PublishAttestation publishes an attestation to the appropriate subnet
func (g *Gossipsub) PublishAttestation(ctx context.Context, att *pb.Attestation, subnet uint64) error {
	if subnet >= AttestationSubnetCount {
		return fmt.Errorf("invalid attestation subnet: %d", subnet)
	}

	topic := AttestationSubnetTopic(g.forkDigest, subnet)

	var buf bytes.Buffer
	if _, err := g.encoder.EncodeGossip(&buf, att); err != nil {
		return fmt.Errorf("failed to encode attestation: %w", err)
	}

	return g.Publish(ctx, topic, buf.Bytes())
}

// PublishSyncCommitteeMessage publishes a sync committee message to the appropriate subnet
func (g *Gossipsub) PublishSyncCommitteeMessage(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64) error { //nolint:staticcheck // deprecated but still functional
	if subnet >= SyncCommitteeSubnetCount {
		return fmt.Errorf("invalid sync committee subnet: %d", subnet)
	}

	topic := SyncCommitteeSubnetTopic(g.forkDigest, subnet)

	var buf bytes.Buffer
	if _, err := g.encoder.EncodeGossip(&buf, msg); err != nil {
		return fmt.Errorf("failed to encode sync committee message: %w", err)
	}

	return g.Publish(ctx, topic, buf.Bytes())
}

// Validation helpers

// RegisterBeaconBlockValidator registers a validator for beacon block messages
func (g *Gossipsub) RegisterBeaconBlockValidator(validator BeaconBlockValidator) error {
	topic := BeaconBlockTopic(g.forkDigest)

	pubsubValidator := g.createBeaconBlockValidator(validator)
	return g.RegisterValidator(topic, pubsubValidator)
}

// RegisterAttestationValidator registers a validator for attestation messages on a specific subnet
func (g *Gossipsub) RegisterAttestationValidator(subnet uint64, validator AttestationValidator) error {
	if subnet >= AttestationSubnetCount {
		return fmt.Errorf("invalid attestation subnet: %d", subnet)
	}

	topic := AttestationSubnetTopic(g.forkDigest, subnet)

	pubsubValidator := g.createAttestationValidator(validator, subnet)
	return g.RegisterValidator(topic, pubsubValidator)
}

// Helper methods for creating typed handlers and validators

// createBeaconBlockHandler creates a pubsub message handler for beacon blocks
func (g *Gossipsub) createBeaconBlockHandler(handler BeaconBlockHandler) pubsub.MessageHandler {
	return func(ctx context.Context, msg *pubsub.Message) error {
		block := &pb.SignedBeaconBlock{}
		if err := g.encoder.DecodeGossip(msg.Data, block); err != nil {
			return fmt.Errorf("failed to decode beacon block: %w", err)
		}

		return handler(ctx, block, msg.From)
	}
}

// createAggregateAndProofHandler creates a pubsub message handler for aggregate and proof
func (g *Gossipsub) createAggregateAndProofHandler(handler BeaconAggregateAndProofHandler) pubsub.MessageHandler {
	return func(ctx context.Context, msg *pubsub.Message) error {
		aggProof := &pb.AggregateAttestationAndProof{}
		if err := g.encoder.DecodeGossip(msg.Data, aggProof); err != nil {
			return fmt.Errorf("failed to decode aggregate and proof: %w", err)
		}

		return handler(ctx, aggProof, msg.From)
	}
}

// createAttestationHandler creates a pubsub message handler for attestations
func (g *Gossipsub) createAttestationHandler(handler AttestationHandler, subnet uint64) pubsub.MessageHandler {
	return func(ctx context.Context, msg *pubsub.Message) error {
		att := &pb.Attestation{}
		if err := g.encoder.DecodeGossip(msg.Data, att); err != nil {
			return fmt.Errorf("failed to decode attestation: %w", err)
		}

		return handler(ctx, att, subnet, msg.From)
	}
}

// createSyncCommitteeHandler creates a pubsub message handler for sync committee messages
func (g *Gossipsub) createSyncCommitteeHandler(handler SyncCommitteeHandler, subnet uint64) pubsub.MessageHandler {
	return func(ctx context.Context, msg *pubsub.Message) error {
		syncMsg := &pb.SyncCommitteeMessage{} //nolint:staticcheck // deprecated but still functional
		if err := g.encoder.DecodeGossip(msg.Data, syncMsg); err != nil {
			return fmt.Errorf("failed to decode sync committee message: %w", err)
		}

		return handler(ctx, syncMsg, subnet, msg.From)
	}
}

// createVoluntaryExitHandler creates a pubsub message handler for voluntary exits
func (g *Gossipsub) createVoluntaryExitHandler(handler VoluntaryExitHandler) pubsub.MessageHandler {
	return func(ctx context.Context, msg *pubsub.Message) error {
		exit := &pb.SignedVoluntaryExit{}
		if err := g.encoder.DecodeGossip(msg.Data, exit); err != nil {
			return fmt.Errorf("failed to decode voluntary exit: %w", err)
		}

		return handler(ctx, exit, msg.From)
	}
}

// createProposerSlashingHandler creates a pubsub message handler for proposer slashings
func (g *Gossipsub) createProposerSlashingHandler(handler ProposerSlashingHandler) pubsub.MessageHandler {
	return func(ctx context.Context, msg *pubsub.Message) error {
		slashing := &pb.ProposerSlashing{}
		if err := g.encoder.DecodeGossip(msg.Data, slashing); err != nil {
			return fmt.Errorf("failed to decode proposer slashing: %w", err)
		}

		return handler(ctx, slashing, msg.From)
	}
}

// createAttesterSlashingHandler creates a pubsub message handler for attester slashings
func (g *Gossipsub) createAttesterSlashingHandler(handler AttesterSlashingHandler) pubsub.MessageHandler {
	return func(ctx context.Context, msg *pubsub.Message) error {
		slashing := &pb.AttesterSlashing{}
		if err := g.encoder.DecodeGossip(msg.Data, slashing); err != nil {
			return fmt.Errorf("failed to decode attester slashing: %w", err)
		}

		return handler(ctx, slashing, msg.From)
	}
}

// createSyncContributionAndProofHandler creates a pubsub message handler for sync contributions
func (g *Gossipsub) createSyncContributionAndProofHandler(handler SyncContributionAndProofHandler) pubsub.MessageHandler {
	return func(ctx context.Context, msg *pubsub.Message) error {
		contrib := &pb.SignedContributionAndProof{}
		if err := g.encoder.DecodeGossip(msg.Data, contrib); err != nil {
			return fmt.Errorf("failed to decode sync contribution and proof: %w", err)
		}

		return handler(ctx, contrib, msg.From)
	}
}

// createBlsToExecutionChangeHandler creates a pubsub message handler for BLS to execution changes
func (g *Gossipsub) createBlsToExecutionChangeHandler(handler BlsToExecutionChangeHandler) pubsub.MessageHandler {
	return func(ctx context.Context, msg *pubsub.Message) error {
		change := &pb.SignedBLSToExecutionChange{}
		if err := g.encoder.DecodeGossip(msg.Data, change); err != nil {
			return fmt.Errorf("failed to decode BLS to execution change: %w", err)
		}

		return handler(ctx, change, msg.From)
	}
}

// Validator creation helpers

// createBeaconBlockValidator creates a pubsub validator for beacon blocks
func (g *Gossipsub) createBeaconBlockValidator(validator BeaconBlockValidator) pubsub.Validator {
	return func(ctx context.Context, msg *pubsub.Message) (pubsub.ValidationResult, error) {
		block := &pb.SignedBeaconBlock{}
		if err := g.encoder.DecodeGossip(msg.Data, block); err != nil {
			return pubsub.ValidationReject, fmt.Errorf("failed to decode beacon block: %w", err)
		}

		if err := validator(block); err != nil {
			return pubsub.ValidationReject, err
		}

		return pubsub.ValidationAccept, nil
	}
}

// createAttestationValidator creates a pubsub validator for attestations
func (g *Gossipsub) createAttestationValidator(validator AttestationValidator, subnet uint64) pubsub.Validator {
	return func(ctx context.Context, msg *pubsub.Message) (pubsub.ValidationResult, error) {
		att := &pb.Attestation{}
		if err := g.encoder.DecodeGossip(msg.Data, att); err != nil {
			return pubsub.ValidationReject, fmt.Errorf("failed to decode attestation: %w", err)
		}

		if err := validator(att, subnet); err != nil {
			return pubsub.ValidationReject, err
		}

		return pubsub.ValidationAccept, nil
	}
}
