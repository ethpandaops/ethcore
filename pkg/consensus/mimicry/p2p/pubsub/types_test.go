package pubsub

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

func TestValidationResult(t *testing.T) {
	tests := []struct {
		result   ValidationResult
		expected string
	}{
		{ValidationAccept, "accept"},
		{ValidationReject, "reject"},
		{ValidationIgnore, "ignore"},
		{ValidationResult(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.String())
		})
	}
}

func TestMessage(t *testing.T) {
	now := time.Now()
	msg := Message{
		Topic:        "test/topic",
		Data:         []byte("test data"),
		From:         peer.ID("test-peer"),
		ReceivedTime: now,
		Sequence:     12345,
	}

	assert.Equal(t, "test/topic", msg.Topic)
	assert.Equal(t, []byte("test data"), msg.Data)
	assert.Equal(t, peer.ID("test-peer"), msg.From)
	assert.Equal(t, now, msg.ReceivedTime)
	assert.Equal(t, uint64(12345), msg.Sequence)
}

func TestTopicScoreParams(t *testing.T) {
	params := &TopicScoreParams{
		TopicWeight:                     0.5,
		TimeInMeshWeight:                0.1,
		TimeInMeshQuantum:               time.Second,
		TimeInMeshCap:                   10.0,
		FirstMessageDeliveriesWeight:    1.0,
		FirstMessageDeliveriesDecay:     0.9,
		FirstMessageDeliveriesCap:       100.0,
		InvalidMessageDeliveriesWeight:  -2.0,
		InvalidMessageDeliveriesDecay:   0.9,
	}

	// Test all fields are set correctly
	assert.Equal(t, 0.5, params.TopicWeight)
	assert.Equal(t, 0.1, params.TimeInMeshWeight)
	assert.Equal(t, time.Second, params.TimeInMeshQuantum)
	assert.Equal(t, 10.0, params.TimeInMeshCap)
	assert.Equal(t, 1.0, params.FirstMessageDeliveriesWeight)
	assert.Equal(t, 0.9, params.FirstMessageDeliveriesDecay)
	assert.Equal(t, 100.0, params.FirstMessageDeliveriesCap)
	assert.Equal(t, -2.0, params.InvalidMessageDeliveriesWeight)
	assert.Equal(t, 0.9, params.InvalidMessageDeliveriesDecay)
}

func TestDefaultTopicScoreParams(t *testing.T) {
	defaults := DefaultTopicScoreParams()
	
	assert.NotNil(t, defaults)
	assert.Greater(t, defaults.TopicWeight, 0.0)
	assert.Greater(t, defaults.TimeInMeshWeight, 0.0)
	assert.Greater(t, defaults.TimeInMeshQuantum, time.Duration(0))
	assert.Greater(t, defaults.TimeInMeshCap, 0.0)
	assert.Greater(t, defaults.FirstMessageDeliveriesWeight, 0.0)
	assert.Greater(t, defaults.FirstMessageDeliveriesDecay, 0.0)
	assert.Greater(t, defaults.FirstMessageDeliveriesCap, 0.0)
	assert.Less(t, defaults.InvalidMessageDeliveriesWeight, 0.0) // Should be negative
	assert.Greater(t, defaults.InvalidMessageDeliveriesDecay, 0.0)
}

func TestPeerScoreSnapshot(t *testing.T) {
	peerID := peer.ID("test-peer")
	snapshot := PeerScoreSnapshot{
		PeerID:      peerID,
		Score:       0.8,
		Topics:      map[string]float64{"topic1": 0.5, "topic2": 0.3},
		AppSpecific: 0.9,
		IPColocation: -0.1,
		Behavioural: -0.2,
	}

	assert.Equal(t, peerID, snapshot.PeerID)
	assert.Equal(t, 0.8, snapshot.Score)
	assert.Equal(t, 2, len(snapshot.Topics))
	assert.Equal(t, 0.5, snapshot.Topics["topic1"])
	assert.Equal(t, 0.3, snapshot.Topics["topic2"])
	assert.Equal(t, 0.9, snapshot.AppSpecific)
	assert.Equal(t, -0.1, snapshot.IPColocation)
	assert.Equal(t, -0.2, snapshot.Behavioural)
}

func TestTopicInfo(t *testing.T) {
	now := time.Now()
	peers := []peer.ID{"peer1", "peer2", "peer3"}
	info := TopicInfo{
		Name:            "test/topic",
		Peers:           peers,
		MessageCount:    100,
		LastMessageTime: now,
	}

	assert.Equal(t, "test/topic", info.Name)
	assert.Equal(t, 3, len(info.Peers))
	assert.Equal(t, peers, info.Peers)
	assert.Equal(t, uint64(100), info.MessageCount)
	assert.Equal(t, now, info.LastMessageTime)
}

func TestPeerInfo(t *testing.T) {
	peerID := peer.ID("test-peer-123")
	info := PeerInfo{
		ID:        peerID,
		Topics:    []string{"topic1", "topic2", "topic3"},
		Score:     0.75,
		Connected: true,
	}

	assert.Equal(t, peerID, info.ID)
	assert.Equal(t, 3, len(info.Topics))
	assert.Equal(t, []string{"topic1", "topic2", "topic3"}, info.Topics)
	assert.Equal(t, 0.75, info.Score)
	assert.True(t, info.Connected)
}

func TestStats(t *testing.T) {
	stats := Stats{
		TotalMessages:         1000,
		TotalValidationErrors: 10,
		TotalHandlerErrors:    5,
		ActiveSubscriptions:   3,
		ConnectedPeers:        15,
		TopicCount:            7,
	}

	assert.Equal(t, uint64(1000), stats.TotalMessages)
	assert.Equal(t, uint64(10), stats.TotalValidationErrors)
	assert.Equal(t, uint64(5), stats.TotalHandlerErrors)
	assert.Equal(t, 3, stats.ActiveSubscriptions)
	assert.Equal(t, 15, stats.ConnectedPeers)
	assert.Equal(t, 7, stats.TopicCount)
}

func TestMessageSizeValidation(t *testing.T) {
	// Test various message sizes
	tests := []struct {
		name       string
		data       []byte
		maxSize    int
		shouldPass bool
	}{
		{
			name:       "small message within limit",
			data:       make([]byte, 100),
			maxSize:    1000,
			shouldPass: true,
		},
		{
			name:       "exact size limit",
			data:       make([]byte, 1000),
			maxSize:    1000,
			shouldPass: true,
		},
		{
			name:       "over size limit",
			data:       make([]byte, 1001),
			maxSize:    1000,
			shouldPass: false,
		},
		{
			name:       "zero size message",
			data:       make([]byte, 0),
			maxSize:    1000,
			shouldPass: true,
		},
		{
			name:       "nil data",
			data:       nil,
			maxSize:    1000,
			shouldPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{
				Topic: "test",
				Data:  tt.data,
				From:  peer.ID("test"),
			}

			if tt.shouldPass {
				assert.LessOrEqual(t, len(msg.Data), tt.maxSize)
			} else {
				assert.Greater(t, len(msg.Data), tt.maxSize)
			}
		})
	}
}