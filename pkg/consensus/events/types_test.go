package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraceEventSerialization(t *testing.T) {
	// Create a sample peer ID.
	peerID, err := peer.Decode("12D3KooWGRpJDPwqhEfbeGxKvPRKi6tZbB5NdLVw9zanhKBM5Gf6")
	require.NoError(t, err)

	// Create a sample multiaddr.
	maddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/9000")
	require.NoError(t, err)

	tests := []struct {
		name  string
		event *TraceEvent
	}{
		{
			name: "connection event",
			event: &TraceEvent{
				Type:      EventConnected,
				PeerID:    peerID,
				Timestamp: time.Now(),
				Payload: &ConnectedData{
					RemotePeer:   peerID,
					RemoteMaddrs: maddr,
					AgentVersion: "test-agent/1.0.0",
					Direction:    network.DirInbound,
					Opened:       time.Now(),
					Transient:    false,
				},
			},
		},
		{
			name: "deliver message event",
			event: &TraceEvent{
				Type:      EventDeliverMessage,
				Topic:     "/eth2/test/beacon_block/ssz_snappy",
				PeerID:    peerID,
				Timestamp: time.Now(),
				Payload: &DeliverMessageData{
					PeerID:      peerID,
					Topic:       "/eth2/test/beacon_block/ssz_snappy",
					MessageID:   "msg-12345",
					MessageSize: 1024,
					LocalPeer:   peerID,
				},
			},
		},
		{
			name: "graft event",
			event: &TraceEvent{
				Type:      EventGraft,
				Topic:     "/eth2/test/beacon_attestation_0/ssz_snappy",
				PeerID:    peerID,
				Timestamp: time.Now(),
				Payload: &GraftData{
					PeerID: peerID,
					Topic:  "/eth2/test/beacon_attestation_0/ssz_snappy",
				},
			},
		},
		{
			name: "peer score event",
			event: &TraceEvent{
				Type:      EventPeerScore,
				PeerID:    peerID,
				Timestamp: time.Now(),
				Payload: &PeerScoreEvent{
					PeerID:             peerID.String(),
					Score:              100.5,
					AppSpecificScore:   50.25,
					IPColocationFactor: 0.9,
					BehaviourPenalty:   -10.0,
					Topics: []TopicScore{
						{
							Topic:                    "/eth2/test/beacon_block/ssz_snappy",
							TimeInMesh:               5 * time.Minute,
							FirstMessageDeliveries:   10.0,
							MeshMessageDeliveries:    8.5,
							InvalidMessageDeliveries: 0.0,
						},
					},
				},
			},
		},
		{
			name: "RPC event",
			event: &TraceEvent{
				Type:      EventRecvRPC,
				PeerID:    peerID,
				Timestamp: time.Now(),
				Payload: &RecvRPCData{
					ReceivedFrom: peerID,
					Meta: &RpcMeta{
						PeerID: peerID,
						Subscriptions: []RpcMetaSub{
							{
								Subscribe: true,
								TopicID:   "/eth2/test/beacon_block/ssz_snappy",
							},
						},
						Control: &RpcMetaControl{
							IHave: []RpcControlIHave{
								{
									TopicID:    "/eth2/test/beacon_block/ssz_snappy",
									MessageIDs: []string{"msg-1", "msg-2"},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize to JSON.
			data, err := json.Marshal(tt.event)
			require.NoError(t, err)

			// Deserialize back.
			var decoded TraceEvent
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			// Basic validation.
			assert.Equal(t, tt.event.Type, decoded.Type)
			assert.Equal(t, tt.event.Topic, decoded.Topic)
			assert.Equal(t, tt.event.PeerID.String(), decoded.PeerID.String())
			assert.WithinDuration(t, tt.event.Timestamp, decoded.Timestamp, time.Millisecond)
		})
	}
}

func TestEventTypeConstants(t *testing.T) {
	// Test that constants are properly defined.
	assert.NotEmpty(t, EventConnected)
	assert.NotEmpty(t, EventDisconnected)
	assert.NotEmpty(t, EventDeliverMessage)
	assert.NotEmpty(t, EventValidateMessage)
	assert.NotEmpty(t, EventGraft)
	assert.NotEmpty(t, EventPrune)
	assert.NotEmpty(t, EventRecvRPC)
	assert.NotEmpty(t, EventSendRPC)
	assert.NotEmpty(t, EventSyntheticHeartbeat)

	// Ensure they have distinct values.
	constants := []string{
		EventConnected,
		EventDisconnected,
		EventDeliverMessage,
		EventValidateMessage,
		EventGraft,
		EventPrune,
		EventRecvRPC,
		EventSendRPC,
		EventSyntheticHeartbeat,
	}

	seen := make(map[string]bool)
	for _, c := range constants {
		assert.False(t, seen[c], "duplicate constant value: %s", c)
		seen[c] = true
	}
}

func TestEventTypeEnums(t *testing.T) {
	// Test EventType enum.
	assert.Equal(t, EventType(0), EventTypeUnknown)
	assert.NotEqual(t, EventTypeUnknown, EventTypeGenericEvent)
	assert.NotEqual(t, EventTypeAddRemovePeer, EventTypeGraftPrune)

	// Test EventSubType enum.
	assert.Equal(t, EventSubType(0), EventSubTypeNone)
	assert.NotEqual(t, EventSubTypeAddPeer, EventSubTypeRemovePeer)
	assert.NotEqual(t, EventSubTypeGraft, EventSubTypePrune)
}

func TestTraceEventPayloadMetaData(t *testing.T) {
	metadata := &TraceEventPayloadMetaData{
		PeerID:  "12D3KooWGRpJDPwqhEfbeGxKvPRKi6tZbB5NdLVw9zanhKBM5Gf6",
		Topic:   "/eth2/test/beacon_block/ssz_snappy",
		Seq:     []byte{1, 2, 3, 4},
		MsgID:   "msg-12345",
		MsgSize: 2048,
	}

	// Serialize to JSON.
	data, err := json.Marshal(metadata)
	require.NoError(t, err)

	// Deserialize back.
	var decoded TraceEventPayloadMetaData
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Validate.
	assert.Equal(t, metadata.PeerID, decoded.PeerID)
	assert.Equal(t, metadata.Topic, decoded.Topic)
	assert.Equal(t, metadata.Seq, decoded.Seq)
	assert.Equal(t, metadata.MsgID, decoded.MsgID)
	assert.Equal(t, metadata.MsgSize, decoded.MsgSize)
}
