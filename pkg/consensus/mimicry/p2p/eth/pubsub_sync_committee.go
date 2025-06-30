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

// syncCommitteeProcessor handles sync committee messages across subnets.
type SyncCommitteeProcessor struct {
	ForkDigest [4]byte
	Encoder    encoder.SszNetworkEncoder
	Subnets    []uint64                                                                                 // currently active subnets
	Handler    func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error                   //nolint:staticcheck // deprecated but still functional
	Validator  func(context.Context, *pb.SyncCommitteeMessage, uint64) (pubsub.ValidationResult, error) //nolint:staticcheck // deprecated but still functional
	Gossipsub  *pubsub.Gossipsub                                                                        // Reference to gossipsub for subscription management
	Log        logrus.FieldLogger
}

func (p *SyncCommitteeProcessor) Topics() []string {
	topics := make([]string, len(p.Subnets))
	for i, subnet := range p.Subnets {
		topics[i] = SyncCommitteeSubnetTopic(p.ForkDigest, subnet)
	}

	return topics
}

func (p *SyncCommitteeProcessor) AllPossibleTopics() []string {
	// Return all 4 possible sync committee subnet topics
	topics := make([]string, SyncCommitteeSubnetCount)
	for i := uint64(0); i < SyncCommitteeSubnetCount; i++ {
		topics[i] = SyncCommitteeSubnetTopic(p.ForkDigest, i)
	}

	return topics
}

func (p *SyncCommitteeProcessor) Subscribe(ctx context.Context, subnets []uint64) error {
	if p.Gossipsub == nil {
		return fmt.Errorf("gossipsub reference not set")
	}

	// Update our active subnets
	p.Subnets = subnets

	// Delegate to gossipsub for subscription management
	return p.Gossipsub.SubscribeToMultiProcessorTopics(ctx, "sync_committee", subnets)
}

func (p *SyncCommitteeProcessor) Unsubscribe(ctx context.Context, subnets []uint64) error {
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
			topic := SyncCommitteeSubnetTopic(p.ForkDigest, subnet)
			if err := p.Gossipsub.Unsubscribe(topic); err != nil {
				p.Log.WithError(err).WithField("subnet", subnet).Error("Failed to unsubscribe from sync committee subnet")
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

func (p *SyncCommitteeProcessor) GetActiveSubnets() []uint64 {
	return append([]uint64(nil), p.Subnets...) // return copy
}

func (p *SyncCommitteeProcessor) TopicIndex(topic string) (int, error) {
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

func (p *SyncCommitteeProcessor) Decode(ctx context.Context, topic string, data []byte) (*pb.SyncCommitteeMessage, error) { //nolint:staticcheck // deprecated but still functional
	syncMsg := &pb.SyncCommitteeMessage{} //nolint:staticcheck // deprecated but still functional
	if err := p.Encoder.DecodeGossip(data, syncMsg); err != nil {
		return nil, fmt.Errorf("failed to decode sync committee message: %w", err)
	}

	return syncMsg, nil
}

func (p *SyncCommitteeProcessor) Validate(ctx context.Context, topic string, syncMsg *pb.SyncCommitteeMessage, from string) (pubsub.ValidationResult, error) { //nolint:staticcheck // deprecated but still functional
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

func (p *SyncCommitteeProcessor) Process(ctx context.Context, topic string, syncMsg *pb.SyncCommitteeMessage, from string) error { //nolint:staticcheck // deprecated but still functional
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

func (p *SyncCommitteeProcessor) GetTopicScoreParams(topic string) *pubsub.TopicScoreParams {
	// Return nil to use default/no scoring for now
	// Users can override this by providing their own processor implementation
	return nil
}

// Compile-time check that syncCommitteeProcessor implements pubsub.MultiProcessor.
var _ pubsub.MultiProcessor[*pb.SyncCommitteeMessage] = (*SyncCommitteeProcessor)(nil) //nolint:staticcheck // deprecated but still functional
