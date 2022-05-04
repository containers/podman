//go:build jsoniter
// +build jsoniter

package json

import (
	"bytes"
	"encoding/json"
	"io"

	jsoniter "github.com/json-iterator/go"
)

var compat jsoniter.API

func init() {
	compat = jsoniter.ConfigCompatibleWithStandardLibrary
}

type Decoder = jsoniter.Decoder
type Encoder = jsoniter.Encoder

func Compact(dst *bytes.Buffer, src []byte) error {
	// TODO Implement Compact using jsoniter and buffer
	return json.Compact(dst, src)
}

func HTMLEscape(dst *bytes.Buffer, src []byte) {
	// TODO Implement HTMLEscape using jsoniter and buffer
	json.HTMLEscape(dst, src)
}

func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	// TODO Implement Indent using jsoniter and buffer
	return json.Indent(dst, src, prefix, indent)
}

func Marshal(v interface{}) ([]byte, error) {
	return compat.Marshal(v)
}

func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return compat.MarshalIndent(v, prefix, indent)
}

func Unmarshal(data []byte, v interface{}) error {
	return compat.Unmarshal(data, v)
}

func Valid(data []byte) bool {
	return compat.Valid(data)
}

func NewDecoder(r io.Reader) *Decoder {
	return compat.NewDecoder(r)
}

func NewEncoder(w io.Writer) *Encoder {
	return compat.NewEncoder(w)
}
