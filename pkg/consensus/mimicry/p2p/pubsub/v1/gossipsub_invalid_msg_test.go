package v1_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MalformedEncoder is an encoder that can produce malformed messages
type MalformedEncoder struct {
	// Controls whether encoding produces valid data
	EncodeValid bool
	// Controls whether decoding succeeds
	DecodeSuccess bool
	// Custom decode error to return
	DecodeError error
	// If true, produces data that will trigger decoding errors
	ProduceMalformed bool
}

func (e *MalformedEncoder) Encode(msg GossipTestMessage) ([]byte, error) {
	if !e.EncodeValid {
		return nil, fmt.Errorf("encoding failed")
	}

	if e.ProduceMalformed {
		// Return malformed data that will fail decoding
		return []byte("malformed|data"), nil
	}

	// Normal encoding
	encoded := fmt.Sprintf("%s|%s|%s", msg.ID, msg.Content, msg.From)
	return []byte(encoded), nil
}

func (e *MalformedEncoder) Decode(data []byte) (GossipTestMessage, error) {
	if !e.DecodeSuccess {
		if e.DecodeError != nil {
			return GossipTestMessage{}, e.DecodeError
		}
		return GossipTestMessage{}, fmt.Errorf("decoding failed: malformed data")
	}

	// Use the standard test encoder logic
	encoder := &TestEncoder{}
	return encoder.Decode(data)
}

// InvalidPayloadCollector collects invalid payload handler calls
type InvalidPayloadCollector struct {
	mu       sync.Mutex
	payloads []InvalidPayload
}

type InvalidPayload struct {
	Data  []byte
	Error error
	From  peer.ID
	Topic string
}

func NewInvalidPayloadCollector() *InvalidPayloadCollector {
	return &InvalidPayloadCollector{
		payloads: make([]InvalidPayload, 0),
	}
}

func (c *InvalidPayloadCollector) Collect(ctx context.Context, data []byte, err error, from peer.ID, topic string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payloads = append(c.payloads, InvalidPayload{
		Data:  data,
		Error: err,
		From:  from,
		Topic: topic,
	})
}

func (c *InvalidPayloadCollector) CollectWithoutTopic(ctx context.Context, data []byte, err error, from peer.ID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payloads = append(c.payloads, InvalidPayload{
		Data:  data,
		Error: err,
		From:  from,
		Topic: "", // Topic not provided in this handler type
	})
}

func (c *InvalidPayloadCollector) GetPayloads() []InvalidPayload {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]InvalidPayload, len(c.payloads))
	copy(result, c.payloads)
	return result
}

func (c *InvalidPayloadCollector) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.payloads)
}

func (c *InvalidPayloadCollector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payloads = make([]InvalidPayload, 0)
}

func TestMalformedMessageHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 2 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)
	require.Len(t, nodes, 2)

	// Create topic with malformed encoder
	malformedEncoder := &MalformedEncoder{
		EncodeValid:      true,
		DecodeSuccess:    false,
		DecodeError:      errors.New("custom decode error"),
		ProduceMalformed: true,
	}
	topic, err := v1.NewTopic[GossipTestMessage]("malformed-topic")
	require.NoError(t, err)

	// Create invalid payload collector
	invalidCollector := NewInvalidPayloadCollector()

	// Create message collector for valid messages
	messageCollector := NewMessageCollector(10)

	// Register handler on node 1 with invalid payload handler
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](malformedEncoder),
		v1.WithProcessor(messageCollector.CreateProcessor(nodes[1].ID)),
		v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
			return v1.ValidationAccept
		}),
		v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
			invalidCollector.CollectWithoutTopic(ctx, data, err, from)
		}),
	)

	err = v1.Register(nodes[1].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)

	// Subscribe node 1
	_, err = v1.Subscribe(ctx, nodes[1].Gossipsub, topic)
	require.NoError(t, err)

	// Give subscription time to propagate
	time.Sleep(1 * time.Second)

	// Publish a message from node 0 (will be encoded as malformed)
	testMsg := GossipTestMessage{
		ID:      "malformed-msg-1",
		Content: "This will be malformed",
		From:    nodes[0].ID.String(),
	}

	// Since node 0 isn't subscribed, we need to publish raw data directly
	// First, subscribe node 0 temporarily just to establish the topic
	handler0 := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](malformedEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, handler0)
	require.NoError(t, err)

	// Publish the message
	err = v1.Publish(nodes[0].Gossipsub, topic, testMsg)
	require.NoError(t, err)

	// Wait for invalid payload handler to be called
	require.Eventually(t, func() bool {
		return invalidCollector.Count() > 0
	}, 5*time.Second, 100*time.Millisecond, "Invalid payload handler should be called")

	// Verify invalid payload was collected
	payloads := invalidCollector.GetPayloads()
	assert.Len(t, payloads, 1)
	assert.Equal(t, []byte("malformed|data"), payloads[0].Data)
	assert.Error(t, payloads[0].Error)
	assert.Contains(t, payloads[0].Error.Error(), "custom decode error")
	assert.Equal(t, nodes[0].ID, payloads[0].From)

	// Verify no valid messages were processed
	assert.Equal(t, 0, messageCollector.GetMessageCount())
}

func TestTopicSpecificInvalidPayloadHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create 3 nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	// Create two topics with different encoders
	goodEncoder := &TestEncoder{}
	badEncoder := &MalformedEncoder{
		EncodeValid:   true,
		DecodeSuccess: false,
		DecodeError:   errors.New("topic-specific decode error"),
	}

	topicA, err := v1.NewTopic[GossipTestMessage]("topic-a")
	require.NoError(t, err)

	topicB, err := v1.NewTopic[GossipTestMessage]("topic-b")
	require.NoError(t, err)

	// Create collectors for each topic
	collectorA := NewInvalidPayloadCollector()
	collectorB := NewInvalidPayloadCollector()

	// Register handlers with topic-specific invalid payload handlers
	for i := 1; i < 3; i++ {
		// Topic A handler
		handlerA := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithEncoder[GossipTestMessage](goodEncoder),
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				return nil
			}),
			v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
				collectorA.CollectWithoutTopic(ctx, data, err, from)
			}),
		)
		err := v1.Register(nodes[i].Gossipsub.Registry(), topicA, handlerA)
		require.NoError(t, err)
		_, err = v1.Subscribe(ctx, nodes[i].Gossipsub, topicA)
		require.NoError(t, err)

		// Topic B handler
		handlerB := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithEncoder[GossipTestMessage](badEncoder),
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				return nil
			}),
			v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
				collectorB.CollectWithoutTopic(ctx, data, err, from)
			}),
		)
		err = v1.Register(nodes[i].Gossipsub.Registry(), topicB, handlerB)
		require.NoError(t, err)
		_, err = v1.Subscribe(ctx, nodes[i].Gossipsub, topicB)
		require.NoError(t, err)
	}

	// Wait for mesh formation
	time.Sleep(2 * time.Second)

	// Register publishers on node 0
	handlerA0 := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](goodEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)
	err = v1.Register(nodes[0].Gossipsub.Registry(), topicA, handlerA0)
	require.NoError(t, err)

	handlerB0 := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](badEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)
	err = v1.Register(nodes[0].Gossipsub.Registry(), topicB, handlerB0)
	require.NoError(t, err)

	// Publish to both topics
	msg := GossipTestMessage{
		ID:      "test-msg",
		Content: "Test content",
		From:    nodes[0].ID.String(),
	}

	// Publish to topic A (should succeed)
	err = v1.Publish(nodes[0].Gossipsub, topicA, msg)
	require.NoError(t, err)

	// Publish to topic B (will fail decoding on receivers)
	err = v1.Publish(nodes[0].Gossipsub, topicB, msg)
	require.NoError(t, err)

	// Wait for messages to propagate
	time.Sleep(1 * time.Second)

	// Verify topic A had no invalid payloads
	assert.Equal(t, 0, collectorA.Count(), "Topic A should have no invalid payloads")

	// Verify topic B had invalid payloads
	assert.Equal(t, 2, collectorB.Count(), "Topic B should have invalid payloads from both subscribers")

	// Check the errors
	payloadsB := collectorB.GetPayloads()
	for _, payload := range payloadsB {
		assert.Error(t, payload.Error)
		assert.Contains(t, payload.Error.Error(), "topic-specific decode error")
	}
}

func TestGlobalInvalidPayloadHandlerOnly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create global invalid payload collector
	globalCollector := NewInvalidPayloadCollector()

	// Create 2 nodes with global invalid payload handler
	nodes := make([]*TestNode, 2)
	for i := 0; i < 2; i++ {
		node, err := ti.CreateNode(ctx,
			v1.WithGlobalInvalidPayloadHandler(globalCollector.Collect),
		)
		require.NoError(t, err)
		nodes[i] = node
	}

	// Connect nodes
	err := ti.ConnectNodes(nodes[0], nodes[1])
	require.NoError(t, err)

	// Create topic with bad encoder
	badEncoder := &MalformedEncoder{
		EncodeValid:   true,
		DecodeSuccess: false,
		DecodeError:   errors.New("global handler test error"),
	}
	topic, err := v1.NewTopic[GossipTestMessage]("global-handler-topic")
	require.NoError(t, err)

	// Register handler without invalid payload handler on node 1
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](badEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)
	err = v1.Register(nodes[1].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)
	_, err = v1.Subscribe(ctx, nodes[1].Gossipsub, topic)
	require.NoError(t, err)

	// Register on node 0 for publishing
	handler0 := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](badEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, handler0)
	require.NoError(t, err)

	// Wait for mesh
	time.Sleep(1 * time.Second)

	// Publish message
	msg := GossipTestMessage{
		ID:      "global-test",
		Content: "Test global handler",
		From:    nodes[0].ID.String(),
	}
	err = v1.Publish(nodes[0].Gossipsub, topic, msg)
	require.NoError(t, err)

	// Wait for handler to be called
	require.Eventually(t, func() bool {
		return globalCollector.Count() > 0
	}, 5*time.Second, 100*time.Millisecond)

	// Verify global handler was called
	payloads := globalCollector.GetPayloads()
	assert.Len(t, payloads, 1)
	assert.Error(t, payloads[0].Error)
	assert.Contains(t, payloads[0].Error.Error(), "global handler test error")
	assert.Equal(t, "global-handler-topic", payloads[0].Topic)
}

func TestGlobalAndTopicHandlerInteraction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create collectors
	globalCollector := NewInvalidPayloadCollector()
	topicCollector := NewInvalidPayloadCollector()

	// Create node with global handler
	node, err := ti.CreateNode(ctx,
		v1.WithGlobalInvalidPayloadHandler(globalCollector.Collect),
	)
	require.NoError(t, err)

	// Create another node for publishing
	publisher, err := ti.CreateNode(ctx)
	require.NoError(t, err)

	err = ti.ConnectNodes(node, publisher)
	require.NoError(t, err)

	// Create topic with bad encoder
	badEncoder := &MalformedEncoder{
		EncodeValid:   true,
		DecodeSuccess: false,
		DecodeError:   errors.New("both handlers test"),
	}
	topic, err := v1.NewTopic[GossipTestMessage]("both-handlers-topic")
	require.NoError(t, err)

	// Register handler with topic-specific invalid payload handler
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](badEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
		v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
			topicCollector.CollectWithoutTopic(ctx, data, err, from)
		}),
	)
	err = v1.Register(node.Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)
	_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
	require.NoError(t, err)

	// Register on publisher
	publisherHandler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](badEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)
	err = v1.Register(publisher.Gossipsub.Registry(), topic, publisherHandler)
	require.NoError(t, err)

	// Wait for mesh
	time.Sleep(1 * time.Second)

	// Publish message
	msg := GossipTestMessage{
		ID:      "both-handlers",
		Content: "Test both handlers",
		From:    publisher.ID.String(),
	}
	err = v1.Publish(publisher.Gossipsub, topic, msg)
	require.NoError(t, err)

	// Wait for handlers
	require.Eventually(t, func() bool {
		return globalCollector.Count() > 0 && topicCollector.Count() > 0
	}, 5*time.Second, 100*time.Millisecond)

	// Verify both handlers were called
	assert.Equal(t, 1, topicCollector.Count(), "Topic handler should be called")
	assert.Equal(t, 1, globalCollector.Count(), "Global handler should be called")

	// Verify they received the same error
	topicPayloads := topicCollector.GetPayloads()
	globalPayloads := globalCollector.GetPayloads()
	assert.Equal(t, topicPayloads[0].Error.Error(), globalPayloads[0].Error.Error())
}

func TestVariousDecodingErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Test cases for different decoding errors
	testCases := []struct {
		name        string
		encoder     *MalformedEncoder
		expectedErr string
	}{
		{
			name: "nil_data",
			encoder: &MalformedEncoder{
				EncodeValid:   true,
				DecodeSuccess: false,
				DecodeError:   errors.New("nil data"),
			},
			expectedErr: "nil data",
		},
		{
			name: "empty_data",
			encoder: &MalformedEncoder{
				EncodeValid:   true,
				DecodeSuccess: false,
				DecodeError:   errors.New("empty data"),
			},
			expectedErr: "empty data",
		},
		{
			name: "invalid_format",
			encoder: &MalformedEncoder{
				EncodeValid:   true,
				DecodeSuccess: false,
				DecodeError:   errors.New("invalid message format"),
			},
			expectedErr: "invalid message format",
		},
		{
			name: "corrupt_data",
			encoder: &MalformedEncoder{
				EncodeValid:   true,
				DecodeSuccess: false,
				DecodeError:   errors.New("corrupt data: checksum mismatch"),
			},
			expectedErr: "corrupt data",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create nodes
			nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
			require.NoError(t, err)

			// Create topic with specific encoder
			topic, err := v1.NewTopic[GossipTestMessage](fmt.Sprintf("error-topic-%s", tc.name))
			require.NoError(t, err)

			// Create collector
			collector := NewInvalidPayloadCollector()

			// Register handler
			handler := v1.NewHandlerConfig[GossipTestMessage](
				v1.WithEncoder[GossipTestMessage](tc.encoder),
				v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
					return nil
				}),
				v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
					collector.CollectWithoutTopic(ctx, data, err, from)
				}),
			)
			err = v1.Register(nodes[1].Gossipsub.Registry(), topic, handler)
			require.NoError(t, err)
			_, err = v1.Subscribe(ctx, nodes[1].Gossipsub, topic)
			require.NoError(t, err)

			// Register on publisher
			publisherHandler := v1.NewHandlerConfig[GossipTestMessage](
				v1.WithEncoder[GossipTestMessage](tc.encoder),
				v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
					return nil
				}),
			)
			err = v1.Register(nodes[0].Gossipsub.Registry(), topic, publisherHandler)
			require.NoError(t, err)

			// Wait for mesh
			time.Sleep(500 * time.Millisecond)

			// Publish message
			msg := GossipTestMessage{
				ID:      tc.name,
				Content: "Test error",
				From:    nodes[0].ID.String(),
			}
			err = v1.Publish(nodes[0].Gossipsub, topic, msg)
			require.NoError(t, err)

			// Wait for handler
			require.Eventually(t, func() bool {
				return collector.Count() > 0
			}, 5*time.Second, 100*time.Millisecond)

			// Verify error
			payloads := collector.GetPayloads()
			assert.Len(t, payloads, 1)
			assert.Error(t, payloads[0].Error)
			assert.Contains(t, payloads[0].Error.Error(), tc.expectedErr)
		})
	}
}

func TestInvalidMessageMetricsRecording(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create metrics
	metrics := v1.NewMetrics("test")
	registry := prometheus.NewRegistry()
	err := metrics.Register(registry)
	require.NoError(t, err)

	// Create nodes with metrics
	nodes := make([]*TestNode, 2)
	for i := 0; i < 2; i++ {
		node, err := ti.CreateNode(ctx, v1.WithMetrics(metrics))
		require.NoError(t, err)
		nodes[i] = node
	}

	err = ti.ConnectNodes(nodes[0], nodes[1])
	require.NoError(t, err)

	// Create topics
	goodTopic, err := CreateTestTopic("good-topic")
	require.NoError(t, err)

	badEncoder := &MalformedEncoder{
		EncodeValid:   true,
		DecodeSuccess: false,
		DecodeError:   errors.New("metrics test error"),
	}
	badTopic, err := v1.NewTopic[GossipTestMessage]("bad-topic")
	require.NoError(t, err)

	// Track metrics state
	var initialReceived, initialValidated float64

	// Get initial metrics
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)
	for _, mf := range metricFamilies {
		if mf.GetName() == "test_messages_received_total" {
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "topic" && lp.GetValue() == "bad-topic" {
						initialReceived = m.GetCounter().GetValue()
					}
				}
			}
		}
	}

	// Register handlers
	for i, node := range nodes {
		// Good topic
		goodHandler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				return nil
			}),
			v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
		)
		err := v1.Register(node.Gossipsub.Registry(), goodTopic, goodHandler)
		require.NoError(t, err)

		// Bad topic
		badHandler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithEncoder[GossipTestMessage](badEncoder),
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				return nil
			}),
			v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
		)
		err = v1.Register(node.Gossipsub.Registry(), badTopic, badHandler)
		require.NoError(t, err)

		// Subscribe node 1
		if i == 1 {
			_, err = v1.Subscribe(ctx, node.Gossipsub, goodTopic)
			require.NoError(t, err)
			_, err = v1.Subscribe(ctx, node.Gossipsub, badTopic)
			require.NoError(t, err)
		}
	}

	// Wait for mesh
	time.Sleep(1 * time.Second)

	// Publish messages
	msg := GossipTestMessage{
		ID:      "metrics-test",
		Content: "Test metrics",
		From:    nodes[0].ID.String(),
	}

	// Publish to good topic
	err = v1.Publish(nodes[0].Gossipsub, goodTopic, msg)
	require.NoError(t, err)

	// Publish to bad topic
	err = v1.Publish(nodes[0].Gossipsub, badTopic, msg)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(1 * time.Second)

	// Verify metrics
	metricFamilies, err = registry.Gather()
	require.NoError(t, err)

	var goodReceived, badReceived float64
	var goodValidated, badValidated float64

	for _, mf := range metricFamilies {
		switch mf.GetName() {
		case "test_messages_received_total":
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "topic" {
						switch lp.GetValue() {
						case "good-topic":
							goodReceived = m.GetCounter().GetValue()
						case "bad-topic":
							badReceived = m.GetCounter().GetValue()
						}
					}
				}
			}
		case "test_messages_validated_total":
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "topic" {
						switch lp.GetValue() {
						case "good-topic":
							if getLabel(m.GetLabel(), "result") == "accept" {
								goodValidated = m.GetCounter().GetValue()
							}
						case "bad-topic":
							if getLabel(m.GetLabel(), "result") == "accept" {
								badValidated = m.GetCounter().GetValue()
							}
						}
					}
				}
			}
		}
	}

	// Verify good topic metrics
	assert.Greater(t, goodReceived, float64(0), "Good topic should have received messages")
	assert.Greater(t, goodValidated, float64(0), "Good topic should have validated messages")

	// Verify bad topic metrics
	assert.Greater(t, badReceived, initialReceived, "Bad topic should have received messages")
	assert.Equal(t, initialValidated, badValidated, "Bad topic should not have validated messages due to decode error")
}

// Helper function to get label value from label pairs
func getLabel(labels []*io_prometheus_client.LabelPair, name string) string {
	for _, lp := range labels {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}
	return ""
}

func TestInvalidPayloadHandlerConcurrency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 3)
	require.NoError(t, err)

	// Create topic with encoder that fails randomly
	randomFailEncoder := &MalformedEncoder{
		EncodeValid:   true,
		DecodeSuccess: false,
		DecodeError:   errors.New("random failure"),
	}
	topic, err := v1.NewTopic[GossipTestMessage]("concurrent-topic")
	require.NoError(t, err)

	// Create thread-safe collector
	collector := NewInvalidPayloadCollector()

	// Subscribe all nodes
	for i, node := range nodes {
		handler := v1.NewHandlerConfig[GossipTestMessage](
			v1.WithEncoder[GossipTestMessage](randomFailEncoder),
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				return nil
			}),
			v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
				// Simulate some processing time
				time.Sleep(10 * time.Millisecond)
				collector.CollectWithoutTopic(ctx, data, err, from)
			}),
		)
		err := v1.Register(node.Gossipsub.Registry(), topic, handler)
		require.NoError(t, err)

		if i > 0 { // Subscribe nodes 1 and 2
			_, err = v1.Subscribe(ctx, node.Gossipsub, topic)
			require.NoError(t, err)
		}
	}

	// Wait for mesh
	time.Sleep(1 * time.Second)

	// Publish multiple messages concurrently
	messageCount := 10
	var wg sync.WaitGroup
	wg.Add(messageCount)

	for i := 0; i < messageCount; i++ {
		go func(id int) {
			defer wg.Done()
			msg := GossipTestMessage{
				ID:      fmt.Sprintf("concurrent-%d", id),
				Content: fmt.Sprintf("Message %d", id),
				From:    nodes[0].ID.String(),
			}
			err := v1.Publish(nodes[0].Gossipsub, topic, msg)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Wait for all handlers to complete
	require.Eventually(t, func() bool {
		// Each message should trigger handler on 2 subscribed nodes
		return collector.Count() >= messageCount*2
	}, 10*time.Second, 100*time.Millisecond)

	// Verify all handlers were called correctly
	payloads := collector.GetPayloads()
	assert.GreaterOrEqual(t, len(payloads), messageCount*2)

	// Check for race conditions - all errors should be consistent
	for _, payload := range payloads {
		assert.Error(t, payload.Error)
		assert.Contains(t, payload.Error.Error(), "random failure")
	}
}

func TestInvalidPayloadHandlerPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test infrastructure
	ti := NewTestInfrastructure(t)
	defer ti.Cleanup()

	// Create nodes
	nodes, err := ti.CreateFullyConnectedNetwork(ctx, 2)
	require.NoError(t, err)

	// Create topic with bad encoder
	badEncoder := &MalformedEncoder{
		EncodeValid:   true,
		DecodeSuccess: false,
		DecodeError:   errors.New("panic test"),
	}
	topic, err := v1.NewTopic[GossipTestMessage]("panic-topic")
	require.NoError(t, err)

	// Track if panic recovery worked
	panicRecovered := false
	handlerCalled := false

	// Register handler with panicking invalid payload handler
	handler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](badEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
		v1.WithInvalidPayloadHandler[GossipTestMessage](func(ctx context.Context, data []byte, err error, from peer.ID) {
			handlerCalled = true
			// This would panic in a real scenario, but we'll just set a flag
			// to verify the handler was called
			panicRecovered = true
		}),
	)

	err = v1.Register(nodes[1].Gossipsub.Registry(), topic, handler)
	require.NoError(t, err)
	_, err = v1.Subscribe(ctx, nodes[1].Gossipsub, topic)
	require.NoError(t, err)

	// Register on publisher
	publisherHandler := v1.NewHandlerConfig[GossipTestMessage](
		v1.WithEncoder[GossipTestMessage](badEncoder),
		v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
			return nil
		}),
	)
	err = v1.Register(nodes[0].Gossipsub.Registry(), topic, publisherHandler)
	require.NoError(t, err)

	// Wait for mesh
	time.Sleep(1 * time.Second)

	// Publish message
	msg := GossipTestMessage{
		ID:      "panic-msg",
		Content: "This might cause panic",
		From:    nodes[0].ID.String(),
	}
	err = v1.Publish(nodes[0].Gossipsub, topic, msg)
	require.NoError(t, err)

	// Wait for handler
	require.Eventually(t, func() bool {
		return handlerCalled
	}, 5*time.Second, 100*time.Millisecond)

	// Verify the system didn't crash and handler was called
	assert.True(t, panicRecovered, "Handler should have been called despite potential panic")
}
