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

// Sync committee topic constants and functions
const (
	// Subnet topic template (requires subnet ID)
	SyncCommitteeSubnetTopicTemplate = "sync_committee_%d"

	// Network constants
	SyncCommitteeSubnetCount = 4
)

// SyncCommitteeSubnetTopic constructs a sync committee subnet gossipsub topic name
func SyncCommitteeSubnetTopic(forkDigest [4]byte, subnet uint64) string {
	name := fmt.Sprintf(SyncCommitteeSubnetTopicTemplate, subnet)
	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, name)
}

// syncCommitteeProcessor handles sync committee messages across subnets
type syncCommitteeProcessor struct {
	forkDigest [4]byte
	encoder    encoder.SszNetworkEncoder
	subnets    []uint64 // currently active subnets
	handler    func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error
	validator  func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error)
	gossipsub  *pubsub.Gossipsub // Reference to gossipsub for subscription management
	log        logrus.FieldLogger
}

func (p *syncCommitteeProcessor) Topics() []string {
	topics := make([]string, len(p.subnets))
	for i, subnet := range p.subnets {
		topics[i] = SyncCommitteeSubnetTopic(p.forkDigest, subnet)
	}
	return topics
}

func (p *syncCommitteeProcessor) AllPossibleTopics() []string {
	// Return all 4 possible sync committee subnet topics
	topics := make([]string, SyncCommitteeSubnetCount)
	for i := uint64(0); i < SyncCommitteeSubnetCount; i++ {
		topics[i] = SyncCommitteeSubnetTopic(p.forkDigest, i)
	}
	return topics
}

func (p *syncCommitteeProcessor) Subscribe(ctx context.Context, subnets []uint64) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	// Update our active subnets
	p.subnets = subnets

	// Delegate to gossipsub for subscription management
	return p.gossipsub.SubscribeToMultiProcessorTopics(ctx, "sync_committee", subnets)
}

func (p *syncCommitteeProcessor) Unsubscribe(ctx context.Context, subnets []uint64) error {
	if p.gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	// Remove specified subnets from our active list
	activeMap := make(map[uint64]bool)
	for _, subnet := range p.subnets {
		activeMap[subnet] = true
	}

	// Unsubscribe from each subnet topic
	for _, subnet := range subnets {
		if activeMap[subnet] {
			topic := SyncCommitteeSubnetTopic(p.forkDigest, subnet)
			if err := p.gossipsub.Unsubscribe(topic); err != nil {
				p.log.WithError(err).WithField("subnet", subnet).Error("Failed to unsubscribe from sync committee subnet")
				// Continue unsubscribing from other subnets even if one fails
			}
			delete(activeMap, subnet)
		}
	}

	// Update our active subnets
	newSubnets := make([]uint64, 0, len(activeMap))
	for subnet := range activeMap {
		newSubnets = append(newSubnets, subnet)
	}
	p.subnets = newSubnets

	return nil
}

func (p *syncCommitteeProcessor) GetActiveSubnets() []uint64 {
	return append([]uint64(nil), p.subnets...) // return copy
}

func (p *syncCommitteeProcessor) TopicIndex(topic string) (int, error) {
	// Extract subnet from topic using regex
	re := regexp.MustCompile(`sync_committee_(\d+)`)
	matches := re.FindStringSubmatch(topic)
	if len(matches) != 2 {
		return -1, fmt.Errorf("invalid sync committee topic format")
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

func (p *syncCommitteeProcessor) Decode(ctx context.Context, topic string, data []byte) (*pb.SyncCommitteeMessage, error) { //nolint:staticcheck // deprecated but still functional
	syncMsg := &pb.SyncCommitteeMessage{} //nolint:staticcheck // deprecated but still functional
	if err := p.encoder.DecodeGossip(data, syncMsg); err != nil {
		return nil, fmt.Errorf("failed to decode sync committee message: %w", err)
	}
	return syncMsg, nil
}

func (p *syncCommitteeProcessor) Validate(ctx context.Context, topic string, syncMsg *pb.SyncCommitteeMessage, from string) (pubsub.ValidationResult, error) { //nolint:staticcheck // deprecated but still functional
	index, err := p.TopicIndex(topic)
	if err != nil {
		return pubsub.ValidationReject, fmt.Errorf("invalid topic index: %w", err)
	}

	subnet := p.subnets[index]

	// Defer all validation to external validator function
	if p.validator != nil {
		return p.validator(ctx, syncMsg, subnet)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *syncCommitteeProcessor) Process(ctx context.Context, topic string, syncMsg *pb.SyncCommitteeMessage, from string) error { //nolint:staticcheck // deprecated but still functional
	if p.handler == nil {
		p.log.Debug("No handler provided, sync committee message received but not processed")
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

	return p.handler(ctx, syncMsg, subnet, peerID)
}

func (p *syncCommitteeProcessor) GetTopicScoreParams(topic string) *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that syncCommitteeProcessor implements pubsub.MultiProcessor
var _ pubsub.MultiProcessor[*pb.SyncCommitteeMessage] = (*syncCommitteeProcessor)(nil)
