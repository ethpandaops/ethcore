package topics_test

import (
	"fmt"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/topics"
)

func ExampleWithFork() {
	// Example of using a regular topic with a fork digest
	forkDigest := [4]byte{0x01, 0x02, 0x03, 0x04}
	
	// Apply fork digest to beacon block topic
	blockTopic := topics.WithFork(topics.BeaconBlock, forkDigest)
	fmt.Println("Beacon block topic:", blockTopic.Name())
	
	// Apply fork digest to voluntary exit topic
	exitTopic := topics.WithFork(topics.VoluntaryExit, forkDigest)
	fmt.Println("Voluntary exit topic:", exitTopic.Name())
	
	// Output:
	// Beacon block topic: /eth2/01020304/beacon_block/ssz_snappy
	// Voluntary exit topic: /eth2/01020304/voluntary_exit/ssz_snappy
}

func Example_subnetTopics() {
	// Example of using subnet topics
	forkDigest := [4]byte{0xaa, 0xbb, 0xcc, 0xdd}
	
	// Get attestation topic for subnet 5
	attestationTopic, err := topics.Attestation.TopicForSubnet(5, forkDigest)
	if err != nil {
		panic(err)
	}
	fmt.Println("Attestation subnet 5:", attestationTopic.Name())
	
	// Get sync committee topic for subnet 2
	syncTopic, err := topics.SyncCommittee.TopicForSubnet(2, forkDigest)
	if err != nil {
		panic(err)
	}
	fmt.Println("Sync committee subnet 2:", syncTopic.Name())
	
	// Parse subnet from topic name
	subnet, err := topics.Attestation.ParseSubnet(attestationTopic.Name())
	if err != nil {
		panic(err)
	}
	fmt.Printf("Parsed subnet: %d\n", subnet)
	
	// Output:
	// Attestation subnet 5: /eth2/aabbccdd/beacon_attestation_5/ssz_snappy
	// Sync committee subnet 2: /eth2/aabbccdd/sync_committee_2/ssz_snappy
	// Parsed subnet: 5
}