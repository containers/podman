// +build !jsoniter
//go:build !jsoniter

package json

import (
	"bytes"
	"encoding/json"
	"io"
)

type Decoder = json.Decoder
type Encoder = json.Encoder

func Compact(dst *bytes.Buffer, src []byte) error {
	return json.Compact(dst, src)
}

func HTMLEscape(dst *bytes.Buffer, src []byte) {
	json.HTMLEscape(dst, src)
}

func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return json.Indent(dst, src, prefix, indent)
}

func Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

func Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func Valid(data []byte) bool {
	return json.Valid(data)
}

func NewDecoder(r io.Reader) *Decoder {
	return json.NewDecoder(r)
}

func NewEncoder(w io.Writer) *Encoder {
	return json.NewEncoder(w)
}
