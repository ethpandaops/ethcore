package p2p_test

import (
	"bytes"
	"testing"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/codec"
	"github.com/protolambda/ztyp/tree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSpecObj is a simple mock implementation of common.SpecObj for testing.
type mockSpecObj struct {
	value uint64
}

func (m *mockSpecObj) Serialize(spec *common.Spec, w *codec.EncodingWriter) error {
	return w.WriteUint64(m.value)
}

func (m *mockSpecObj) Deserialize(spec *common.Spec, r *codec.DecodingReader) error {
	val, err := r.ReadUint64()
	if err != nil {
		return err
	}

	m.value = val

	return nil
}

func (m *mockSpecObj) ByteLength(spec *common.Spec) uint64 {
	return 8 // uint64 is 8 bytes
}

func (m *mockSpecObj) FixedLength(spec *common.Spec) uint64 {
	return 8
}

func (m *mockSpecObj) HashTreeRoot(spec *common.Spec, hFn tree.HashFn) common.Root {
	return common.Root{}
}

// mockSSZObj is a simple mock implementation of common.SSZObj for testing.
type mockSSZObj struct {
	value uint32
}

func (m *mockSSZObj) Serialize(w *codec.EncodingWriter) error {
	return w.WriteUint32(m.value)
}

func (m *mockSSZObj) Deserialize(r *codec.DecodingReader) error {
	val, err := r.ReadUint32()
	if err != nil {
		return err
	}

	m.value = val

	return nil
}

func (m *mockSSZObj) ByteLength() uint64 {
	return 4 // uint32 is 4 bytes
}

func (m *mockSSZObj) FixedLength() uint64 {
	return 4
}

func (m *mockSSZObj) HashTreeRoot(hFn tree.HashFn) common.Root {
	return common.Root{}
}

func TestWrappedSpecObjectEncoder_MarshalSSZ(t *testing.T) {
	spec := &common.Spec{}

	tests := []struct {
		name    string
		obj     common.SpecObj
		want    []byte
		wantErr bool
	}{
		{
			name: "marshal mock object",
			obj:  &mockSpecObj{value: 42},
			want: []byte{42, 0, 0, 0, 0, 0, 0, 0}, // little-endian uint64
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := p2p.WrapSpecObject(spec, tt.obj)
			got, err := w.MarshalSSZ()
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWrappedSpecObjectEncoder_MarshalSSZTo(t *testing.T) {
	spec := &common.Spec{}

	obj := &mockSpecObj{value: 42}
	w := p2p.WrapSpecObject(spec, obj)

	dst := []byte{1, 2, 3}
	expected := append([]byte{1, 2, 3}, []byte{42, 0, 0, 0, 0, 0, 0, 0}...)

	got, err := w.MarshalSSZTo(dst)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestWrappedSpecObjectEncoder_UnmarshalSSZ(t *testing.T) {
	spec := &common.Spec{}

	tests := []struct {
		name    string
		data    []byte
		want    uint64
		wantErr bool
	}{
		{
			name: "unmarshal valid data",
			data: []byte{42, 0, 0, 0, 0, 0, 0, 0},
			want: 42,
		},
		{
			name:    "unmarshal invalid data (too short)",
			data:    []byte{42, 0, 0, 0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &mockSpecObj{}
			w := p2p.WrapSpecObject(spec, obj)

			err := w.UnmarshalSSZ(tt.data)
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, obj.value)
		})
	}
}

func TestWrappedSpecObjectEncoder_SizeSSZ(t *testing.T) {
	spec := &common.Spec{}

	obj := &mockSpecObj{value: 42}
	w := p2p.WrapSpecObject(spec, obj)

	size := w.SizeSSZ()
	assert.Equal(t, 8, size)
}

func TestWrappedSSZObjectEncoder_MarshalSSZ(t *testing.T) {
	tests := []struct {
		name    string
		obj     common.SSZObj
		want    []byte
		wantErr bool
	}{
		{
			name: "marshal mock object",
			obj:  &mockSSZObj{value: 42},
			want: []byte{42, 0, 0, 0}, // little-endian uint32
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := p2p.WrapSSZObject(tt.obj)
			got, err := w.MarshalSSZ()
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWrappedSSZObjectEncoder_MarshalSSZTo(t *testing.T) {
	obj := &mockSSZObj{value: 42}
	w := p2p.WrapSSZObject(obj)

	dst := []byte{1, 2, 3}
	expected := append([]byte{1, 2, 3}, []byte{42, 0, 0, 0}...)

	got, err := w.MarshalSSZTo(dst)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestWrappedSSZObjectEncoder_UnmarshalSSZ(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    uint32
		wantErr bool
	}{
		{
			name: "unmarshal valid data",
			data: []byte{42, 0, 0, 0},
			want: 42,
		},
		{
			name:    "unmarshal invalid data (too short)",
			data:    []byte{42, 0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &mockSSZObj{}
			w := p2p.WrapSSZObject(obj)

			err := w.UnmarshalSSZ(tt.data)
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, obj.value)
		})
	}
}

func TestWrappedSSZObjectEncoder_SizeSSZ(t *testing.T) {
	obj := &mockSSZObj{value: 42}
	w := p2p.WrapSSZObject(obj)

	size := w.SizeSSZ()
	assert.Equal(t, 4, size)
}

func TestWrappedSSZObjectEncoder_RealTypes(t *testing.T) {
	// Test with a real Status message
	status := &common.Status{
		ForkDigest:     common.ForkDigest{1, 2, 3, 4},
		FinalizedRoot:  common.Root{5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		FinalizedEpoch: common.Epoch(100),
		HeadRoot:       common.Root{32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
		HeadSlot:       common.Slot(3200),
	}

	w := p2p.WrapSSZObject(status)

	// Test marshal
	data, err := w.MarshalSSZ()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test size
	size := w.SizeSSZ()
	assert.Equal(t, len(data), size)

	// Test unmarshal
	newStatus := &common.Status{}
	w2 := p2p.WrapSSZObject(newStatus)
	err = w2.UnmarshalSSZ(data)
	require.NoError(t, err)
	assert.Equal(t, status, newStatus)
}

// Test round-trip encoding/decoding.
func TestEncodingRoundTrip(t *testing.T) {
	t.Run("SpecObject round trip", func(t *testing.T) {
		spec := &common.Spec{}

		original := &mockSpecObj{value: 12345}
		w := p2p.WrapSpecObject(spec, original)

		// Marshal
		data, err := w.MarshalSSZ()
		require.NoError(t, err)

		// Unmarshal
		decoded := &mockSpecObj{}
		w2 := p2p.WrapSpecObject(spec, decoded)
		err = w2.UnmarshalSSZ(data)
		require.NoError(t, err)

		assert.Equal(t, original.value, decoded.value)
	})

	t.Run("SSZObject round trip", func(t *testing.T) {
		original := &mockSSZObj{value: 54321}
		w := p2p.WrapSSZObject(original)

		// Marshal
		data, err := w.MarshalSSZ()
		require.NoError(t, err)

		// Unmarshal
		decoded := &mockSSZObj{}
		w2 := p2p.WrapSSZObject(decoded)
		err = w2.UnmarshalSSZ(data)
		require.NoError(t, err)

		assert.Equal(t, original.value, decoded.value)
	})
}

// errorSpecObj is a mock that returns errors.
type errorSpecObj struct{}

func (e *errorSpecObj) Serialize(spec *common.Spec, w *codec.EncodingWriter) error {
	return bytes.ErrTooLarge
}

func (e *errorSpecObj) Deserialize(spec *common.Spec, r *codec.DecodingReader) error {
	return bytes.ErrTooLarge
}

func (e *errorSpecObj) ByteLength(spec *common.Spec) uint64 {
	return 0
}

func (e *errorSpecObj) FixedLength(spec *common.Spec) uint64 {
	return 0
}

func (e *errorSpecObj) HashTreeRoot(spec *common.Spec, hFn tree.HashFn) common.Root {
	return common.Root{}
}

// errorSSZObj is a mock that returns errors.
type errorSSZObj struct{}

func (e *errorSSZObj) Serialize(w *codec.EncodingWriter) error {
	return bytes.ErrTooLarge
}

func (e *errorSSZObj) Deserialize(r *codec.DecodingReader) error {
	return bytes.ErrTooLarge
}

func (e *errorSSZObj) ByteLength() uint64 {
	return 0
}

func (e *errorSSZObj) FixedLength() uint64 {
	return 0
}

func (e *errorSSZObj) HashTreeRoot(hFn tree.HashFn) common.Root {
	return common.Root{}
}

func TestWrappedSpecObjectEncoder_Errors(t *testing.T) {
	spec := &common.Spec{}

	obj := &errorSpecObj{}
	w := p2p.WrapSpecObject(spec, obj)

	t.Run("MarshalSSZ error", func(t *testing.T) {
		_, err := w.MarshalSSZ()
		assert.Error(t, err)
	})

	t.Run("MarshalSSZTo error", func(t *testing.T) {
		dst := []byte{1, 2, 3}
		_, err := w.MarshalSSZTo(dst)
		assert.Error(t, err)
	})

	t.Run("UnmarshalSSZ error", func(t *testing.T) {
		err := w.UnmarshalSSZ([]byte{1, 2, 3, 4})
		assert.Error(t, err)
	})
}

func TestWrappedSSZObjectEncoder_Errors(t *testing.T) {
	obj := &errorSSZObj{}
	w := p2p.WrapSSZObject(obj)

	t.Run("MarshalSSZ error", func(t *testing.T) {
		_, err := w.MarshalSSZ()
		assert.Error(t, err)
	})

	t.Run("MarshalSSZTo error", func(t *testing.T) {
		dst := []byte{1, 2, 3}
		_, err := w.MarshalSSZTo(dst)
		assert.Error(t, err)
	})

	t.Run("UnmarshalSSZ error", func(t *testing.T) {
		err := w.UnmarshalSSZ([]byte{1, 2, 3, 4})
		assert.Error(t, err)
	})
}

// BenchmarkWrappedSpecObjectEncoder benchmarks SpecObject encoding.
func BenchmarkWrappedSpecObjectEncoder_MarshalSSZ(b *testing.B) {
	spec := &common.Spec{}
	obj := &mockSpecObj{value: 42}
	w := p2p.WrapSpecObject(spec, obj)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := w.MarshalSSZ()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWrappedSSZObjectEncoder benchmarks SSZObject encoding.
func BenchmarkWrappedSSZObjectEncoder_MarshalSSZ(b *testing.B) {
	obj := &mockSSZObj{value: 42}
	w := p2p.WrapSSZObject(obj)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := w.MarshalSSZ()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Test concurrent access.
func TestWrappedEncoders_Concurrent(t *testing.T) {
	t.Run("SpecObject concurrent", func(t *testing.T) {
		spec := &common.Spec{}
		obj := &mockSpecObj{value: 42}
		w := p2p.WrapSpecObject(spec, obj)

		// Run multiple goroutines marshaling/unmarshaling
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				data, err := w.MarshalSSZ()
				require.NoError(t, err)
				assert.NotEmpty(t, data)

				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("SSZObject concurrent", func(t *testing.T) {
		obj := &mockSSZObj{value: 42}
		w := p2p.WrapSSZObject(obj)

		// Run multiple goroutines marshaling/unmarshaling
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				data, err := w.MarshalSSZ()
				require.NoError(t, err)
				assert.NotEmpty(t, data)

				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// Test edge cases.
func TestWrappedEncoders_EdgeCases(t *testing.T) {
	t.Run("MarshalSSZTo with nil destination", func(t *testing.T) {
		obj := &mockSSZObj{value: 42}
		w := p2p.WrapSSZObject(obj)

		got, err := w.MarshalSSZTo(nil)
		require.NoError(t, err)
		assert.Equal(t, []byte{42, 0, 0, 0}, got)
	})

	t.Run("UnmarshalSSZ with empty data", func(t *testing.T) {
		obj := &mockSSZObj{}
		w := p2p.WrapSSZObject(obj)

		err := w.UnmarshalSSZ([]byte{})
		assert.Error(t, err)
	})

	t.Run("UnmarshalSSZ with nil data", func(t *testing.T) {
		obj := &mockSSZObj{}
		w := p2p.WrapSSZObject(obj)

		err := w.UnmarshalSSZ(nil)
		assert.Error(t, err)
	})
}
