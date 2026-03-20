package eth

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/codec"
	"github.com/protolambda/ztyp/tree"
)

// PeerStatus associates a peer ID with its consensus status.
type PeerStatus struct {
	PeerID peer.ID
	Status *common.Status
}

// StatusV2 is the Fulu (EIP-7594) version of the eth2 status message.
// It extends Status with EarliestAvailableSlot for data availability advertisement.
type StatusV2 struct {
	ForkDigest            common.ForkDigest `json:"forkDigest" yaml:"forkDigest"`
	FinalizedRoot         common.Root       `json:"finalizedRoot" yaml:"finalizedRoot"`
	FinalizedEpoch        common.Epoch      `json:"finalizedEpoch" yaml:"finalizedEpoch"`
	HeadRoot              common.Root       `json:"headRoot" yaml:"headRoot"`
	HeadSlot              common.Slot       `json:"headSlot" yaml:"headSlot"`
	EarliestAvailableSlot common.Slot       `json:"earliestAvailableSlot" yaml:"earliestAvailableSlot"`
}

// StatusV2ByteLen is the fixed SSZ byte length of StatusV2 (4+32+8+32+8+8 = 92).
const StatusV2ByteLen = 4 + 32 + 8 + 32 + 8 + 8

// Deserialize implements codec.Deserializable.
func (s *StatusV2) Deserialize(dr *codec.DecodingReader) error {
	return dr.FixedLenContainer(
		&s.ForkDigest, &s.FinalizedRoot, &s.FinalizedEpoch,
		&s.HeadRoot, &s.HeadSlot, &s.EarliestAvailableSlot,
	)
}

// Serialize implements codec.Serializable.
func (s *StatusV2) Serialize(w *codec.EncodingWriter) error {
	return w.FixedLenContainer(
		&s.ForkDigest, &s.FinalizedRoot, &s.FinalizedEpoch,
		&s.HeadRoot, &s.HeadSlot, &s.EarliestAvailableSlot,
	)
}

// ByteLength returns the SSZ byte length of StatusV2.
func (s StatusV2) ByteLength() uint64 {
	return StatusV2ByteLen
}

// FixedLength returns the fixed SSZ byte length of StatusV2.
func (*StatusV2) FixedLength() uint64 {
	return StatusV2ByteLen
}

// HashTreeRoot computes the SSZ hash tree root of StatusV2.
func (s *StatusV2) HashTreeRoot(hFn tree.HashFn) common.Root {
	return hFn.HashTreeRoot(
		&s.ForkDigest, &s.FinalizedRoot, &s.FinalizedEpoch,
		&s.HeadRoot, &s.HeadSlot, &s.EarliestAvailableSlot,
	)
}

// String returns a human-readable representation of StatusV2.
func (s *StatusV2) String() string {
	return fmt.Sprintf(
		"StatusV2(fork_digest: %s, finalized_root: %s, finalized_epoch: %d, head_root: %s, head_slot: %d, earliest_available_slot: %d)",
		s.ForkDigest.String(), s.FinalizedRoot.String(), s.FinalizedEpoch, s.HeadRoot.String(), s.HeadSlot, s.EarliestAvailableSlot,
	)
}

// ToV1 converts a StatusV2 to a common.Status by dropping EarliestAvailableSlot.
func (s *StatusV2) ToV1() *common.Status {
	return &common.Status{
		ForkDigest:     s.ForkDigest,
		FinalizedRoot:  s.FinalizedRoot,
		FinalizedEpoch: s.FinalizedEpoch,
		HeadRoot:       s.HeadRoot,
		HeadSlot:       s.HeadSlot,
	}
}

// StatusV2FromV1 converts a common.Status to a StatusV2, defaulting EarliestAvailableSlot to 0.
func StatusV2FromV1(s *common.Status) *StatusV2 {
	return &StatusV2{
		ForkDigest:            s.ForkDigest,
		FinalizedRoot:         s.FinalizedRoot,
		FinalizedEpoch:        s.FinalizedEpoch,
		HeadRoot:              s.HeadRoot,
		HeadSlot:              s.HeadSlot,
		EarliestAvailableSlot: 0,
	}
}
