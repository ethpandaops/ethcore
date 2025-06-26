package p2p

// Thanks to Mario: https://github.com/marioevz/blobber/blob/main/p2p/encoder.go

import (
	"bytes"

	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/codec"
	fastssz "github.com/prysmaticlabs/fastssz"
)

// Marshaler combines fastssz marshaling and unmarshaling capabilities.
type Marshaler interface {
	fastssz.Marshaler
	fastssz.Unmarshaler
}

type wrappedSpecObjectEncoder struct {
	common.SpecObj
	*common.Spec
}

// WrapSpecObject wraps a SpecObj with SSZ encoding capabilities.
func WrapSpecObject(spec *common.Spec, specObj common.SpecObj) Marshaler {
	return &wrappedSpecObjectEncoder{
		SpecObj: specObj,
		Spec:    spec,
	}
}

// MarshalSSZTo marshals the object and appends to the destination slice.
func (w *wrappedSpecObjectEncoder) MarshalSSZTo(dst []byte) ([]byte, error) {
	marshalledObj, err := w.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	return append(dst, marshalledObj...), nil
}

// MarshalSSZ returns the SSZ encoded bytes of the object.
func (w *wrappedSpecObjectEncoder) MarshalSSZ() ([]byte, error) {
	var buf bytes.Buffer
	if err := w.Serialize(w.Spec, codec.NewEncodingWriter(&buf)); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// SizeSSZ returns the size of the object when SSZ encoded.
func (w *wrappedSpecObjectEncoder) SizeSSZ() int {
	return int(w.SpecObj.ByteLength(w.Spec)) //nolint:gosec,staticcheck // not concerned about overflow here.
}

// UnmarshalSSZ unmarshals the object from SSZ encoded bytes.
func (w *wrappedSpecObjectEncoder) UnmarshalSSZ(b []byte) error {
	return w.Deserialize(w.Spec, codec.NewDecodingReader(bytes.NewReader(b), uint64(len(b))))
}

type wrappedSSZObjectEncoder struct {
	common.SSZObj
}

// WrapSSZObject wraps an SSZObj with fastssz marshaling interface.
func WrapSSZObject(sszObj common.SSZObj) Marshaler {
	return &wrappedSSZObjectEncoder{
		SSZObj: sszObj,
	}
}

// MarshalSSZTo marshals the object and appends to the destination slice.
func (w *wrappedSSZObjectEncoder) MarshalSSZTo(dst []byte) ([]byte, error) {
	marshalledObj, err := w.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	return append(dst, marshalledObj...), nil
}

// MarshalSSZ returns the SSZ encoded bytes of the object.
func (w *wrappedSSZObjectEncoder) MarshalSSZ() ([]byte, error) {
	var buf bytes.Buffer
	if err := w.Serialize(codec.NewEncodingWriter(&buf)); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// SizeSSZ returns the size of the object when SSZ encoded.
func (w *wrappedSSZObjectEncoder) SizeSSZ() int {
	return int(w.SSZObj.ByteLength()) //nolint:gosec,staticcheck // not concerned about overflow here.
}

// UnmarshalSSZ unmarshals the object from SSZ encoded bytes.
func (w *wrappedSSZObjectEncoder) UnmarshalSSZ(b []byte) error {
	return w.Deserialize(codec.NewDecodingReader(bytes.NewReader(b), uint64(len(b))))
}
