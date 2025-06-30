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

// attestationProcessor handles attestation messages across subnets.
type AttestationProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Subnets    []uint64 // currently active subnets
	Handler    func(context.Context, *pb.Attestation, uint64, peer.ID) error
	Validator  func(context.Context, *pb.Attestation, uint64) (pubsub.ValidationResult, error)
	Gossipsub  *pubsub.Gossipsub // Reference to gossipsub for subscription management
	Log        logrus.FieldLogger
}

func (p *AttestationProcessor) Topics() []string {
	topics := make([]string, len(p.Subnets))
	for i, subnet := range p.Subnets {
		topics[i] = AttestationSubnetTopic(p.ForkDigest, subnet)
	}

	return topics
}

func (p *AttestationProcessor) AllPossibleTopics() []string {
	// Return all 64 possible attestation subnet topics
	topics := make([]string, 64)
	for i := uint64(0); i < 64; i++ {
		topics[i] = AttestationSubnetTopic(p.ForkDigest, i)
	}

	return topics
}

func (p *AttestationProcessor) Subscribe(ctx context.Context, subnets []uint64) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	// Update our active subnets
	p.Subnets = subnets

	// Delegate to gossipsub for subscription management
	return p.Gossipsub.SubscribeToMultiProcessorTopics(ctx, "attestation", subnets)
}

func (p *AttestationProcessor) Unsubscribe(ctx context.Context, subnets []uint64) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	// Remove specified subnets from our active list
	activeMap := make(map[uint64]bool)
	for _, subnet := range p.Subnets {
		activeMap[subnet] = true
	}

	// Unsubscribe from each subnet topic
	for _, subnet := range subnets {
		if activeMap[subnet] {
			topic := AttestationSubnetTopic(p.ForkDigest, subnet)
			if err := p.Gossipsub.Unsubscribe(topic); err != nil {
				p.Log.WithError(err).WithField("subnet", subnet).Error("Failed to unsubscribe from attestation subnet")
			}

			delete(activeMap, subnet)
		}
	}

	// Update our active subnets
	newSubnets := make([]uint64, 0, len(activeMap))
	for subnet := range activeMap {
		newSubnets = append(newSubnets, subnet)
	}

	p.Subnets = newSubnets

	return nil
}

func (p *AttestationProcessor) GetActiveSubnets() []uint64 {
	return append([]uint64(nil), p.Subnets...) // return copy
}

func (p *AttestationProcessor) TopicIndex(topic string) (int, error) {
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

func (p *AttestationProcessor) Decode(ctx context.Context, topic string, data []byte) (*pb.Attestation, error) {
	att := &pb.Attestation{}
	if err := p.Encoder.DecodeGossip(data, att); err != nil {
		return nil, fmt.Errorf("failed to decode attestation: %w", err)
	}

	return att, nil
}

func (p *AttestationProcessor) Validate(ctx context.Context, topic string, att *pb.Attestation, from string) (pubsub.ValidationResult, error) {
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

func (p *AttestationProcessor) Process(ctx context.Context, topic string, att *pb.Attestation, from string) error {
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

func (p *AttestationProcessor) GetTopicScoreParams(topic string) *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that attestationProcessor implements pubsub.MultiProcessor.
var _ pubsub.MultiProcessor[*pb.Attestation] = (*AttestationProcessor)(nil)
