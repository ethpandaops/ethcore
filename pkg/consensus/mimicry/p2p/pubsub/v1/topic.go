package v1

import (
	"fmt"
	"strconv"
	"strings"
)

// Encoder defines the interface for encoding and decoding messages.
type Encoder[T any] interface {
	// Encode encodes the message into bytes.
	Encode(msg T) ([]byte, error)
	// Decode decodes bytes into the message type.
	Decode(data []byte) (T, error)
}

// Topic represents a type-safe gossipsub topic.
type Topic[T any] struct {
	name string
}

// NewTopic creates a new type-safe topic.
func NewTopic[T any](name string) (*Topic[T], error) {
	if name == "" {
		return nil, fmt.Errorf("topic name cannot be empty")
	}

	return &Topic[T]{
		name: name,
	}, nil
}

// Name returns the topic name.
func (t *Topic[T]) Name() string {
	return t.name
}

// WithForkDigest returns a new topic with the fork digest appended.
func (t *Topic[T]) WithForkDigest(forkDigest [4]byte) *Topic[T] {
	// Format: /eth2/<fork_digest>/<topic_name>/ssz_snappy
	name := fmt.Sprintf("/eth2/%x/%s/ssz_snappy", forkDigest, t.name)

	return &Topic[T]{
		name: name,
	}
}

// SubnetTopic represents a type-safe gossipsub topic with subnet support.
type SubnetTopic[T any] struct {
	pattern    string
	maxSubnets uint64
}

// NewSubnetTopic creates a new type-safe subnet topic.
func NewSubnetTopic[T any](pattern string, maxSubnets uint64) (*SubnetTopic[T], error) {
	if pattern == "" {
		return nil, fmt.Errorf("subnet topic pattern cannot be empty")
	}

	if !strings.Contains(pattern, "%d") {
		return nil, fmt.Errorf("subnet topic pattern must contain '%%d' placeholder for subnet ID")
	}

	if maxSubnets == 0 {
		return nil, fmt.Errorf("maxSubnets must be greater than 0")
	}

	return &SubnetTopic[T]{
		pattern:    pattern,
		maxSubnets: maxSubnets,
	}, nil
}

// TopicForSubnet returns a topic for a specific subnet.
func (st *SubnetTopic[T]) TopicForSubnet(subnet uint64, forkDigest [4]byte) (*Topic[T], error) {
	if subnet >= st.maxSubnets {
		return nil, fmt.Errorf("subnet %d exceeds maximum %d", subnet, st.maxSubnets-1)
	}

	// Format the base topic name with subnet ID
	baseName := fmt.Sprintf(st.pattern, subnet)

	// Create topic and apply fork digest
	topic := &Topic[T]{
		name: baseName,
	}

	return topic.WithForkDigest(forkDigest), nil
}

// ParseSubnet extracts the subnet ID from a topic name.
func (st *SubnetTopic[T]) ParseSubnet(topicName string) (uint64, error) {
	// First, strip the Ethereum gossipsub prefix if present
	// Format: /eth2/<fork_digest>/<topic_name>/ssz_snappy
	if strings.HasPrefix(topicName, "/eth2/") {
		parts := strings.Split(topicName, "/")
		if len(parts) >= 4 {
			// Extract the base topic name (index 3)
			topicName = parts[3]
		}
	}

	// Find where the pattern starts and ends
	patternPrefix := strings.Split(st.pattern, "%d")[0]

	patternSuffix := ""
	if parts := strings.Split(st.pattern, "%d"); len(parts) > 1 {
		patternSuffix = parts[1]
	}

	// Check if the topic matches the pattern
	if !strings.HasPrefix(topicName, patternPrefix) {
		return 0, fmt.Errorf("topic '%s' does not match pattern prefix '%s'", topicName, patternPrefix)
	}

	if patternSuffix != "" && !strings.HasSuffix(topicName, patternSuffix) {
		return 0, fmt.Errorf("topic '%s' does not match pattern suffix '%s'", topicName, patternSuffix)
	}

	// Extract the subnet ID part
	subnetStr := topicName[len(patternPrefix):]
	if patternSuffix != "" {
		subnetStr = subnetStr[:len(subnetStr)-len(patternSuffix)]
	}

	// Parse the subnet ID
	subnet, err := strconv.ParseUint(subnetStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse subnet ID from '%s': %w", subnetStr, err)
	}

	if subnet >= st.maxSubnets {
		return 0, fmt.Errorf("parsed subnet %d exceeds maximum %d", subnet, st.maxSubnets-1)
	}

	return subnet, nil
}

// MaxSubnets returns the maximum number of subnets.
func (st *SubnetTopic[T]) MaxSubnets() uint64 {
	return st.maxSubnets
}
