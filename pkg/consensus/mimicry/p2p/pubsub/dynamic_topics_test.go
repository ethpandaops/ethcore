package pubsub_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
)

// TestMultiProcessorDynamicTopics demonstrates dynamic topic subscription changes.
func TestMultiProcessorDynamicTopics(t *testing.T) {
	// Create a mock MultiProcessor that supports dynamic topic changes
	processor := &mockMultiProcessor{
		allTopics:    []string{"topic_0", "topic_1", "topic_2", "topic_3"},
		activeTopics: []string{"topic_0"}, // start with only topic 0
	}

	// Test initial active topics
	active := processor.ActiveTopics()
	assert.Equal(t, []string{"topic_0"}, active)

	// Test switching from topic 0 to topic 45 (simulated by using topic_1)
	newTopics := []string{"topic_1"}
	toSubscribe, toUnsubscribe, err := processor.UpdateActiveTopics(newTopics)
	require.NoError(t, err)

	// Should unsubscribe from topic_0 and subscribe to topic_1
	assert.Equal(t, []string{"topic_1"}, toSubscribe)
	assert.Equal(t, []string{"topic_0"}, toUnsubscribe)

	// Verify the processor's state was updated
	active = processor.ActiveTopics()
	assert.Equal(t, []string{"topic_1"}, active)

	// Test switching to multiple topics
	newTopics = []string{"topic_1", "topic_2", "topic_3"}
	toSubscribe, toUnsubscribe, err = processor.UpdateActiveTopics(newTopics)
	require.NoError(t, err)

	// Should subscribe to topic_2 and topic_3, no unsubscriptions (topic_1 stays)
	assert.ElementsMatch(t, []string{"topic_2", "topic_3"}, toSubscribe)
	assert.Empty(t, toUnsubscribe)

	// Verify final state
	active = processor.ActiveTopics()
	assert.ElementsMatch(t, []string{"topic_1", "topic_2", "topic_3"}, active)
}

// mockMultiProcessor implements MultiProcessor for testing
type mockMultiProcessor struct {
	allTopics    []string
	activeTopics []string
}

func (m *mockMultiProcessor) AllPossibleTopics() []string {
	return append([]string(nil), m.allTopics...)
}

func (m *mockMultiProcessor) ActiveTopics() []string {
	return append([]string(nil), m.activeTopics...)
}

func (m *mockMultiProcessor) UpdateActiveTopics(newActiveTopics []string) (toSubscribe []string, toUnsubscribe []string, err error) {
	// Convert current and new topics to sets
	currentSet := make(map[string]bool)
	for _, topic := range m.activeTopics {
		currentSet[topic] = true
	}

	newSet := make(map[string]bool)
	for _, topic := range newActiveTopics {
		newSet[topic] = true
	}

	// Find topics to subscribe to (in new but not in current)
	for topic := range newSet {
		if !currentSet[topic] {
			toSubscribe = append(toSubscribe, topic)
		}
	}

	// Find topics to unsubscribe from (in current but not in new)
	for topic := range currentSet {
		if !newSet[topic] {
			toUnsubscribe = append(toUnsubscribe, topic)
		}
	}

	// Update active topics
	m.activeTopics = append([]string(nil), newActiveTopics...)

	return toSubscribe, toUnsubscribe, nil
}

func (m *mockMultiProcessor) GetTopicScoreParams(topic string) *pubsub.TopicScoreParams {
	return nil
}

func (m *mockMultiProcessor) TopicIndex(topic string) (int, error) {
	for i, t := range m.allTopics {
		if t == topic {
			return i, nil
		}
	}
	return -1, pubsub.NewError(pubsub.ErrInvalidTopic, "topic_index")
}

func (m *mockMultiProcessor) Decode(ctx context.Context, topic string, data []byte) (string, error) {
	return string(data), nil
}

func (m *mockMultiProcessor) Validate(ctx context.Context, topic string, msg string, from string) (pubsub.ValidationResult, error) {
	return pubsub.ValidationAccept, nil
}

func (m *mockMultiProcessor) Process(ctx context.Context, topic string, msg string, from string) error {
	return nil
}
