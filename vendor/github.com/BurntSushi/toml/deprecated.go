package toml

import (
	"encoding"
	"io"
)

// TextMarshaler is an alias for encoding.TextMarshaler.
//
// Deprecated: use encoding.TextMarshaler
type TextMarshaler encoding.TextMarshaler

// TextUnmarshaler is an alias for encoding.TextUnmarshaler.
//
// Deprecated: use encoding.TextUnmarshaler
type TextUnmarshaler encoding.TextUnmarshaler

// PrimitiveDecode is an alias for MetaData.PrimitiveDecode().
//
// Deprecated: use MetaData.PrimitiveDecode.
func PrimitiveDecode(primValue Primitive, v interface{}) error {
	md := MetaData{decoded: make(map[string]struct{})}
	return md.unify(primValue.undecoded, rvalue(v))
}

// DecodeReader is an alias for NewDecoder(r).Decode(v).
//
// Deprecated: use NewDecoder(reader).Decode(&value).
func DecodeReader(r io.Reader, v interface{}) (MetaData, error) { return NewDecoder(r).Decode(v) }
