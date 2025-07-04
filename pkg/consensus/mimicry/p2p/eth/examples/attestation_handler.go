// Package examples provides practical examples of using the eth p2p library.
// This example demonstrates how to handle attestation subnet messages in production.
//
// Attestations in Ethereum are broadcast across multiple subnets (0-63) to
// distribute network load. This example shows how to:
// - Handle dynamic subnet subscription management
// - Validate attestations based on subnet assignment
// - Process attestations into an attestation pool
// - Monitor subnet-specific metrics
// - Use the v1 gossipsub API for subnet topics
//
// The attestation handler demonstrates production patterns for managing
// subnet subscriptions dynamically based on validator duties.
package examples

import (
	"context"
	"fmt"
	"slices"
	"sync"
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

// AttestationPool represents a simple in-memory attestation pool.
// In production, this would be backed by a database or more sophisticated storage.
type AttestationPool struct {
	mu           sync.RWMutex
	attestations map[string]*eth.Attestation // key: attestation root
	bySlot       map[uint64][]*eth.Attestation
	maxSize      int
}

// AttestationHandler manages attestation processing across multiple subnets.
// It handles dynamic subnet subscriptions based on validator duties.
type AttestationHandler struct {
	pool          *AttestationPool
	log           logrus.FieldLogger
	metrics       *AttestationMetrics
	activeSubnets map[uint64]*v1.Subscription // Currently subscribed subnets
	mu            sync.RWMutex
	gs            *v1.Gossipsub
}

// AttestationMetrics tracks attestation processing metrics per subnet.
type AttestationMetrics struct {
	mu                    sync.RWMutex
	attestationsReceived  map[uint64]uint64 // per subnet
	attestationsValidated map[uint64]uint64
	attestationsProcessed map[uint64]uint64
	validationErrors      map[uint64]uint64
	processingErrors      map[uint64]uint64
}

// 5. Monitoring metrics across subnets.
func ExampleAttestationSetup(ctx context.Context, log logrus.FieldLogger) error {
	// Create a libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/9000"),
		libp2p.Identity(nil), // Will generate a new identity
	)
	if err != nil {
		return fmt.Errorf("failed to create host: %w", err)
	}
	defer h.Close()

	// Create the attestation handler with pool
	handler := &AttestationHandler{
		pool: &AttestationPool{
			attestations: make(map[string]*eth.Attestation),
			bySlot:       make(map[uint64][]*eth.Attestation),
			maxSize:      10000, // Store up to 10k attestations
		},
		log: log.WithField("component", "attestation_handler"),
		metrics: &AttestationMetrics{
			attestationsReceived:  make(map[uint64]uint64),
			attestationsValidated: make(map[uint64]uint64),
			attestationsProcessed: make(map[uint64]uint64),
			validationErrors:      make(map[uint64]uint64),
			processingErrors:      make(map[uint64]uint64),
		},
		activeSubnets: make(map[uint64]*v1.Subscription),
	}

	// Fork digest for the current network fork
	forkDigest := [4]byte{0x6a, 0x95, 0xa1, 0xa3} // Example mainnet fork digest

	// Genesis root
	genesisRoot := "0x0000000000000000000000000000000000000000000000000000000000000000"
	genesisRootBytes := common.HexToHash(genesisRoot)

	// Create metrics for monitoring
	metrics := v1.NewMetrics("attestation_gossipsub")

	// Create a Gossipsub instance with production settings optimized for attestations
	gs, err := v1.New(ctx, h,
		v1.WithLogger(log),
		v1.WithMetrics(metrics),
		v1.WithPublishTimeout(5*time.Second), // Faster timeout for attestations
		v1.WithGossipSubParams(pubsub.DefaultGossipSubParams()),
		v1.WithPubsubOptions(
			pubsub.WithMaxMessageSize(1*1024*1024), // 1 MB max for attestations
			pubsub.WithValidateWorkers(200),        // Higher concurrency for attestations
			pubsub.WithMessageIdFn(func(pmsg *pubsub_pb.Message) string { // Custom message ID function, critical for Ethereum
				return p2p.MsgID(genesisRootBytes[:], pmsg)
			}),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create gossipsub: %w", err)
	}

	defer func() {
		if err := gs.Stop(); err != nil {
			log.WithError(err).Error("Failed to stop gossipsub")
		}
	}()

	handler.gs = gs

	// Create event channel for monitoring with larger buffer for high-volume attestations
	events := make(chan v1.Event, 5000)
	go handler.monitorEvents(ctx, events)

	// Example: Subscribe to specific subnets based on validator duties
	// In production, this would be driven by your validator's duty schedule
	initialSubnets := []uint64{0, 15, 31, 47} // Example subnet IDs

	// Set up handlers for the initial subnets
	for _, subnet := range initialSubnets {
		if err := handler.subscribeToSubnet(ctx, subnet, forkDigest, events); err != nil {
			return fmt.Errorf("failed to subscribe to subnet %d: %w", subnet, err)
		}
	}

	// Start dynamic subnet management
	// In production, this would be driven by validator duty changes
	go handler.manageDynamicSubnets(ctx, forkDigest, events)

	// Log successful setup
	log.WithFields(logrus.Fields{
		"fork_digest":     fmt.Sprintf("%x", forkDigest),
		"host_id":         h.ID(),
		"initial_subnets": initialSubnets,
	}).Info("attestation subnet handler setup complete")

	// Example: Publish a test attestation to demonstrate publishing
	if err := handler.publishExampleAttestation(ctx, 0, forkDigest); err != nil {
		log.WithError(err).Warn("failed to publish example attestation")
	}

	// Keep running until context is cancelled
	<-ctx.Done()

	return nil
}

// subscribeToSubnet subscribes to a specific attestation subnet with proper validation and processing.
func (h *AttestationHandler) subscribeToSubnet(_ context.Context, subnet uint64, forkDigest [4]byte, events chan<- v1.Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if already subscribed
	if _, exists := h.activeSubnets[subnet]; exists {
		return nil
	}

	// Create the attestation topic for this subnet
	topic, err := topics.Attestation.TopicForSubnet(subnet, forkDigest)
	if err != nil {
		return fmt.Errorf("failed to create topic for subnet %d: %w", subnet, err)
	}

	// Create typed topic
	typedTopic, err := v1.NewTopic[*eth.Attestation](topic.Name())
	if err != nil {
		return fmt.Errorf("failed to create typed topic for subnet %d: %w", subnet, err)
	}

	// Create subnet-specific validator and processor
	validator := h.createSubnetValidator(subnet)
	processor := h.createSubnetProcessor(subnet)

	// Create handler configuration for this subnet
	handlerConfig := v1.NewHandlerConfig[*eth.Attestation](
		v1.WithValidator(validator),
		v1.WithProcessor(processor),
		v1.WithScoreParams[*eth.Attestation](createAttestationScoreParams()),
		v1.WithEvents[*eth.Attestation](events),
	)

	// Register the handler
	if regErr := v1.Register(h.gs.Registry(), typedTopic, handlerConfig); regErr != nil {
		return fmt.Errorf("failed to register handler for subnet %d: %w", subnet, regErr)
	}

	// Subscribe to the topic
	sub, err := v1.Subscribe(context.Background(), h.gs, typedTopic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to subnet %d: %w", subnet, err)
	}

	// Store the subscription
	h.activeSubnets[subnet] = sub

	h.log.WithFields(logrus.Fields{
		"subnet": subnet,
		"topic":  topic.Name(),
	}).Info("subscribed to attestation subnet")

	return nil
}

// unsubscribeFromSubnet unsubscribes from a specific attestation subnet.
func (h *AttestationHandler) unsubscribeFromSubnet(subnet uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if sub, exists := h.activeSubnets[subnet]; exists {
		sub.Cancel()
		delete(h.activeSubnets, subnet)
		h.log.WithField("subnet", subnet).Info("unsubscribed from attestation subnet")
	}
}

// createSubnetValidator creates a validator function for a specific subnet.
func (h *AttestationHandler) createSubnetValidator(subnet uint64) v1.Validator[*eth.Attestation] {
	return func(ctx context.Context, att *eth.Attestation, from peer.ID) v1.ValidationResult {
		return h.validateAttestationSubnet(ctx, att, from, subnet)
	}
}

// createSubnetProcessor creates a processor function for a specific subnet.
func (h *AttestationHandler) createSubnetProcessor(subnet uint64) v1.Processor[*eth.Attestation] {
	return func(ctx context.Context, att *eth.Attestation, from peer.ID) error {
		return h.processAttestationSubnet(ctx, att, from, subnet)
	}
}

// validateAttestationSubnet validates attestations for a specific subnet.
func (h *AttestationHandler) validateAttestationSubnet(
	ctx context.Context,
	att *eth.Attestation,
	from peer.ID,
	subnet uint64,
) v1.ValidationResult {
	h.metrics.incrementReceived(subnet)

	// Do your validation here

	return v1.ValidationAccept
}

// processAttestationSubnet processes validated attestations from a subnet.
func (h *AttestationHandler) processAttestationSubnet(
	ctx context.Context,
	att *eth.Attestation,
	from peer.ID,
	subnet uint64,
) error {
	// Add timeout for processing
	processCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_ = processCtx // Context available for future use

	// Add to attestation pool
	if err := h.pool.Add(att); err != nil {
		h.log.WithFields(logrus.Fields{
			"subnet": subnet,
			"slot":   att.Data.Slot,
			"error":  err,
		}).Error("failed to add attestation to pool")
		h.metrics.incrementProcessingError(subnet)

		return fmt.Errorf("failed to add to pool: %w", err)
	}

	h.log.WithFields(logrus.Fields{
		"subnet":          subnet,
		"slot":            att.Data.Slot,
		"committee_index": att.Data.CommitteeIndex,
		"beacon_root":     fmt.Sprintf("%x", att.Data.BeaconBlockRoot),
		"from":            from.String(),
	}).Debug("processed attestation")

	h.metrics.incrementProcessed(subnet)

	// Trigger any downstream processing asynchronously
	go h.notifyAttestationProcessed(att, subnet)

	return nil
}

// manageDynamicSubnets demonstrates dynamic subnet subscription management.
// In production, this would be driven by validator duty changes.
func (h *AttestationHandler) manageDynamicSubnets(ctx context.Context, forkDigest [4]byte, events chan<- v1.Event) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Simulate changing validator duties by rotating subnets
			// In production, this would be based on actual validator assignments
			h.rotateSubnets(ctx, forkDigest, events)
		}
	}
}

// rotateSubnets demonstrates how to change subnet subscriptions.
func (h *AttestationHandler) rotateSubnets(ctx context.Context, forkDigest [4]byte, events chan<- v1.Event) {
	h.mu.RLock()

	currentSubnets := make([]uint64, 0, len(h.activeSubnets))
	for subnet := range h.activeSubnets {
		currentSubnets = append(currentSubnets, subnet)
	}
	h.mu.RUnlock()

	// Simulate new duty assignment (in production, get this from beacon node)
	unixTime := time.Now().Unix()

	var newSubnet uint64

	if unixTime >= 0 {
		newSubnet = uint64(unixTime) % topics.AttestationSubnetCount
	} else {
		newSubnet = 0 // Fallback for negative time (should never happen)
	}

	// Check if we're already subscribed
	alreadySubscribed := false

	alreadySubscribed = slices.Contains(currentSubnets, newSubnet)

	if !alreadySubscribed && len(currentSubnets) < 8 { // Limit concurrent subscriptions
		if err := h.subscribeToSubnet(ctx, newSubnet, forkDigest, events); err != nil {
			h.log.WithError(err).WithField("subnet", newSubnet).Error("failed to subscribe to new subnet")
		}
	}

	// Optionally unsubscribe from old subnets (when duties end)
	if len(currentSubnets) > 0 {
		oldSubnet := currentSubnets[0] // Remove oldest subscription
		h.unsubscribeFromSubnet(oldSubnet)
	}
}

// publishExampleAttestation demonstrates how to publish an attestation.
func (h *AttestationHandler) publishExampleAttestation(_ context.Context, subnet uint64, forkDigest [4]byte) error {
	// Create example attestation
	// Note: In production, you would create proper attestations from your beacon node
	// The following is a simplified example that may need type adjustments based on your prysm version
	exampleAtt := &eth.Attestation{
		AggregationBits: []byte{0x01}, // Single bit set
		Data: &eth.AttestationData{
			Slot:            100, // Example slot
			CommitteeIndex:  1,   // Example committee index
			BeaconBlockRoot: make([]byte, 32),
			Source: &eth.Checkpoint{
				Epoch: 10,
				Root:  make([]byte, 32),
			},
			Target: &eth.Checkpoint{
				Epoch: 11,
				Root:  make([]byte, 32),
			},
		},
		Signature: make([]byte, 96),
	}

	// Create topic and publish
	topic, err := topics.Attestation.TopicForSubnet(subnet, forkDigest)
	if err != nil {
		return fmt.Errorf("failed to create topic for subnet %d: %w", subnet, err)
	}

	typedTopic, err := v1.NewTopic[*eth.Attestation](topic.Name())
	if err != nil {
		return fmt.Errorf("failed to create typed topic: %w", err)
	}

	if err := v1.Publish(h.gs, typedTopic, exampleAtt); err != nil {
		return fmt.Errorf("failed to publish attestation: %w", err)
	}

	h.log.WithFields(logrus.Fields{
		"subnet": subnet,
		"slot":   exampleAtt.Data.Slot,
	}).Info("published example attestation")

	return nil
}

// monitorEvents processes events from the event channel for observability.
func (h *AttestationHandler) monitorEvents(ctx context.Context, events <-chan v1.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-events:
			if e, ok := event.(*v1.GossipsubEvent); ok {
				switch e.EventType {
				case v1.EventTypeMessageValidated:
					h.log.WithFields(logrus.Fields{
						"topic": e.Topic,
						"from":  e.PeerID,
					}).Trace("attestation validated")
				case v1.EventTypeMessageProcessed:
					if data, ok := e.Data.(*v1.MessageEventData); ok {
						h.log.WithFields(logrus.Fields{
							"topic":    e.Topic,
							"duration": data.ProcessingDuration,
						}).Trace("attestation processed")
					}
				case v1.EventTypeError:
					if data, ok := e.Data.(*v1.ErrorEventData); ok {
						h.log.WithFields(logrus.Fields{
							"topic":     e.Topic,
							"operation": data.Operation,
							"error":     e.Error,
						}).Warn("attestation processing error")
					}
				}
			}
		}
	}
}

// notifyAttestationProcessed triggers downstream processing.
func (h *AttestationHandler) notifyAttestationProcessed(att *eth.Attestation, subnet uint64) {
	// In production, this might:
	// - Update attestation aggregator
	// - Notify fork choice
	// - Update API subscribers
	h.log.WithFields(logrus.Fields{
		"subnet": subnet,
		"slot":   att.Data.Slot,
	}).Debug("attestation processing notification sent")
}

// computeSubnetForAttestation calculates which subnet an attestation belongs to.
func computeSubnetForAttestation(att *eth.Attestation) uint64 {
	if att.Data == nil {
		return 0
	}

	// Simplified subnet calculation (production uses proper spec algorithm)
	committeeIndex := uint64(att.Data.CommitteeIndex)

	return committeeIndex % topics.AttestationSubnetCount
}

// createAttestationScoreParams creates topic scoring parameters for attestation subnets.
func createAttestationScoreParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		TopicWeight:                     0.5, // Lower weight than beacon blocks
		TimeInMeshWeight:                0.0324,
		TimeInMeshQuantum:               12 * time.Second,
		TimeInMeshCap:                   300,
		FirstMessageDeliveriesWeight:    1.0,
		FirstMessageDeliveriesDecay:     0.99,
		FirstMessageDeliveriesCap:       10,
		MeshMessageDeliveriesWeight:     -1.0,
		MeshMessageDeliveriesDecay:      0.99,
		MeshMessageDeliveriesCap:        10,
		MeshMessageDeliveriesThreshold:  5,
		MeshMessageDeliveriesWindow:     10 * time.Millisecond,
		MeshMessageDeliveriesActivation: 30 * time.Second,
		MeshFailurePenaltyWeight:        -1.0,
		MeshFailurePenaltyDecay:         0.99,
		InvalidMessageDeliveriesWeight:  -100.0,
		InvalidMessageDeliveriesDecay:   0.99,
	}
}

// AttestationPool methods

// Add adds an attestation to the pool.
func (p *AttestationPool) Add(att *eth.Attestation) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Generate a unique key for the attestation
	key := fmt.Sprintf("%x_%d", att.Data.BeaconBlockRoot, att.Data.CommitteeIndex)

	// Check if we already have this attestation
	if _, exists := p.attestations[key]; exists {
		return nil // Already have it
	}

	// Check pool size limit
	if len(p.attestations) >= p.maxSize {
		// In production, implement proper eviction policy (e.g., remove oldest)
		return fmt.Errorf("attestation pool full")
	}

	// Add to pool
	p.attestations[key] = att
	slot := uint64(att.Data.Slot)
	p.bySlot[slot] = append(p.bySlot[slot], att)

	return nil
}

// GetBySlot returns all attestations for a given slot.
func (p *AttestationPool) GetBySlot(slot uint64) []*eth.Attestation {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.bySlot[slot]
}

// Count returns the total number of attestations in the pool.
func (p *AttestationPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.attestations)
}

// AttestationMetrics methods

func (m *AttestationMetrics) incrementReceived(subnet uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attestationsReceived[subnet]++
}

func (m *AttestationMetrics) incrementValidated(subnet uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attestationsValidated[subnet]++
}

func (m *AttestationMetrics) incrementProcessed(subnet uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attestationsProcessed[subnet]++
}

func (m *AttestationMetrics) incrementValidationError(subnet uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.validationErrors[subnet]++
}

func (m *AttestationMetrics) incrementProcessingError(subnet uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processingErrors[subnet]++
}

// GetStats returns current metrics for a subnet.
func (m *AttestationMetrics) GetStats(subnet uint64) (received, validated, processed, valErrors, procErrors uint64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.attestationsReceived[subnet],
		m.attestationsValidated[subnet],
		m.attestationsProcessed[subnet],
		m.validationErrors[subnet],
		m.processingErrors[subnet]
}

// GetActiveSubnets returns the currently subscribed subnets.
func (h *AttestationHandler) GetActiveSubnets() []uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subnets := make([]uint64, 0, len(h.activeSubnets))
	for subnet := range h.activeSubnets {
		subnets = append(subnets, subnet)
	}

	return subnets
}
