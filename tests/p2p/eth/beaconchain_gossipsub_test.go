package eth_test

import (
	"bytes"
	"context"
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

		// Create processor on node 2
		processor2 := bg2.CreateBeaconBlockProcessor(
			nil, // no validator
			func(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
				receivedBlocks <- block
				return nil
			},
			nil, // no score params
		)

		// Subscribe on node 2
		err = bg2.SubscribeBeaconBlock(ctx, processor2)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1 to ensure topic exists
		processor1 := bg1.CreateBeaconBlockProcessor(nil, nil, nil)
		err = bg1.SubscribeBeaconBlock(ctx, processor1)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish the block
		testSignature := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testSignature {
			testSignature[i] = byte(i % 256)
		}
		testParentRoot := make([]byte, 32) // Hash is 32 bytes
		for i := range testParentRoot {
			testParentRoot[i] = byte((i + 100) % 256)
		}
		testStateRoot := make([]byte, 32) // Hash is 32 bytes
		for i := range testStateRoot {
			testStateRoot[i] = byte((i + 230) % 256)
		}
		testRandaoReveal := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testRandaoReveal {
			testRandaoReveal[i] = byte((i + 240) % 256)
		}
		testDepositRoot := make([]byte, 32) // Hash is 32 bytes
		for i := range testDepositRoot {
			testDepositRoot[i] = byte((i + 250) % 256)
		}
		testBlockHash := make([]byte, 32) // Hash is 32 bytes
		for i := range testBlockHash {
			testBlockHash[i] = byte((i + 260) % 256)
		}
		testGraffiti := make([]byte, 32) // Graffiti is 32 bytes
		for i := range testGraffiti {
			testGraffiti[i] = byte((i + 270) % 256)
		}
		testBlock := &pb.SignedBeaconBlock{
			Block: &pb.BeaconBlock{
				Slot:       12345,
				ParentRoot: testParentRoot,
				StateRoot:  testStateRoot,
				Body: &pb.BeaconBlockBody{
					RandaoReveal: testRandaoReveal,
					Eth1Data: &pb.Eth1Data{
						DepositRoot: testDepositRoot,
						BlockHash:   testBlockHash,
					},
					Graffiti: testGraffiti,
				},
			},
			Signature: testSignature,
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

		// Create processor on node 2 for subnet 10
		processor2 := bg2.CreateAttestationProcessor(
			[]uint64{10},
			nil, // no validator
			func(ctx context.Context, att *pb.Attestation, subnet uint64, from peer.ID) error {
				assert.Equal(t, uint64(10), subnet)
				receivedAtts <- att
				return nil
			},
		)

		// Subscribe on node 2
		err = bg2.SubscribeAttestation(ctx, processor2)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1 to ensure topic exists
		processor1 := bg1.CreateAttestationProcessor([]uint64{10}, nil, nil)
		err = bg1.SubscribeAttestation(ctx, processor1)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish attestation
		testAttBeaconBlockRoot := make([]byte, 32) // Hash is 32 bytes
		for i := range testAttBeaconBlockRoot {
			testAttBeaconBlockRoot[i] = byte((i + 150) % 256)
		}
		testAttSignature := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testAttSignature {
			testAttSignature[i] = byte((i + 160) % 256)
		}
		testAttRoot := make([]byte, 32) // Hash is 32 bytes
		for i := range testAttRoot {
			testAttRoot[i] = byte((i + 170) % 256)
		}
		testAtt := &pb.Attestation{
			Data: &pb.AttestationData{
				Slot:            54321,
				BeaconBlockRoot: testAttBeaconBlockRoot,
				Source: &pb.Checkpoint{
					Root: testAttRoot,
				},
				Target: &pb.Checkpoint{
					Root: testAttRoot,
				},
			},
			Signature: testAttSignature,
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

		// Create processor on node 2
		processor2 := bg2.CreateAggregateAndProofProcessor(
			nil, // no validator
			func(ctx context.Context, agg *pb.AggregateAttestationAndProof, from peer.ID) error {
				receivedAggs <- agg
				return nil
			},
		)

		// Subscribe on node 2
		err = bg2.SubscribeAggregateAndProof(ctx, processor2)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		processor1 := bg1.CreateAggregateAndProofProcessor(nil, nil)
		err = bg1.SubscribeAggregateAndProof(ctx, processor1)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish aggregate
		testSelectionProof := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testSelectionProof {
			testSelectionProof[i] = byte((i + 10) % 256)
		}
		testAggregateSignature := make([]byte, 96) // BLS signature is 96 bytes  
		for i := range testAggregateSignature {
			testAggregateSignature[i] = byte((i + 20) % 256)
		}
		testBeaconBlockRoot := make([]byte, 32) // Hash is 32 bytes
		for i := range testBeaconBlockRoot {
			testBeaconBlockRoot[i] = byte((i + 30) % 256)
		}
		testAggRoot := make([]byte, 32) // Hash is 32 bytes
		for i := range testAggRoot {
			testAggRoot[i] = byte((i + 40) % 256)
		}
		testAgg := &pb.AggregateAttestationAndProof{
			AggregatorIndex: 100,
			Aggregate: &pb.Attestation{
				Data: &pb.AttestationData{
					Slot:            99999,
					BeaconBlockRoot: testBeaconBlockRoot,
					Source: &pb.Checkpoint{
						Root: testAggRoot,
					},
					Target: &pb.Checkpoint{
						Root: testAggRoot,
					},
				},
				Signature: testAggregateSignature,
			},
			SelectionProof: testSelectionProof,
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

		// Create processor on node 2 for subnet 5
		processor2 := bg2.CreateSyncCommitteeProcessor(
			[]uint64{5},
			nil, // no validator
			func(ctx context.Context, msg *pb.SyncCommitteeMessage, subnet uint64, from peer.ID) error {
				assert.Equal(t, uint64(5), subnet)
				receivedSyncs <- msg
				return nil
			},
		)

		// Subscribe on node 2
		err = bg2.SubscribeSyncCommittee(ctx, processor2)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		processor1 := bg1.CreateSyncCommitteeProcessor([]uint64{5}, nil, nil)
		err = bg1.SubscribeSyncCommittee(ctx, processor1)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish sync committee message
		testBlockRoot := make([]byte, 32)
		for i := range testBlockRoot {
			testBlockRoot[i] = byte(i % 256)
		}
		testSyncSignature := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testSyncSignature {
			testSyncSignature[i] = byte((i + 170) % 256)
		}
		testSync := &pb.SyncCommitteeMessage{
			Slot:      777,
			BlockRoot: testBlockRoot, // 32 bytes for BlockRoot
			Signature: testSyncSignature,
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

		// Create processor on node 2
		processor2 := bg2.CreateVoluntaryExitProcessor(
			nil, // no validator
			func(ctx context.Context, exit *pb.SignedVoluntaryExit, from peer.ID) error {
				receivedExits <- exit
				return nil
			},
		)

		// Subscribe on node 2
		err = bg2.SubscribeVoluntaryExit(ctx, processor2)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		processor1 := bg1.CreateVoluntaryExitProcessor(nil, nil)
		err = bg1.SubscribeVoluntaryExit(ctx, processor1)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish voluntary exit
		testExitSignature := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testExitSignature {
			testExitSignature[i] = byte((i + 40) % 256)
		}
		testExit := &pb.SignedVoluntaryExit{
			Exit: &pb.VoluntaryExit{
				Epoch:          111,
				ValidatorIndex: 222,
			},
			Signature: testExitSignature,
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

		// Create processor on node 2
		processor2 := bg2.CreateProposerSlashingProcessor(
			nil, // no validator
			func(ctx context.Context, slashing *pb.ProposerSlashing, from peer.ID) error {
				receivedSlashings <- slashing
				return nil
			},
		)

		// Subscribe on node 2
		err = bg2.SubscribeProposerSlashing(ctx, processor2)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		processor1 := bg1.CreateProposerSlashingProcessor(nil, nil)
		err = bg1.SubscribeProposerSlashing(ctx, processor1)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish proposer slashing
		testParentRoot1 := make([]byte, 32) // Hash is 32 bytes
		for i := range testParentRoot1 {
			testParentRoot1[i] = byte((i + 50) % 256)
		}
		testParentRoot2 := make([]byte, 32) // Hash is 32 bytes 
		for i := range testParentRoot2 {
			testParentRoot2[i] = byte((i + 60) % 256)
		}
		testSlashingSignature1 := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testSlashingSignature1 {
			testSlashingSignature1[i] = byte((i + 70) % 256)
		}
		testSlashingSignature2 := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testSlashingSignature2 {
			testSlashingSignature2[i] = byte((i + 80) % 256)
		}
		testStateRoot1 := make([]byte, 32) // Hash is 32 bytes
		for i := range testStateRoot1 {
			testStateRoot1[i] = byte((i + 90) % 256)
		}
		testStateRoot2 := make([]byte, 32) // Hash is 32 bytes
		for i := range testStateRoot2 {
			testStateRoot2[i] = byte((i + 100) % 256)
		}
		testBodyRoot1 := make([]byte, 32) // Hash is 32 bytes
		for i := range testBodyRoot1 {
			testBodyRoot1[i] = byte((i + 110) % 256)
		}
		testBodyRoot2 := make([]byte, 32) // Hash is 32 bytes
		for i := range testBodyRoot2 {
			testBodyRoot2[i] = byte((i + 120) % 256)
		}
		testSlashing := &pb.ProposerSlashing{
			Header_1: &pb.SignedBeaconBlockHeader{
				Header: &pb.BeaconBlockHeader{
					Slot:       333,
					ParentRoot: testParentRoot1,
					StateRoot:  testStateRoot1,
					BodyRoot:   testBodyRoot1,
				},
				Signature: testSlashingSignature1,
			},
			Header_2: &pb.SignedBeaconBlockHeader{
				Header: &pb.BeaconBlockHeader{
					Slot:       333,
					ParentRoot: testParentRoot2,
					StateRoot:  testStateRoot2,
					BodyRoot:   testBodyRoot2,
				},
				Signature: testSlashingSignature2,
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

		// Create processor on node 2
		processor2 := bg2.CreateAttesterSlashingProcessor(
			nil, // no validator
			func(ctx context.Context, slashing *pb.AttesterSlashing, from peer.ID) error {
				receivedSlashings <- slashing
				return nil
			},
		)

		// Subscribe on node 2
		err = bg2.SubscribeAttesterSlashing(ctx, processor2)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		processor1 := bg1.CreateAttesterSlashingProcessor(nil, nil)
		err = bg1.SubscribeAttesterSlashing(ctx, processor1)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish attester slashing
		testBeaconBlockRoot1 := make([]byte, 32) // Hash is 32 bytes
		for i := range testBeaconBlockRoot1 {
			testBeaconBlockRoot1[i] = byte((i + 110) % 256)
		}
		testBeaconBlockRoot2 := make([]byte, 32) // Hash is 32 bytes
		for i := range testBeaconBlockRoot2 {
			testBeaconBlockRoot2[i] = byte((i + 120) % 256)
		}
		testIndexedSignature1 := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testIndexedSignature1 {
			testIndexedSignature1[i] = byte((i + 130) % 256)
		}
		testIndexedSignature2 := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testIndexedSignature2 {
			testIndexedSignature2[i] = byte((i + 140) % 256)
		}
		testAttesterRoot1 := make([]byte, 32) // Hash is 32 bytes
		for i := range testAttesterRoot1 {
			testAttesterRoot1[i] = byte((i + 150) % 256)
		}
		testAttesterRoot2 := make([]byte, 32) // Hash is 32 bytes
		for i := range testAttesterRoot2 {
			testAttesterRoot2[i] = byte((i + 160) % 256)
		}
		testSlashing := &pb.AttesterSlashing{
			Attestation_1: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					Slot:            444,
					BeaconBlockRoot: testBeaconBlockRoot1,
					Source: &pb.Checkpoint{
						Root: testAttesterRoot1,
					},
					Target: &pb.Checkpoint{
						Root: testAttesterRoot1,
					},
				},
				Signature: testIndexedSignature1,
			},
			Attestation_2: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					Slot:            445,
					BeaconBlockRoot: testBeaconBlockRoot2,
					Source: &pb.Checkpoint{
						Root: testAttesterRoot2,
					},
					Target: &pb.Checkpoint{
						Root: testAttesterRoot2,
					},
				},
				Signature: testIndexedSignature2,
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

		// Create processor on node 2
		processor2 := bg2.CreateSyncContributionAndProofProcessor(
			nil, // no validator
			func(ctx context.Context, contrib *pb.SignedContributionAndProof, from peer.ID) error {
				receivedContribs <- contrib
				return nil
			},
		)

		// Subscribe on node 2
		err = bg2.SubscribeSyncContributionAndProof(ctx, processor2)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		processor1 := bg1.CreateSyncContributionAndProofProcessor(nil, nil)
		err = bg1.SubscribeSyncContributionAndProof(ctx, processor1)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish sync contribution
		testContribBlockRoot := make([]byte, 32) // Hash is 32 bytes
		for i := range testContribBlockRoot {
			testContribBlockRoot[i] = byte((i + 180) % 256)
		}
		testContribSelectionProof := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testContribSelectionProof {
			testContribSelectionProof[i] = byte((i + 190) % 256)
		}
		testContribSignature := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testContribSignature {
			testContribSignature[i] = byte((i + 200) % 256)
		}
		testContribAggBits := make([]byte, 16) // Aggregation bits are 16 bytes
		for i := range testContribAggBits {
			testContribAggBits[i] = byte((i + 210) % 256)
		}
		testContribSyncSig := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testContribSyncSig {
			testContribSyncSig[i] = byte((i + 220) % 256)
		}
		testContrib := &pb.SignedContributionAndProof{
			Message: &pb.ContributionAndProof{
				AggregatorIndex: 555,
				Contribution: &pb.SyncCommitteeContribution{
					Slot:            666,
					BlockRoot:       testContribBlockRoot,
					AggregationBits: testContribAggBits,
					Signature:       testContribSyncSig,
				},
				SelectionProof: testContribSelectionProof,
			},
			Signature: testContribSignature,
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

		// Create processor on node 2
		processor2 := bg2.CreateBlsToExecutionChangeProcessor(
			nil, // no validator
			func(ctx context.Context, change *pb.SignedBLSToExecutionChange, from peer.ID) error {
				receivedChanges <- change
				return nil
			},
		)

		// Subscribe on node 2
		err = bg2.SubscribeBlsToExecutionChange(ctx, processor2)
		require.NoError(t, err)

		// Wait for subscription to propagate
		time.Sleep(100 * time.Millisecond)

		// Also subscribe on node 1
		processor1 := bg1.CreateBlsToExecutionChangeProcessor(nil, nil)
		err = bg1.SubscribeBlsToExecutionChange(ctx, processor1)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Publish BLS to execution change
		testBlsChangeBlsPubkey := make([]byte, 48) // BLS public key is 48 bytes
		for i := range testBlsChangeBlsPubkey {
			testBlsChangeBlsPubkey[i] = byte((i + 280) % 256)
		}
		testBlsChangeSignature := make([]byte, 96) // BLS signature is 96 bytes
		for i := range testBlsChangeSignature {
			testBlsChangeSignature[i] = byte((i + 290) % 256)
		}
		testBlsChangeExecAddress := make([]byte, 20) // Execution address is 20 bytes
		for i := range testBlsChangeExecAddress {
			testBlsChangeExecAddress[i] = byte((i + 300) % 256)
		}
		testChange := &pb.SignedBLSToExecutionChange{
			Message: &pb.BLSToExecutionChange{
				ValidatorIndex:     777,
				FromBlsPubkey:      testBlsChangeBlsPubkey,
				ToExecutionAddress: testBlsChangeExecAddress,
			},
			Signature: testBlsChangeSignature,
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