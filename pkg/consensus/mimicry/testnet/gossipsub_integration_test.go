// Package testnet provides comprehensive integration tests for the gossipsub implementation.
//
// This package contains integration tests that verify the end-to-end functionality of both
// the generic pubsub.Gossipsub implementation and the Ethereum-specific eth.Gossipsub wrapper.
//
// The tests create real networks of libp2p nodes with actual gossipsub instances to test:
//
// TestGossipsubIntegration includes the following test scenarios:
//   - NetworkTopology: Verifies proper peer connection setup across multiple nodes
//   - BasicSubscriptionAndPublishing: Tests basic pubsub functionality with generic topics
//   - BeaconBlockPropagation: Tests Ethereum beacon block propagation and decoding
//   - AttestationPropagation: Tests attestation propagation across subnet topics
//   - MultipleTopicHandling: Tests concurrent handling of multiple Ethereum message types
//   - MessageValidation: Tests custom validation rules and message rejection
//   - PeerScoring: Tests peer scoring configuration and score retrieval
//   - ConcurrentOperations: Tests thread safety under concurrent publishing and subscribing
//
// TestGossipsubPerformance includes performance and scalability tests:
//   - HighVolumeMessagePropagation: Tests handling of high message volumes
//   - LatencyMeasurement: Measures end-to-end message propagation latency
//   - ThroughputMeasurement: Measures sustained throughput over time
//   - ConcurrentSubscriptions: Tests performance with many concurrent subscriptions
//   - LargeMessageHandling: Tests handling of large message payloads
//
// The tests use real libp2p hosts, actual gossipsub protocol implementation, and proper
// Ethereum message encoding/decoding to ensure production-ready reliability.
//
// Test Configuration:
//   - Network size: 5 nodes (configurable via testNetworkSize)
//   - Test timeout: 30 seconds per test (configurable via testTimeout)
//   - Performance tests: 1000 messages with 60-second timeout
//   - Message validation: Custom validators that reject specific message patterns
//   - Peer scoring: Default scoring parameters with topic-specific weights
//
// Usage:
//   - Run with -short flag to skip integration tests during regular development
//   - Run without -short flag for full integration testing
//   - Individual tests can be run using -run flag with specific test names
//
// The tests are designed to be:
//   - Production-ready: Use real implementations, not mocks
//   - Comprehensive: Cover both generic pubsub and Ethereum-specific functionality
//   - Performance-oriented: Include latency, throughput, and scalability measurements
//   - Robust: Handle network delays, message loss, and cleanup properly
//   - Thread-safe: Test concurrent operations across multiple goroutines
package testnet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Test network configuration
	testNetworkSize       = 5
	testMessageCount      = 100
	testConcurrentWorkers = 10
	testTimeout           = 30 * time.Second
	testMessageTimeout    = 5 * time.Second
	networkSetupTimeout   = 10 * time.Second

	// Performance test parameters
	performanceMessageCount = 1000
	performanceTimeout      = 60 * time.Second
	maxLatencyThreshold     = 100 * time.Millisecond
	minThroughputThreshold  = 50 // messages per second
)

var (
	// Test fork digest for Ethereum mainnet
	testForkDigest = [4]byte{0x01, 0x02, 0x03, 0x04}
)

// TestNetwork represents a test gossipsub network
type TestNetwork struct {
	hosts       []host.Host
	gossipsubs  []*pubsub.Gossipsub
	ethWrappers []*eth.Gossipsub
	encoder     encoder.SszNetworkEncoder
	logger      logrus.FieldLogger
	cancel      context.CancelFunc
}

// createTestLogger creates a logger for testing with appropriate level
func createTestLogger() logrus.FieldLogger {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel) // Set to info to see test progress
	return logger.WithField("component", "gossipsub_integration_test")
}

// createTestHost creates a libp2p host for testing
func createTestHost(t *testing.T) host.Host {
	t.Helper()
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		libp2p.DisableRelay(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { h.Close() })
	return h
}

// createTestEncoder creates an SSZ encoder for testing
func createTestEncoder() encoder.SszNetworkEncoder {
	return encoder.SszNetworkEncoder{}
}

// setupTestNetwork creates a test network with the specified number of nodes
func setupTestNetwork(t *testing.T, nodeCount int) *TestNetwork {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	logger := createTestLogger()
	enc := createTestEncoder()

	// Create hosts
	hosts := make([]host.Host, nodeCount)
	for i := 0; i < nodeCount; i++ {
		hosts[i] = createTestHost(t)
	}

	// Connect all hosts in a mesh topology
	for i := 0; i < nodeCount; i++ {
		for j := i + 1; j < nodeCount; j++ {
			err := hosts[i].Connect(ctx, peer.AddrInfo{
				ID:    hosts[j].ID(),
				Addrs: hosts[j].Addrs(),
			})
			require.NoError(t, err)
		}
	}

	// Create gossipsub instances
	gossipsubs := make([]*pubsub.Gossipsub, nodeCount)
	ethWrappers := make([]*eth.Gossipsub, nodeCount)

	for i := 0; i < nodeCount; i++ {
		config := pubsub.DefaultConfig()
		config.EnablePeerScoring = true
		config.ValidationConcurrency = 2
		config.ValidationBufferSize = 100

		gs, err := pubsub.NewGossipsub(logger, hosts[i], config)
		require.NoError(t, err)

		// Use a separate context for gossipsub that won't be cancelled during test cleanup
		gsCtx := context.Background()
		err = gs.Start(gsCtx)
		require.NoError(t, err)

		gossipsubs[i] = gs
		ethWrappers[i] = eth.NewGossipsub(logger, gs, testForkDigest, enc)
	}

	// Wait for network to stabilize
	time.Sleep(2 * time.Second)

	network := &TestNetwork{
		hosts:       hosts,
		gossipsubs:  gossipsubs,
		ethWrappers: ethWrappers,
		encoder:     enc,
		logger:      logger,
		cancel:      cancel,
	}

	t.Cleanup(func() {
		network.Cleanup()
	})

	return network
}

// Cleanup cleans up the test network
func (tn *TestNetwork) Cleanup() {
	tn.cancel()

	// Stop all gossipsub instances
	for _, gs := range tn.gossipsubs {
		if gs != nil {
			_ = gs.Stop()
		}
	}

	// Close all hosts
	for _, h := range tn.hosts {
		if h != nil {
			h.Close()
		}
	}
}

// createTestBeaconBlock creates a test beacon block for publishing
func createTestBeaconBlock(slot uint64) *pb.SignedBeaconBlock {
	return &pb.SignedBeaconBlock{
		Block: &pb.BeaconBlock{
			Slot:          primitives.Slot(slot),
			ProposerIndex: primitives.ValidatorIndex(rand.Intn(100)),
			ParentRoot:    make([]byte, 32),
			StateRoot:     make([]byte, 32),
			Body: &pb.BeaconBlockBody{
				RandaoReveal: make([]byte, 96),
				Eth1Data: &pb.Eth1Data{
					DepositRoot:  make([]byte, 32),
					DepositCount: uint64(rand.Intn(1000)),
					BlockHash:    make([]byte, 32),
				},
				Graffiti:          make([]byte, 32),
				ProposerSlashings: []*pb.ProposerSlashing{},
				AttesterSlashings: []*pb.AttesterSlashing{},
				Attestations:      []*pb.Attestation{},
				Deposits:          []*pb.Deposit{},
				VoluntaryExits:    []*pb.SignedVoluntaryExit{},
			},
		},
		Signature: make([]byte, 96),
	}
}

// createTestAttestation creates a test attestation for publishing
func createTestAttestation(slot uint64, committeeIndex uint64) *pb.Attestation {
	return &pb.Attestation{
		AggregationBits: []byte{0xFF, 0xFF, 0xFF, 0xFF},
		Data: &pb.AttestationData{
			Slot:            primitives.Slot(slot),
			CommitteeIndex:  primitives.CommitteeIndex(committeeIndex),
			BeaconBlockRoot: make([]byte, 32),
			Source: &pb.Checkpoint{
				Epoch: primitives.Epoch(slot / 32),
				Root:  make([]byte, 32),
			},
			Target: &pb.Checkpoint{
				Epoch: primitives.Epoch(slot / 32),
				Root:  make([]byte, 32),
			},
		},
		Signature: make([]byte, 96),
	}
}

// TestGossipsubIntegration tests comprehensive gossipsub functionality across multiple nodes
func TestGossipsubIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	network := setupTestNetwork(t, testNetworkSize)

	t.Run("NetworkTopology", func(t *testing.T) {
		// Verify all nodes are connected
		for i, host := range network.hosts {
			peers := host.Network().Peers()
			assert.GreaterOrEqual(t, len(peers), testNetworkSize-2,
				"Node %d should be connected to at least %d peers", i, testNetworkSize-2)
		}
	})

	t.Run("BasicSubscriptionAndPublishing", func(t *testing.T) {
		testBasicSubscriptionAndPublishing(t, network)
	})

	t.Run("BeaconBlockPropagation", func(t *testing.T) {
		testBeaconBlockPropagation(t, network)
	})

	t.Run("AttestationPropagation", func(t *testing.T) {
		testAttestationPropagation(t, network)
	})

	t.Run("MultipleTopicHandling", func(t *testing.T) {
		testMultipleTopicHandling(t, network)
	})

	t.Run("MessageValidation", func(t *testing.T) {
		testMessageValidation(t, network)
	})

	t.Run("PeerScoring", func(t *testing.T) {
		testPeerScoring(t, network)
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		testConcurrentOperations(t, network)
	})
}

// testBasicSubscriptionAndPublishing tests basic gossipsub subscription and publishing
func testBasicSubscriptionAndPublishing(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	testTopic := "test-basic-topic"
	testMessage := []byte("test message for basic functionality")

	// Subscribe all nodes except the publisher
	var receivedMessages [][]byte
	var receivedMutex sync.Mutex

	handler := func(ctx context.Context, msg *pubsub.Message) error {
		receivedMutex.Lock()
		receivedMessages = append(receivedMessages, msg.Data)
		receivedMutex.Unlock()
		return nil
	}

	// Subscribe all nodes except the first one
	for i := 1; i < len(network.gossipsubs); i++ {
		err := network.gossipsubs[i].Subscribe(ctx, testTopic, handler)
		require.NoError(t, err)
	}

	// Give subscriptions time to propagate
	time.Sleep(1 * time.Second)

	// Publish from the first node
	err := network.gossipsubs[0].Publish(ctx, testTopic, testMessage)
	require.NoError(t, err)

	// Wait for message propagation
	time.Sleep(2 * time.Second)

	// Verify message was received by all subscribers
	receivedMutex.Lock()
	defer receivedMutex.Unlock()

	// Should receive the message on each subscribing node
	assert.GreaterOrEqual(t, len(receivedMessages), testNetworkSize-2,
		"Expected at least %d messages, got %d", testNetworkSize-2, len(receivedMessages))

	// Verify message content
	for _, msg := range receivedMessages {
		assert.Equal(t, testMessage, msg)
	}
}

// testBeaconBlockPropagation tests Ethereum beacon block propagation
func testBeaconBlockPropagation(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var receivedBlocks []*pb.SignedBeaconBlock
	var receivedMutex sync.Mutex
	var receivedFromPeers []peer.ID

	handler := func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
		receivedMutex.Lock()
		receivedBlocks = append(receivedBlocks, block)
		receivedFromPeers = append(receivedFromPeers, from)
		receivedMutex.Unlock()
		return nil
	}

	// Subscribe all nodes except the publisher to beacon blocks
	for i := 1; i < len(network.ethWrappers); i++ {
		err := network.ethWrappers[i].SubscribeBeaconBlock(ctx, handler)
		require.NoError(t, err)
	}

	// Give subscriptions time to propagate
	time.Sleep(1 * time.Second)

	// Create and publish a test beacon block
	testBlock := createTestBeaconBlock(12345)
	err := network.ethWrappers[0].PublishBeaconBlock(ctx, testBlock)
	require.NoError(t, err)

	// Wait for message propagation
	time.Sleep(3 * time.Second)

	// Verify block was received
	receivedMutex.Lock()
	defer receivedMutex.Unlock()

	assert.GreaterOrEqual(t, len(receivedBlocks), testNetworkSize-2,
		"Expected at least %d blocks, got %d", testNetworkSize-2, len(receivedBlocks))

	// Verify block content
	for _, block := range receivedBlocks {
		assert.Equal(t, testBlock.Block.Slot, block.Block.Slot)
		assert.Equal(t, testBlock.Block.ProposerIndex, block.Block.ProposerIndex)
	}

	// Verify we received from the publisher
	publisherID := network.hosts[0].ID()
	assert.Contains(t, receivedFromPeers, publisherID, "Should receive block from publisher")
}

// testAttestationPropagation tests attestation propagation on subnets
func testAttestationPropagation(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	testSubnet := uint64(5) // Use subnet 5 for testing

	var receivedAttestations []*pb.Attestation
	var receivedMutex sync.Mutex

	handler := func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error {
		receivedMutex.Lock()
		receivedAttestations = append(receivedAttestations, att)
		receivedMutex.Unlock()

		assert.Equal(t, testSubnet, subnet, "Received attestation should be from the correct subnet")
		return nil
	}

	// Subscribe all nodes except the publisher to attestations
	for i := 1; i < len(network.ethWrappers); i++ {
		err := network.ethWrappers[i].SubscribeAttestation(ctx, testSubnet, handler)
		require.NoError(t, err)
	}

	// Give subscriptions time to propagate
	time.Sleep(1 * time.Second)

	// Create and publish test attestations
	for slot := uint64(1000); slot < 1005; slot++ {
		testAttestation := createTestAttestation(slot, 1)
		err := network.ethWrappers[0].PublishAttestation(ctx, testAttestation, testSubnet)
		require.NoError(t, err)

		// Small delay between publications
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for message propagation
	time.Sleep(3 * time.Second)

	// Verify attestations were received
	receivedMutex.Lock()
	defer receivedMutex.Unlock()

	// Should receive multiple attestations across all subscribing nodes
	expectedMin := int((testNetworkSize - 1) * 5 * 0.8) // Allow for 20% loss in test network
	assert.GreaterOrEqual(t, len(receivedAttestations), expectedMin,
		"Expected at least %d attestations, got %d", expectedMin, len(receivedAttestations))
}

// testMultipleTopicHandling tests handling multiple topics simultaneously
func testMultipleTopicHandling(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var receivedBeaconBlocks int
	var receivedAttestations int
	var receivedVoluntaryExits int
	var mutex sync.Mutex

	// Define handlers
	beaconBlockHandler := func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
		mutex.Lock()
		receivedBeaconBlocks++
		mutex.Unlock()
		return nil
	}

	attestationHandler := func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error {
		mutex.Lock()
		receivedAttestations++
		mutex.Unlock()
		return nil
	}

	voluntaryExitHandler := func(ctx context.Context, exit *pb.SignedVoluntaryExit, from peer.ID) error {
		mutex.Lock()
		receivedVoluntaryExits++
		mutex.Unlock()
		return nil
	}

	// Subscribe all nodes except the first to all topics
	for i := 1; i < len(network.ethWrappers); i++ {
		err := network.ethWrappers[i].SubscribeBeaconBlock(ctx, beaconBlockHandler)
		require.NoError(t, err)

		err = network.ethWrappers[i].SubscribeAttestation(ctx, 0, attestationHandler)
		require.NoError(t, err)

		err = network.ethWrappers[i].SubscribeVoluntaryExit(ctx, voluntaryExitHandler)
		require.NoError(t, err)
	}

	// Give subscriptions time to propagate
	time.Sleep(1 * time.Second)

	// Publish different message types concurrently
	var wg sync.WaitGroup

	// Publish beacon blocks
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			block := createTestBeaconBlock(uint64(2000 + i))
			err := network.ethWrappers[0].PublishBeaconBlock(ctx, block)
			if err != nil {
				t.Logf("Error publishing beacon block: %v", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Publish attestations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			att := createTestAttestation(uint64(2000+i), 0)
			err := network.ethWrappers[0].PublishAttestation(ctx, att, 0)
			if err != nil {
				t.Logf("Error publishing attestation: %v", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Publish voluntary exits
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			exit := &pb.SignedVoluntaryExit{
				Exit: &pb.VoluntaryExit{
					Epoch:          primitives.Epoch(100 + i),
					ValidatorIndex: primitives.ValidatorIndex(i),
				},
				Signature: make([]byte, 96),
			}

			topic := eth.VoluntaryExitTopic(testForkDigest)
			var buf bytes.Buffer
			_, err := network.encoder.EncodeGossip(&buf, exit)
			if err != nil {
				t.Logf("Error encoding voluntary exit: %v", err)
				continue
			}

			err = network.gossipsubs[0].Publish(ctx, topic, buf.Bytes())
			if err != nil {
				t.Logf("Error publishing voluntary exit: %v", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Wait for message propagation
	time.Sleep(3 * time.Second)

	// Verify messages were received
	mutex.Lock()
	defer mutex.Unlock()

	expectedNodes := testNetworkSize - 1
	assert.GreaterOrEqual(t, receivedBeaconBlocks, expectedNodes*3,
		"Expected at least %d beacon blocks, got %d", expectedNodes*3, receivedBeaconBlocks)
	assert.GreaterOrEqual(t, receivedAttestations, expectedNodes*3,
		"Expected at least %d attestations, got %d", expectedNodes*3, receivedAttestations)
	assert.GreaterOrEqual(t, receivedVoluntaryExits, expectedNodes*2,
		"Expected at least %d voluntary exits, got %d", expectedNodes*2, receivedVoluntaryExits)
}

// testMessageValidation tests message validation functionality
func testMessageValidation(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var validMessages, invalidMessages int
	var mutex sync.Mutex

	// Create a validator that rejects blocks with even slot numbers
	validator := func(block *pb.SignedBeaconBlock) error {
		if block.Block.Slot%2 == 0 {
			return fmt.Errorf("rejecting block with even slot: %d", block.Block.Slot)
		}
		return nil
	}

	// Register validator on all nodes
	for i := 0; i < len(network.ethWrappers); i++ {
		err := network.ethWrappers[i].RegisterBeaconBlockValidator(validator)
		require.NoError(t, err)
	}

	// Handler to count received messages
	handler := func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
		mutex.Lock()
		if block.Block.Slot%2 == 0 {
			invalidMessages++
		} else {
			validMessages++
		}
		mutex.Unlock()
		return nil
	}

	// Subscribe all nodes except the publisher
	for i := 1; i < len(network.ethWrappers); i++ {
		err := network.ethWrappers[i].SubscribeBeaconBlock(ctx, handler)
		require.NoError(t, err)
	}

	// Give subscriptions time to propagate
	time.Sleep(1 * time.Second)

	// Publish valid and invalid blocks
	for slot := uint64(3000); slot < 3010; slot++ {
		block := createTestBeaconBlock(slot)
		err := network.ethWrappers[0].PublishBeaconBlock(ctx, block)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for message propagation and validation
	time.Sleep(5 * time.Second)

	// Verify validation results
	mutex.Lock()
	defer mutex.Unlock()

	// Should receive only odd slot blocks (valid ones)
	assert.Greater(t, validMessages, 0, "Should receive some valid messages")
	assert.Equal(t, 0, invalidMessages, "Should not receive any invalid messages")

	// Should receive approximately (testNetworkSize-1) * 5 valid messages
	expectedValid := (testNetworkSize - 1) * 5
	expectedValidMin := int(float64(expectedValid) * 0.8)
	assert.GreaterOrEqual(t, validMessages, expectedValidMin,
		"Expected at least %d valid messages, got %d", expectedValidMin, validMessages)
}

// testPeerScoring tests peer scoring functionality
func testPeerScoring(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Set up peer scoring parameters
	scoreParams := pubsub.DefaultTopicScoreParams()
	scoreParams.TopicWeight = 1.0
	scoreParams.TimeInMeshWeight = 1.0
	scoreParams.FirstMessageDeliveriesWeight = 1.0

	topic := eth.BeaconBlockTopic(testForkDigest)

	// Configure scoring on all nodes
	for i := 0; i < len(network.gossipsubs); i++ {
		err := network.gossipsubs[i].SetTopicScoreParams(topic, scoreParams)
		require.NoError(t, err)
	}

	// Subscribe all nodes to create mesh
	handler := func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
		return nil
	}

	for i := 0; i < len(network.ethWrappers); i++ {
		err := network.ethWrappers[i].SubscribeBeaconBlock(ctx, handler)
		require.NoError(t, err)
	}

	// Give time for mesh formation
	time.Sleep(3 * time.Second)

	// Publish some messages to establish scoring
	for i := 0; i < 10; i++ {
		block := createTestBeaconBlock(uint64(4000 + i))
		err := network.ethWrappers[i%len(network.ethWrappers)].PublishBeaconBlock(ctx, block)
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)
	}

	// Wait for scoring to update
	time.Sleep(5 * time.Second)

	// Check peer scores
	for i := 0; i < len(network.gossipsubs); i++ {
		scores, err := network.gossipsubs[i].GetAllPeerScores()
		require.NoError(t, err)

		network.logger.WithFields(logrus.Fields{
			"node":       i,
			"peer_count": len(scores),
		}).Info("Peer scores retrieved")

		// Should have scores for connected peers
		assert.GreaterOrEqual(t, len(scores), 1, "Node %d should have scores for at least 1 peer", i)

		// Verify score structure
		for _, score := range scores {
			assert.NotEmpty(t, score.PeerID, "Peer ID should not be empty")
			assert.NotNil(t, score.Topics, "Topics should not be nil")
		}
	}
}

// testConcurrentOperations tests concurrent operations across the network
func testConcurrentOperations(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var publishCount, receiveCount int64
	var mutex sync.Mutex

	handler := func(ctx context.Context, msg *pubsub.Message) error {
		mutex.Lock()
		receiveCount++
		mutex.Unlock()
		return nil
	}

	// Subscribe all nodes to a test topic
	testTopic := "concurrent-test-topic"
	for i := 0; i < len(network.gossipsubs); i++ {
		err := network.gossipsubs[i].Subscribe(ctx, testTopic, handler)
		require.NoError(t, err)
	}

	// Give subscriptions time to propagate
	time.Sleep(1 * time.Second)

	// Launch concurrent publishers
	var wg sync.WaitGroup
	messageCount := 20

	for nodeIdx := 0; nodeIdx < len(network.gossipsubs); nodeIdx++ {
		wg.Add(1)
		go func(nodeIndex int) {
			defer wg.Done()

			for msgIdx := 0; msgIdx < messageCount; msgIdx++ {
				message := []byte(fmt.Sprintf("message-%d-%d", nodeIndex, msgIdx))
				err := network.gossipsubs[nodeIndex].Publish(ctx, testTopic, message)
				if err == nil {
					mutex.Lock()
					publishCount++
					mutex.Unlock()
				}

				// Small random delay to create realistic timing
				time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
			}
		}(nodeIdx)
	}

	// Wait for all publishing to complete
	wg.Wait()

	// Wait for message propagation
	time.Sleep(5 * time.Second)

	// Verify results
	mutex.Lock()
	defer mutex.Unlock()

	expectedPublishCount := int64(len(network.gossipsubs) * messageCount)
	assert.Equal(t, expectedPublishCount, publishCount,
		"Should publish %d messages, published %d", expectedPublishCount, publishCount)

	// Should receive significantly more messages than published due to network propagation
	assert.Greater(t, receiveCount, publishCount,
		"Should receive more messages (%d) than published (%d) due to network propagation", receiveCount, publishCount)
}

// TestGossipsubPerformance tests performance characteristics of the gossipsub implementation
func TestGossipsubPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	network := setupTestNetwork(t, testNetworkSize)

	t.Run("HighVolumeMessagePropagation", func(t *testing.T) {
		testHighVolumeMessagePropagation(t, network)
	})

	t.Run("LatencyMeasurement", func(t *testing.T) {
		testLatencyMeasurement(t, network)
	})

	t.Run("ThroughputMeasurement", func(t *testing.T) {
		testThroughputMeasurement(t, network)
	})

	t.Run("ConcurrentSubscriptions", func(t *testing.T) {
		testConcurrentSubscriptions(t, network)
	})

	t.Run("LargeMessageHandling", func(t *testing.T) {
		testLargeMessageHandling(t, network)
	})
}

// testHighVolumeMessagePropagation tests handling of high message volumes
func testHighVolumeMessagePropagation(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), performanceTimeout)
	defer cancel()

	testTopic := "high-volume-test"
	var receivedCount int64
	var mutex sync.Mutex

	handler := func(ctx context.Context, msg *pubsub.Message) error {
		mutex.Lock()
		receivedCount++
		mutex.Unlock()
		return nil
	}

	// Subscribe all nodes except the publisher
	for i := 1; i < len(network.gossipsubs); i++ {
		err := network.gossipsubs[i].Subscribe(ctx, testTopic, handler)
		require.NoError(t, err)
	}

	// Give subscriptions time to propagate
	time.Sleep(2 * time.Second)

	start := time.Now()

	// Publish high volume of messages
	for i := 0; i < performanceMessageCount; i++ {
		message := []byte(fmt.Sprintf("high-volume-message-%d", i))
		err := network.gossipsubs[0].Publish(ctx, testTopic, message)
		require.NoError(t, err)

		// Very small delay to prevent overwhelming
		if i%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	publishDuration := time.Since(start)

	// Wait for message propagation
	time.Sleep(10 * time.Second)

	// Measure results
	mutex.Lock()
	finalReceivedCount := receivedCount
	mutex.Unlock()

	publishRate := float64(performanceMessageCount) / publishDuration.Seconds()
	propagationRate := float64(finalReceivedCount) / publishDuration.Seconds()

	network.logger.WithFields(logrus.Fields{
		"published":        performanceMessageCount,
		"received":         finalReceivedCount,
		"publish_rate":     fmt.Sprintf("%.2f msg/s", publishRate),
		"propagation_rate": fmt.Sprintf("%.2f msg/s", propagationRate),
		"publish_duration": publishDuration,
	}).Info("High volume test results")

	// Verify performance
	assert.Greater(t, publishRate, float64(minThroughputThreshold),
		"Publish rate %.2f should exceed threshold %d msg/s", publishRate, minThroughputThreshold)

	// Should receive at least 80% of messages across all nodes
	expectedMinReceived := int64(float64(performanceMessageCount*(testNetworkSize-1)) * 0.8)
	assert.GreaterOrEqual(t, finalReceivedCount, expectedMinReceived,
		"Should receive at least %d messages, got %d", expectedMinReceived, finalReceivedCount)
}

// testLatencyMeasurement measures message propagation latency
func testLatencyMeasurement(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	testTopic := "latency-test"
	var latencies []time.Duration
	var mutex sync.Mutex

	// Record publish time in message metadata

	handler := func(ctx context.Context, msg *pubsub.Message) error {
		if len(msg.Data) >= 16 { // Basic validation
			// Parse timestamp (simplified for test)
			publishTime := time.Unix(0, int64(binary.BigEndian.Uint64(msg.Data[:8])))
			receiveTime := time.Now()
			latency := receiveTime.Sub(publishTime)

			mutex.Lock()
			latencies = append(latencies, latency)
			mutex.Unlock()
		}
		return nil
	}

	// Subscribe all nodes except the publisher
	for i := 1; i < len(network.gossipsubs); i++ {
		err := network.gossipsubs[i].Subscribe(ctx, testTopic, handler)
		require.NoError(t, err)
	}

	// Give subscriptions time to propagate
	time.Sleep(1 * time.Second)

	// Send timestamped messages
	messageCount := 50
	for i := 0; i < messageCount; i++ {
		now := time.Now()
		message := make([]byte, 16)
		binary.BigEndian.PutUint64(message[:8], uint64(now.UnixNano()))
		binary.BigEndian.PutUint64(message[8:], uint64(i))

		err := network.gossipsubs[0].Publish(ctx, testTopic, message)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
	}

	// Wait for message propagation
	time.Sleep(5 * time.Second)

	// Analyze latencies
	mutex.Lock()
	defer mutex.Unlock()

	require.Greater(t, len(latencies), 0, "Should have measured some latencies")

	// Calculate statistics
	var total time.Duration
	var maxLatency time.Duration
	minLatency := time.Hour // Start with large value

	for _, latency := range latencies {
		total += latency
		if latency > maxLatency {
			maxLatency = latency
		}
		if latency < minLatency {
			minLatency = latency
		}
	}

	avgLatency := total / time.Duration(len(latencies))

	network.logger.WithFields(logrus.Fields{
		"samples":     len(latencies),
		"avg_latency": avgLatency,
		"min_latency": minLatency,
		"max_latency": maxLatency,
	}).Info("Latency measurement results")

	// Verify latency is reasonable
	assert.Less(t, avgLatency, maxLatencyThreshold,
		"Average latency %v should be less than %v", avgLatency, maxLatencyThreshold)
	assert.Greater(t, avgLatency, time.Millisecond,
		"Average latency %v should be greater than 1ms (sanity check)", avgLatency)
}

// testThroughputMeasurement measures sustained throughput
func testThroughputMeasurement(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), performanceTimeout)
	defer cancel()

	testTopic := "throughput-test"
	var receivedCount int64
	var mutex sync.Mutex

	handler := func(ctx context.Context, msg *pubsub.Message) error {
		mutex.Lock()
		receivedCount++
		mutex.Unlock()
		return nil
	}

	// Subscribe all nodes except the publisher
	for i := 1; i < len(network.gossipsubs); i++ {
		err := network.gossipsubs[i].Subscribe(ctx, testTopic, handler)
		require.NoError(t, err)
	}

	// Give subscriptions time to propagate
	time.Sleep(2 * time.Second)

	// Measure sustained throughput over time
	testDuration := 30 * time.Second
	start := time.Now()
	messagesSent := 0

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	endTime := start.Add(testDuration)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Now().After(endTime) {
				goto measureResults
			}

			message := []byte(fmt.Sprintf("throughput-message-%d", messagesSent))
			err := network.gossipsubs[0].Publish(ctx, testTopic, message)
			if err == nil {
				messagesSent++
			}
		}
	}

measureResults:
	actualDuration := time.Since(start)

	// Wait for final message propagation
	time.Sleep(5 * time.Second)

	mutex.Lock()
	finalReceivedCount := receivedCount
	mutex.Unlock()

	publishThroughput := float64(messagesSent) / actualDuration.Seconds()
	receiveThroughput := float64(finalReceivedCount) / actualDuration.Seconds()

	network.logger.WithFields(logrus.Fields{
		"duration":           actualDuration,
		"messages_sent":      messagesSent,
		"messages_received":  finalReceivedCount,
		"publish_throughput": fmt.Sprintf("%.2f msg/s", publishThroughput),
		"receive_throughput": fmt.Sprintf("%.2f msg/s", receiveThroughput),
	}).Info("Throughput measurement results")

	// Verify sustained throughput
	assert.Greater(t, publishThroughput, float64(minThroughputThreshold),
		"Publish throughput %.2f should exceed %d msg/s", publishThroughput, minThroughputThreshold)

	// Verify message delivery ratio
	expectedMinReceived := float64(messagesSent*(testNetworkSize-1)) * 0.8
	assert.GreaterOrEqual(t, float64(finalReceivedCount), expectedMinReceived,
		"Should receive at least %.0f messages, got %d", expectedMinReceived, finalReceivedCount)
}

// testConcurrentSubscriptions tests handling many concurrent subscriptions
func testConcurrentSubscriptions(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	topicCount := 20
	var totalReceived int64
	var mutex sync.Mutex

	handler := func(ctx context.Context, msg *pubsub.Message) error {
		mutex.Lock()
		totalReceived++
		mutex.Unlock()
		return nil
	}

	// Create subscriptions to multiple topics on all nodes
	for nodeIdx := 0; nodeIdx < len(network.gossipsubs); nodeIdx++ {
		for topicIdx := 0; topicIdx < topicCount; topicIdx++ {
			topic := fmt.Sprintf("concurrent-topic-%d", topicIdx)
			err := network.gossipsubs[nodeIdx].Subscribe(ctx, topic, handler)
			require.NoError(t, err)
		}
	}

	// Give subscriptions time to propagate
	time.Sleep(3 * time.Second)

	// Publish to all topics from different nodes
	var wg sync.WaitGroup
	for topicIdx := 0; topicIdx < topicCount; topicIdx++ {
		wg.Add(1)
		go func(tIdx int) {
			defer wg.Done()

			topic := fmt.Sprintf("concurrent-topic-%d", tIdx)
			nodeIdx := tIdx % len(network.gossipsubs)

			for msgIdx := 0; msgIdx < 5; msgIdx++ {
				message := []byte(fmt.Sprintf("message-%d-%d", tIdx, msgIdx))
				err := network.gossipsubs[nodeIdx].Publish(ctx, topic, message)
				if err != nil {
					t.Logf("Error publishing to topic %s: %v", topic, err)
				}
				time.Sleep(50 * time.Millisecond)
			}
		}(topicIdx)
	}

	wg.Wait()

	// Wait for message propagation
	time.Sleep(5 * time.Second)

	// Verify subscription handling
	mutex.Lock()
	defer mutex.Unlock()

	// Should receive messages from all topics and nodes
	expectedMin := int64(float64(topicCount*5*testNetworkSize) * 0.8) // Allow for some loss
	assert.GreaterOrEqual(t, totalReceived, expectedMin,
		"Should receive at least %d messages across all topics, got %d", expectedMin, totalReceived)

	// Verify subscription state
	for i := 0; i < len(network.gossipsubs); i++ {
		subscriptions := network.gossipsubs[i].GetSubscriptions()
		assert.Equal(t, topicCount, len(subscriptions),
			"Node %d should have %d active subscriptions", i, topicCount)
	}
}

// testLargeMessageHandling tests handling of large messages
func testLargeMessageHandling(t *testing.T, network *TestNetwork) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	testTopic := "large-message-test"
	var receivedSizes []int
	var mutex sync.Mutex

	handler := func(ctx context.Context, msg *pubsub.Message) error {
		mutex.Lock()
		receivedSizes = append(receivedSizes, len(msg.Data))
		mutex.Unlock()
		return nil
	}

	// Subscribe all nodes except the publisher
	for i := 1; i < len(network.gossipsubs); i++ {
		err := network.gossipsubs[i].Subscribe(ctx, testTopic, handler)
		require.NoError(t, err)
	}

	// Give subscriptions time to propagate
	time.Sleep(1 * time.Second)

	// Test different message sizes
	messageSizes := []int{1024, 4096, 16384, 65536, 262144} // 1KB to 256KB

	for _, size := range messageSizes {
		// Create large message
		message := make([]byte, size)
		for i := range message {
			message[i] = byte(i % 256)
		}

		err := network.gossipsubs[0].Publish(ctx, testTopic, message)
		require.NoError(t, err)

		// Wait between messages
		time.Sleep(500 * time.Millisecond)
	}

	// Wait for message propagation
	time.Sleep(5 * time.Second)

	// Verify large message handling
	mutex.Lock()
	defer mutex.Unlock()

	assert.Greater(t, len(receivedSizes), 0, "Should receive some large messages")

	// Verify we received messages of expected sizes
	sizeMap := make(map[int]int)
	for _, size := range receivedSizes {
		sizeMap[size]++
	}

	network.logger.WithField("received_sizes", sizeMap).Info("Large message test results")

	// Should receive messages of different sizes across nodes
	expectedSizesReceived := len(messageSizes) * (testNetworkSize - 1)
	expectedSizesMin := int(float64(expectedSizesReceived) * 0.8)
	assert.GreaterOrEqual(t, len(receivedSizes), expectedSizesMin,
		"Should receive at least 80%% of large messages")
}
