package eth_test

import (
	"fmt"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1/eth"
)

// DummyNetworkEncoder for examples.
type DummyNetworkEncoder struct{}

func (e *DummyNetworkEncoder) EncodeNetwork(msg any) ([]byte, error)        { return nil, nil }
func (e *DummyNetworkEncoder) DecodeNetwork(data []byte, msgType any) error { return nil }

// Example shows how to use the eth package for compile-time safe protocol creation.
func Example() {
	// Create a status protocol with your own types
	type MyStatus struct {
		ForkDigest     [4]byte
		FinalizedRoot  [32]byte
		FinalizedEpoch uint64
		HeadRoot       [32]byte
		HeadSlot       uint64
	}

	// Create protocol with compile-time validated ID
	networkEncoder := &DummyNetworkEncoder{}
	statusProtocol := eth.NewStatus[MyStatus, MyStatus](
		84, // Status request size
		84, // Status response size
		networkEncoder,
	)

	fmt.Println("Status protocol ID:", statusProtocol.ID())
	// Output: Status protocol ID: /eth2/beacon_chain/req/status/1/ssz_snappy
}

// Example_chunkedProtocol shows how to handle chunked protocols.
func Example_chunkedProtocol() {
	// Define your block type
	type MyBeaconBlock struct {
		Slot          uint64
		ProposerIndex uint64
		// ... other fields
	}

	type BlocksByRangeRequest struct {
		StartSlot uint64
		Count     uint64
	}

	// Create chunked protocol
	networkEncoder := &DummyNetworkEncoder{}
	blocksByRange := eth.NewBeaconBlocksByRangeV2[BlocksByRangeRequest, MyBeaconBlock](
		12,           // Request size
		10*1024*1024, // Max response size per chunk (10MB)
		networkEncoder,
	)

	// The protocol is chunked because we used NewBeaconBlocksByRangeV2
	fmt.Printf("Blocks by range protocol ID: %s\n", blocksByRange.ID())

	// Output: Blocks by range protocol ID: /eth2/beacon_chain/req/beacon_blocks_by_range/2/ssz_snappy
}

// Example_pingProtocol shows a simple ping/pong implementation.
func Example_pingProtocol() {
	// Create ping protocol with uint64 request/response
	networkEncoder := &DummyNetworkEncoder{}
	pingProtocol := eth.NewPing[uint64, uint64](
		8, // uint64 request size
		8, // uint64 response size
		networkEncoder,
	)

	fmt.Println("Ping protocol ID:", pingProtocol.ID())
	// Output: Ping protocol ID: /eth2/beacon_chain/req/ping/1/ssz_snappy
}
