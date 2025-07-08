package eth_test

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1/eth"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// MyNetworkEncoder is an example network encoder implementation.
// In production, this would use SSZ + Snappy.
type MyNetworkEncoder struct{}

func (e *MyNetworkEncoder) EncodeNetwork(msg any) ([]byte, error) {
	// In production: SSZ marshal + Snappy compress
	return nil, nil
}

func (e *MyNetworkEncoder) DecodeNetwork(data []byte, msgType any) error {
	// In production: Snappy decompress + SSZ unmarshal
	return nil
}

// Example_simplifiedAPI shows the new simplified API.
func Example_simplifiedAPI() {
	// 1. Define your message types
	type Status struct {
		ForkDigest     [4]byte
		FinalizedRoot  [32]byte
		FinalizedEpoch uint64
		HeadRoot       [32]byte
		HeadSlot       uint64
	}

	// 2. Create service with simplified config
	var h host.Host // Would be created with libp2p
	logger := logrus.New()

	service := v1.New(h, v1.Config{}, logger)

	// 3. Create protocol with compile-time safety
	networkEncoder := &MyNetworkEncoder{}
	statusProtocol := eth.NewStatus[Status, Status](84, 84, networkEncoder)

	// 4. Register handler using convenient wrapper
	statusHandler := func(ctx context.Context, req Status, from peer.ID) (Status, error) {
		fmt.Printf("Received status from %s\n", from)

		return Status{
			ForkDigest:     [4]byte{0x00, 0x00, 0x00, 0x01},
			FinalizedEpoch: 1000,
			HeadSlot:       2000,
		}, nil
	}

	// Register stream handler with convenient wrapper
	err := v1.RegisterStreamHandler(service, statusProtocol, statusHandler)
	_ = err

	// 5. Making outbound requests with convenient wrapper
	ctx := context.Background()
	var targetPeer peer.ID // Would be an actual peer

	response, err2 := v1.SendRequest[Status, Status](ctx, h, targetPeer, statusProtocol,
		Status{FinalizedEpoch: 500, HeadSlot: 1000})
	_ = response
	_ = err2

	fmt.Println("Simplified API example complete")
	// Output: Simplified API example complete
}

// Example_chunkedProtocolSimplified shows chunked protocols with the new API.
func Example_chunkedProtocolSimplified() {
	// Define types
	type BlockRequest struct {
		StartSlot uint64
		Count     uint64
	}
	type Block struct {
		Slot uint64
		Data []byte
	}

	// Create chunked protocol
	networkEncoder := &MyNetworkEncoder{}
	protocol := eth.NewBeaconBlocksByRangeV2[BlockRequest, Block](12, 1<<20, networkEncoder)

	var service *v1.ReqResp // Would be properly initialized

	// Register chunked handler using convenient wrapper
	chunkedHandler := func(ctx context.Context, req BlockRequest, from peer.ID, w v1.ChunkedResponseWriter[Block]) error {
		// Send blocks one by one
		for i := uint64(0); i < req.Count; i++ {
			block := Block{
				Slot: req.StartSlot + i,
				Data: fmt.Appendf(nil, "block-%d", i),
			}
			if err := w.WriteChunk(block); err != nil {
				return err
			}
		}

		return nil
	}

	err := service.RegisterHandler(protocol.ID(), func(stream network.Stream) {
		if err := v1.HandleChunkedStream(stream, protocol, chunkedHandler); err != nil {
			fmt.Printf("Handle error: %v\n", err)
		}
	})
	_ = err

	fmt.Println("Chunked protocol registered")
	// Output: Chunked protocol registered
}

// The following are referenced types for documentation purposes.
var (
	_ host.Host
	_ peer.ID
	_ logrus.FieldLogger
	_ time.Duration
	_ context.Context
	_ v1.NetworkEncoder
	_ v1.Config
)
