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
	"sync"
	"time"

	eth "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/topics"
	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
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
type AttestationHandler struct {
	pool          *AttestationPool
	log           logrus.FieldLogger
	metrics       *AttestationMetrics
	activeSubnets map[uint64]bool // Currently subscribed subnets
	mu            sync.RWMutex
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

// ExampleAttestationSetup demonstrates how to set up attestation subnet handlers
// with dynamic subscription management using the v1 API.
func ExampleAttestationSetup(ctx context.Context, log logrus.FieldLogger) error {
	// Create a libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/9000"),
		libp2p.Identity(nil), // Will generate a new identity
	)
	if err != nil {
		return fmt.Errorf("failed to create host: %w", err)
	}

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
		activeSubnets: make(map[uint64]bool),
	}

	// Fork digest for the current network fork
	forkDigest := [4]byte{0xab, 0xcd, 0xef, 0x01} // Example fork digest

	// Create a Gossipsub instance with production settings
	_, err = v1.New(ctx, h,
		v1.WithLogger(log),
		v1.WithMaxMessageSize(1*1024*1024), // 1 MB max for attestations
		v1.WithValidationConcurrency(200),  // Higher concurrency for attestations
		v1.WithPublishTimeout(5*time.Second),
		v1.WithGossipSubParams(pubsub.DefaultGossipSubParams()),
	)
	if err != nil {
		return fmt.Errorf("failed to create gossipsub: %w", err)
	}
	// NOTE: In production, you would use the gossipsub instance (g) for registration and subscription

	// Create event channel for monitoring
	events := make(chan v1.Event, 5000) // Larger buffer for high-volume attestations
	go handler.monitorEvents(ctx, events)

	// Create the subnet topic with SSZ encoder
	encoder := topics.NewSSZSnappyEncoderWithMaxLen[*eth.Attestation](1 * 1024 * 1024)
	subnetTopic, err := topics.CreateSubnetTopicWithEncoder(
		topics.BeaconAttestationTopicPattern,
		topics.AttestationSubnetCount,
		encoder,
	)
	if err != nil {
		return fmt.Errorf("failed to create subnet topic: %w", err)
	}

	// Example: Register handlers for specific subnets
	// In production, this would be done dynamically based on validator duties
	initialSubnets := []uint64{0, 15, 31, 47} // Example subnet IDs

	// NOTE: The v1 API currently requires type erasure for registration.
	// This is a complex pattern that involves wrapping typed handlers with any-typed wrappers.
	// For this example, we demonstrate the conceptual approach and show how to structure
	// the subnet handling logic.

	// In a production implementation, you would either:
	// 1. Use the older pubsub API that handles type erasure internally
	// 2. Create a type erasure helper library for the v1 API
	// 3. Wait for the v1 API to provide built-in type erasure helpers

	// The following code shows the intended pattern:
	for _, subnet := range initialSubnets {
		// Create topic for this specific subnet
		topic, err := subnetTopic.TopicForSubnet(subnet, forkDigest)
		if err != nil {
			return fmt.Errorf("failed to create topic for subnet %d: %w", subnet, err)
		}

		// Create a subnet-specific validator closure
		validator := createSubnetValidator(handler, subnet)
		processor := createSubnetProcessor(handler, subnet)

		// Create handler configuration for this subnet
		handlerConfig := v1.NewHandlerConfig[*eth.Attestation](
			v1.WithValidator(validator),
			v1.WithProcessor(processor),
			v1.WithScoreParams[*eth.Attestation](createAttestationScoreParams()),
			v1.WithEvents[*eth.Attestation](events),
		)

		// In production, registration would happen here with proper type erasure
		_ = topic
		_ = handlerConfig

		handler.log.WithFields(logrus.Fields{
			"subnet": subnet,
			"topic":  topic.Name(),
		}).Info("would register attestation subnet handler")
	}

	// Subscribe to the registered subnets
	// NOTE: In production, this would happen after successful registration
	// For this example, we show the subscription pattern
	/*
		for _, subnet := range initialSubnets {
			topic, err := subnetTopic.TopicForSubnet(subnet, forkDigest)
			if err != nil {
				return fmt.Errorf("failed to create topic for subnet %d: %w", subnet, err)
			}
			if err := handler.subscribeToSubnet(ctx, g, topic, subnet); err != nil {
				return fmt.Errorf("failed to subscribe to subnet %d: %w", subnet, err)
			}
		}

		// Start periodic subnet rotation (simulating changing validator duties)
		go handler.manageSubnetSubscriptions(ctx, g, subnetTopic)
	*/

	// Log successful setup
	log.WithFields(logrus.Fields{
		"fork_digest":     fmt.Sprintf("%x", forkDigest),
		"host_id":         h.ID(),
		"initial_subnets": initialSubnets,
	}).Info("attestation subnet handler setup complete")

	return nil
}

// createSubnetValidator creates a validator function for a specific subnet.
func createSubnetValidator(h *AttestationHandler, subnet uint64) v1.Validator[*eth.Attestation] {
	return func(ctx context.Context, att *eth.Attestation, from peer.ID) v1.ValidationResult {
		return h.validateAttestationSubnet(ctx, att, from, subnet)
	}
}

// createSubnetProcessor creates a processor function for a specific subnet.
func createSubnetProcessor(h *AttestationHandler, subnet uint64) v1.Processor[*eth.Attestation] {
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

	// Validate basic attestation structure
	if att == nil || att.Data == nil {
		h.log.WithFields(logrus.Fields{
			"subnet": subnet,
			"from":   from,
		}).Debug("received nil attestation or data")
		h.metrics.incrementValidationError(subnet)
		return v1.ValidationReject
	}

	// Check if attestation belongs to this subnet
	expectedSubnet := computeSubnetForAttestation(att)
	if expectedSubnet != subnet {
		h.log.WithFields(logrus.Fields{
			"subnet":          subnet,
			"expected_subnet": expectedSubnet,
			"slot":            att.Data.Slot,
			"committee_index": att.Data.CommitteeIndex,
		}).Debug("attestation on wrong subnet")
		h.metrics.incrementValidationError(subnet)
		return v1.ValidationReject
	}

	// Validate attestation slot is within acceptable range
	// In production, this would check against current slot from beacon node
	currentSlot := uint64(time.Now().Unix() / 12) // Simplified slot calculation
	if uint64(att.Data.Slot) > currentSlot+1 {
		h.log.WithFields(logrus.Fields{
			"subnet":       subnet,
			"att_slot":     att.Data.Slot,
			"current_slot": currentSlot,
		}).Debug("attestation from future slot")
		h.metrics.incrementValidationError(subnet)
		return v1.ValidationIgnore // Future attestations are ignored, not rejected
	}

	// Check if attestation is too old
	if uint64(att.Data.Slot)+64 < currentSlot { // Keep attestations for ~13 minutes
		h.log.WithFields(logrus.Fields{
			"subnet":       subnet,
			"att_slot":     att.Data.Slot,
			"current_slot": currentSlot,
		}).Debug("attestation too old")
		h.metrics.incrementValidationError(subnet)
		return v1.ValidationIgnore
	}

	h.metrics.incrementValidated(subnet)
	return v1.ValidationAccept
}

// processAttestationSubnet processes validated attestations from a subnet.
func (h *AttestationHandler) processAttestationSubnet(
	ctx context.Context,
	att *eth.Attestation,
	from peer.ID,
	subnet uint64,
) error {
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
	return nil
}

// subscribeToSubnet subscribes to a specific attestation subnet.
// This demonstrates the subscription pattern for attestation subnets.
func (h *AttestationHandler) subscribeToSubnet(
	ctx context.Context,
	g *v1.Gossipsub,
	topic *v1.Topic[*eth.Attestation],
	subnet uint64,
) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.activeSubnets[subnet] {
		return nil // Already subscribed
	}

	// Subscribe to the subnet topic
	sub, err := v1.Subscribe(context.Background(), g, topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to subnet %d: %w", subnet, err)
	}

	// In production, you would store the subscription for later cleanup
	// For this example, we just mark it as active
	h.activeSubnets[subnet] = true

	h.log.WithFields(logrus.Fields{
		"subnet": subnet,
		"topic":  topic.Name(),
	}).Info("subscribed to attestation subnet")

	// Cancel subscription after some time (simulating duty rotation)
	go func() {
		time.Sleep(5 * time.Minute) // Subscribe for 5 minutes
		sub.Cancel()
		h.mu.Lock()
		delete(h.activeSubnets, subnet)
		h.mu.Unlock()
		h.log.WithField("subnet", subnet).Info("unsubscribed from attestation subnet")
	}()

	return nil
}

// manageSubnetSubscriptions periodically rotates subnet subscriptions.
// In production, this would be driven by validator duty assignments.
// This demonstrates how dynamic subnet management would work.
func (h *AttestationHandler) manageSubnetSubscriptions(
	ctx context.Context,
	g *v1.Gossipsub,
	subnetTopic *v1.SubnetTopic[*eth.Attestation],
) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Simulate changing validator duties by subscribing to new random subnets
			// In production, this would be based on actual validator assignments
			newSubnet := uint64(time.Now().Unix() % topics.AttestationSubnetCount)

			h.mu.RLock()
			alreadySubscribed := h.activeSubnets[newSubnet]
			h.mu.RUnlock()

			if !alreadySubscribed {
				// Create topic for the new subnet
				// Note: In production, you would store the fork digest somewhere accessible
				forkDigest := [4]byte{0xab, 0xcd, 0xef, 0x01} // Example fork digest
				topic, err := subnetTopic.TopicForSubnet(newSubnet, forkDigest)
				if err != nil {
					h.log.WithError(err).WithField("subnet", newSubnet).Error("failed to create topic for subnet")
					continue
				}

				if err := h.subscribeToSubnet(ctx, g, topic, newSubnet); err != nil {
					h.log.WithError(err).WithField("subnet", newSubnet).Error("failed to subscribe to new subnet")
				}
			}
		}
	}
}

// monitorEvents processes events from the event channel.
func (h *AttestationHandler) monitorEvents(ctx context.Context, events <-chan v1.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-events:
			// Process events for metrics and monitoring
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

// computeSubnetForAttestation calculates which subnet an attestation belongs to.
// This is based on the committee index and slot.
func computeSubnetForAttestation(att *eth.Attestation) uint64 {
	if att.Data == nil {
		return 0
	}

	// Simplified subnet calculation
	// In production, this would use the proper algorithm from the spec
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
	key := fmt.Sprintf("%x", att.Data.BeaconBlockRoot)

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
	p.bySlot[uint64(att.Data.Slot)] = append(p.bySlot[uint64(att.Data.Slot)], att)

	return nil
}

// GetBySlot returns all attestations for a given slot.
func (p *AttestationPool) GetBySlot(slot uint64) []*eth.Attestation {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.bySlot[slot]
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
