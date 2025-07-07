package eth_test

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1/eth"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// Example_setupService shows how to set up a reqresp service with handlers.
func Example_setupService() {
	// This example demonstrates the API usage (not executable)
	fmt.Println("Setting up reqresp service...")

	// 1. Define your message types
	type Status struct {
		ForkDigest     [4]byte
		FinalizedRoot  [32]byte
		FinalizedEpoch uint64
		HeadRoot       [32]byte
		HeadSlot       uint64
	}

	// 2. Create an encoder that implements v1.Encoder
	// type MyEncoder struct{}
	// func (e *MyEncoder) Encode(msg any) ([]byte, error) { return nil, nil }
	// func (e *MyEncoder) Decode(data []byte, msgType any) error { return nil }

	// 3. Create the service
	// var h host.Host // Would be created with libp2p
	// logger := logrus.New()
	// service := v1.New(h, v1.ServiceConfig{
	//     ClientConfig: v1.ClientConfig{
	//         DefaultTimeout: 10 * time.Second,
	//         MaxRetries:     3,
	//     },
	// }, logger)

	// 4. Create protocol with compile-time safety
	statusProtocol := eth.NewStatus[Status, Status](84, 84)

	// 5. Register handler
	// encoder := &MyEncoder{}
	// v1.RegisterProtocol(service, statusProtocol,
	//     func(ctx context.Context, req Status, from peer.ID) (Status, error) {
	//         return Status{}, nil
	//     },
	//     v1.HandlerOptions{Encoder: encoder},
	// )

	fmt.Printf("Protocol ID: %s\n", statusProtocol.ID())
	// Output: Setting up reqresp service...
	// Protocol ID: /eth2/beacon_chain/req/status/1/ssz_snappy
}

// Example_chunkedHandler shows how to work with chunked protocol handlers.
func Example_chunkedHandler() {
	// Define types
	type BlocksByRangeRequest struct {
		StartSlot uint64
		Count     uint64
	}
	type BeaconBlock struct {
		Slot uint64
		Body []byte
	}

	// Create chunked protocol
	protocol := eth.NewBeaconBlocksByRangeV2[BlocksByRangeRequest, BeaconBlock](
		12,           // Request size
		10*1024*1024, // Max response size per chunk
	)

	// Register chunked handler (example code structure)
	// v1.RegisterChunkedProtocol(service, protocol,
	//     func(ctx context.Context, req BlocksByRangeRequest, from peer.ID, w v1.ChunkedResponseWriter[BeaconBlock]) error {
	//         // Write blocks one by one
	//         for i := uint64(0); i < req.Count; i++ {
	//             block := BeaconBlock{Slot: req.StartSlot + i}
	//             if err := w.WriteChunk(block); err != nil {
	//                 return err
	//             }
	//         }
	//         return nil
	//     },
	//     v1.HandlerOptions{Encoder: encoder},
	// )

	fmt.Printf("Chunked protocol ID: %s\n", protocol.ID())
	// Output: Chunked protocol ID: /eth2/beacon_chain/req/beacon_blocks_by_range/2/ssz_snappy
}

// Example_protocolTypes shows how to create all Ethereum protocol types.
func Example_protocolTypes() {
	// You can create any Ethereum protocol with your own types
	type MyStatus struct{ Version uint64 }
	type MyPing uint64
	type MyMetadata struct{ SeqNumber uint64 }
	type MyBlockRequest struct{ StartSlot uint64 }
	type MyBlock struct{ Slot uint64 }

	// Single response protocols
	_ = eth.NewStatus[MyStatus, MyStatus](100, 100)
	_ = eth.NewGoodbye[uint64, struct{}](8, 0)
	_ = eth.NewPing[MyPing, MyPing](8, 8)
	_ = eth.NewMetadataV2[struct{}, MyMetadata](0, 200)

	// Chunked response protocols
	_ = eth.NewBeaconBlocksByRangeV2[MyBlockRequest, MyBlock](20, 1<<20)
	_ = eth.NewBlobSidecarsByRangeV1[MyBlockRequest, MyBlock](20, 200<<10)

	fmt.Println("All protocols created with compile-time safety")
	// Output: All protocols created with compile-time safety
}

// The following are referenced types for documentation purposes.
var (
	_ host.Host
	_ peer.ID
	_ logrus.FieldLogger
	_ time.Duration
	_ context.Context
	_ v1.Encoder
	_ v1.ServiceConfig
	_ v1.HandlerOptions
)
