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

// Sync committee topic constants and functions.
const (
	// Subnet topic template (requires subnet ID).
	SyncCommitteeSubnetTopicTemplate = "sync_committee_%d"

	// Network constants.
	SyncCommitteeSubnetCount = 4
)

// SyncCommitteeSubnetTopic constructs a sync committee subnet gossipsub topic name.
func SyncCommitteeSubnetTopic(forkDigest [4]byte, subnet uint64) string {
	name := fmt.Sprintf(SyncCommitteeSubnetTopicTemplate, subnet)

	return fmt.Sprintf(GossipsubTopicFormat, forkDigest, name)
}

// SyncCommitteeProcessor defines the interface for handling sync committee messages across subnets.
type SyncCommitteeProcessor interface {
	pubsub.MultiProcessor[*pb.SyncCommitteeMessage]
	// GetActiveSubnets returns a copy of the currently active subnets.
	GetActiveSubnets() []uint64
}

// DefaultSyncCommitteeProcessor handles sync committee messages across subnets.
type DefaultSyncCommitteeProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Subnets    []uint64                                                                                 // currently active subnets
	Handler    func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error                   //nolint:staticcheck // deprecated but still functional
	Validator  func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error) //nolint:staticcheck // deprecated but still functional
	Log        logrus.FieldLogger
}

func (p *DefaultSyncCommitteeProcessor) Topics() []string {
	topics := make([]string, len(p.Subnets))
	for i, subnet := range p.Subnets {
		topics[i] = SyncCommitteeSubnetTopic(p.ForkDigest, subnet)
	}

	return topics
}

func (p *DefaultSyncCommitteeProcessor) AllPossibleTopics() []string {
	// Return all 4 possible sync committee subnet topics
	topics := make([]string, SyncCommitteeSubnetCount)
	for i := uint64(0); i < SyncCommitteeSubnetCount; i++ {
		topics[i] = SyncCommitteeSubnetTopic(p.ForkDigest, i)
	}

	return topics
}


func (p *DefaultSyncCommitteeProcessor) GetActiveSubnets() []uint64 {
	return append([]uint64(nil), p.Subnets...) // return copy
}

func (p *DefaultSyncCommitteeProcessor) ActiveTopics() []string {
	return p.Topics()
}

func (p *DefaultSyncCommitteeProcessor) UpdateActiveTopics(newActiveTopics []string) (toSubscribe []string, toUnsubscribe []string, err error) {
	// Parse subnet IDs from topic names
	newSubnets := make([]uint64, 0, len(newActiveTopics))
	for _, topic := range newActiveTopics {
		re := regexp.MustCompile(`sync_committee_(\d+)`)
		matches := re.FindStringSubmatch(topic)
		if len(matches) != 2 {
			return nil, nil, fmt.Errorf("invalid sync committee topic format: %s", topic)
		}
		
		subnet, parseErr := strconv.ParseUint(matches[1], 10, 64)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("failed to parse subnet from topic %s: %w", topic, parseErr)
		}
		
		if subnet >= SyncCommitteeSubnetCount {
			return nil, nil, fmt.Errorf("subnet %d exceeds maximum subnet count %d", subnet, SyncCommitteeSubnetCount)
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
		toSubscribe[i] = SyncCommitteeSubnetTopic(p.ForkDigest, subnet)
	}
	
	toUnsubscribe = make([]string, len(toUnsubscribeSubnets))
	for i, subnet := range toUnsubscribeSubnets {
		toUnsubscribe[i] = SyncCommitteeSubnetTopic(p.ForkDigest, subnet)
	}
	
	// Update the processor's subnet list
	p.Subnets = newSubnets
	
	return toSubscribe, toUnsubscribe, nil
}

func (p *DefaultSyncCommitteeProcessor) TopicIndex(topic string) (int, error) {
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
	for i, s := range p.Subnets {
		if s == subnet {
			return i, nil
		}
	}

	return -1, fmt.Errorf("subnet %d not found in processor list", subnet)
}

func (p *DefaultSyncCommitteeProcessor) Decode(ctx context.Context, topic string, data []byte) (*pb.SyncCommitteeMessage, error) { //nolint:staticcheck // deprecated but still functional
	syncMsg := &pb.SyncCommitteeMessage{} //nolint:staticcheck // deprecated but still functional
	if err := p.Encoder.DecodeGossip(data, syncMsg); err != nil {
		return nil, fmt.Errorf("failed to decode sync committee message: %w", err)
	}

	return syncMsg, nil
}

func (p *DefaultSyncCommitteeProcessor) Validate(ctx context.Context, topic string, syncMsg *pb.SyncCommitteeMessage, from string) (pubsub.ValidationResult, error) { //nolint:staticcheck // deprecated but still functional
	index, err := p.TopicIndex(topic)
	if err != nil {
		return pubsub.ValidationReject, fmt.Errorf("invalid topic index: %w", err)
	}

	subnet := p.Subnets[index]

	// Defer all validation to external Validator function
	if p.Validator != nil {
		return p.Validator(ctx, syncMsg, subnet)
	}

	// Default to accept if no validator provided
	return pubsub.ValidationAccept, nil
}

func (p *DefaultSyncCommitteeProcessor) Process(ctx context.Context, topic string, syncMsg *pb.SyncCommitteeMessage, from string) error { //nolint:staticcheck // deprecated but still functional
	if p.Handler == nil {
		p.Log.Debug("No handler provided, sync committee message received but not processed")

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

	return p.Handler(ctx, syncMsg, subnet, peerID)
}

func (p *DefaultSyncCommitteeProcessor) GetTopicScoreParams(topic string) *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that DefaultSyncCommitteeProcessor implements SyncCommitteeProcessor.
var _ SyncCommitteeProcessor = (*DefaultSyncCommitteeProcessor)(nil)
