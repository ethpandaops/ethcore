package serialize

import (
	"testing"

	"github.com/attestantio/go-eth2-client/spec/deneb"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootAsString(t *testing.T) {
	tests := []struct {
		name string
		root phase0.Root
		want string
	}{
		{
			name: "Zero root",
			root: phase0.Root{},
			want: "0x0000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "Root with data",
			root: phase0.Root{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
			want: "0x0123456789abcdef000000000000000000000000000000000000000000000000",
		},
		{
			name: "Full root",
			root: phase0.Root{
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
			want: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RootAsString(tt.root)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSlotAsString(t *testing.T) {
	tests := []struct {
		name string
		slot phase0.Slot
		want string
	}{
		{
			name: "Zero slot",
			slot: phase0.Slot(0),
			want: "0",
		},
		{
			name: "Small slot",
			slot: phase0.Slot(123),
			want: "123",
		},
		{
			name: "Large slot",
			slot: phase0.Slot(1234567890),
			want: "1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SlotAsString(tt.slot)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEpochAsString(t *testing.T) {
	tests := []struct {
		name  string
		epoch phase0.Epoch
		want  string
	}{
		{
			name:  "Zero epoch",
			epoch: phase0.Epoch(0),
			want:  "0",
		},
		{
			name:  "Small epoch",
			epoch: phase0.Epoch(456),
			want:  "456",
		},
		{
			name:  "Large epoch",
			epoch: phase0.Epoch(9876543210),
			want:  "9876543210",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EpochAsString(tt.epoch)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBLSSignatureToString(t *testing.T) {
	tests := []struct {
		name string
		sig  *phase0.BLSSignature
		want string
	}{
		{
			name: "Zero signature",
			sig:  &phase0.BLSSignature{},
			want: "0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "Signature with data",
			sig: &phase0.BLSSignature{
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
			},
			want: "0x0123456789abcdef00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BLSSignatureToString(tt.sig)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKzgCommitmentToString(t *testing.T) {
	tests := []struct {
		name string
		comm deneb.KZGCommitment
		want string
	}{
		{
			name: "Zero commitment",
			comm: deneb.KZGCommitment{},
			want: "0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "Commitment with data",
			comm: deneb.KZGCommitment{
				0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
			},
			want: "0xaabbccddeeff000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KzgCommitmentToString(tt.comm)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersionedHashToString(t *testing.T) {
	tests := []struct {
		name string
		hash deneb.VersionedHash
		want string
	}{
		{
			name: "Zero hash",
			hash: deneb.VersionedHash{},
			want: "0x0000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "Hash with version byte",
			hash: deneb.VersionedHash{0x01},
			want: "0x0100000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "Full hash",
			hash: deneb.VersionedHash{
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
			},
			want: "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VersionedHashToString(tt.hash)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBytesToString(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
		want  string
	}{
		{
			name:  "Empty bytes",
			bytes: []byte{},
			want:  "",
		},
		{
			name:  "Single byte",
			bytes: []byte{0xff},
			want:  "0xff",
		},
		{
			name:  "Multiple bytes",
			bytes: []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
			want:  "0x0123456789abcdef",
		},
		{
			name:  "Nil bytes",
			bytes: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BytesToString(tt.bytes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStringToRoot(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    phase0.Root
		wantErr bool
		errMsg  string
	}{
		{
			name:  "Valid zero root",
			input: "0x0000000000000000000000000000000000000000000000000000000000000000",
			want:  phase0.Root{},
		},
		{
			name:  "Valid root with data",
			input: "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want: phase0.Root{
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
			},
		},
		{
			name:  "Valid uppercase hex",
			input: "0x0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF",
			want: phase0.Root{
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
			},
		},
		{
			name:    "Invalid length - too short",
			input:   "0x00",
			wantErr: true,
			errMsg:  "invalid root length",
		},
		{
			name:    "Invalid length - too long",
			input:   "0x00000000000000000000000000000000000000000000000000000000000000001",
			wantErr: true,
			errMsg:  "invalid root length",
		},
		{
			name:    "Missing 0x prefix",
			input:   "0000000000000000000000000000000000000000000000000000000000000000",
			wantErr: true,
			errMsg:  "invalid root length",
		},
		{
			name:    "Invalid hex characters",
			input:   "0x000000000000000000000000000000000000000000000000000000000000000g",
			wantErr: true,
			errMsg:  "invalid root",
		},
		{
			name:    "Empty string",
			input:   "",
			wantErr: true,
			errMsg:  "invalid root length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StringToRoot(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTrimmedString(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "Empty",
			s:    "",
			want: "",
		},
		{
			name: "Short",
			s:    "abc",
			want: "abc",
		},
		{
			name: "Long",
			s:    "abcdefg",
			want: "abcdefg",
		},
		{
			name: "Longer",
			s:    "abcdefghijk",
			want: "abcdefghijk",
		},
		{
			name: "Longer-trimmed",
			s:    "abcdefghijklmno",
			want: "abcde...klmno",
		},
		{
			name: "hex-trimmed",
			s:    "0xfd3963b996723a6055b3323014c4de94345a7b519b17758b386d6b57a1a16b6d",
			want: "0xfd3...16b6d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrimmedString(tt.s); got != tt.want {
				t.Errorf("TrimmedString() = %v, want %v", got, tt.want)
			}
		})
	}
}
