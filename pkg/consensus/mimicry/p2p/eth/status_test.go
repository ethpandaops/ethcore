package eth_test

import (
	"bytes"
	"testing"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/codec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusV2_ByteLength(t *testing.T) {
	s := &eth.StatusV2{}
	assert.Equal(t, uint64(92), s.ByteLength())
	assert.Equal(t, uint64(92), s.FixedLength())
}

func TestStatusV2_SerializeDeserialize(t *testing.T) {
	original := &eth.StatusV2{
		ForkDigest:            common.ForkDigest{0x01, 0x02, 0x03, 0x04},
		FinalizedRoot:         common.Root{0xaa},
		FinalizedEpoch:        42,
		HeadRoot:              common.Root{0xbb},
		HeadSlot:              100,
		EarliestAvailableSlot: 50,
	}

	// Serialize
	var buf bytes.Buffer
	err := original.Serialize(codec.NewEncodingWriter(&buf))
	require.NoError(t, err)
	assert.Equal(t, 92, buf.Len())

	// Deserialize
	decoded := &eth.StatusV2{}
	err = decoded.Deserialize(codec.NewDecodingReader(bytes.NewReader(buf.Bytes()), uint64(buf.Len())))
	require.NoError(t, err)

	assert.Equal(t, original.ForkDigest, decoded.ForkDigest)
	assert.Equal(t, original.FinalizedRoot, decoded.FinalizedRoot)
	assert.Equal(t, original.FinalizedEpoch, decoded.FinalizedEpoch)
	assert.Equal(t, original.HeadRoot, decoded.HeadRoot)
	assert.Equal(t, original.HeadSlot, decoded.HeadSlot)
	assert.Equal(t, original.EarliestAvailableSlot, decoded.EarliestAvailableSlot)
}

func TestStatusV2_ToV1(t *testing.T) {
	v2 := &eth.StatusV2{
		ForkDigest:            common.ForkDigest{0x01, 0x02, 0x03, 0x04},
		FinalizedRoot:         common.Root{0xaa},
		FinalizedEpoch:        42,
		HeadRoot:              common.Root{0xbb},
		HeadSlot:              100,
		EarliestAvailableSlot: 50,
	}

	v1 := v2.ToV1()

	assert.Equal(t, v2.ForkDigest, v1.ForkDigest)
	assert.Equal(t, v2.FinalizedRoot, v1.FinalizedRoot)
	assert.Equal(t, v2.FinalizedEpoch, v1.FinalizedEpoch)
	assert.Equal(t, v2.HeadRoot, v1.HeadRoot)
	assert.Equal(t, v2.HeadSlot, v1.HeadSlot)
}

func TestStatusV2FromV1(t *testing.T) {
	v1 := &common.Status{
		ForkDigest:     common.ForkDigest{0x01, 0x02, 0x03, 0x04},
		FinalizedRoot:  common.Root{0xaa},
		FinalizedEpoch: 42,
		HeadRoot:       common.Root{0xbb},
		HeadSlot:       100,
	}

	v2 := eth.StatusV2FromV1(v1)

	assert.Equal(t, v1.ForkDigest, v2.ForkDigest)
	assert.Equal(t, v1.FinalizedRoot, v2.FinalizedRoot)
	assert.Equal(t, v1.FinalizedEpoch, v2.FinalizedEpoch)
	assert.Equal(t, v1.HeadRoot, v2.HeadRoot)
	assert.Equal(t, v1.HeadSlot, v2.HeadSlot)
	assert.Equal(t, common.Slot(0), v2.EarliestAvailableSlot)
}

func TestStatusV2_RoundTrip(t *testing.T) {
	original := &common.Status{
		ForkDigest:     common.ForkDigest{0xde, 0xad, 0xbe, 0xef},
		FinalizedRoot:  common.Root{0x11, 0x22, 0x33},
		FinalizedEpoch: 999,
		HeadRoot:       common.Root{0x44, 0x55, 0x66},
		HeadSlot:       12345,
	}

	// V1 -> V2 -> V1 should preserve all shared fields.
	v2 := eth.StatusV2FromV1(original)
	result := v2.ToV1()

	assert.Equal(t, original, result)
}
