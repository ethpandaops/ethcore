package eth_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBeaconchainGossipsub_RegisterAndSubscribe(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create two hosts for testing
	h1, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	defer h1.Close()

	h2, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	defer h2.Close()

	// Connect hosts
	err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	require.NoError(t, err)

	// Create gossipsub instances
	gs1, err := pubsub.NewGossipsub(logger.WithField("node", "1"), h1, nil)
	require.NoError(t, err)

	gs2, err := pubsub.NewGossipsub(logger.WithField("node", "2"), h2, nil)
	require.NoError(t, err)

	// Start gossipsub services
	err = gs1.Start(ctx)
	require.NoError(t, err)
	defer gs1.Stop()

	err = gs2.Start(ctx)
	require.NoError(t, err)
	defer gs2.Stop()

	// Create BeaconchainGossipsub instances
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	sszEncoder := encoder.SszNetworkEncoder{}

	bg1 := eth.NewBeaconchainGossipsub(gs1, forkDigest, sszEncoder, logger.WithField("node", "1"))
	bg2 := eth.NewBeaconchainGossipsub(gs2, forkDigest, sszEncoder, logger.WithField("node", "2"))

	t.Run("RegisterAndSubscribeBeaconBlock", func(t *testing.T) {
		receivedBlocks := make(chan *pb.SignedBeaconBlock, 1)

		// Register handler on node 2
		err := bg2.RegisterBeaconBlock(
			nil, // no validator
			func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
				receivedBlocks <- block
				return nil
			},
			nil, // no score params
		)
		require.NoError(t, err)

		// Subscribe on node 2
		err = bg2.SubscribeBeaconBlock(ctx)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1 to ensure topic exists
		err = bg1.RegisterBeaconBlock(nil, nil, nil)
		require.NoError(t, err)
		err = bg1.SubscribeBeaconBlock(ctx)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish the block
		testBlock := &pb.SignedBeaconBlock{
			Block: &pb.BeaconBlock{
				Slot: 12345,
			},
		}

		topic := eth.BeaconBlockTopic(forkDigest)
		var buf bytes.Buffer
		_, err = sszEncoder.EncodeGossip(&buf, testBlock)
		require.NoError(t, err)
		data := buf.Bytes()

		err = gs1.Publish(ctx, topic, data)
		require.NoError(t, err)

		// Verify reception
		select {
		case received := <-receivedBlocks:
			assert.Equal(t, uint64(12345), received.Block.Slot)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for beacon block")
		}
	})

	t.Run("RegisterAndSubscribeAttestation", func(t *testing.T) {
		receivedAtts := make(chan *pb.Attestation, 1)

		// Register handler on node 2 for subnet 10
		err := bg2.RegisterAttestation(
			[]uint64{10},
			nil, // no validator
			func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error {
				assert.Equal(t, uint64(10), subnet)
				receivedAtts <- att
				return nil
			},
		)
		require.NoError(t, err)

		// Subscribe on node 2 using the generated name
		attName := fmt.Sprintf("attestation_subnets_%v", []uint64{10})
		err = bg2.SubscribeAttestation(ctx, attName)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1 to ensure topic exists
		err = bg1.RegisterAttestation([]uint64{10}, nil, nil)
		require.NoError(t, err)
		err = bg1.SubscribeAttestation(ctx, attName)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish attestation
		testAtt := &pb.Attestation{
			Data: &pb.AttestationData{
				Slot: 54321,
			},
		}

		topic := eth.AttestationSubnetTopic(forkDigest, 10)
		var buf bytes.Buffer
		_, err = sszEncoder.EncodeGossip(&buf, testAtt)
		require.NoError(t, err)
		data := buf.Bytes()

		err = gs1.Publish(ctx, topic, data)
		require.NoError(t, err)

		// Verify reception
		select {
		case received := <-receivedAtts:
			assert.Equal(t, uint64(54321), received.Data.Slot)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for attestation")
		}
	})

	t.Run("RegisterAndSubscribeAggregateAndProof", func(t *testing.T) {
		receivedAggs := make(chan *pb.AggregateAttestationAndProof, 1)

		// Register handler on node 2
		err := bg2.RegisterAggregateAndProof(
			nil, // no validator
			func(ctx context.Context, agg *pb.AggregateAttestationAndProof, from peer.ID) error {
				receivedAggs <- agg
				return nil
			},
		)
		require.NoError(t, err)

		// Subscribe on node 2
		err = bg2.SubscribeAggregateAndProof(ctx)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		err = bg1.RegisterAggregateAndProof(nil, nil)
		require.NoError(t, err)
		err = bg1.SubscribeAggregateAndProof(ctx)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish aggregate
		testAgg := &pb.AggregateAttestationAndProof{
			AggregatorIndex: 100,
			Aggregate: &pb.Attestation{
				Data: &pb.AttestationData{
					Slot: 99999,
				},
			},
		}

		topic := eth.BeaconAggregateAndProofTopic(forkDigest)
		var buf bytes.Buffer
		_, err = sszEncoder.EncodeGossip(&buf, testAgg)
		require.NoError(t, err)
		data := buf.Bytes()

		err = gs1.Publish(ctx, topic, data)
		require.NoError(t, err)

		// Verify reception
		select {
		case received := <-receivedAggs:
			assert.Equal(t, uint64(100), received.AggregatorIndex)
			assert.Equal(t, uint64(99999), received.Aggregate.Data.Slot)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for aggregate and proof")
		}
	})

	t.Run("RegisterAndSubscribeSyncCommittee", func(t *testing.T) {
		receivedSyncs := make(chan *pb.SyncCommitteeMessage, 1)

		// Register handler on node 2 for subnet 5
		err := bg2.RegisterSyncCommittee(
			[]uint64{5},
			nil, // no validator
			func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64, from peer.ID) error {
				assert.Equal(t, uint64(5), subnet)
				receivedSyncs <- msg
				return nil
			},
		)
		require.NoError(t, err)

		// Subscribe on node 2 using the generated name
		syncName := fmt.Sprintf("sync_committee_subnets_%v", []uint64{5})
		err = bg2.SubscribeSyncCommittee(ctx, syncName)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		err = bg1.RegisterSyncCommittee([]uint64{5}, nil, nil)
		require.NoError(t, err)
		err = bg1.SubscribeSyncCommittee(ctx, syncName)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish sync committee message
		testSync := &pb.SyncCommitteeMessage{
			Slot:      777,
			BlockRoot: []byte("test-block-root"),
		}

		topic := eth.SyncCommitteeSubnetTopic(forkDigest, 5)
		var buf bytes.Buffer
		_, err = sszEncoder.EncodeGossip(&buf, testSync)
		require.NoError(t, err)
		data := buf.Bytes()

		err = gs1.Publish(ctx, topic, data)
		require.NoError(t, err)

		// Verify reception
		select {
		case received := <-receivedSyncs:
			assert.Equal(t, uint64(777), received.Slot)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for sync committee message")
		}
	})

	t.Run("RegisterAndSubscribeVoluntaryExit", func(t *testing.T) {
		receivedExits := make(chan *pb.SignedVoluntaryExit, 1)

		// Register handler on node 2
		err := bg2.RegisterVoluntaryExit(
			nil, // no validator
			func(ctx context.Context, exit *pb.SignedVoluntaryExit, from peer.ID) error {
				receivedExits <- exit
				return nil
			},
		)
		require.NoError(t, err)

		// Subscribe on node 2
		err = bg2.SubscribeVoluntaryExit(ctx)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		err = bg1.RegisterVoluntaryExit(nil, nil)
		require.NoError(t, err)
		err = bg1.SubscribeVoluntaryExit(ctx)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish voluntary exit
		testExit := &pb.SignedVoluntaryExit{
			Exit: &pb.VoluntaryExit{
				Epoch:          111,
				ValidatorIndex: 222,
			},
		}

		topic := eth.VoluntaryExitTopic(forkDigest[:])
		var buf bytes.Buffer
		_, err = sszEncoder.EncodeGossip(&buf, testExit)
		require.NoError(t, err)
		data := buf.Bytes()

		err = gs1.Publish(ctx, topic, data)
		require.NoError(t, err)

		// Verify reception
		select {
		case received := <-receivedExits:
			assert.Equal(t, uint64(111), received.Exit.Epoch)
			assert.Equal(t, uint64(222), received.Exit.ValidatorIndex)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for voluntary exit")
		}
	})

	t.Run("RegisterAndSubscribeProposerSlashing", func(t *testing.T) {
		receivedSlashings := make(chan *pb.ProposerSlashing, 1)

		// Register handler on node 2
		err := bg2.RegisterProposerSlashing(
			nil, // no validator
			func(ctx context.Context, slashing *pb.ProposerSlashing, from peer.ID) error {
				receivedSlashings <- slashing
				return nil
			},
		)
		require.NoError(t, err)

		// Subscribe on node 2
		err = bg2.SubscribeProposerSlashing(ctx)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		err = bg1.RegisterProposerSlashing(nil, nil)
		require.NoError(t, err)
		err = bg1.SubscribeProposerSlashing(ctx)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish proposer slashing
		testSlashing := &pb.ProposerSlashing{
			Header_1: &pb.SignedBeaconBlockHeader{
				Header: &pb.BeaconBlockHeader{
					Slot: 333,
				},
			},
			Header_2: &pb.SignedBeaconBlockHeader{
				Header: &pb.BeaconBlockHeader{
					Slot: 333,
				},
			},
		}

		topic := eth.ProposerSlashingTopic(forkDigest[:])
		var buf bytes.Buffer
		_, err = sszEncoder.EncodeGossip(&buf, testSlashing)
		require.NoError(t, err)
		data := buf.Bytes()

		err = gs1.Publish(ctx, topic, data)
		require.NoError(t, err)

		// Verify reception
		select {
		case received := <-receivedSlashings:
			assert.Equal(t, uint64(333), received.Header_1.Header.Slot)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for proposer slashing")
		}
	})

	t.Run("RegisterAndSubscribeAttesterSlashing", func(t *testing.T) {
		receivedSlashings := make(chan *pb.AttesterSlashing, 1)

		// Register handler on node 2
		err := bg2.RegisterAttesterSlashing(
			nil, // no validator
			func(ctx context.Context, slashing *pb.AttesterSlashing, from peer.ID) error {
				receivedSlashings <- slashing
				return nil
			},
		)
		require.NoError(t, err)

		// Subscribe on node 2
		err = bg2.SubscribeAttesterSlashing(ctx)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		err = bg1.RegisterAttesterSlashing(nil, nil)
		require.NoError(t, err)
		err = bg1.SubscribeAttesterSlashing(ctx)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish attester slashing
		testSlashing := &pb.AttesterSlashing{
			Attestation_1: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					Slot: 444,
				},
			},
			Attestation_2: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					Slot: 445,
				},
			},
		}

		topic := eth.AttesterSlashingTopic(forkDigest)
		var buf bytes.Buffer
		_, err = sszEncoder.EncodeGossip(&buf, testSlashing)
		require.NoError(t, err)
		data := buf.Bytes()

		err = gs1.Publish(ctx, topic, data)
		require.NoError(t, err)

		// Verify reception
		select {
		case received := <-receivedSlashings:
			assert.Equal(t, uint64(444), received.Attestation_1.Data.Slot)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for attester slashing")
		}
	})

	t.Run("RegisterAndSubscribeSyncContributionAndProof", func(t *testing.T) {
		receivedContribs := make(chan *pb.SignedContributionAndProof, 1)

		// Register handler on node 2
		err := bg2.RegisterSyncContributionAndProof(
			nil, // no validator
			func(ctx context.Context, contrib *pb.SignedContributionAndProof, from peer.ID) error {
				receivedContribs <- contrib
				return nil
			},
		)
		require.NoError(t, err)

		// Subscribe on node 2
		err = bg2.SubscribeSyncContributionAndProof(ctx)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		err = bg1.RegisterSyncContributionAndProof(nil, nil)
		require.NoError(t, err)
		err = bg1.SubscribeSyncContributionAndProof(ctx)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish sync contribution
		testContrib := &pb.SignedContributionAndProof{
			Message: &pb.ContributionAndProof{
				AggregatorIndex: 555,
				Contribution: &pb.SyncCommitteeContribution{
					Slot: 666,
				},
			},
		}

		topic := eth.SyncContributionAndProofTopic(forkDigest)
		var buf bytes.Buffer
		_, err = sszEncoder.EncodeGossip(&buf, testContrib)
		require.NoError(t, err)
		data := buf.Bytes()

		err = gs1.Publish(ctx, topic, data)
		require.NoError(t, err)

		// Verify reception
		select {
		case received := <-receivedContribs:
			assert.Equal(t, uint64(555), received.Message.AggregatorIndex)
			assert.Equal(t, uint64(666), received.Message.Contribution.Slot)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for sync contribution")
		}
	})

	t.Run("RegisterAndSubscribeBlsToExecutionChange", func(t *testing.T) {
		receivedChanges := make(chan *pb.SignedBLSToExecutionChange, 1)

		// Register handler on node 2
		err := bg2.RegisterBlsToExecutionChange(
			nil, // no validator
			func(ctx context.Context, change *pb.SignedBLSToExecutionChange, from peer.ID) error {
				receivedChanges <- change
				return nil
			},
		)
		require.NoError(t, err)

		// Subscribe on node 2
		err = bg2.SubscribeBlsToExecutionChange(ctx)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		err = bg1.RegisterBlsToExecutionChange(nil, nil)
		require.NoError(t, err)
		err = bg1.SubscribeBlsToExecutionChange(ctx)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish BLS to execution change
		testChange := &pb.SignedBLSToExecutionChange{
			Message: &pb.BLSToExecutionChange{
				ValidatorIndex:     777,
				FromBlsPubkey:      []byte("test-bls-pubkey"),
				ToExecutionAddress: []byte("test-exec-address"),
			},
		}

		topic := eth.BlsToExecutionChangeTopic(forkDigest)
		var buf bytes.Buffer
		_, err = sszEncoder.EncodeGossip(&buf, testChange)
		require.NoError(t, err)
		data := buf.Bytes()

		err = gs1.Publish(ctx, topic, data)
		require.NoError(t, err)

		// Verify reception
		select {
		case received := <-receivedChanges:
			assert.Equal(t, uint64(777), received.Message.ValidatorIndex)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for BLS to execution change")
		}
	})
}