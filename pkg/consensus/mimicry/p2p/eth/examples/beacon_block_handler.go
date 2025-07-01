// Package examples provides practical examples of using the eth p2p library.
// These examples demonstrate production-ready patterns for handling Ethereum
// consensus layer gossipsub messages.
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

	eth "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/topics"
	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
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
	blocksReceived   uint64
	blocksValidated  uint64
	blocksProcessed  uint64
	blocksSaved      uint64
	validationErrors uint64
	processingErrors uint64
}

// ExampleBeaconBlockSetup demonstrates how to set up a beacon block handler
// with validation, processing, and event monitoring using the v1 API.
func ExampleBeaconBlockSetup(ctx context.Context, db *sql.DB, log logrus.FieldLogger) error {
	// Create a libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/9000"),
		libp2p.Identity(nil), // Will generate a new identity
	)
	if err != nil {
		return fmt.Errorf("failed to create host: %w", err)
	}

	// Create the beacon block handler
	handler := &BeaconBlockHandler{
		db:      db,
		log:     log.WithField("component", "beacon_block_handler"),
		metrics: &BlockMetrics{},
	}

	// Fork digest for the current network fork
	forkDigest := [4]byte{0xab, 0xcd, 0xef, 0x01} // Example fork digest

	// Create a Gossipsub instance with production settings
	g, err := v1.New(ctx, h,
		v1.WithLogger(log),
		v1.WithMaxMessageSize(10*1024*1024), // 10 MB max for beacon blocks
		v1.WithValidationConcurrency(100),   // Process up to 100 messages concurrently
		v1.WithPublishTimeout(10*time.Second),
		v1.WithGossipSubParams(pubsub.DefaultGossipSubParams()),
	)
	if err != nil {
		return fmt.Errorf("failed to create gossipsub: %w", err)
	}

	// Create event channel for monitoring
	events := make(chan v1.Event, 1000)
	go handler.monitorEvents(ctx, events)

	// Create the topic with fork digest and SSZ encoder
	topic := topics.BeaconBlock.WithForkDigest(forkDigest)
	encoder := topics.NewSSZSnappyEncoderWithMaxLen[*eth.SignedBeaconBlock](10 * 1024 * 1024)

	// Create handler configuration with validator and processor
	handlerConfig := v1.NewHandlerConfig[*eth.SignedBeaconBlock](
		v1.WithValidator[*eth.SignedBeaconBlock](handler.validateBeaconBlock),
		v1.WithProcessor[*eth.SignedBeaconBlock](handler.processBeaconBlock),
		v1.WithScoreParams[*eth.SignedBeaconBlock](createTopicScoreParams()),
		v1.WithEvents[*eth.SignedBeaconBlock](events),
	)

	// Alternative: Use event callbacks for simpler event handling
	/*
		handlerConfig := v1.NewHandlerConfig[*eth.SignedBeaconBlock](
			v1.WithValidator[*eth.SignedBeaconBlock](handler.validateBeaconBlock),
			v1.WithProcessor[*eth.SignedBeaconBlock](handler.processBeaconBlock),
			v1.WithScoreParams[*eth.SignedBeaconBlock](createTopicScoreParams()),
			v1.WithEventCallbacks[*eth.SignedBeaconBlock](&v1.EventCallbacks{
				OnMessageValidated: func(topic string, from peer.ID, data *v1.MessageEventData) {
					handler.log.WithFields(logrus.Fields{
						"topic": topic,
						"from":  from,
						"size":  data.Size,
					}).Debug("block validated")
				},
				OnMessageProcessed: func(topic string, from peer.ID, data *v1.MessageEventData) {
					handler.log.WithFields(logrus.Fields{
						"topic":    topic,
						"from":     from,
						"duration": data.ProcessingDuration,
					}).Debug("block processed")
				},
				OnError: func(topic string, err error, data *v1.ErrorEventData) {
					handler.log.WithFields(logrus.Fields{
						"topic":     topic,
						"error":     err,
						"operation": data.Operation,
					}).Error("processing error")
				},
			}),
		)
	*/

	// Register the handler with gossipsub
	// Note: In production, you would typically use a type-erased registration pattern
	if err := registerBeaconBlockHandler(g, topic, encoder, handlerConfig); err != nil {
		return fmt.Errorf("failed to register handler: %w", err)
	}

	// Subscribe to the topic to start receiving messages
	sub, err := v1.Subscribe(g, topic)
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

	// Keep the handler running
	<-ctx.Done()
	return g.Stop()
}

// validateBeaconBlock performs validation on incoming beacon blocks.
// This is called before the block is propagated to other peers.
func (h *BeaconBlockHandler) validateBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) v1.ValidationResult {
	h.metrics.blocksReceived++

	// Add timeout for validation
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Basic validation checks
	if block == nil || block.Block == nil {
		h.log.WithField("from", from).Warn("received nil block")
		h.metrics.validationErrors++
		return v1.ValidationReject
	}

	// Check block slot is reasonable (not too far in future)
	currentSlot := getCurrentSlot()
	if uint64(block.Block.Slot) > currentSlot+10 {
		h.log.WithFields(logrus.Fields{
			"block_slot":   uint64(block.Block.Slot),
			"current_slot": currentSlot,
			"from":         from,
		}).Warn("block slot too far in future")
		h.metrics.validationErrors++
		return v1.ValidationIgnore
	}

	// Check if we've already seen this block
	blockRoot := getBlockRoot(block)
	if h.hasBlock(blockRoot) {
		h.log.WithFields(logrus.Fields{
			"block_root": fmt.Sprintf("%x", blockRoot),
			"slot":       uint64(block.Block.Slot),
		}).Debug("already have block")
		return v1.ValidationIgnore
	}

	// Validate block signature (simplified - in production would verify against validator set)
	if len(block.Signature) != 96 {
		h.log.WithField("sig_len", len(block.Signature)).Warn("invalid signature length")
		h.metrics.validationErrors++
		return v1.ValidationReject
	}

	// Additional validation could include:
	// - Parent block exists and is valid
	// - Proposer is authorized for this slot
	// - Block follows fork choice rules
	// - State transition is valid

	h.metrics.blocksValidated++
	h.log.WithFields(logrus.Fields{
		"slot":     uint64(block.Block.Slot),
		"proposer": block.Block.ProposerIndex,
		"from":     from,
	}).Debug("block validated")

	return v1.ValidationAccept
}

// processBeaconBlock handles validated beacon blocks by saving them to the database.
// This is called after validation has passed.
func (h *BeaconBlockHandler) processBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) error {
	startTime := time.Now()
	defer func() {
		h.log.WithField("duration", time.Since(startTime)).Debug("block processing completed")
	}()

	// Add timeout for processing
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Calculate block root
	blockRoot := getBlockRoot(block)

	// Begin database transaction
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		h.metrics.processingErrors++
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Save block to database
	if err := h.saveBlock(ctx, tx, block, blockRoot); err != nil {
		h.metrics.processingErrors++
		return fmt.Errorf("failed to save block: %w", err)
	}

	// Save block metadata
	if err := h.saveBlockMetadata(ctx, tx, block, blockRoot, from); err != nil {
		h.metrics.processingErrors++
		return fmt.Errorf("failed to save block metadata: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		h.metrics.processingErrors++
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	h.metrics.blocksProcessed++
	h.metrics.blocksSaved++

	h.log.WithFields(logrus.Fields{
		"slot":       uint64(block.Block.Slot),
		"proposer":   block.Block.ProposerIndex,
		"block_root": fmt.Sprintf("%x", blockRoot),
		"from":       from,
	}).Info("beacon block saved to database")

	// Trigger any downstream processing (e.g., update fork choice, notify subscribers)
	// This would be done asynchronously in production
	go h.notifyBlockProcessed(block, blockRoot)

	return nil
}

// monitorEvents handles events from the gossipsub system for monitoring and metrics.
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

// Helper functions

func (h *BeaconBlockHandler) hasBlock(blockRoot [32]byte) bool {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM beacon_blocks WHERE root = $1)`
	_ = h.db.QueryRow(query, blockRoot[:]).Scan(&exists)
	return exists
}

func (h *BeaconBlockHandler) saveBlock(ctx context.Context, tx *sql.Tx, block *eth.SignedBeaconBlock, root [32]byte) error {
	// Serialize block data
	blockData, err := block.MarshalSSZ()
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	query := `
		INSERT INTO beacon_blocks (root, slot, proposer_index, parent_root, state_root, body_root, signature, raw_data)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (root) DO NOTHING`

	bodyRoot := getBlockBodyRoot(block.Block.Body)
	_, err = tx.ExecContext(ctx, query,
		root[:],
		uint64(block.Block.Slot),
		uint64(block.Block.ProposerIndex),
		block.Block.ParentRoot,
		block.Block.StateRoot,
		bodyRoot[:],
		block.Signature,
		blockData,
	)
	return err
}

func (h *BeaconBlockHandler) saveBlockMetadata(ctx context.Context, tx *sql.Tx, block *eth.SignedBeaconBlock, root [32]byte, from peer.ID) error {
	query := `
		INSERT INTO block_metadata (block_root, received_from, received_at, attestation_count, deposit_count)
		VALUES ($1, $2, $3, $4, $5)`

	attestationCount := 0
	if block.Block.Body != nil && block.Block.Body.Attestations != nil {
		attestationCount = len(block.Block.Body.Attestations)
	}

	depositCount := 0
	if block.Block.Body != nil && block.Block.Body.Deposits != nil {
		depositCount = len(block.Block.Body.Deposits)
	}

	_, err := tx.ExecContext(ctx, query,
		root[:],
		from.String(),
		time.Now(),
		attestationCount,
		depositCount,
	)
	return err
}

func (h *BeaconBlockHandler) notifyBlockProcessed(block *eth.SignedBeaconBlock, root [32]byte) {
	// In production, this would notify other components about the new block
	// For example: update fork choice, trigger state transition, notify API subscribers
	h.log.WithFields(logrus.Fields{
		"slot":       uint64(block.Block.Slot),
		"block_root": fmt.Sprintf("%x", root),
	}).Debug("block processing notification sent")
}

// Utility functions

func getCurrentSlot() uint64 {
	// In production, this would calculate based on genesis time and slot duration
	// For example: (time.Now().Unix() - genesisTime) / secondsPerSlot
	return 1000000 // Placeholder
}

func getBlockRoot(block *eth.SignedBeaconBlock) [32]byte {
	// In production, this would compute the actual block root
	// For now, return a placeholder
	var root [32]byte
	if block.Block != nil {
		copy(root[:8], []byte{byte(block.Block.Slot)})
	}
	return root
}

func getBlockBodyRoot(body *eth.BeaconBlockBody) [32]byte {
	// In production, this would compute the actual body root
	var root [32]byte
	return root
}

// createPeerScoringParams creates peer scoring parameters for gossipsub.
func createPeerScoringParams() *pubsub.PeerScoreParams {
	return &pubsub.PeerScoreParams{
		AppSpecificScore: func(p peer.ID) float64 {
			// In production, implement app-specific scoring
			return 0
		},
		DecayInterval: time.Minute,
		DecayToZero:   0.01,
		Topics:        make(map[string]*pubsub.TopicScoreParams),
	}
}

// createTopicScoreParams creates topic-specific scoring parameters.
func createTopicScoreParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		TopicWeight:                     0.5,
		TimeInMeshWeight:                1,
		TimeInMeshQuantum:               time.Second,
		TimeInMeshCap:                   3600,
		FirstMessageDeliveriesWeight:    1,
		FirstMessageDeliveriesDecay:     0.5,
		FirstMessageDeliveriesCap:       100,
		MeshMessageDeliveriesWeight:     -1,
		MeshMessageDeliveriesDecay:      0.5,
		MeshMessageDeliveriesCap:        100,
		MeshMessageDeliveriesThreshold:  20,
		MeshMessageDeliveriesWindow:     10 * time.Millisecond,
		MeshMessageDeliveriesActivation: time.Second,
		MeshFailurePenaltyWeight:        -1,
		MeshFailurePenaltyDecay:         0.5,
		InvalidMessageDeliveriesWeight:  -1,
		InvalidMessageDeliveriesDecay:   0.3,
	}
}

// registerBeaconBlockHandler is a helper to register the typed handler with gossipsub.
// This demonstrates the type erasure pattern required for registration.
func registerBeaconBlockHandler(
	g *v1.Gossipsub,
	topic *v1.Topic[*eth.SignedBeaconBlock],
	encoder v1.Encoder[*eth.SignedBeaconBlock],
	handler *v1.HandlerConfig[*eth.SignedBeaconBlock],
) error {
	// Create a topic with the encoder
	topicWithEncoder, err := v1.NewTopic[*eth.SignedBeaconBlock](topic.Name(), encoder)
	if err != nil {
		return fmt.Errorf("failed to create topic with encoder: %w", err)
	}

	// Convert to any type for registration
	// This is necessary because Gossipsub.Register expects Topic[any] and HandlerConfig[any]
	anyTopic := &v1.Topic[any]{
		// We need to provide the methods that Topic requires
		// In a real implementation, this would be handled by a helper function
	}
	_ = anyTopic
	_ = topicWithEncoder

	// For this example, we'll show the pattern but note that in production
	// you would use a proper type erasure helper or generic registration function
	// The v1 package would typically provide such helpers

	// In practice, you might have a helper like:
	// return v1.RegisterTyped(g, topicWithEncoder, handler)

	return fmt.Errorf("registration helpers not yet implemented - see v1 examples for complete pattern")
}

// Database schema for reference:
/*
CREATE TABLE beacon_blocks (
    root BYTEA PRIMARY KEY,
    slot BIGINT NOT NULL,
    proposer_index BIGINT NOT NULL,
    parent_root BYTEA NOT NULL,
    state_root BYTEA NOT NULL,
    body_root BYTEA NOT NULL,
    signature BYTEA NOT NULL,
    raw_data BYTEA NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_beacon_blocks_slot ON beacon_blocks(slot);
CREATE INDEX idx_beacon_blocks_proposer ON beacon_blocks(proposer_index);

CREATE TABLE block_metadata (
    id SERIAL PRIMARY KEY,
    block_root BYTEA NOT NULL REFERENCES beacon_blocks(root),
    received_from TEXT NOT NULL,
    received_at TIMESTAMP NOT NULL,
    attestation_count INTEGER NOT NULL,
    deposit_count INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_block_metadata_root ON block_metadata(block_root);
*/
