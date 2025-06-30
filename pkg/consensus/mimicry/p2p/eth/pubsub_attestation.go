package eth

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// Attestation topic constants and functions.
const (
	// Subnet topic template (requires subnet ID).
	AttestationSubnetTopicTemplate = "beacon_attestation_%d"

	// Network constants.
	AttestationSubnetCount = 64
)

// AttestationSubnetTopic constructs an attestation subnet gossipsub topic name.
func AttestationSubnetTopic(forkDigest [4]byte, subnet uint64) string {
	name := fmt.Sprintf(AttestationSubnetTopicTemplate, subnet)

	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, name)
}

// AttestationProcessor defines the interface for handling attestation messages across subnets.
type AttestationProcessor interface {
	pubsub.MultiProcessor[*pb.Attestation]
	// GetActiveSubnets returns a copy of the currently active subnets.
	GetActiveSubnets() []uint64
}

// DefaultAttestationProcessor handles attestation messages across subnets.
type DefaultAttestationProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Subnets    []uint64 // currently active subnets
	Handler    func(context.Context, *pb.Attestation, uint64, peer.ID) error
	Validator  func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error)
	Log        logrus.FieldLogger
}

func (p *DefaultAttestationProcessor) Topics() []string {
	topics := make([]string, len(p.Subnets))
	for i, subnet := range p.Subnets {
		topics[i] = AttestationSubnetTopic(p.ForkDigest, subnet)
	}

	return topics
}

func (p *DefaultAttestationProcessor) AllPossibleTopics() []string {
	// Return all 64 possible attestation subnet topics
	topics := make([]string, 64)
	for i := uint64(0); i < 64; i++ {
		topics[i] = AttestationSubnetTopic(p.ForkDigest, i)
	}

	return topics
}


func (p *DefaultAttestationProcessor) GetActiveSubnets() []uint64 {
	return append([]uint64(nil), p.Subnets...) // return copy
}

func (p *DefaultAttestationProcessor) ActiveTopics() []string {
	return p.Topics()
}

func (p *DefaultAttestationProcessor) UpdateActiveTopics(newActiveTopics []string) (toSubscribe []string, toUnsubscribe []string, err error) {
	// Parse subnet IDs from topic names
	newSubnets := make([]uint64, 0, len(newActiveTopics))
	for _, topic := range newActiveTopics {
		re := regexp.MustCompile(`beacon_attestation_(\d+)`)
		matches := re.FindStringSubmatch(topic)
		if len(matches) != 2 {
			return nil, nil, fmt.Errorf("invalid attestation topic format: %s", topic)
		}
		
		subnet, parseErr := strconv.ParseUint(matches[1], 10, 64)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("failed to parse subnet from topic %s: %w", topic, parseErr)
		}
		
		if subnet >= AttestationSubnetCount {
			return nil, nil, fmt.Errorf("subnet %d exceeds maximum subnet count %d", subnet, AttestationSubnetCount)
		}
		
		newSubnets = append(newSubnets, subnet)
	}
	
	// Convert current and new subnets to sets for comparison
	currentSet := make(map[uint64]bool)
	for _, subnet := range p.Subnets {
		currentSet[subnet] = true
	}
	
	newSet := make(map[uint64]bool)
	for _, subnet := range newSubnets {
		newSet[subnet] = true
	}
	
	// Find subnets to subscribe to (in new but not in current)
	toSubscribeSubnets := make([]uint64, 0)
	for subnet := range newSet {
		if !currentSet[subnet] {
			toSubscribeSubnets = append(toSubscribeSubnets, subnet)
		}
	}
	
	// Find subnets to unsubscribe from (in current but not in new)
	toUnsubscribeSubnets := make([]uint64, 0)
	for subnet := range currentSet {
		if !newSet[subnet] {
			toUnsubscribeSubnets = append(toUnsubscribeSubnets, subnet)
		}
	}
	
	// Convert subnet IDs back to topic names
	toSubscribe = make([]string, len(toSubscribeSubnets))
	for i, subnet := range toSubscribeSubnets {
		toSubscribe[i] = AttestationSubnetTopic(p.ForkDigest, subnet)
	}
	
	toUnsubscribe = make([]string, len(toUnsubscribeSubnets))
	for i, subnet := range toUnsubscribeSubnets {
		toUnsubscribe[i] = AttestationSubnetTopic(p.ForkDigest, subnet)
	}
	
	// Update the processor's subnet list
	p.Subnets = newSubnets
	
	return toSubscribe, toUnsubscribe, nil
}

func (p *DefaultAttestationProcessor) TopicIndex(topic string) (int, error) {
	// Extract subnet from topic using regex
	re := regexp.MustCompile(`beacon_attestation_(\d+)`)
	matches := re.FindStringSubmatch(topic)

	if len(matches) != 2 {
		return -1, fmt.Errorf("invalid attestation topic format")
	}

	subnet, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return -1, err
	}

	// Find index in our subnet list
	for i, s := range p.Subnets {
		if s == subnet {
			return i, nil
		}
	}

	return -1, fmt.Errorf("subnet %d not found in processor list", subnet)
}

func (p *DefaultAttestationProcessor) Decode(ctx context.Context, topic string, data []byte) (*pb.Attestation, error) {
	att := &pb.Attestation{}
	if err := p.Encoder.DecodeGossip(data, att); err != nil {
		return nil, fmt.Errorf("failed to decode attestation: %w", err)
	}

	return att, nil
}

func (p *DefaultAttestationProcessor) Validate(ctx context.Context, topic string, att *pb.Attestation, from string) (pubsub.ValidationResult, error) {
	index, err := p.TopicIndex(topic)
	if err != nil {
		return pubsub.ValidationReject, fmt.Errorf("invalid topic index: %w", err)
	}

	subnet := p.Subnets[index]

	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, att, subnet)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *DefaultAttestationProcessor) Process(ctx context.Context, topic string, att *pb.Attestation, from string) error {
	if p.Handler == nil {
		p.Log.Debug("No handler provided, attestation received but not processed")

		return nil
	}

	index, err := p.TopicIndex(topic)
	if err != nil {
		return fmt.Errorf("invalid topic index: %w", err)
	}

	subnet := p.Subnets[index]

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.Handler(ctx, att, subnet, peerID)
}

func (p *DefaultAttestationProcessor) GetTopicScoreParams(topic string) *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that DefaultAttestationProcessor implements AttestationProcessor.
var _ AttestationProcessor = (*DefaultAttestationProcessor)(nil)
