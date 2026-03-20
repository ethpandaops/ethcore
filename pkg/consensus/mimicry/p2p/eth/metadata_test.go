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

func TestMetaDataV3_ByteLength(t *testing.T) {
	m := &eth.MetaDataV3{}
	assert.Equal(t, uint64(25), m.ByteLength())
	assert.Equal(t, uint64(25), m.FixedLength())
}

func TestMetaDataV3_SerializeDeserialize(t *testing.T) {
	original := &eth.MetaDataV3{
		SeqNumber:         7,
		Attnets:           common.AttnetBits{0xff, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff, 0x00},
		Syncnets:          common.SyncnetBits{0x0f},
		CustodyGroupCount: 4,
	}

	// Serialize
	var buf bytes.Buffer
	err := original.Serialize(codec.NewEncodingWriter(&buf))
	require.NoError(t, err)
	assert.Equal(t, 25, buf.Len())

	// Deserialize
	decoded := &eth.MetaDataV3{}
	err = decoded.Deserialize(codec.NewDecodingReader(bytes.NewReader(buf.Bytes()), uint64(buf.Len())))
	require.NoError(t, err)

	assert.Equal(t, original.SeqNumber, decoded.SeqNumber)
	assert.Equal(t, original.Attnets, decoded.Attnets)
	assert.Equal(t, original.Syncnets, decoded.Syncnets)
	assert.Equal(t, original.CustodyGroupCount, decoded.CustodyGroupCount)
}

func TestMetaDataV3_ToV2(t *testing.T) {
	v3 := &eth.MetaDataV3{
		SeqNumber:         7,
		Attnets:           common.AttnetBits{0xff},
		Syncnets:          common.SyncnetBits{0x0f},
		CustodyGroupCount: 4,
	}

	v2 := v3.ToV2()

	assert.Equal(t, v3.SeqNumber, v2.SeqNumber)
	assert.Equal(t, v3.Attnets, v2.Attnets)
	assert.Equal(t, v3.Syncnets, v2.Syncnets)
}

func TestMetaDataV3FromV2(t *testing.T) {
	v2 := &common.MetaData{
		SeqNumber: 7,
		Attnets:   common.AttnetBits{0xff},
		Syncnets:  common.SyncnetBits{0x0f},
	}

	v3 := eth.MetaDataV3FromV2(v2)

	assert.Equal(t, v2.SeqNumber, v3.SeqNumber)
	assert.Equal(t, v2.Attnets, v3.Attnets)
	assert.Equal(t, v2.Syncnets, v3.Syncnets)
	assert.Equal(t, eth.DefaultCustodyGroupCount, v3.CustodyGroupCount)
}

func TestMetaDataV3_RoundTrip(t *testing.T) {
	original := &common.MetaData{
		SeqNumber: 42,
		Attnets:   common.AttnetBits{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88},
		Syncnets:  common.SyncnetBits{0x0a},
	}

	// V2 -> V3 -> V2 should preserve all shared fields.
	v3 := eth.MetaDataV3FromV2(original)
	result := v3.ToV2()

	assert.Equal(t, original, result)
}
