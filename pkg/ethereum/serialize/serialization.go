package serialize

import (
	"encoding/hex"
	"fmt"

	"github.com/attestantio/go-eth2-client/spec/deneb"
	"github.com/attestantio/go-eth2-client/spec/phase0"
)

// RootAsString converts a phase0.Root to a string.
func RootAsString(root phase0.Root) string {
	return fmt.Sprintf("%#x", root)
}

// SlotAsString converts a phase0.Slot to a string.
func SlotAsString(slot phase0.Slot) string {
	return fmt.Sprintf("%d", slot)
}

// EpochAsString converts a phase0.Epoch to a string.
func EpochAsString(epoch phase0.Epoch) string {
	return fmt.Sprintf("%d", epoch)
}

// BLSSignatureToString converts a phase0.BLSSignature to a string.
func BLSSignatureToString(s *phase0.BLSSignature) string {
	return fmt.Sprintf("%#x", s)
}

// KzgCommitmentToString converts a deneb.KZGCommitment to a string.
func KzgCommitmentToString(c deneb.KZGCommitment) string {
	return fmt.Sprintf("%#x", c)
}

// VersionedHashToString converts a deneb.VersionedHash to a string.
func VersionedHashToString(h deneb.VersionedHash) string {
	return fmt.Sprintf("%#x", h)
}

// BytesToString converts a []byte to a string.
func BytesToString(b []byte) string {
	return fmt.Sprintf("%#x", b)
}

// StringToRoot converts a string to a phase0.Root.
func StringToRoot(s string) (phase0.Root, error) {
	var root phase0.Root
	if len(s) != 66 {
		return root, fmt.Errorf("invalid root length")
	}

	if s[:2] != "0x" {
		return root, fmt.Errorf("invalid root prefix")
	}

	_, err := hex.Decode(root[:], []byte(s[2:]))
	if err != nil {
		return root, fmt.Errorf("invalid root: %v", err)
	}

	return root, nil
}

// TrimmedString trims a string to 12 characters.
func TrimmedString(s string) string {
	if len(s) <= 12 {
		return s
	}

	return s[:5] + "..." + s[len(s)-5:]
}
