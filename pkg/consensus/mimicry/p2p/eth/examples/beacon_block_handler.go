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
//
// This example shows the complete setup process:
// 1. Create a libp2p host
// 2. Initialize the beacon block handler
// 3. Create a Gossipsub instance with production settings
// 4. Set up the beacon block topic with proper encoding
// 5. Configure handler with validation and processing
// 6. Register and subscribe to the topic
// 7. Monitor events for debugging and metrics
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

	// Create metrics for monitoring (optional)
	metrics := v1.NewMetrics("gossipsub_example")

	// Create a Gossipsub instance with production-ready settings
	gs, err := v1.New(ctx, h,
		v1.WithLogger(log),
		v1.WithMetrics(metrics),                     // Enable metrics collection
		v1.WithMaxMessageSize(10*1024*1024),         // 10 MB max for beacon blocks
		v1.WithValidationConcurrency(100),           // Process up to 100 messages concurrently
		v1.WithPublishTimeout(10*time.Second),       // Timeout for publishing
		v1.WithGossipSubParams(pubsub.DefaultGossipSubParams()),
	)
	if err != nil {
		return fmt.Errorf("failed to create gossipsub: %w", err)
	}
	defer gs.Stop()

	// Create event channel for monitoring (optional but recommended)
	events := make(chan v1.Event, 1000)
	go handler.monitorEvents(ctx, events)

	// Create the beacon block topic with fork digest and SSZ encoder
	topic := topics.BeaconBlock.WithForkDigest(forkDigest)
	encoder := topics.NewSSZSnappyEncoderWithMaxLen[*eth.SignedBeaconBlock](10 * 1024 * 1024)

	// Create a typed topic with the encoder
	typedTopic, err := v1.NewTopic[*eth.SignedBeaconBlock](topic.Name(), encoder)
	if err != nil {
		return fmt.Errorf("failed to create typed topic: %w", err)
	}

	// Create handler configuration with validator and processor
	handlerConfig := v1.NewHandlerConfig[*eth.SignedBeaconBlock](
		v1.WithValidator[*eth.SignedBeaconBlock](handler.validateBeaconBlock),
		v1.WithProcessor[*eth.SignedBeaconBlock](handler.processBeaconBlock),
		v1.WithScoreParams[*eth.SignedBeaconBlock](createTopicScoreParams()),
		v1.WithEvents[*eth.SignedBeaconBlock](events),
	)

	// Register the handler with gossipsub
	if err := v1.Register(gs.Registry(), typedTopic, handlerConfig); err != nil {
		return fmt.Errorf("failed to register handler: %w", err)
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
	h.metrics.blocksReceived++

	// Add timeout for validation to prevent blocking
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
		return v1.ValidationIgnore // Future blocks are ignored, not rejected
	}

	// Check if block is too old
	if uint64(block.Block.Slot)+32 < currentSlot { // Keep blocks for ~6 minutes
		h.log.WithFields(logrus.Fields{
			"block_slot":   uint64(block.Block.Slot),
			"current_slot": currentSlot,
		}).Debug("block too old")
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

	// Validate block signature length (simplified validation)
	if len(block.Signature) != 96 {
		h.log.WithField("sig_len", len(block.Signature)).Warn("invalid signature length")
		h.metrics.validationErrors++
		return v1.ValidationReject
	}

	// Additional validation in production would include:
	// - Parent block exists and is valid
	// - Proposer is authorized for this slot
	// - Block follows fork choice rules
	// - State transition is valid
	// - BLS signature verification

	h.metrics.blocksValidated++
	h.log.WithFields(logrus.Fields{
		"slot":     uint64(block.Block.Slot),
		"proposer": block.Block.ProposerIndex,
		"from":     from,
	}).Debug("block validated")

	return v1.ValidationAccept
}

// processBeaconBlock handles validated beacon blocks by saving them to the database.
// This is called after validation has passed and the block has been accepted.
// Any error returned here will be logged but won't affect message propagation.
func (h *BeaconBlockHandler) processBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) error {
	startTime := time.Now()
	defer func() {
		h.log.WithField("duration", time.Since(startTime)).Debug("block processing completed")
	}()

	// Add timeout for processing to prevent hanging
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Calculate block root for storage
	blockRoot := getBlockRoot(block)

	// Begin database transaction for atomic operations
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

	// Save block metadata (peer, timing, etc.)
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

	// Trigger any downstream processing asynchronously
	// In production, this might update fork choice, notify subscribers, etc.
	go h.notifyBlockProcessed(block, blockRoot)

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

// Helper functions for database operations

func (h *BeaconBlockHandler) hasBlock(blockRoot [32]byte) bool {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM beacon_blocks WHERE root = $1)`
	_ = h.db.QueryRow(query, blockRoot[:]).Scan(&exists)
	return exists
}

func (h *BeaconBlockHandler) saveBlock(ctx context.Context, tx *sql.Tx, block *eth.SignedBeaconBlock, root [32]byte) error {
	// Serialize block data using SSZ
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
	return uint64(time.Now().Unix()/12) % 1000000 // Simplified calculation
}

func getBlockRoot(block *eth.SignedBeaconBlock) [32]byte {
	// In production, this would compute the actual SSZ tree hash root
	// For now, return a deterministic hash based on slot and proposer
	var root [32]byte
	if block.Block != nil {
		copy(root[:8], []byte(fmt.Sprintf("%08d", block.Block.Slot)))
		copy(root[8:16], []byte(fmt.Sprintf("%08d", block.Block.ProposerIndex)))
	}
	return root
}

func getBlockBodyRoot(body *eth.BeaconBlockBody) [32]byte {
	// In production, this would compute the actual body root
	var root [32]byte
	return root
}

// createTopicScoreParams creates topic-specific scoring parameters.
// These parameters help protect against spam and encourage good behavior.
func createTopicScoreParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		TopicWeight:                     0.5,  // Weight of this topic in overall peer score
		TimeInMeshWeight:                1,    // Reward for staying in the mesh
		TimeInMeshQuantum:               time.Second,
		TimeInMeshCap:                   3600, // Cap the time in mesh score
		FirstMessageDeliveriesWeight:    1,    // Reward for first delivery
		FirstMessageDeliveriesDecay:     0.5,  // Decay rate for first deliveries
		FirstMessageDeliveriesCap:       100,  // Cap for first deliveries
		MeshMessageDeliveriesWeight:     -1,   // Penalty for not delivering in mesh
		MeshMessageDeliveriesDecay:      0.5,
		MeshMessageDeliveriesCap:        100,
		MeshMessageDeliveriesThreshold:  20,   // Threshold for mesh deliveries
		MeshMessageDeliveriesWindow:     10 * time.Millisecond,
		MeshMessageDeliveriesActivation: time.Second,
		MeshFailurePenaltyWeight:        -1,   // Penalty for mesh failures
		MeshFailurePenaltyDecay:         0.5,
		InvalidMessageDeliveriesWeight:  -1,   // Penalty for invalid messages
		InvalidMessageDeliveriesDecay:   0.3,
	}
}

// Example database schema for reference:
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