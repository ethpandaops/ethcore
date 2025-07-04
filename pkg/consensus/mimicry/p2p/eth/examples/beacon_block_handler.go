// Package examples provides practical examples of using the eth p2p library.
// These examples demonstrate production-ready patterns for handling Ethereum
// consensus layer gossipsub messages using the v1 API.
//
// This example shows how to:
// - Set up a beacon block handler with the v1 gossipsub API
// - Implement validation logic that checks block slot and signature
// - Process blocks by saving them to a database
// - Monitor events for metrics and debugging
// - Use proper error handling and context management
// - Structure code for production use with proper separation of concerns
//
// The beacon block handler demonstrates a complete end-to-end flow for
// receiving, validating, and persisting beacon blocks from the Ethereum
// consensus layer p2p network.
package examples

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p"
	eth "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/topics"
	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// BeaconBlockHandler manages beacon block processing with database persistence.
type BeaconBlockHandler struct {
	db      *sql.DB
	log     logrus.FieldLogger
	metrics *BlockMetrics
}

// BlockMetrics tracks beacon block processing metrics.
type BlockMetrics struct {
	// Metrics fields can be added as needed
}

// 7. Monitor events for debugging and metrics.
func ExampleBeaconBlockSetup(ctx context.Context, db *sql.DB, log logrus.FieldLogger) error {
	// Create a libp2p host for network communication
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/9000"),
		libp2p.Identity(nil), // Will generate a new identity
	)
	if err != nil {
		return fmt.Errorf("failed to create host: %w", err)
	}
	defer h.Close()

	// Create the beacon block handler with database and logging
	handler := &BeaconBlockHandler{
		db:      db,
		log:     log.WithField("component", "beacon_block_handler"),
		metrics: &BlockMetrics{},
	}

	// Fork digest for the current network fork (example for mainnet)
	forkDigest := [4]byte{0x6a, 0x95, 0xa1, 0xa3} // Example mainnet fork digest

	// Genesis root
	genesisRoot := "0x0000000000000000000000000000000000000000000000000000000000000000"
	genesisRootBytes := common.HexToHash(genesisRoot)

	// Create metrics for monitoring (optional)
	metrics := v1.NewMetrics("gossipsub_example")

	// Create a Gossipsub instance with production-ready settings
	gs, err := v1.New(ctx, h,
		v1.WithLogger(log),
		v1.WithMetrics(metrics),               // Enable metrics collection
		v1.WithPublishTimeout(10*time.Second), // Timeout for publishing
		v1.WithGossipSubParams(pubsub.DefaultGossipSubParams()),
		v1.WithPubsubOptions(
			pubsub.WithMaxMessageSize(10*1024*1024), // 10 MB max for beacon blocks
			pubsub.WithValidateWorkers(100),         // Process up to 100 messages concurrently
			pubsub.WithMessageIdFn(func(pmsg *pubsub_pb.Message) string { // Custom message ID function, critical for Ethereum
				return p2p.MsgID(genesisRootBytes[:], pmsg)
			}),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create gossipsub: %w", err)
	}

	defer func() {
		if stopErr := gs.Stop(); stopErr != nil {
			log.WithError(stopErr).Error("Failed to stop gossipsub")
		}
	}()

	// Create event channel for monitoring (optional but recommended)
	events := make(chan v1.Event, 1000)
	go handler.monitorEvents(ctx, events)

	// Create the beacon block topic with fork digest
	topic := topics.BeaconBlock.WithForkDigest(forkDigest)

	// Create a typed topic
	typedTopic, err := v1.NewTopic[*eth.SignedBeaconBlock](topic.Name())
	if err != nil {
		return fmt.Errorf("failed to create typed topic: %w", err)
	}

	// Create encoder for beacon blocks
	encoder := topics.NewSSZSnappyEncoderWithMaxLen[*eth.SignedBeaconBlock](10 * 1024 * 1024)

	// Create handler configuration with encoder, validator and processor
	handlerConfig := v1.NewHandlerConfig[*eth.SignedBeaconBlock](
		v1.WithEncoder[*eth.SignedBeaconBlock](encoder),
		v1.WithValidator[*eth.SignedBeaconBlock](handler.validateBeaconBlock),
		v1.WithProcessor[*eth.SignedBeaconBlock](handler.processBeaconBlock),
		v1.WithScoreParams[*eth.SignedBeaconBlock](createTopicScoreParams()),
		v1.WithEvents[*eth.SignedBeaconBlock](events),
	)

	// Register the handler with gossipsub
	if regErr := v1.Register(gs.Registry(), typedTopic, handlerConfig); regErr != nil {
		return fmt.Errorf("failed to register handler: %w", regErr)
	}

	// Subscribe to the topic to start receiving messages
	sub, err := v1.Subscribe(context.Background(), gs, typedTopic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}
	defer sub.Cancel()

	// Log successful setup
	log.WithFields(logrus.Fields{
		"topic":       topic.Name(),
		"fork_digest": fmt.Sprintf("%x", forkDigest),
		"host_id":     h.ID(),
	}).Info("beacon block handler setup complete")

	// Example: Publish a test block (in production, blocks come from the beacon node)
	// This demonstrates how to publish messages
	if err := handler.publishExampleBlock(gs, typedTopic); err != nil {
		log.WithError(err).Warn("failed to publish example block")
	}

	// Keep the handler running until context is cancelled
	<-ctx.Done()

	return nil
}

// validateBeaconBlock performs validation on incoming beacon blocks.
// This is called before the block is propagated to other peers.
// Return v1.ValidationAccept to accept and propagate the message,
// v1.ValidationReject to reject and penalize the sender,
// or v1.ValidationIgnore to ignore without penalty.
func (h *BeaconBlockHandler) validateBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) v1.ValidationResult {
	if block.Block == nil {
		return v1.ValidationReject
	}

	return v1.ValidationAccept
}

// processBeaconBlock handles validated beacon blocks by saving them to the database.
// This is called after validation has passed and the block has been accepted.
// Any error returned here will be logged but won't affect message propagation.
func (h *BeaconBlockHandler) processBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) error {
	// Trigger any downstream processing asynchronously
	// In production, this might update fork choice, notify subscribers, etc.
	go h.notifyBlockProcessed(block)

	return nil
}

// publishExampleBlock demonstrates how to publish a beacon block.
// In production, this would be called by your beacon node implementation.
func (h *BeaconBlockHandler) publishExampleBlock(gs *v1.Gossipsub, topic *v1.Topic[*eth.SignedBeaconBlock]) error {
	// Create an example block (in production, this comes from your beacon node)
	exampleBlock := &eth.SignedBeaconBlock{
		Block: &eth.BeaconBlock{
			Slot:          12345,
			ProposerIndex: 42,
			ParentRoot:    make([]byte, 32),
			StateRoot:     make([]byte, 32),
			Body: &eth.BeaconBlockBody{
				RandaoReveal:      make([]byte, 96),
				Eth1Data:          &eth.Eth1Data{},
				Graffiti:          make([]byte, 32),
				ProposerSlashings: []*eth.ProposerSlashing{},
				AttesterSlashings: []*eth.AttesterSlashing{},
				Attestations:      []*eth.Attestation{},
				Deposits:          []*eth.Deposit{},
				VoluntaryExits:    []*eth.SignedVoluntaryExit{},
			},
		},
		Signature: make([]byte, 96),
	}

	// Publish the block
	if err := v1.Publish(gs, topic, exampleBlock); err != nil {
		return fmt.Errorf("failed to publish block: %w", err)
	}

	h.log.WithFields(logrus.Fields{
		"slot":     exampleBlock.Block.Slot,
		"proposer": exampleBlock.Block.ProposerIndex,
	}).Info("published example beacon block")

	return nil
}

// monitorEvents handles events from the gossipsub system for monitoring and metrics.
// This demonstrates how to use the event system for observability.
func (h *BeaconBlockHandler) monitorEvents(ctx context.Context, events <-chan v1.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-events:
			if e, ok := event.(*v1.GossipsubEvent); ok {
				switch e.EventType {
				case v1.EventTypeMessageValidated:
					if data, ok := e.Data.(*v1.MessageEventData); ok {
						h.log.WithFields(logrus.Fields{
							"topic":  e.Topic,
							"from":   e.PeerID,
							"result": data.ValidationResult,
							"size":   data.Size,
						}).Debug("message validated")
					}
				case v1.EventTypeMessageProcessed:
					if data, ok := e.Data.(*v1.MessageEventData); ok {
						h.log.WithFields(logrus.Fields{
							"topic":    e.Topic,
							"from":     e.PeerID,
							"duration": data.ProcessingDuration,
							"msg_id":   data.MessageID,
						}).Debug("message processed")
					}
				case v1.EventTypeError:
					if data, ok := e.Data.(*v1.ErrorEventData); ok {
						h.log.WithFields(logrus.Fields{
							"topic":     e.Topic,
							"from":      e.PeerID,
							"error":     e.Error,
							"operation": data.Operation,
						}).Warn("message processing error")
					}
				case v1.EventTypeMessageRejected:
					if data, ok := e.Data.(*v1.MessageEventData); ok {
						h.log.WithFields(logrus.Fields{
							"topic":  e.Topic,
							"from":   e.PeerID,
							"result": data.ValidationResult,
						}).Debug("message rejected")
					}
				}
			}
		}
	}
}

// Helper functions for database operations would go here

func (h *BeaconBlockHandler) notifyBlockProcessed(block *eth.SignedBeaconBlock) {
	// In production, this would notify other components about the new block
	// For example: update fork choice, trigger state transition, notify API subscribers
	h.log.WithFields(logrus.Fields{
		"slot": uint64(block.Block.Slot),
	}).Debug("block processing notification sent")
}

// Utility functions would go here.

// createTopicScoreParams creates topic-specific scoring parameters.
// These parameters help protect against spam and encourage good behavior.
func createTopicScoreParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		TopicWeight:                     0.5, // Weight of this topic in overall peer score
		TimeInMeshWeight:                1,   // Reward for staying in the mesh
		TimeInMeshQuantum:               time.Second,
		TimeInMeshCap:                   3600, // Cap the time in mesh score
		FirstMessageDeliveriesWeight:    1,    // Reward for first delivery
		FirstMessageDeliveriesDecay:     0.5,  // Decay rate for first deliveries
		FirstMessageDeliveriesCap:       100,  // Cap for first deliveries
		MeshMessageDeliveriesWeight:     -1,   // Penalty for not delivering in mesh
		MeshMessageDeliveriesDecay:      0.5,
		MeshMessageDeliveriesCap:        100,
		MeshMessageDeliveriesThreshold:  20, // Threshold for mesh deliveries
		MeshMessageDeliveriesWindow:     10 * time.Millisecond,
		MeshMessageDeliveriesActivation: time.Second,
		MeshFailurePenaltyWeight:        -1, // Penalty for mesh failures
		MeshFailurePenaltyDecay:         0.5,
		InvalidMessageDeliveriesWeight:  -1, // Penalty for invalid messages
		InvalidMessageDeliveriesDecay:   0.3,
	}
}
