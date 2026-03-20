package eth

import (
	"fmt"

	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/codec"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
)

// CustodyGroupCount represents the number of data column custody groups a node participates in.
type CustodyGroupCount view.Uint64View

// Deserialize implements codec.Deserializable.
func (c *CustodyGroupCount) Deserialize(dr *codec.DecodingReader) error {
	return (*view.Uint64View)(c).Deserialize(dr)
}

// Serialize implements codec.Serializable.
func (c CustodyGroupCount) Serialize(w *codec.EncodingWriter) error {
	return w.WriteUint64(uint64(c))
}

// ByteLength returns the SSZ byte length.
func (CustodyGroupCount) ByteLength() uint64 { return 8 }

// FixedLength returns the fixed SSZ byte length.
func (CustodyGroupCount) FixedLength() uint64 { return 8 }

// HashTreeRoot computes the SSZ hash tree root.
func (c CustodyGroupCount) HashTreeRoot(hFn tree.HashFn) common.Root {
	return view.Uint64View(c).HashTreeRoot(hFn)
}

// DefaultCustodyGroupCount is the Fulu spec default for CUSTODY_REQUIREMENT.
const DefaultCustodyGroupCount CustodyGroupCount = 4

// MetaDataV3 is the Fulu (EIP-7594) version of the eth2 metadata message.
// It extends MetaData with CustodyGroupCount for PeerDAS.
type MetaDataV3 struct {
	SeqNumber         common.SeqNr       `json:"seqNumber" yaml:"seqNumber"`
	Attnets           common.AttnetBits  `json:"attnets" yaml:"attnets"`
	Syncnets          common.SyncnetBits `json:"syncnets" yaml:"syncnets"`
	CustodyGroupCount CustodyGroupCount  `json:"custodyGroupCount" yaml:"custodyGroupCount"`
}

// MetaDataV3ByteLen is the fixed SSZ byte length of MetaDataV3 (8+8+1+8 = 25).
const MetaDataV3ByteLen = 8 + 8 + 1 + 8

// Deserialize implements codec.Deserializable.
func (m *MetaDataV3) Deserialize(dr *codec.DecodingReader) error {
	return dr.FixedLenContainer(&m.SeqNumber, &m.Attnets, &m.Syncnets, &m.CustodyGroupCount)
}

// Serialize implements codec.Serializable.
func (m *MetaDataV3) Serialize(w *codec.EncodingWriter) error {
	return w.FixedLenContainer(&m.SeqNumber, &m.Attnets, &m.Syncnets, &m.CustodyGroupCount)
}

// ByteLength returns the SSZ byte length of MetaDataV3.
func (m MetaDataV3) ByteLength() uint64 {
	return MetaDataV3ByteLen
}

// FixedLength returns the fixed SSZ byte length of MetaDataV3.
func (*MetaDataV3) FixedLength() uint64 {
	return MetaDataV3ByteLen
}

// HashTreeRoot computes the SSZ hash tree root of MetaDataV3.
func (m *MetaDataV3) HashTreeRoot(hFn tree.HashFn) common.Root {
	return hFn.HashTreeRoot(&m.SeqNumber, &m.Attnets, &m.Syncnets, &m.CustodyGroupCount)
}

// String returns a human-readable representation of MetaDataV3.
func (m *MetaDataV3) String() string {
	return fmt.Sprintf(
		"MetaDataV3(seq: %d, attnet bits: %08b, syncnet bits: %08b, custody_group_count: %d)",
		m.SeqNumber, m.Attnets, m.Syncnets, m.CustodyGroupCount,
	)
}

// ToV2 converts a MetaDataV3 to a common.MetaData by dropping CustodyGroupCount.
func (m *MetaDataV3) ToV2() *common.MetaData {
	return &common.MetaData{
		SeqNumber: m.SeqNumber,
		Attnets:   m.Attnets,
		Syncnets:  m.Syncnets,
	}
}

// MetaDataV3FromV2 converts a common.MetaData to a MetaDataV3,
// defaulting CustodyGroupCount to DefaultCustodyGroupCount.
func MetaDataV3FromV2(m *common.MetaData) *MetaDataV3 {
	return &MetaDataV3{
		SeqNumber:         m.SeqNumber,
		Attnets:           m.Attnets,
		Syncnets:          m.Syncnets,
		CustodyGroupCount: DefaultCustodyGroupCount,
	}
}
