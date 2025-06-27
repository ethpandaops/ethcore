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

// Attestation topic constants and functions
const (
	// Subnet topic template (requires subnet ID)
	AttestationSubnetTopicTemplate = "beacon_attestation_%d"

	// Network constants
	AttestationSubnetCount = 64
)

// AttestationSubnetTopic constructs an attestation subnet gossipsub topic name
func AttestationSubnetTopic(forkDigest [4]byte, subnet uint64) string {
	name := fmt.Sprintf(AttestationSubnetTopicTemplate, subnet)
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, name)
}

// attestationProcessor handles attestation messages across subnets
type attestationProcessor struct {
	forkDigest [4]byte
	encoder    encoder.SszNetworkEncoder
	subnets    []uint64 // currently active subnets
	handler    func(context.Context, *pb.Attestation, uint64, peer.ID) error
	validator  func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error)
	gossipsub  *pubsub.Gossipsub // Reference to gossipsub for subscription management
	log        logrus.FieldLogger
}

func (p *attestationProcessor) Topics() []string {
	topics := make([]string, len(p.subnets))
	for i, subnet := range p.subnets {
		topics[i] = AttestationSubnetTopic(p.forkDigest, subnet)
	}
	return topics
}

func (p *attestationProcessor) AllPossibleTopics() []string {
	// Return all 64 possible attestation subnet topics
	topics := make([]string, 64)
	for i := uint64(0); i < 64; i++ {
		topics[i] = AttestationSubnetTopic(p.forkDigest, i)
	}
	return topics
}

func (p *attestationProcessor) Subscribe(ctx context.Context, subnets []uint64) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	// Update our active subnets
	p.subnets = subnets

	// Delegate to gossipsub for subscription management
	return p.gossipsub.SubscribeToMultiProcessorTopics(ctx, "attestation", subnets)
}

func (p *attestationProcessor) Unsubscribe(ctx context.Context, subnets []uint64) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	// Remove specified subnets from our active list
	activeMap := make(map[uint64]bool)
	for _, subnet := range p.subnets {
		activeMap[subnet] = true
	}

	for _, subnet := range subnets {
		delete(activeMap, subnet)
	}

	// Update our active subnets
	newSubnets := make([]uint64, 0, len(activeMap))
	for subnet := range activeMap {
		newSubnets = append(newSubnets, subnet)
	}
	p.subnets = newSubnets

	// For now, return error indicating full implementation needed
	return fmt.Errorf("selective unsubscribe not yet implemented - use full resubscribe")
}

func (p *attestationProcessor) GetActiveSubnets() []uint64 {
	return append([]uint64(nil), p.subnets...) // return copy
}

func (p *attestationProcessor) TopicIndex(topic string) (int, error) {
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
	for i, s := range p.subnets {
		if s == subnet {
			return i, nil
		}
	}

	return -1, fmt.Errorf("subnet %d not found in processor list", subnet)
}

func (p *attestationProcessor) Decode(ctx context.Context, topic string, data []byte) (*pb.Attestation, error) {
	att := &pb.Attestation{}
	if err := p.encoder.DecodeGossip(data, att); err != nil {
		return nil, fmt.Errorf("failed to decode attestation: %w", err)
	}
	return att, nil
}

func (p *attestationProcessor) Validate(ctx context.Context, topic string, att *pb.Attestation, from string) (pubsub.ValidationResult, error) {
	index, err := p.TopicIndex(topic)
	if err != nil {
		return pubsub.ValidationReject, fmt.Errorf("invalid topic index: %w", err)
	}

	subnet := p.subnets[index]

	// Defer all validation to external validator function
	if p.validator != nil {
		return p.validator(ctx, att, subnet)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *attestationProcessor) Process(ctx context.Context, topic string, att *pb.Attestation, from string) error {
	if p.handler == nil {
		p.log.Debug("No handler provided, attestation received but not processed")
		return nil
	}

	index, err := p.TopicIndex(topic)
	if err != nil {
		return fmt.Errorf("invalid topic index: %w", err)
	}

	subnet := p.subnets[index]

	peerID, err := peer.Decode(from)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.handler(ctx, att, subnet, peerID)
}

func (p *attestationProcessor) GetTopicScoreParams(topic string) *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that attestationProcessor implements pubsub.MultiProcessor
var _ pubsub.MultiProcessor[*pb.Attestation] = (*attestationProcessor)(nil)
