package v1_test

import (
	"encoding/binary"
	"testing"

	v1 "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub/v1"
	fastssz "github.com/prysmaticlabs/fastssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SSZTestMessage implements fastssz.Marshaler and fastssz.Unmarshaler for testing.
type SSZTestMessage struct {
	Value uint64
	Data  []byte
}

func (m *SSZTestMessage) MarshalSSZ() ([]byte, error) {
	buf := make([]byte, 8+len(m.Data))
	binary.LittleEndian.PutUint64(buf[0:8], m.Value)
	copy(buf[8:], m.Data)

	return buf, nil
}

func (m *SSZTestMessage) UnmarshalSSZ(data []byte) error {
	if len(data) < 8 {
		return fastssz.ErrSize
	}
	m.Value = binary.LittleEndian.Uint64(data[0:8])
	m.Data = make([]byte, len(data)-8)
	copy(m.Data, data[8:])

	return nil
}

func (m *SSZTestMessage) MarshalSSZTo(buf []byte) ([]byte, error) {
	return m.MarshalSSZ()
}

func (m *SSZTestMessage) SizeSSZ() int {
	return 8 + len(m.Data)
}

func TestSSZEncoder(t *testing.T) {
	t.Run("encode and decode", func(t *testing.T) {
		encoder := v1.NewSSZEncoder[*SSZTestMessage]()

		msg := &SSZTestMessage{
			Value: 42,
			Data:  []byte("test data"),
		}

		// Encode
		encoded, err := encoder.Encode(msg)
		require.NoError(t, err)
		assert.NotEmpty(t, encoded)

		// Decode
		decoded, err := encoder.Decode(encoded)
		require.NoError(t, err)
		assert.Equal(t, msg.Value, decoded.Value)
		assert.Equal(t, msg.Data, decoded.Data)
	})

	t.Run("encode non-marshaler type", func(t *testing.T) {
		type NonMarshalerType struct {
			Value string
		}

		encoder := v1.NewSSZEncoder[NonMarshalerType]()
		msg := NonMarshalerType{Value: "test"}

		_, err := encoder.Encode(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not implement fastssz.Marshaler")
	})

	t.Run("decode non-unmarshaler type", func(t *testing.T) {
		type NonUnmarshalerType struct {
			Value string
		}

		encoder := v1.NewSSZEncoder[NonUnmarshalerType]()

		_, err := encoder.Decode([]byte{1, 2, 3})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not implement fastssz.Unmarshaler")
	})

	t.Run("decode invalid data", func(t *testing.T) {
		encoder := v1.NewSSZEncoder[*SSZTestMessage]()

		// Too short data
		_, err := encoder.Decode([]byte{1, 2, 3})
		assert.Error(t, err)
	})

	t.Run("empty message", func(t *testing.T) {
		encoder := v1.NewSSZEncoder[*SSZTestMessage]()

		msg := &SSZTestMessage{
			Value: 0,
			Data:  []byte{},
		}

		// Encode
		encoded, err := encoder.Encode(msg)
		require.NoError(t, err)
		assert.Len(t, encoded, 8) // Just the uint64

		// Decode
		decoded, err := encoder.Decode(encoded)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), decoded.Value)
		assert.Empty(t, decoded.Data)
	})
}
