package eth

import (
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/reqresp/v1"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// NewStatus creates a status protocol with compile-time validated protocol ID.
func NewStatus[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.Protocol[TReq, TResp] {
	return v1.NewProtocol(
		protocol.ID(eth.StatusV1ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}

// NewGoodbye creates a goodbye protocol with compile-time validated protocol ID.
func NewGoodbye[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.Protocol[TReq, TResp] {
	return v1.NewProtocol(
		protocol.ID(eth.GoodbyeV1ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}

// NewPing creates a ping protocol with compile-time validated protocol ID.
func NewPing[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.Protocol[TReq, TResp] {
	return v1.NewProtocol(
		protocol.ID(eth.PingV1ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}

// NewMetadataV1 creates a metadata V1 protocol with compile-time validated protocol ID.
func NewMetadataV1[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.Protocol[TReq, TResp] {
	return v1.NewProtocol(
		protocol.ID(eth.MetaDataV1ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}

// NewMetadataV2 creates a metadata V2 protocol with compile-time validated protocol ID.
func NewMetadataV2[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.Protocol[TReq, TResp] {
	return v1.NewProtocol(
		protocol.ID(eth.MetaDataV2ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}

// NewBeaconBlocksByRangeV1 creates a beacon blocks by range V1 protocol with compile-time validated protocol ID.
func NewBeaconBlocksByRangeV1[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.ChunkedProtocol[TReq, TResp] {
	return v1.NewChunkedProtocol(
		protocol.ID(eth.BeaconBlocksByRangeV1ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}

// NewBeaconBlocksByRangeV2 creates a beacon blocks by range V2 protocol with compile-time validated protocol ID.
func NewBeaconBlocksByRangeV2[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.ChunkedProtocol[TReq, TResp] {
	return v1.NewChunkedProtocol(
		protocol.ID(eth.BeaconBlocksByRangeV2ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}

// NewBeaconBlocksByRootV1 creates a beacon blocks by root V1 protocol with compile-time validated protocol ID.
func NewBeaconBlocksByRootV1[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.ChunkedProtocol[TReq, TResp] {
	return v1.NewChunkedProtocol(
		protocol.ID(eth.BeaconBlocksByRootV1ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}

// NewBeaconBlocksByRootV2 creates a beacon blocks by root V2 protocol with compile-time validated protocol ID.
func NewBeaconBlocksByRootV2[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.ChunkedProtocol[TReq, TResp] {
	return v1.NewChunkedProtocol(
		protocol.ID(eth.BeaconBlocksByRootV2ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}

// NewBlobSidecarsByRangeV1 creates a blob sidecars by range V1 protocol with compile-time validated protocol ID.
func NewBlobSidecarsByRangeV1[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.ChunkedProtocol[TReq, TResp] {
	return v1.NewChunkedProtocol(
		protocol.ID(eth.BlobSidecarsByRangeV1ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}

// NewBlobSidecarsByRootV1 creates a blob sidecars by root V1 protocol with compile-time validated protocol ID.
func NewBlobSidecarsByRootV1[TReq, TResp any](maxRequestSize, maxResponseSize uint64) v1.ChunkedProtocol[TReq, TResp] {
	return v1.NewChunkedProtocol(
		protocol.ID(eth.BlobSidecarsByRootV1ProtocolID),
		maxRequestSize,
		maxResponseSize,
	)
}
