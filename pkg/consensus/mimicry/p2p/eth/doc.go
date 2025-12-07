// Package eth provides Ethereum consensus layer p2p communication utilities.
//
// This package builds on top of the gossipsub v1 package to provide Ethereum-specific
// functionality for handling consensus layer messages like beacon blocks, attestations,
// sync committee messages, and other consensus protocol communications.
//
// # Overview
//
// The eth package provides:
//   - Pre-defined topic definitions for all Ethereum consensus layer topics
//   - SSZ+Snappy encoding for efficient message serialization
//   - Fork digest support for network isolation
//   - Subnet management for attestations and sync committees
//   - Production-ready message handlers and validators
//   - Integration with Ethereum consensus client architectures
//
// # Core Components
//
// ## Topics
//
// The topics subpackage defines all standard Ethereum consensus layer topics:
//
//	import "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/topics"
//
//	// Beacon block topic
//	blockTopic := topics.BeaconBlock.WithForkDigest(forkDigest)
//
//	// Attestation subnet topics
//	attTopic := topics.BeaconAttestation.WithSubnet(0).WithForkDigest(forkDigest)
//
//	// Sync committee topics
//	syncTopic := topics.SyncCommittee.WithSubnet(5).WithForkDigest(forkDigest)
//
// Available topics:
//   - BeaconBlock: Beacon block proposals
//   - BeaconAttestation: Attestations (64 subnets)
//   - BeaconAggregateAndProof: Aggregated attestations
//   - VoluntaryExit: Validator voluntary exits
//   - ProposerSlashing: Proposer slashing evidence
//   - AttesterSlashing: Attester slashing evidence
//   - SyncCommittee: Sync committee messages (4 subnets)
//   - SyncCommitteeContribution: Sync committee contributions
//   - BlsToExecutionChange: BLS to execution changes
//
// ## Encoders
//
// The package provides optimized SSZ+Snappy encoders for Ethereum types:
//
//	encoder := topics.NewSSZSnappyEncoder[*eth.SignedBeaconBlock]()
//	encoderWithLimit := topics.NewSSZSnappyEncoderWithMaxLen[*eth.SignedBeaconBlock](10*1024*1024)
//
// These encoders handle:
//   - SSZ serialization using the fastssz library
//   - Snappy compression for bandwidth efficiency
//   - Size limits to prevent oversized messages
//   - Error handling for malformed data
//
// ## Fork Digests
//
// Fork digests isolate messages between different network forks:
//
//	// Mainnet Capella fork digest (example)
//	forkDigest := [4]byte{0x6a, 0x95, 0xa1, 0xa3}
//
//	// Apply fork digest to topic
//	topic := topics.BeaconBlock.WithForkDigest(forkDigest)
//	// Results in: "/eth2/6a95a1a3/beacon_block/ssz_snappy"
//
// Fork digests are computed from:
//   - Current fork version
//   - Genesis validators root
//   - Domain type
//
// # Quick Start
//
// Here's a minimal example of setting up beacon block handling:
//
//	package main
//
//	import (
//		"context"
//		"log"
//
//		eth "github.com/OffchainLabs/prysm/v7/proto/prysm/v1alpha1"
//		"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/topics"
//		v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
//		"github.com/libp2p/go-libp2p"
//		"github.com/libp2p/go-libp2p/core/peer"
//		"github.com/sirupsen/logrus"
//	)
//
//	func main() {
//		// Create libp2p host
//		host, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/9000"))
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer host.Close()
//
//		// Create gossipsub instance
//		gs, err := v1.New(context.Background(), host,
//			v1.WithLogger(logrus.StandardLogger()),
//			v1.WithMaxMessageSize(10*1024*1024),
//		)
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer gs.Stop()
//
//		// Set up beacon block handling
//		forkDigest := [4]byte{0x6a, 0x95, 0xa1, 0xa3} // Mainnet Capella
//		if err := setupBeaconBlocks(gs, forkDigest); err != nil {
//			log.Fatal(err)
//		}
//
//		// Keep running
//		select {}
//	}
//
//	func setupBeaconBlocks(gs *v1.Gossipsub, forkDigest [4]byte) error {
//		// Create topic with fork digest and encoder
//		topic := topics.BeaconBlock.WithForkDigest(forkDigest)
//		encoder := topics.NewSSZSnappyEncoderWithMaxLen[*eth.SignedBeaconBlock](10*1024*1024)
//
//		// Create typed topic
//		typedTopic, err := v1.NewTopic[*eth.SignedBeaconBlock](topic.Name(), encoder)
//		if err != nil {
//			return err
//		}
//
//		// Create handler with validation and processing
//		handler := v1.NewHandlerConfig[*eth.SignedBeaconBlock](
//			v1.WithValidator(validateBeaconBlock),
//			v1.WithProcessor(processBeaconBlock),
//		)
//
//		// Register and subscribe
//		if err := v1.Register(gs.Registry(), typedTopic, handler); err != nil {
//			return err
//		}
//
//		_, err = v1.Subscribe(context.Background(), gs, typedTopic)
//		return err
//	}
//
//	func validateBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) v1.ValidationResult {
//		// Implement validation logic
//		if block == nil || block.Block == nil {
//			return v1.ValidationReject
//		}
//		// Add more validation...
//		return v1.ValidationAccept
//	}
//
//	func processBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) error {
//		// Process the block (save to database, update fork choice, etc.)
//		log.Printf("Received beacon block slot %d from %s", block.Block.Slot, from)
//		return nil
//	}
//
// # Attestation Subnets
//
// Attestations are distributed across 64 subnets to reduce bandwidth requirements.
// Here's how to handle attestation subnets:
//
//	func setupAttestationSubnets(gs *v1.Gossipsub, forkDigest [4]byte, subnets []uint64) error {
//		encoder := topics.NewSSZSnappyEncoder[*eth.Attestation]()
//
//		for _, subnet := range subnets {
//			// Create subnet-specific topic
//			topic := topics.BeaconAttestation.WithSubnet(subnet).WithForkDigest(forkDigest)
//
//			// Create typed topic
//			typedTopic, err := v1.NewTopic[*eth.Attestation](topic.Name(), encoder)
//			if err != nil {
//				return err
//			}
//
//			// Create handler with subnet-specific validation
//			handler := v1.NewHandlerConfig[*eth.Attestation](
//				v1.WithValidator(createAttestationValidator(subnet)),
//				v1.WithProcessor(processAttestation),
//			)
//
//			// Register and subscribe
//			if err := v1.Register(gs.Registry(), typedTopic, handler); err != nil {
//				return err
//			}
//
//			if _, err := v1.Subscribe(context.Background(), gs, typedTopic); err != nil {
//				return err
//			}
//		}
//
//		return nil
//	}
//
//	func createAttestationValidator(subnet uint64) v1.Validator[*eth.Attestation] {
//		return func(ctx context.Context, att *eth.Attestation, from peer.ID) v1.ValidationResult {
//			// Validate attestation belongs to this subnet
//			expectedSubnet := computeSubnetForAttestation(att)
//			if expectedSubnet != subnet {
//				return v1.ValidationReject
//			}
//			// Add more validation...
//			return v1.ValidationAccept
//		}
//	}
//
// # Dynamic Subnet Management
//
// For validators, subnet subscriptions change based on duty assignments:
//
//	type SubnetManager struct {
//		gs            *v1.Gossipsub
//		forkDigest    [4]byte
//		subscriptions map[uint64]*v1.Subscription
//		mu            sync.RWMutex
//	}
//
//	func (sm *SubnetManager) UpdateSubnets(newSubnets []uint64) error {
//		sm.mu.Lock()
//		defer sm.mu.Unlock()
//
//		// Calculate which subnets to add/remove
//		current := make(map[uint64]bool)
//		for subnet := range sm.subscriptions {
//			current[subnet] = true
//		}
//
//		target := make(map[uint64]bool)
//		for _, subnet := range newSubnets {
//			target[subnet] = true
//		}
//
//		// Remove old subscriptions
//		for subnet := range current {
//			if !target[subnet] {
//				sm.subscriptions[subnet].Cancel()
//				delete(sm.subscriptions, subnet)
//			}
//		}
//
//		// Add new subscriptions
//		for subnet := range target {
//			if !current[subnet] {
//				if err := sm.subscribeToSubnet(subnet); err != nil {
//					return err
//				}
//			}
//		}
//
//		return nil
//	}
//
// # Message Publishing
//
// Publishing messages to the network:
//
//	func publishBeaconBlock(gs *v1.Gossipsub, block *eth.SignedBeaconBlock, forkDigest [4]byte) error {
//		// Create topic and encoder
//		topic := topics.BeaconBlock.WithForkDigest(forkDigest)
//		encoder := topics.NewSSZSnappyEncoderWithMaxLen[*eth.SignedBeaconBlock](10*1024*1024)
//
//		// Create typed topic
//		typedTopic, err := v1.NewTopic[*eth.SignedBeaconBlock](topic.Name(), encoder)
//		if err != nil {
//			return err
//		}
//
//		// Publish the block
//		return v1.Publish(gs, typedTopic, block)
//	}
//
// # Production Considerations
//
// ## Validation
//
// Implement comprehensive validation to protect against spam and invalid messages:
//
//	func validateBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) v1.ValidationResult {
//		// Timeout for validation
//		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
//		defer cancel()
//
//		// Basic structure validation
//		if block == nil || block.Block == nil {
//			return v1.ValidationReject
//		}
//
//		// Slot range validation
//		currentSlot := getCurrentSlot()
//		if uint64(block.Block.Slot) > currentSlot+10 {
//			return v1.ValidationIgnore // Future blocks
//		}
//
//		// Signature validation
//		if len(block.Signature) != 96 {
//			return v1.ValidationReject
//		}
//
//		// Additional validations:
//		// - Parent block exists
//		// - Proposer is valid for slot
//		// - State transition is valid
//		// - BLS signature verification
//
//		return v1.ValidationAccept
//	}
//
// ## Error Handling
//
// Handle errors gracefully without blocking message processing:
//
//	func processBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID) error {
//		// Timeout for processing
//		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
//		defer cancel()
//
//		// Process with error recovery
//		if err := saveBlockToDatabase(ctx, block); err != nil {
//			// Log error but don't fail processing
//			log.WithError(err).Error("failed to save block to database")
//			// Could implement retry logic here
//		}
//
//		// Trigger fork choice update
//		go updateForkChoice(block)
//
//		return nil
//	}
//
// ## Metrics and Monitoring
//
// Monitor performance and behavior:
//
//	metrics := v1.NewMetrics("eth_consensus")
//	gs, err := v1.New(ctx, host,
//		v1.WithMetrics(metrics),
//		v1.WithLogger(logger),
//	)
//
//	// Register metrics with Prometheus
//	prometheus.MustRegister(metrics.Collectors()...)
//
// ## Topic Scoring
//
// Configure topic scoring to prevent spam:
//
//	scoreParams := &pubsub.TopicScoreParams{
//		TopicWeight:                     0.5,
//		TimeInMeshWeight:                1,
//		FirstMessageDeliveriesWeight:    1,
//		MeshMessageDeliveriesWeight:     -1,
//		InvalidMessageDeliveriesWeight:  -100,
//		// ... more parameters
//	}
//
//	handler := v1.NewHandlerConfig[T](
//		v1.WithValidator(validator),
//		v1.WithProcessor(processor),
//		v1.WithScoreParams(scoreParams),
//	)
//
// # Integration with Consensus Clients
//
// This package is designed to integrate with Ethereum consensus clients:
//
//	type ConsensusClient struct {
//		gossipsub    *v1.Gossipsub
//		blockPool    *BlockPool
//		attestPool   *AttestationPool
//		forkChoice   *ForkChoice
//		stateManager *StateManager
//	}
//
//	func (c *ConsensusClient) Start(ctx context.Context) error {
//		// Initialize gossipsub
//		gs, err := v1.New(ctx, c.host, c.gossipsubOptions()...)
//		if err != nil {
//			return err
//		}
//		c.gossipsub = gs
//
//		// Set up all topic handlers
//		if err := c.setupTopicHandlers(); err != nil {
//			return err
//		}
//
//		// Start background processes
//		go c.manageSubnetSubscriptions(ctx)
//		go c.publishDutyBlocks(ctx)
//
//		return nil
//	}
//
// # Fork Management
//
// Handle network forks by updating fork digests:
//
//	func (c *ConsensusClient) UpdateFork(newForkDigest [4]byte) error {
//		// Update all topic subscriptions with new fork digest
//		c.mu.Lock()
//		defer c.mu.Unlock()
//
//		c.forkDigest = newForkDigest
//
//		// Resubscribe to all topics with new fork digest
//		return c.resubscribeAllTopics()
//	}
//
// # Testing
//
// The package includes comprehensive examples and test utilities:
//
//	// See examples directory for complete examples:
//	// - examples/beacon_block_handler.go
//	// - examples/attestation_handler.go
//
// Run tests with:
//
//	go test -race ./...
//
// # Security Considerations
//
//   - Always validate message structure before processing
//   - Implement timeouts to prevent DoS attacks
//   - Use appropriate topic scoring parameters
//   - Monitor for unusual peer behavior
//   - Validate fork digests to prevent cross-network messages
//   - Implement rate limiting for message processing
//   - Use secure BLS signature verification
//
// # Performance Optimization
//
//   - Use SSZ+Snappy encoding for efficiency
//   - Configure appropriate validation concurrency
//   - Implement message deduplication
//   - Use efficient data structures for pools
//   - Monitor memory usage under load
//   - Optimize database operations
//   - Use connection pooling for external services
//
// # Best Practices
//
//  1. Implement comprehensive logging with structured fields
//  2. Use metrics for observability and alerting
//  3. Handle network partitions gracefully
//  4. Implement proper error recovery mechanisms
//  5. Test under various network conditions
//  6. Monitor peer diversity and connection health
//  7. Implement proper backpressure mechanisms
//  8. Use appropriate buffer sizes for high-volume topics
//  9. Implement graceful shutdown procedures
//  10. Follow Ethereum specification requirements exactly
package eth
