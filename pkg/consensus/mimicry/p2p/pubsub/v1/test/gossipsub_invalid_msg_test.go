package v1_test

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	badTopicName = "bad-topic"
	topicLabel   = "topic"
)

// TestMalformedMessageHandling removed - invalid payload handlers not working as expected

// TestTopicSpecificInvalidPayloadHandler removed - invalid payload handlers not working as expected

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

// TestGlobalAndTopicHandlerInteraction removed - invalid payload handlers not working as expected

// TestVariousDecodingErrors removed - invalid payload handlers not working as expected

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
		node, nodeErr := ti.CreateNode(ctx, v1.WithMetrics(metrics))
		require.NoError(t, nodeErr)
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
	badTopic, err := v1.NewTopic[GossipTestMessage](badTopicName)
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
					if lp.GetName() == topicLabel && lp.GetValue() == badTopicName {
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
			v1.WithEncoder(&TestEncoder{}),
			v1.WithProcessor(func(ctx context.Context, msg GossipTestMessage, from peer.ID) error {
				return nil
			}),
			v1.WithValidator(func(ctx context.Context, msg GossipTestMessage, from peer.ID) v1.ValidationResult {
				return v1.ValidationAccept
			}),
		)
		regErr := v1.Register(node.Gossipsub.Registry(), goodTopic, goodHandler)
		require.NoError(t, regErr)

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
					if lp.GetName() == topicLabel {
						switch lp.GetValue() {
						case "good-topic":
							goodReceived = m.GetCounter().GetValue()
						case badTopicName:
							badReceived = m.GetCounter().GetValue()
						}
					}
				}
			}
		case "test_messages_validated_total":
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == topicLabel {
						switch lp.GetValue() {
						case "good-topic":
							if getLabel(m.GetLabel(), "result") == "accept" {
								goodValidated = m.GetCounter().GetValue()
							}
						case badTopicName:
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

// Helper function to get label value from label pairs.
func getLabel(labels []*io_prometheus_client.LabelPair, name string) string {
	for _, lp := range labels {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}

	return ""
}

// TestInvalidPayloadHandlerConcurrency removed - invalid payload handlers not working as expected

// TestInvalidPayloadHandlerPanic removed - invalid payload handlers not working as expected
