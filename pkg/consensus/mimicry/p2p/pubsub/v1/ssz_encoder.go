package v1

import (
	"fmt"
	"reflect"

	fastssz "github.com/prysmaticlabs/fastssz"
)

// SSZEncoder implements Encoder for SSZ serialization without compression.
// Compression should be configured separately using WithCompressor option.
type SSZEncoder[T any] struct{}

// NewSSZEncoder creates a new SSZ encoder.
// For SSZ with Snappy compression, use this encoder with WithCompressor(compression.NewSnappyCompressor(maxLength)).
func NewSSZEncoder[T any]() *SSZEncoder[T] {
	return &SSZEncoder[T]{}
}

// Encode encodes the message using SSZ serialization.
func (e *SSZEncoder[T]) Encode(msg T) ([]byte, error) {
	marshaler, ok := any(msg).(fastssz.Marshaler)
	if !ok {
		return nil, fmt.Errorf("type %T does not implement fastssz.Marshaler", msg)
	}

	return marshaler.MarshalSSZ()
}

// Decode decodes the message from SSZ serialization.
func (e *SSZEncoder[T]) Decode(data []byte) (T, error) {
	var zero T

	// Use reflection to handle both pointer and non-pointer types
	msgType := reflect.TypeOf(zero)

	// Create a new instance
	var target reflect.Value
	if msgType.Kind() == reflect.Ptr {
		// T is already a pointer type (e.g., *MyStruct)
		target = reflect.New(msgType.Elem())
	} else {
		// T is a non-pointer type
		target = reflect.New(msgType)
	}

	// Check if the target implements Unmarshaler
	unmarshaler, ok := target.Interface().(fastssz.Unmarshaler)
	if !ok {
		return zero, fmt.Errorf("type %T does not implement fastssz.Unmarshaler", zero)
	}

	// Unmarshal the data
	if err := unmarshaler.UnmarshalSSZ(data); err != nil {
		return zero, fmt.Errorf("failed to unmarshal SSZ: %w", err)
	}

	// Return the appropriate value
	if msgType.Kind() == reflect.Ptr {
		val, convertOk := target.Interface().(T)
		if !convertOk {
			return zero, fmt.Errorf("failed to convert to target type")
		}

		return val, nil
	}

	val, convertOk := target.Elem().Interface().(T)
	if !convertOk {
		return zero, fmt.Errorf("failed to convert to target type")
	}

	return val, nil
}
